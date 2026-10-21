package httpserver

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"intelli.sim.core/api"
	"intelli.sim.core/pkg/httpapi"
)

type HttpServer struct {
	e      *echo.Echo
	logger *zap.Logger
	api    *api.API
	cancel context.CancelFunc
	done   chan struct{}
}

func (server *HttpServer) ID() string {
	return "http-server"
}

func (server *HttpServer) OnInit() error {
	port := 8080
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		} // Maybe put logs about wrong port
	}

	server.logger.Info("Start Http server", zap.Int("port", port))

	ctx, cancel := context.WithCancel(context.Background())
	server.cancel = cancel
	server.done = make(chan struct{})

	sc := echo.StartConfig{
		Address:         ":" + strconv.Itoa(port),
		HideBanner:      true,
		GracefulTimeout: 10 * time.Second,
	}

	go func() {
		defer close(server.done)
		err := sc.Start(ctx, server.e)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.logger.Fatal("failed to start server", zap.Error(err))
		}
	}()
	return nil
}

func (server *HttpServer) OnDestroy() error {
	server.logger.Info("Stop Http server")
	defer server.logger.Info("Http server is stopped")

	// Cancelling the start context triggers Echo's graceful shutdown.
	if server.cancel != nil {
		server.cancel()
	}

	if server.done == nil {
		return nil
	}
	select {
	case <-server.done:
	case <-time.After(15 * time.Second):
		server.logger.Warn("Http server shutdown timed out")
	}
	return nil
}

func (server *HttpServer) GetServer() *echo.Echo {
	return server.e
}

func (server *HttpServer) GetGroup(path string) *echo.Group {
	return server.e.Group(path)
}

func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	default:
		return "error"
	}
}

// newHTTPErrorHandler builds the shared echo.HTTPErrorHandler: it serializes
// *httpapi.APIError as-is, maps *echo.HTTPError and any other error carrying
// an HTTPStatusCoder (notably echo's own router sentinels ErrNotFound /
// ErrMethodNotAllowed, which are the unexported *httpError type) into the
// same envelope, and otherwise falls back to a logged 500.
func newHTTPErrorHandler(logger *zap.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
			return
		}
		if apiErr, ok := errors.AsType[*httpapi.APIError](err); ok {
			_ = c.JSON(apiErr.Status, apiErr)
			return
		}
		if httpErr, ok := errors.AsType[*echo.HTTPError](err); ok {
			_ = c.JSON(httpErr.Code, &httpapi.APIError{
				Code:    codeForStatus(httpErr.Code),
				Message: httpErr.Message,
			})
			return
		}
		if status := echo.StatusCode(err); status != 0 {
			_ = c.JSON(status, &httpapi.APIError{
				Code:    codeForStatus(status),
				Message: http.StatusText(status),
			})
			return
		}
		logger.Error("unhandled request error", zap.Error(err))
		_ = c.JSON(http.StatusInternalServerError, &httpapi.APIError{Code: "internal", Message: "internal server error"})
	}
}

func getAllowedOrigins() []string {
	origin := os.Getenv("CORS_ALLOWED_ORIGIN")
	if origin == "" {
		return []string{"http://localhost:3000", "http://127.0.0.1:3000"} // Dev defaults
	}
	if origin == "*" {
		return []string{"*"}
	}
	return strings.Split(origin, ",")
}

func NewHttpServer(
	lc fx.Lifecycle,
	logger *zap.Logger,
	api *api.API,
) *HttpServer {
	app := echo.New()
	app.Validator = httpapi.NewGoPlaygroundValidator()

	// Serialize *httpapi.APIError returns as the shared envelope; map echo's
	// own errors (404 route-not-found, 405 method-not-allowed etc.) into the
	// same shape.
	app.HTTPErrorHandler = newHTTPErrorHandler(logger)

	app.Use(middleware.Recover())
	app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: getAllowedOrigins(),
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	app.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogMethod:  true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			logger.Info("request",
				zap.String("URI", v.URI),
				zap.Int("status", v.Status),
				zap.String("method", v.Method),
				zap.Duration("latency", v.Latency),
				zap.Error(v.Error),
			)
			return nil
		},
	}))

	// Register routes BEFORE server starts
	api.SetupRoutes(app)

	server := &HttpServer{
		e:      app,
		logger: logger.Named("http-server"),
		api:    api,
	}

	// Lifecycle hooks on HttpServer itself
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return server.OnInit()
		},
		OnStop: func(ctx context.Context) error {
			return server.OnDestroy()
		},
	})

	return server
}
