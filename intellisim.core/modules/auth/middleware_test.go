package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/pkg/httpapi"
)

type stubVerifier struct {
	principal *UserPrincipal
	err       error
}

func (s *stubVerifier) VerifyToken(_ context.Context, _ string) (*UserPrincipal, error) {
	return s.principal, s.err
}

// newTestApp builds an echo app with the production error handler so the
// envelope assertions match real behavior.
func newTestApp(t *testing.T, verifier TokenVerifier) *echo.Echo {
	t.Helper()
	app := echo.New()
	app.HTTPErrorHandler = func(c *echo.Context, err error) {
		if apiErr, ok := errors.AsType[*httpapi.APIError](err); ok {
			_ = c.JSON(apiErr.Status, apiErr)
			return
		}
		_ = c.JSON(http.StatusInternalServerError, map[string]string{"code": "internal"})
	}
	mw := NewMiddleware(zap.NewNop(), verifier)
	g := app.Group("/p", mw.Require())
	g.GET("/whoami", func(c *echo.Context) error {
		p, ok := PrincipalFromContext(c.Request().Context())
		require.True(t, ok)
		return c.JSON(http.StatusOK, p)
	})
	return app
}

func TestRequire_MissingToken401(t *testing.T) {
	app := newTestApp(t, &stubVerifier{})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/whoami", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "unauthorized", body["code"])
}

func TestRequire_InvalidToken401(t *testing.T) {
	app := newTestApp(t, &stubVerifier{err: errors.New("bad signature")})
	req := httptest.NewRequest(http.MethodGet, "/p/whoami", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequire_ValidTokenInjectsPrincipal(t *testing.T) {
	app := newTestApp(t, &stubVerifier{principal: &UserPrincipal{ID: "dev.user", Sub: "dev-user-1"}})
	req := httptest.NewRequest(http.MethodGet, "/p/whoami", nil)
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var p UserPrincipal
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, "dev.user", p.ID)
}

func TestRequire_EmptyBearerToken401(t *testing.T) {
	app := newTestApp(t, &stubVerifier{err: errors.New("empty token")})
	req := httptest.NewRequest(http.MethodGet, "/p/whoami", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequire_LowercaseSchemeRejected(t *testing.T) {
	// Scheme matching is deliberately strict (exact "Bearer "); pin it.
	app := newTestApp(t, &stubVerifier{principal: &UserPrincipal{ID: "x"}})
	req := httptest.NewRequest(http.MethodGet, "/p/whoami", nil)
	req.Header.Set("Authorization", "bearer good")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequire_NonBearerSchemeRejected(t *testing.T) {
	app := newTestApp(t, &stubVerifier{principal: &UserPrincipal{ID: "x"}})
	req := httptest.NewRequest(http.MethodGet, "/p/whoami", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
