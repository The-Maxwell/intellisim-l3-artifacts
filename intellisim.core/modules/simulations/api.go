package simulations

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/modules/auth"
	"intelli.sim.core/pkg/httpapi"
)

const sseHeartbeatInterval = 25 * time.Second

// API is the simulations HTTP handler set. It mirrors the models module's
// ModelAPI: thin handlers that bind+validate, call the service, and map
// service-layer sentinels onto the shared APIError envelope.
type API struct {
	logger   *zap.Logger
	svc      Service
	hub      *EventsHub
	runSvc   RunService
	stream   *StreamHub
	cfg      RunConfig
	analysis AnalysisService
}

func NewAPI(logger *zap.Logger, svc Service, hub *EventsHub, runSvc RunService, stream *StreamHub, cfg RunConfig, analysis AnalysisService) *API {
	return &API{
		logger:   logger.Named("simulation-api"),
		svc:      svc,
		hub:      hub,
		runSvc:   runSvc,
		stream:   stream,
		cfg:      cfg,
		analysis: analysis,
	}
}

func (a *API) SetupRoutes(group *echo.Group) {
	group.POST("", a.Create)
	group.GET("", a.List)
	group.GET("/:id", a.Get)
	group.PUT("/:id", a.Update)
	group.POST("/:id/generate", a.Generate)
	group.POST("/:id/duplicate", a.Duplicate)
	group.DELETE("/:id", a.Delete)
	group.GET("/:id/events", a.StreamEvents)

	// Run-phase routes. The WebSocket route authenticates via a single-use connect
	// ticket (see StreamSimulation) — the header bearer-auth middleware is skipped
	// for it in api/api.go so a browser handshake (which cannot set Authorization)
	// can still reach it; every other route here stays bearer-authenticated.
	group.POST("/:id/run", a.RunSimulation)
	group.POST("/:id/control", a.ControlSimulation)
	group.POST("/:id/stream-ticket", a.IssueStreamTicket)
	group.GET("/:id/steps", a.ListSteps)
	group.GET("/:id/steps/export", a.ExportSteps)
	group.GET("/:id/model-snapshot", a.GetModelSnapshot)
	group.GET("/:id/agents/:agentId", a.GetAgent)
	group.GET("/:id/ws", a.StreamSimulation)

	// Level-3 analysis routes (bearer-authenticated; owner/admin gated in the service).
	group.POST("/:id/analyses", a.Analyze)
	group.GET("/:id/analyses", a.ListAnalyses)
	group.GET("/:id/analyses/:aid", a.GetAnalysis)
	group.DELETE("/:id/analyses/:aid", a.DeleteAnalysis)
	group.GET("/:id/analyses/:aid/file", a.DownloadAnalysisFile)
	// Results-query routes (power the analysis selection UI).
	group.GET("/:id/results/summary", a.ResultsSummary)
	group.GET("/:id/results/series", a.ResultsSeries)
}

// mapError translates service sentinels into the shared APIError envelope. A
// *httpapi.APIError (e.g. a bind/validation failure) passes through unchanged;
// a *ValidationError becomes a 422 with its issues in the fields map.
func (a *API) mapError(err error) error {
	if apiErr, ok := errors.AsType[*httpapi.APIError](err); ok {
		return apiErr
	}
	if validationErr, ok := errors.AsType[*ValidationError](err); ok {
		fields := make(map[string]string, len(validationErr.Result.Errors))
		for _, iss := range validationErr.Result.Errors {
			fields[iss.Path] = iss.Reason
		}
		return &httpapi.APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "validation_failed",
			Message: "simulation failed validation",
			Fields:  fields,
		}
	}
	switch {
	case errors.Is(err, ErrSimulationNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "not_found", "simulation not found")
	case errors.Is(err, ErrAgentNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "agent_not_found", "agent not found in simulation")
	case errors.Is(err, ErrModelSnapshotNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "model_snapshot_unavailable",
			"the simulation has no model snapshot")
	case errors.Is(err, ErrModelNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "model_not_found", "model not found")
	case errors.Is(err, ErrNotEditable):
		return httpapi.NewAPIError(http.StatusConflict, "not_editable",
			"the simulation is not in an editable state")
	case errors.Is(err, ErrGenerationInFlight):
		return httpapi.NewAPIError(http.StatusConflict, "generation_in_progress",
			"a generation is already in progress for this simulation")
	case errors.Is(err, ErrModelNotReady):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "model_not_ready",
			"the model is not ready for simulation generation")
	case errors.Is(err, ErrInvalidLLMTarget):
		// Shared by run (simulation_execution) and analysis (results_analysis)
		// endpoints — keep the message capability-neutral.
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "invalid_llm_target",
			"the selected LLM target does not support the required capability")
	case errors.Is(err, ErrRunInFlight):
		return httpapi.NewAPIError(http.StatusConflict, "run_in_flight",
			"a run is already in flight for this simulation")
	case errors.Is(err, ErrNotRunnable):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "not_runnable",
			"the simulation is not runnable")
	case errors.Is(err, ErrRunCapacity):
		return httpapi.NewAPIError(http.StatusTooManyRequests, "run_capacity",
			"the engine is at its concurrent run capacity")
	case errors.Is(err, ErrInvalidTicket):
		return httpapi.NewAPIError(http.StatusForbidden, "invalid_ticket",
			"the stream ticket is invalid or expired")
	case errors.Is(err, ErrNotRunning):
		return httpapi.NewAPIError(http.StatusConflict, "not_running",
			"the simulation is not running")
	case errors.Is(err, ErrNotStreamable):
		return httpapi.NewAPIError(http.StatusConflict, "not_streamable",
			"the simulation has no live run to stream")
	case errors.Is(err, ErrAnalysisNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "analysis_not_found", "analysis not found")
	case errors.Is(err, ErrNotAnalyzable):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "not_analyzable",
			"the simulation must be finished or paused to be analyzed")
	case errors.Is(err, ErrNoResults):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "no_results",
			"the simulation has no recorded results to analyze")
	case errors.Is(err, ErrAnalysisNotReady):
		return httpapi.NewAPIError(http.StatusConflict, "analysis_not_ready",
			"the analysis is still in progress")
	case errors.Is(err, auth.ErrNoPrincipal):
		return httpapi.NewAPIError(http.StatusUnauthorized, "unauthorized", "no principal")
	case errors.Is(err, auth.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, "forbidden",
			"you do not have access to this simulation")
	default:
		a.logger.Error("simulation request failed", zap.Error(err))
		return httpapi.NewAPIError(http.StatusInternalServerError, "internal", "internal server error")
	}
}

func bindAndValidate(c *echo.Context, target any) error {
	if err := c.Bind(target); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	return c.Validate(target) // returns *APIError (validation_failed) on failure
}

func (a *API) Create(c *echo.Context) error {
	var req CreateSimulationRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.svc.Create(c.Request().Context(), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

func (a *API) List(c *echo.Context) error {
	page := atoiDefault(c.QueryParam("page"), 1)
	pageSize := atoiDefault(c.QueryParam("page_size"), defaultPageSize)
	// model_id is repeatable (?model_id=a&model_id=b) to filter by several models;
	// empties are dropped. Sort fields/order are whitelisted in the repository, so
	// unrecognized tokens fall back to the default (updated_at desc) harmlessly.
	modelIDs := make([]string, 0)
	for _, id := range c.QueryParams()["model_id"] {
		if id != "" {
			modelIDs = append(modelIDs, id)
		}
	}
	sortBy := c.QueryParam("sort_by")
	sortOrder := c.QueryParam("sort_order")
	resp, err := a.svc.List(c.Request().Context(), page, pageSize, modelIDs, sortBy, sortOrder)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *API) Get(c *echo.Context) error {
	resp, err := a.svc.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetModelSnapshot returns the full frozen source-model definition the
// simulation was generated from (same JSON shape as GET /models/:id).
func (a *API) GetModelSnapshot(c *echo.Context) error {
	resp, err := a.svc.GetModelSnapshot(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *API) Update(c *echo.Context) error {
	var req UpdateSimulationRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.svc.Update(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *API) Generate(c *echo.Context) error {
	var req GenerateRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.svc.Generate(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

func (a *API) Duplicate(c *echo.Context) error {
	var req DuplicateRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.svc.Duplicate(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (a *API) Delete(c *echo.Context) error {
	if err := a.svc.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return a.mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// StreamEvents is the per-simulation SSE stream (auth + ownership enforced).
// Protocol: named events (generation.progress / generation.completed /
// generation.failed / simulation.updated / simulation.deleted) with JSON data;
// ": ping" heartbeat every 25s. No replay — on reconnect clients refetch the
// simulation.
func (a *API) StreamEvents(c *echo.Context) error {
	simulationID := c.Param("id")
	// Ownership gate (also yields 404 for missing/foreign simulations).
	if _, err := a.svc.Get(c.Request().Context(), simulationID); err != nil {
		return a.mapError(err)
	}

	sub, cancel := a.hub.Subscribe(simulationID)
	defer cancel()

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")
	resp.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(resp)
	if err := rc.Flush(); err != nil {
		return nil // client cannot stream; bail quietly
	}

	writeEvent := func(ev Event) error {
		data, err := json.Marshal(ev.Payload)
		if err != nil {
			a.logger.Error("failed to marshal SSE payload", zap.Error(err))
			return nil
		}
		if _, err := fmt.Fprintf(resp, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
			return err
		}
		return rc.Flush()
	}

	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request().Context().Done():
			return nil
		case ev, ok := <-sub:
			if !ok {
				return nil // hub disconnected us (overflow or simulation deleted)
			}
			if err := writeEvent(ev); err != nil {
				return nil
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(resp, ": ping\n\n"); err != nil {
				return nil
			}
			if err := rc.Flush(); err != nil {
				return nil
			}
		}
	}
}

// atoiDefault parses s as an int, returning def on empty/invalid input.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
