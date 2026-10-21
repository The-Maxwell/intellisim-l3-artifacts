package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/modules/auth"
	"intelli.sim.core/pkg/httpapi"
)

const sseHeartbeatInterval = 25 * time.Second

type ModelAPI struct {
	logger       *zap.Logger
	modelService ModelService
	hub          *EventsHub
}

func NewModelAPI(logger *zap.Logger, service ModelService, hub *EventsHub) *ModelAPI {
	return &ModelAPI{logger: logger.Named("model-api"), modelService: service, hub: hub}
}

func (a *ModelAPI) SetupRoutes(group *echo.Group) {
	group.POST("", a.CreateModel)
	group.GET("", a.ListModels)
	group.GET("/options", a.ListModelOptions)
	group.GET("/:id", a.GetModel)
	group.PUT("/:id", a.UpdateModel)
	group.DELETE("/:id", a.DeleteModel)
	group.POST("/:id/duplicate", a.DuplicateModel)
	group.GET("/:id/validation", a.GetValidation)
	group.GET("/:id/chat", a.GetChat)
	group.PUT("/:id/chat", a.SetChatTarget)
	group.GET("/:id/chat/messages", a.ListMessages)
	group.POST("/:id/chat/messages", a.SendMessage)
	group.GET("/:id/chat/events", a.StreamEvents)
	group.POST("/:id/images", a.AddModelImage)
	group.DELETE("/:id/images/:imageId", a.DeleteModelImage)
	group.POST("/:id/environment/generate", a.GenerateEnvironment)
}

// mapError translates service sentinels into the shared APIError envelope. A
// *httpapi.APIError (e.g. the 422 from UpdateModel) is passed through unchanged.
func (a *ModelAPI) mapError(err error) error {
	var apiErr *httpapi.APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	switch {
	case errors.Is(err, ErrModelNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, "not_found", "model not found")
	case errors.Is(err, ErrModelUnavailable):
		return httpapi.NewAPIError(http.StatusConflict, "model_unavailable",
			"the model's LLM is not available right now — select another model to continue")
	case errors.Is(err, ErrEffortNotAllowed):
		return httpapi.NewAPIError(http.StatusConflict, "effort_not_allowed",
			"the requested effort is not allowed for this model")
	case errors.Is(err, ErrTargetCapability):
		return httpapi.NewAPIError(http.StatusConflict, "capability_not_supported",
			"the selected LLM does not support the required capability")
	case errors.Is(err, ErrGenerationInProgress):
		return httpapi.NewAPIError(http.StatusConflict, "generation_in_progress",
			"a generation is already in progress for this model")
	case errors.Is(err, ErrNotSpatial):
		return httpapi.NewAPIError(http.StatusConflict, "not_spatial",
			"environment operations require a spatial model")
	case errors.Is(err, ErrSimGenTargetRequired):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "sim_generation_target_required",
			"select a simulation-generation LLM — the model uses the LLM to seed the simulation")
	case errors.Is(err, ErrSimExecTargetRequired):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "sim_execution_target_required",
			"select a simulation-execution LLM — the model uses the LLM during the run")
	case errors.Is(err, ErrNoChat):
		return httpapi.NewAPIError(http.StatusConflict, "no_chat", "model has no companion chat")
	case errors.Is(err, ErrInvalidCursor):
		return httpapi.NewAPIError(http.StatusBadRequest, "invalid_cursor",
			"cursor message does not belong to this model chat")
	case errors.Is(err, ErrNoChanges):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, "validation_failed",
			"update request has no changes")
	case errors.Is(err, auth.ErrNoPrincipal):
		return httpapi.NewAPIError(http.StatusUnauthorized, "unauthorized", "no principal")
	case errors.Is(err, auth.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, "forbidden",
			"you do not have access to this model")
	default:
		a.logger.Error("model request failed", zap.Error(err))
		return httpapi.NewAPIError(http.StatusInternalServerError, "internal", "internal server error")
	}
}

func bindAndValidate(c *echo.Context, target any) error {
	if err := c.Bind(target); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	return c.Validate(target) // returns *APIError (validation_failed) on failure
}

func (a *ModelAPI) CreateModel(c *echo.Context) error {
	var req CreateModelRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	model, err := a.modelService.CreateModel(c.Request().Context(), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusCreated, model)
}

func (a *ModelAPI) ListModels(c *echo.Context) error {
	var q ListModelsQuery
	if err := bindAndValidate(c, &q); err != nil {
		return err
	}
	resp, err := a.modelService.ListModels(c.Request().Context(), q)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *ModelAPI) GetModel(c *echo.Context) error {
	model, err := a.modelService.GetModel(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, model)
}

// ListModelOptions returns the caller's models as a lightweight id+name list
// (admin: all), unpaginated — for selector UIs like the simulations model filter.
func (a *ModelAPI) ListModelOptions(c *echo.Context) error {
	resp, err := a.modelService.ListModelOptions(c.Request().Context())
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// misplacedLLMConfigKeys are the simulation LLM config keys that belong under
// model.simulation_config but have historically been submitted directly on
// model (a sibling of simulation_config, one level too shallow). Echo's
// default JSON binder silently ignores unknown keys rather than rejecting
// them, so such a request looked like it took effect (200 OK) while the
// intended config was dropped on the floor — the ISS-12 footgun.
var misplacedLLMConfigKeys = []string{
	"llm_simulation_execution_config",
	"llm_simulation_generation_config",
}

// checkMisplacedLLMConfigKeys inspects the raw request body's top-level
// "model" object for any of misplacedLLMConfigKeys and, if found, returns a
// 422 validation_failed pointing at exactly which key(s) are misplaced. It is
// deliberately narrow (unlike enabling a global DisallowUnknownFields): any
// OTHER unrecognized key is still ignored, matching the rest of the API.
// Malformed JSON is not reported here — it surfaces via the normal bind step.
func checkMisplacedLLMConfigKeys(body []byte) error {
	var raw struct {
		Model map[string]json.RawMessage `json:"model"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Model == nil {
		return nil
	}
	var fields map[string]string
	for _, key := range misplacedLLMConfigKeys {
		if _, present := raw.Model[key]; present {
			if fields == nil {
				fields = map[string]string{}
			}
			fields["model."+key] = "unknown field — must be nested under model.simulation_config"
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &httpapi.APIError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_failed",
		Message: "model has unrecognized top-level field(s)",
		Fields:  fields,
	}
}

func (a *ModelAPI) UpdateModel(c *echo.Context) error {
	// Read the raw body once (so we can both scan it for misplaced top-level
	// keys AND let the normal binder consume it) and re-seed the request body
	// for c.Bind, which can only read the stream once.
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if err := checkMisplacedLLMConfigKeys(bodyBytes); err != nil {
		return err
	}

	var req UpdateModelRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	model, err := a.modelService.UpdateModel(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, model)
}

func (a *ModelAPI) DeleteModel(c *echo.Context) error {
	if err := a.modelService.DeleteModel(c.Request().Context(), c.Param("id")); err != nil {
		return a.mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// DuplicateModel deep-copies a model (and its companion chat, if any) and returns
// the new model.
func (a *ModelAPI) DuplicateModel(c *echo.Context) error {
	var req DuplicateModelRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	model, err := a.modelService.DuplicateModel(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusCreated, model)
}

// GetValidation returns the advisory completeness validation for the model.
func (a *ModelAPI) GetValidation(c *echo.Context) error {
	model, err := a.modelService.GetModel(c.Request().Context(), c.Param("id"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, model.Validation)
}

func (a *ModelAPI) GetChat(c *echo.Context) error {
	resp, err := a.modelService.GetOrCreateChat(c.Request().Context(), c.Param("id"), nil)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *ModelAPI) SetChatTarget(c *echo.Context) error {
	var req SetModelChatTargetRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.modelService.SetChatTarget(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *ModelAPI) ListMessages(c *echo.Context) error {
	var q ListMessagesQuery
	if err := bindAndValidate(c, &q); err != nil {
		return err
	}
	page, err := a.modelService.ListMessages(c.Request().Context(), c.Param("id"), q)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, page)
}

func (a *ModelAPI) SendMessage(c *echo.Context) error {
	var req SendModelMessageRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.modelService.SendMessage(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

// AddModelImage appends one (compressed) image to the model's environment-generation
// image library and returns the full updated model.
func (a *ModelAPI) AddModelImage(c *echo.Context) error {
	var req AddModelImageRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	model, err := a.modelService.AddModelImage(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusCreated, model)
}

// DeleteModelImage removes one image (by UID) and returns the full updated model.
func (a *ModelAPI) DeleteModelImage(c *echo.Context) error {
	model, err := a.modelService.DeleteModelImage(c.Request().Context(), c.Param("id"), c.Param("imageId"))
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusOK, model)
}

// GenerateEnvironment starts async environment generation from the model's stored
// image library (202 + SSE model.updated when done).
func (a *ModelAPI) GenerateEnvironment(c *echo.Context) error {
	var req GenerateEnvironmentRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := a.modelService.GenerateEnvironment(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return a.mapError(err)
	}
	return c.JSON(http.StatusAccepted, resp)
}

// StreamEvents is the per-model SSE stream (auth + ownership enforced).
// Protocol: named events (chat.status / message.created / model.updated /
// chat.error / model.deleted) with JSON data; ": ping" heartbeat every 25s.
// No replay — on reconnect clients refetch the model + latest messages page.
func (a *ModelAPI) StreamEvents(c *echo.Context) error {
	modelID := c.Param("id")
	// Ownership gate (also yields 404 for missing/foreign models).
	if _, err := a.modelService.GetModel(c.Request().Context(), modelID); err != nil {
		return a.mapError(err)
	}

	sub, cancel := a.hub.Subscribe(modelID)
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
				return nil // hub disconnected us (overflow or model deleted)
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
