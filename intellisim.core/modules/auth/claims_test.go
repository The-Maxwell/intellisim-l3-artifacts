package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func baseCfg() Config {
	return Config{
		AdminEnabled:     true,
		AdminPermission:  "intellisim:admin",
		PermissionsClaim: "permissions",
	}
}

func TestPrincipalFromClaims_PreferredUsernameAsID(t *testing.T) {
	p := principalFromClaims(map[string]any{
		"sub":                "dev-user-1",
		"preferred_username": "dev.user",
		"name":               "Dev User",
		"email":              "dev@intellisim.local",
		"permissions":        []any{"x"},
	}, baseCfg())

	assert.Equal(t, "dev.user", p.ID)
	assert.Equal(t, "dev-user-1", p.Sub)
	assert.Equal(t, "Dev User", p.Name)
	assert.Equal(t, []string{"x"}, p.Permissions)
	assert.False(t, p.IsAdmin)
}

func TestPrincipalFromClaims_FallbackToSub(t *testing.T) {
	p := principalFromClaims(map[string]any{"sub": "abc"}, baseCfg())
	assert.Equal(t, "abc", p.ID)
	assert.Equal(t, "abc", p.Name) // name falls back to ID
}

func TestPrincipalFromClaims_AdminFlag(t *testing.T) {
	claims := map[string]any{"sub": "a", "permissions": []any{"intellisim:admin"}}

	p := principalFromClaims(claims, baseCfg())
	assert.True(t, p.IsAdmin)

	cfg := baseCfg()
	cfg.AdminEnabled = false // master switch off → never admin
	p = principalFromClaims(claims, cfg)
	assert.False(t, p.IsAdmin)
}

func TestPrincipalFromClaims_DotPathClaim(t *testing.T) {
	cfg := baseCfg()
	cfg.PermissionsClaim = "realm_access.roles"
	p := principalFromClaims(map[string]any{
		"sub":          "a",
		"realm_access": map[string]any{"roles": []any{"intellisim:admin", 42, "other"}},
	}, cfg)
	assert.Equal(t, []string{"intellisim:admin", "other"}, p.Permissions) // non-strings skipped
	assert.True(t, p.IsAdmin)
}

func TestPrincipalFromClaims_MissingClaimIsEmpty(t *testing.T) {
	p := principalFromClaims(map[string]any{"sub": "a"}, baseCfg())
	assert.Empty(t, p.Permissions)
	assert.False(t, p.IsAdmin)
}

func TestPrincipalFromClaims_NameFallsBackToEmail(t *testing.T) {
	p := principalFromClaims(map[string]any{
		"email": "dev@intellisim.local",
	}, baseCfg())
	assert.Equal(t, "", p.ID)    // no sub or preferred_username
	assert.Equal(t, "dev@intellisim.local", p.Name)
}

func TestPrincipalFromClaims_EmptyPermissionsClaim(t *testing.T) {
	cfg := baseCfg()
	cfg.PermissionsClaim = ""
	p := principalFromClaims(map[string]any{"sub": "a", "permissions": []any{"x"}}, cfg)
	assert.Empty(t, p.Permissions) // misconfigured claim path → safe nil
}
