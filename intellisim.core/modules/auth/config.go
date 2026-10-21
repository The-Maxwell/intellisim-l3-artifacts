package auth

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config is the auth module's environment configuration.
type Config struct {
	// AuthServer is the OIDC issuer URL used for discovery + JWKS (e.g. the
	// dev mock http://localhost:9090, later a Keycloak realm URL).
	AuthServer string `env:"AUTH_SERVER,required"`
	// Audience, when non-empty, must match the token's aud claim.
	Audience string `env:"AUTH_AUDIENCE"`
	// AdminEnabled is the master switch for the admin capability (default off).
	AdminEnabled bool `env:"AUTH_ADMIN_ENABLED" envDefault:"false"`
	// AdminPermission is the permission value that grants admin access.
	AdminPermission string `env:"AUTH_ADMIN_PERMISSION" envDefault:"intellisim:admin"`
	// PermissionsClaim is a dot-path into token claims holding the user's
	// permissions array ("permissions"; for Keycloak: "realm_access.roles").
	PermissionsClaim string `env:"AUTH_PERMISSIONS_CLAIM" envDefault:"permissions"`
}

func NewConfig() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse auth config: %w", err)
	}
	return cfg, nil
}
