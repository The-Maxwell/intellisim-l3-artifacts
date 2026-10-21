package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanAccess(t *testing.T) {
	owner := &UserPrincipal{ID: "alice"}
	assert.True(t, owner.CanAccess("alice"), "owner accesses own resource")
	assert.False(t, owner.CanAccess("bob"), "non-owner blocked")

	admin := &UserPrincipal{ID: "root", IsAdmin: true}
	assert.True(t, admin.CanAccess("alice"), "admin accesses any resource")
	assert.True(t, admin.CanAccess("root"))
}

func TestAuthorize(t *testing.T) {
	owner := &UserPrincipal{ID: "alice"}
	assert.NoError(t, owner.Authorize("alice"))
	assert.ErrorIs(t, owner.Authorize("bob"), ErrForbidden)

	admin := &UserPrincipal{ID: "root", IsAdmin: true}
	assert.NoError(t, admin.Authorize("bob"))
}

func TestRequirePrincipal(t *testing.T) {
	_, err := RequirePrincipal(context.Background())
	assert.ErrorIs(t, err, ErrNoPrincipal, "missing principal -> ErrNoPrincipal")

	want := &UserPrincipal{ID: "alice"}
	got, err := RequirePrincipal(WithPrincipal(context.Background(), want))
	assert.NoError(t, err)
	assert.Same(t, want, got)
}
