package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/pkg/httpapi"
)

type AuthAPI struct {
	logger *zap.Logger
}

func NewAuthAPI(logger *zap.Logger) *AuthAPI {
	return &AuthAPI{logger: logger.Named("auth-api")}
}

// SetupRoutes registers auth endpoints; the group must already be protected
// by Middleware.Require().
func (a *AuthAPI) SetupRoutes(group *echo.Group) {
	group.GET("/me", a.GetMeHandler)
}

func (a *AuthAPI) GetMeHandler(c *echo.Context) error {
	principal, ok := PrincipalFromContext(c.Request().Context())
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, "unauthorized", "no principal in context")
	}
	return c.JSON(http.StatusOK, principal)
}
