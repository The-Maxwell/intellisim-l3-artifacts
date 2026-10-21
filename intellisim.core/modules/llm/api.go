package llm

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/pkg/httpapi"
)

type LLMAPI struct {
	logger  *zap.Logger
	service Service
}

func NewLLMAPI(logger *zap.Logger, service Service) *LLMAPI {
	return &LLMAPI{logger: logger.Named("llm-api"), service: service}
}

func (a *LLMAPI) SetupRoutes(apiGroup *echo.Group) {
	apiGroup.GET("", a.GetLLMProvidersHandler)
}

// GetLLMProvidersHandler returns connected providers with optional filtering.
//
// Query params:
//   - capability: only providers/models with this capability (closed list, e.g. "vision_analysis")
//   - model: only providers offering this model UID (e.g. "gpt-4o")
func (a *LLMAPI) GetLLMProvidersHandler(c *echo.Context) error {
	capability := c.QueryParam("capability")
	model := c.QueryParam("model")

	var providers []ProviderInfo
	switch {
	case capability != "":
		mc := common.ModelCapability(capability)
		if !mc.Valid() {
			return &httpapi.APIError{
				Status:  http.StatusUnprocessableEntity,
				Code:    "validation_failed",
				Message: "request validation failed",
				Fields: map[string]string{
					"capability": fmt.Sprintf("unknown capability %q (allowed: %v)", capability, common.AllModelCapabilities),
				},
			}
		}
		providers = a.service.ProvidersByCapability(mc)
	case model != "":
		providers = a.service.ProvidersByModel(model)
	default:
		providers = a.service.Providers()
	}

	if providers == nil {
		providers = []ProviderInfo{}
	}
	return c.JSON(http.StatusOK, map[string]any{"list": providers})
}
