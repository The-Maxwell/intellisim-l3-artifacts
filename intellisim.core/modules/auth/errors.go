package auth

import (
	"context"
	"errors"
)

// Cross-cutting authorization sentinels. The HTTP layer maps these to the
// shared httpapi.APIError envelope (401 / 403). Modules should return these
// (bare or wrapped with %w) from their service layer instead of minting their
// own per-module copies, so authorization behaves identically everywhere.
var (
	// ErrNoPrincipal means no authenticated principal was found in context.
	// In practice this is a wiring bug (a handler reached without passing
	// Middleware.Require) or a detached background context that lost its
	// principal. Maps to 401 Unauthorized.
	ErrNoPrincipal = errors.New("auth: no authenticated principal in context")

	// ErrForbidden means an authenticated principal attempted to access a
	// resource it neither owns nor has admin rights over. Maps to 403
	// Forbidden. Unlike a not-found response, returning this DOES disclose
	// that the resource exists — use it only where that disclosure is
	// acceptable (callers that prefer to hide existence should map the same
	// condition to their own not-found sentinel instead).
	ErrForbidden = errors.New("auth: principal is not allowed to access this resource")
)

// RequirePrincipal extracts the authenticated principal from ctx, returning
// ErrNoPrincipal when absent. Use it at the top of any service-layer method
// that must run as a known user (every owner-scoped use case).
func RequirePrincipal(ctx context.Context) (*UserPrincipal, error) {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		return nil, ErrNoPrincipal
	}
	return p, nil
}
