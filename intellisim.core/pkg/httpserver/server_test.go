package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/pkg/httpapi"
)

// newTestApp wires a bare echo instance with the same error handler used by
// NewHttpServer, plus one registered GET route, so router-level 404/405
// behavior can be exercised without booting the full fx application.
func newTestApp() *echo.Echo {
	app := echo.New()
	app.HTTPErrorHandler = newHTTPErrorHandler(zap.NewNop())
	app.GET("/known", func(c *echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
	return app
}

func decodeAPIError(t *testing.T, body []byte) httpapi.APIError {
	t.Helper()
	var apiErr httpapi.APIError
	require.NoError(t, json.Unmarshal(body, &apiErr))
	return apiErr
}

func TestErrorHandler_UnknownRoute_Returns404NotFound(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	apiErr := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "not_found", apiErr.Code)
}

func TestErrorHandler_WrongMethod_Returns405MethodNotAllowed(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest(http.MethodPost, "/known", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	apiErr := decodeAPIError(t, rec.Body.Bytes())
	require.Equal(t, "method_not_allowed", apiErr.Code)
}

func TestCodeForStatus_MethodNotAllowed(t *testing.T) {
	require.Equal(t, "method_not_allowed", codeForStatus(http.StatusMethodNotAllowed))
}
