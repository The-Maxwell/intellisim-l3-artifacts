package auth

import "context"

// UserPrincipal is the authenticated identity attached to each request.
//
// ID is the human-readable identity used for ownership: created_by/updated_by
// stamping and owner checks. It comes from the preferred_username claim
// (Keycloak default), falling back to sub when absent. Sub is kept separately
// as the stable technical subject id.
type UserPrincipal struct {
	ID          string   `json:"id"`
	Sub         string   `json:"sub"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	IsAdmin     bool     `json:"is_admin"`
}

// CanAccess reports whether this principal may act on a resource owned by
// ownerID: true for the resource's owner, or for any admin. ownerID is the
// created_by value stored on the resource (i.e. a UserPrincipal.ID).
func (p *UserPrincipal) CanAccess(ownerID string) bool {
	return p.IsAdmin || p.ID == ownerID
}

// Authorize returns nil when the principal may act on a resource owned by
// ownerID, or ErrForbidden otherwise. It is the error-returning counterpart of
// CanAccess for use directly in service methods.
func (p *UserPrincipal) Authorize(ownerID string) error {
	if !p.CanAccess(ownerID) {
		return ErrForbidden
	}
	return nil
}

type principalCtxKey struct{}

// WithPrincipal returns a context carrying the authenticated principal.
func WithPrincipal(ctx context.Context, p *UserPrincipal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext extracts the principal set by the auth middleware.
func PrincipalFromContext(ctx context.Context) (*UserPrincipal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(*UserPrincipal)
	return p, ok
}
