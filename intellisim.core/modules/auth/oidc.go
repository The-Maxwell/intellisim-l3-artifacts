package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
)

// TokenVerifier validates a raw bearer token and returns the principal.
// Interface so the middleware is unit-testable with a stub.
type TokenVerifier interface {
	VerifyToken(ctx context.Context, rawToken string) (*UserPrincipal, error)
}

// OIDCService verifies JWT access tokens against the configured issuer:
// OIDC discovery + remote JWKS (cached, auto-rotating via go-oidc), checking
// signature, issuer and expiry; audience only when Config.Audience is set.
//
// Discovery is lazy with retry-on-next-request so core does not crash when the
// auth server boots later than core.
type OIDCService struct {
	logger *zap.Logger
	cfg    Config

	mu       sync.RWMutex
	verifier *oidc.IDTokenVerifier
}

func NewOIDCService(logger *zap.Logger, cfg Config) TokenVerifier {
	return &OIDCService{logger: logger.Named("oidc"), cfg: cfg}
}

func (s *OIDCService) ensureVerifier() (*oidc.IDTokenVerifier, error) {
	// Fast path: verifier already initialised — no write contention.
	s.mu.RLock()
	if s.verifier != nil {
		v := s.verifier
		s.mu.RUnlock()
		return v, nil
	}
	s.mu.RUnlock()

	// Slow path: need to run OIDC discovery.
	// Bound the HTTP call so a wedged auth server does not hold the write lock
	// indefinitely — without a timeout every in-flight request piles up behind
	// the mutex and the service appears frozen during auth-server outages.
	s.mu.Lock()
	defer s.mu.Unlock()
	// Re-check: another goroutine may have initialised while we waited.
	if s.verifier != nil {
		return s.verifier, nil
	}
	// go-oidc retains the HTTP client extracted from this context inside the
	// verifier's remote key set (Provider.client → remoteKeySet → NewRemoteKeySet),
	// so it must be long-lived. HTTP timeouts (discovery + future JWKS refreshes)
	// are bounded via the injected client instead of a cancellable context — a
	// wedged auth server must neither pin the lock nor poison the verifier.
	httpClient := &http.Client{Timeout: 10 * time.Second}
	provCtx := oidc.ClientContext(context.Background(), httpClient)
	provider, err := oidc.NewProvider(provCtx, s.cfg.AuthServer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery against %q: %w", s.cfg.AuthServer, err)
	}
	oc := &oidc.Config{ClientID: s.cfg.Audience}
	if s.cfg.Audience == "" {
		oc.SkipClientIDCheck = true
	}
	s.verifier = provider.Verifier(oc)
	s.logger.Info("OIDC verifier initialized", zap.String("issuer", s.cfg.AuthServer))
	return s.verifier, nil
}

func (s *OIDCService) VerifyToken(ctx context.Context, rawToken string) (*UserPrincipal, error) {
	verifier, err := s.ensureVerifier()
	if err != nil {
		return nil, err
	}
	token, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode token claims: %w", err)
	}
	principal := principalFromClaims(claims, s.cfg)
	if principal.ID == "" {
		return nil, fmt.Errorf("token has no usable identity (missing preferred_username and sub)")
	}
	return principal, nil
}
