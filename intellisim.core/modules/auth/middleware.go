package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/pkg/httpapi"
)

// Middleware enforces bearer-token auth on a route group and injects the
// verified UserPrincipal into the request context.
type Middleware struct {
	logger   *zap.Logger
	verifier TokenVerifier
}

func NewMiddleware(logger *zap.Logger, verifier TokenVerifier) *Middleware {
	return &Middleware{logger: logger.Named("auth-middleware"), verifier: verifier}
}

// Require rejects requests without a valid "Authorization: Bearer <jwt>"
// header (header only — tokens are never accepted via URL).
func (m *Middleware) Require() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get(echo.HeaderAuthorization)
			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				return httpapi.NewAPIError(http.StatusUnauthorized, "unauthorized", "missing bearer token")
			}
			principal, err := m.verifier.VerifyToken(c.Request().Context(), strings.TrimPrefix(header, prefix))
			if err != nil {
				m.logger.Debug("token verification failed", zap.Error(err))
				return httpapi.NewAPIError(http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			}
			// Principal lives on the REQUEST context; retrieve via PrincipalFromContext(c.Request().Context()), not c.Get.
			ctx := WithPrincipal(c.Request().Context(), principal)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
