package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/pkg/httpapi"
)

// fakeModelSvc is a minimal ModelService stub for exercising ModelAPI handlers
// in isolation. Only UpdateModel is exercised by these tests; every other
// method is a no-op satisfying the interface.
type fakeModelSvc struct {
	updateCalls int
	updateID    string
	updateReq   UpdateModelRequest
	updateResp  *ModelResponse
	updateErr   error
}

func (f *fakeModelSvc) CreateModel(context.Context, CreateModelRequest) (*ModelResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) GetModel(context.Context, string) (*ModelResponse, error) { return nil, nil }
func (f *fakeModelSvc) UpdateModel(_ context.Context, id string, req UpdateModelRequest) (*ModelResponse, error) {
	f.updateCalls++
	f.updateID = id
	f.updateReq = req
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp != nil {
		return f.updateResp, nil
	}
	return &ModelResponse{ID: id}, nil
}
func (f *fakeModelSvc) DeleteModel(context.Context, string) error { return nil }
func (f *fakeModelSvc) DuplicateModel(context.Context, string, DuplicateModelRequest) (*ModelResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) ListModels(context.Context, ListModelsQuery) (*ListModelsResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) ListModelOptions(context.Context) (*ListModelOptionsResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) GetOrCreateChat(context.Context, string, *ExecLLMTargetDTO) (*ModelChatResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) SetChatTarget(context.Context, string, SetModelChatTargetRequest) (*ModelChatResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) ListMessages(context.Context, string, ListMessagesQuery) (*MessagesPageResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) SendMessage(context.Context, string, SendModelMessageRequest) (*SendMessageResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) AddModelImage(context.Context, string, AddModelImageRequest) (*ModelResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) DeleteModelImage(context.Context, string, string) (*ModelResponse, error) {
	return nil, nil
}
func (f *fakeModelSvc) GenerateEnvironment(context.Context, string, GenerateEnvironmentRequest) (*SendMessageResponse, error) {
	return nil, nil
}

func newModelAPI(svc ModelService) *ModelAPI {
	return NewModelAPI(zap.NewNop(), svc, NewEventsHub())
}

// newModelReq builds an echo.Context for a PUT-style request with a JSON body
// and the :id path param set (mirrors modules/simulations' run_api_test.go newReq).
func newModelReq(t *testing.T, method, target, id, body string) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	e.Validator = httpapi.NewGoPlaygroundValidator()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(r, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: id}})
	return c, rec
}

// TestUpdateModel_RejectsMisplacedExecutionConfig pins ISS-12: a top-level
// model.llm_simulation_execution_config (one level too shallow; the real slot
// is model.simulation_config.llm_simulation_execution_config) must 422 with a
// field message pointing at the correct nesting, and must never reach the service.
func TestUpdateModel_RejectsMisplacedExecutionConfig(t *testing.T) {
	svc := &fakeModelSvc{}
	api := newModelAPI(svc)
	body := `{"model":{"name":"X","llm_simulation_execution_config":{"target":{"provider_uid":"p","model_uid":"m"}}}}`
	c, _ := newModelReq(t, http.MethodPut, "/models/m1", "m1", body)

	err := api.UpdateModel(c)
	apiErr, ok := err.(*httpapi.APIError)
	require.True(t, ok, "err type=%T want *httpapi.APIError", err)
	require.Equal(t, http.StatusUnprocessableEntity, apiErr.Status)
	require.Equal(t, "validation_failed", apiErr.Code)
	msg, present := apiErr.Fields["model.llm_simulation_execution_config"]
	require.True(t, present, "fields=%v missing the misplaced-key pointer", apiErr.Fields)
	require.Contains(t, msg, "simulation_config")
	require.Zero(t, svc.updateCalls, "the service must not be called when the request is rejected")
}

// TestUpdateModel_RejectsMisplacedGenerationConfig mirrors the above for the
// generation-config sibling key.
func TestUpdateModel_RejectsMisplacedGenerationConfig(t *testing.T) {
	svc := &fakeModelSvc{}
	api := newModelAPI(svc)
	body := `{"model":{"name":"X","llm_simulation_generation_config":{"target":{"provider_uid":"p","model_uid":"m"}}}}`
	c, _ := newModelReq(t, http.MethodPut, "/models/m1", "m1", body)

	err := api.UpdateModel(c)
	apiErr, ok := err.(*httpapi.APIError)
	require.True(t, ok, "err type=%T want *httpapi.APIError", err)
	require.Equal(t, http.StatusUnprocessableEntity, apiErr.Status)
	require.Equal(t, "validation_failed", apiErr.Code)
	_, present := apiErr.Fields["model.llm_simulation_generation_config"]
	require.True(t, present, "fields=%v missing the misplaced-key pointer", apiErr.Fields)
	require.Zero(t, svc.updateCalls)
}

// TestUpdateModel_AllowsCorrectlyNestedConfig verifies the correctly-nested
// shape passes straight through to the service (no false positive).
func TestUpdateModel_AllowsCorrectlyNestedConfig(t *testing.T) {
	svc := &fakeModelSvc{}
	api := newModelAPI(svc)
	body := `{"model":{"name":"X","simulation_config":{"llm_simulation_execution_config":{"target":{"provider_uid":"p","model_uid":"m"}}}}}`
	c, rec := newModelReq(t, http.MethodPut, "/models/m1", "m1", body)

	err := api.UpdateModel(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, svc.updateCalls)
	require.Equal(t, "p", svc.updateReq.Model.SimulationConfig.LLMSimulationExecutionConfig.Target.ProviderUID)
}

// TestUpdateModel_UnrelatedUnknownKeysStillIgnored verifies the fix is narrow:
// an unrelated unknown top-level key is still silently ignored (lenient
// elsewhere), matching Echo's default binder behaviour everywhere else.
func TestUpdateModel_UnrelatedUnknownKeysStillIgnored(t *testing.T) {
	svc := &fakeModelSvc{}
	api := newModelAPI(svc)
	body := `{"model":{"name":"X","totally_unknown_field":123}}`
	c, rec := newModelReq(t, http.MethodPut, "/models/m1", "m1", body)

	err := api.UpdateModel(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, svc.updateCalls)
}
