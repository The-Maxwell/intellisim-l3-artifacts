package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/pkg/httpapi"
)

// newTestAPI builds an LLMAPI backed by a real registry + service with one
// registered fakeProvider (reuses newTestProvider from service_test.go).
func newTestAPI(t *testing.T) (*LLMAPI, func()) {
	t.Helper()
	logger := zap.NewNop()
	reg := NewRegistry(logger)
	provider := newTestProvider()
	reg.Register(provider)
	fitter := NewContextFitter(logger, NewTokenEstimator(logger))
	svc := NewService(logger, reg, fitter)
	return NewLLMAPI(logger, svc), func() { reg.Unregister(provider.UID()) }
}

func doGet(t *testing.T, api *LLMAPI, rawURL string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, rawURL, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := api.GetLLMProvidersHandler(c)
	if err != nil {
		// echo handlers may return an HTTPError or an *httpapi.APIError; write
		// either into the recorder the way the real error handler would.
		if apiErr, ok := err.(*httpapi.APIError); ok {
			_ = c.JSON(apiErr.Status, apiErr)
		} else if he, ok := err.(*echo.HTTPError); ok {
			rec.WriteHeader(he.Code)
		} else {
			rec.WriteHeader(http.StatusInternalServerError)
		}
	}
	return rec
}

// Case 1: GET with no params → 200, "list" contains 1 provider.
func TestGetLLMProviders_NoParams(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := doGet(t, api, "/")
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	var list []ProviderInfo
	require.NoError(t, json.Unmarshal(body["list"], &list))
	assert.Len(t, list, 1)
}

// Case 2: GET ?capability=results_analysis → 200, models filtered to "analyst".
func TestGetLLMProviders_ByCapability(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := doGet(t, api, "/?capability=results_analysis")
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	var list []ProviderInfo
	require.NoError(t, json.Unmarshal(body["list"], &list))
	require.Len(t, list, 1)
	require.Len(t, list[0].Models, 1)
	assert.Equal(t, "analyst", list[0].Models[0].UID)
}

// Case 3: GET ?capability=bogus → 422 validation_failed, fields.capability
// contains "unknown capability".
func TestGetLLMProviders_UnknownCapability(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := doGet(t, api, "/?capability=bogus")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var apiErr httpapi.APIError
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apiErr))
	assert.Equal(t, "validation_failed", apiErr.Code)
	assert.Contains(t, apiErr.Fields["capability"], "unknown capability")
}

// Case 4: GET ?model=general → 200, list has 1 provider with only the "general" model.
func TestGetLLMProviders_ByModel(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := doGet(t, api, "/?model=general")
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	var list []ProviderInfo
	require.NoError(t, json.Unmarshal(body["list"], &list))
	require.Len(t, list, 1)
	require.Len(t, list[0].Models, 1)
	assert.Equal(t, "general", list[0].Models[0].UID)
}

// Case 5: Empty registry → 200, "list" is [] not null.
func TestGetLLMProviders_EmptyRegistry(t *testing.T) {
	logger := zap.NewNop()
	reg := NewRegistry(logger)
	fitter := NewContextFitter(logger, NewTokenEstimator(logger))
	svc := NewService(logger, reg, fitter)
	api := NewLLMAPI(logger, svc)

	rec := doGet(t, api, "/?")
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.True(t, strings.Contains(body, `"list":[]`), "expected list:[] but got: %s", body)
}
