package auth

import (
	"slices"
	"strings"
)

// principalFromClaims maps raw token claims to a UserPrincipal per Config.
// ID resolution: preferred_username → sub. Name resolution: name → ID → email.
// If all identity sources are absent, ID and Name are empty strings; the middleware rejects unverifiable tokens before this mapping runs.
func principalFromClaims(claims map[string]any, cfg Config) *UserPrincipal {
	p := &UserPrincipal{}
	p.Sub, _ = claims["sub"].(string)
	p.ID, _ = claims["preferred_username"].(string)
	if p.ID == "" {
		p.ID = p.Sub
	}
	p.Email, _ = claims["email"].(string)
	p.Name, _ = claims["name"].(string)
	if p.Name == "" {
		p.Name = p.ID
	}
	if p.Name == "" {
		p.Name = p.Email
	}
	p.Permissions = stringSliceClaim(claims, cfg.PermissionsClaim)
	p.IsAdmin = cfg.AdminEnabled && slices.Contains(p.Permissions, cfg.AdminPermission)
	return p
}

// stringSliceClaim walks a dot-path ("realm_access.roles") into the claims map
// and returns the string members of the array found there (nil when absent).
func stringSliceClaim(claims map[string]any, dotPath string) []string {
	var cur any = claims
	for _, part := range strings.Split(dotPath, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[part]
		if !ok {
			return nil
		}
	}
	arr, ok := cur.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
