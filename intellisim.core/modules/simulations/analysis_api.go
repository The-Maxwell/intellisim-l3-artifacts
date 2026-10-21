package simulations

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

// Analyze starts a Level-3 results analysis. POST /:id/analyses → 202 AnalysisResponse.
func (a *API) Analyze(c *echo.Context) error {
	var req AnalyzeRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.analysis.Create(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

// ListAnalyses lists a simulation's analyses. GET /:id/analyses → 200.
func (a *API) ListAnalyses(c *echo.Context) error {
	resp, err := a.analysis.List(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetAnalysis returns one analysis. GET /:id/analyses/:aid → 200.
func (a *API) GetAnalysis(c *echo.Context) error {
	resp, err := a.analysis.Get(c.Request().Context(), c.Param("id"), c.Param("aid"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// DeleteAnalysis deletes an analysis + its file. DELETE /:id/analyses/:aid → 204.
func (a *API) DeleteAnalysis(c *echo.Context) error {
	if err := a.analysis.Delete(c.Request().Context(), c.Param("id"), c.Param("aid")); err != nil {
		return a.mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// DownloadAnalysisFile streams the PDF. GET /:id/analyses/:aid/file?download=0|1.
func (a *API) DownloadAnalysisFile(c *echo.Context) error {
	rc, info, name, err := a.analysis.OpenFile(c.Request().Context(), c.Param("id"), c.Param("aid"))
	if err != nil {
		return a.mapError(err)
	}
	defer rc.Close()

	resp := c.Response()
	resp.Header().Set("Content-Type", "application/pdf")
	disposition := "inline"
	if c.QueryParam("download") == "1" {
		disposition = "attachment"
	}
	// Provide an ASCII fallback for legacy clients and an RFC 5987 form for
	// Cyrillic (non-ASCII) filenames so they render correctly and stay header-safe.
	resp.Header().Set("Content-Disposition",
		disposition+`; filename="`+asciiFallback(name)+`"; filename*=UTF-8''`+url.PathEscape(name))
	if info != nil && info.Length > 0 {
		resp.Header().Set("Content-Length", strconv.FormatInt(info.Length, 10))
	}
	resp.WriteHeader(http.StatusOK)
	if _, err := io.Copy(resp, rc); err != nil {
		a.logger.Debug("analysis file stream interrupted", zap.Error(err))
	}
	return nil
}

// ResultsSummary returns selectable dimensions + headline stats. GET /:id/results/summary.
func (a *API) ResultsSummary(c *echo.Context) error {
	resp, err := a.analysis.ResultsSummary(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ResultsSeries returns an aggregated property/population series. GET /:id/results/series.
func (a *API) ResultsSeries(c *echo.Context) error {
	// axios serializes array params as "agents[]"; accept both forms.
	agents := c.QueryParams()["agents"]
	if len(agents) == 0 {
		agents = c.QueryParams()["agents[]"]
	}
	q := ResultsSeriesQuery{
		PropertyUID: c.QueryParam("property"),
		Aggregation: c.QueryParam("aggregation"),
		AgentUIDs:   agents,
		EntityType:  c.QueryParam("entity_type"),
	}
	if v := c.QueryParam("from"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.From = &n
		}
	}
	if v := c.QueryParam("to"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.To = &n
		}
	}
	resp, err := a.analysis.ResultsSeries(c.Request().Context(), c.Param("id"), q)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// asciiFallback builds an ASCII-only Content-Disposition filename: it strips the
// control/quote characters that would break the header (matching the old
// headerSafe), then replaces every remaining non-ASCII rune with '_'.
func asciiFallback(s string) string {
	stripped := strings.NewReplacer("\"", "", "\n", "", "\r", "").Replace(s)
	var b strings.Builder
	for _, r := range stripped {
		if r > 127 {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
