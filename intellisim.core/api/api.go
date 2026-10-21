package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"intelli.sim.core/modules/auth"
	"intelli.sim.core/modules/chats"
	"intelli.sim.core/modules/llm"
	"intelli.sim.core/modules/models"
	"intelli.sim.core/modules/simulations"
)

type API struct {
	logger        *zap.Logger
	authMW        *auth.Middleware
	authAPI       *auth.AuthAPI
	llmAPI        *llm.LLMAPI
	chatAPI       *chats.ChatAPI
	modelAPI      *models.ModelAPI
	simulationAPI *simulations.API
}

func NewAPI(
	logger *zap.Logger,
	authMW *auth.Middleware,
	authAPI *auth.AuthAPI,
	llmAPI *llm.LLMAPI,
	chatAPI *chats.ChatAPI,
	modelAPI *models.ModelAPI,
	simulationAPI *simulations.API,
) *API {
	return &API{
		logger:        logger.Named("api"),
		authMW:        authMW,
		authAPI:       authAPI,
		llmAPI:        llmAPI,
		chatAPI:       chatAPI,
		modelAPI:      modelAPI,
		simulationAPI: simulationAPI,
	}
}

func (a *API) SetupRoutes(e *echo.Echo) {
	apiGroup := e.Group("/api/v1/intellisim")

	apiGroup.GET("/status", a.GetStatusHandler)

	requireAuth := a.authMW.Require()

	authGroup := apiGroup.Group("/auth", requireAuth)
	a.authAPI.SetupRoutes(authGroup)

	llmsGroup := apiGroup.Group("/llms", requireAuth)
	a.llmAPI.SetupRoutes(llmsGroup)

	chatGroup := apiGroup.Group("/chats", requireAuth)
	a.chatAPI.SetupRoutes(chatGroup)

	modelGroup := apiGroup.Group("/models", requireAuth)
	a.modelAPI.SetupRoutes(modelGroup)

	// The simulations group is bearer-authenticated like the others, EXCEPT the
	// run WebSocket route (".../ws"): a browser WS handshake cannot carry an
	// Authorization header, so that route authenticates via a single-use connect
	// ticket inside the handler instead. skipWS bypasses the bearer middleware for
	// it while keeping every other simulations route authenticated.
	simulationGroup := apiGroup.Group("/simulations", skipWS(requireAuth))
	a.simulationAPI.SetupRoutes(simulationGroup)
}

// skipWS wraps an auth middleware so it is bypassed for run WebSocket routes
// (request path suffix "/ws"), which authenticate via a connect ticket in the
// handler rather than the Authorization header. All other paths run mw unchanged.
func skipWS(mw echo.MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		authed := mw(next)
		return func(c *echo.Context) error {
			if strings.HasSuffix(c.Request().URL.Path, "/ws") {
				return next(c)
			}
			return authed(c)
		}
	}
}

func (a *API) GetStatusHandler(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status": "Ok",
	})
}
