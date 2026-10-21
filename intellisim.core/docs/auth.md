# Authentication Reference

The `modules/auth` package provides OIDC/JWT-based authentication for the IntelliSim core backend. It is designed to work with any spec-compliant OIDC provider — the dev environment uses `intellisim.auth.mock`, and production targets Keycloak. The entire authentication surface is configured through environment variables; no code change is needed to switch providers.

---

## Architecture

The module contains six components:

- **`Config`** (`config.go`) — parses auth-related env vars via `caarlos0/env`.
- **`OIDCService`** (`oidc.go`) — implements `TokenVerifier`; performs lazy OIDC discovery and JWKS-based JWT verification using `go-oidc/v3`.
- **`Middleware`** (`middleware.go`) — an Echo middleware that extracts and validates the bearer token, then injects the resulting `UserPrincipal` into the request context.
- **`AuthAPI`** (`api.go`) — a single `GET /auth/me` handler that returns the caller's principal as JSON.
- **`UserPrincipal`** (`principal.go`) — the authenticated identity struct stored on request contexts, plus its `CanAccess`/`Authorize` ownership helpers.
- **Authorization helpers** (`errors.go`) — the reusable `RequirePrincipal(ctx)` accessor and the cross-cutting `ErrNoPrincipal` (401) / `ErrForbidden` (403) sentinels that any module reuses (see [Authorization Helpers](#authorization-helpers)).

The `Middleware` depends on the `TokenVerifier` interface, making it independently unit-testable with a stub. The FX module (`fx.go`) wires everything together.

---

## Token Verification

**Discovery is lazy.** `OIDCService` does not contact the auth server at startup. The first incoming request triggers OIDC discovery (`GET {AUTH_SERVER}/.well-known/openid-configuration`). This means core starts successfully even when the auth server is not yet running.

Once the discovery succeeds, the `oidc.IDTokenVerifier` is cached. Subsequent requests skip discovery entirely (a read-lock check on a non-nil pointer). If discovery fails on the first request, it will be retried on the next request — there is no exponential backoff, but failures do not propagate through a write-lock stall.

**HTTP client for discovery and JWKS:** A dedicated `http.Client{Timeout: 10s}` is constructed and passed into go-oidc via `oidc.ClientContext`. This client is retained inside go-oidc's remote key set for ongoing JWKS rotations. The 10-second timeout prevents a wedged auth server from blocking verification threads indefinitely.

**What is verified per token:**
1. Signature (asymmetric, using the JWKS published by `AUTH_SERVER`).
2. Issuer (`iss`) — must match `AUTH_SERVER`.
3. Expiry (`exp`).
4. Audience (`aud`) — only when `AUTH_AUDIENCE` is non-empty; skipped otherwise.

**Header only.** The `Middleware.Require()` function reads only the `Authorization` header. Query-parameter or cookie tokens are never accepted.

---

## UserPrincipal

After successful verification, token claims are mapped to `UserPrincipal`:

```go
type UserPrincipal struct {
    ID          string   // primary identity for ownership; preferred_username → sub
    Sub         string   // stable technical subject (always "sub" claim)
    Email       string
    Name        string   // "name" claim → ID → Email (first non-empty)
    Permissions []string // extracted from AUTH_PERMISSIONS_CLAIM (dot-path)
    IsAdmin     bool     // cfg.AdminEnabled && Permissions contains AUTH_ADMIN_PERMISSION
}
```

**ID resolution:** `preferred_username` is used when present (Keycloak's default human-readable username). If the claim is absent, `sub` is used. This means `created_by` fields in the database hold a consistent, human-readable value for Keycloak-based deployments and the dev mock, while still working for providers that only issue `sub`.

**Permissions:** The `AUTH_PERMISSIONS_CLAIM` env var is a dot-path into the claims object. The default value `permissions` works for flat arrays; `realm_access.roles` is the Keycloak standard path (see Keycloak migration notes below). The value at that path must be a JSON array of strings.

**IsAdmin:** `true` only when both conditions hold simultaneously: `AUTH_ADMIN_ENABLED=true` (the master switch) AND the `AUTH_ADMIN_PERMISSION` string is present in the resolved permissions slice.

The principal is stored on the **request context** (not on `echo.Context.Get`):
```go
// Set by middleware:
ctx := WithPrincipal(c.Request().Context(), principal)
// Read by handlers and services:
p, ok := auth.PrincipalFromContext(c.Request().Context())
```

---

## Authorization Helpers

These live in the `auth` package so every module enforces ownership identically instead of re-implementing the checks. The chats module uses them; models/simulations should adopt the same pattern.

**`RequirePrincipal(ctx) (*UserPrincipal, error)`** — the service-layer accessor. Returns `ErrNoPrincipal` when the context carries no principal (a wiring bug, or a detached background context). Use it at the top of any owner-scoped method instead of calling `PrincipalFromContext` and minting a local "no principal" error:

```go
p, err := auth.RequirePrincipal(ctx)
if err != nil {
    return nil, err // -> 401 via the module's error mapper
}
```

**`(*UserPrincipal).CanAccess(ownerID string) bool`** — `true` for the resource owner (`ownerID == p.ID`) or any admin. **`(*UserPrincipal).Authorize(ownerID string) error`** — the error-returning form, yielding `ErrForbidden` when access is denied:

```go
if err := p.Authorize(chat.CreatedBy); err != nil {
    return err // -> 403 via the module's error mapper
}
```

**Sentinels and HTTP mapping.** Each module's error mapper translates these to the shared `httpapi.APIError` envelope:

| Sentinel | HTTP | Code | Meaning |
|---|---|---|---|
| `auth.ErrNoPrincipal` | 401 | `unauthorized` | No authenticated principal in context. |
| `auth.ErrForbidden` | 403 | `forbidden` | Authenticated, but neither owner nor admin. |

**404 vs 403 — an explicit choice.** Returning `ErrForbidden` (403) for a resource owned by someone else *discloses that the resource exists*. The chats module accepts that disclosure: a foreign chat returns **403**, while a genuinely missing or malformed id returns **404** (`ErrChatNotFound`). A module that must hide existence instead should map the foreign-owner case to its own not-found sentinel rather than returning `ErrForbidden`.

---

## Admin Mode

Admin mode is a master switch. When `AUTH_ADMIN_ENABLED=false` (the default), `IsAdmin` is always `false` for every token, regardless of permissions. This means the feature can be enabled only in controlled environments without any risk of accidental privilege escalation in production before the RBAC policy is configured.

When `AUTH_ADMIN_ENABLED=true`, a principal is admin if and only if `AUTH_ADMIN_PERMISSION` appears in their decoded permissions array.

Admin effects in the chats module:
- `GET /chats` returns **all** users' chats (not just the caller's).
- `GET /chats/:id`, `PUT /chats/:id`, `DELETE /chats/:id`, `POST /chats/:id/messages`, and `GET /chats/:id/events` succeed for chats owned by other users.
- The 403 a non-admin receives on a foreign chat does not apply to admins — every owned-or-foreign chat is accessible.

---

## Protected Surface

The following route groups are protected by `Middleware.Require()`:

| Group prefix | Routes |
|---|---|
| `/api/v1/intellisim/auth` | `GET /auth/me` |
| `/api/v1/intellisim/llms` | All LLM provider routes |
| `/api/v1/intellisim/chats` | All chat routes |

`GET /api/v1/intellisim/status` is public (no auth required). Per-API permission scoping (beyond the admin flag) is not implemented yet — all authenticated users have equal access to all protected routes. This is planned as future work once the permission model is defined.

---

## Environment Reference

| Env var | Required | Default | Description |
|---------|----------|---------|-------------|
| `AUTH_SERVER` | yes | — | OIDC issuer URL (e.g. `http://localhost:9090` for the dev mock, or a Keycloak realm URL like `https://keycloak.example.com/realms/intellisim`). Used for discovery + JWKS. |
| `AUTH_AUDIENCE` | no | `""` | When non-empty, tokens whose `aud` claim does not include this value are rejected. Leave empty to skip audience validation (convenient for dev). |
| `AUTH_ADMIN_ENABLED` | no | `false` | Master switch for admin privilege. Set to `true` only when the permission policy is ready. |
| `AUTH_ADMIN_PERMISSION` | no | `intellisim:admin` | The exact permission string that grants admin access. |
| `AUTH_PERMISSIONS_CLAIM` | no | `permissions` | Dot-path into the token claims where the permissions array lives. Use `realm_access.roles` for Keycloak. |

---

## Dev Setup with intellisim.auth.mock

`intellisim.auth.mock` is a Node.js OIDC server (`oauth2-mock-server` + Express/CORS) that runs on port 9090 by default. It publishes a standard OIDC discovery document and JWKS endpoint, issues real signed JWTs, and supports both `authorization_code` and `refresh_token` grants — no login form, auto-redirect on `/authorize` for frictionless E2E testing.

**Start the mock:**
```powershell
cd intellisim.auth.mock
npm install
npm start           # port 9090, user: dev-user
```

**Core `.env` for dev:**
```env
AUTH_SERVER=http://localhost:9090
AUTH_AUDIENCE=                   # leave empty for dev
AUTH_ADMIN_ENABLED=false
```

**Switch active user:**
```powershell
$env:MOCK_ACTIVE_USER = 'dev-admin'
npm start
```

The two built-in users are:
- `dev-user` — `preferred_username: dev.user`, no permissions.
- `dev-admin` — `preferred_username: dev.admin`, `permissions: ["intellisim:admin"]`.

**Shorten token lifetime for testing silent renew:**
```powershell
$env:MOCK_TOKEN_TTL = '60'
npm start
```

**Get a token for curl/PowerShell scripts:**
```powershell
$body = @{ grant_type = 'client_credentials'; client_id = 'cli'; scope = 'openid' }
$tok = (Invoke-RestMethod -Method Post -Uri http://localhost:9090/token -Body $body).access_token

# Test the me endpoint
Invoke-RestMethod -Uri http://localhost:8080/api/v1/intellisim/auth/me `
  -Headers @{ Authorization = "Bearer $tok" }
```

---

## `GET /auth/me`

Returns the caller's resolved principal:
```json
{
  "id": "dev.user",
  "sub": "dev-user-sub-123",
  "email": "dev@intellisim.local",
  "name": "Dev User",
  "permissions": [],
  "is_admin": false
}
```

The `id` field is the value used for ownership stamping (`created_by`). The frontend calls this endpoint on login to populate the user avatar and determine admin display.

---

## Keycloak Migration Notes

Migrating from the dev mock to Keycloak requires only `.env` changes in `intellisim.core`:

1. Set `AUTH_SERVER` to the Keycloak realm URL:
   ```
   AUTH_SERVER=https://keycloak.example.com/realms/intellisim
   ```

2. Set `AUTH_AUDIENCE` to the Keycloak client ID registered for the backend API:
   ```
   AUTH_AUDIENCE=intellisim-api
   ```

3. Switch the permissions claim path to Keycloak's default location for realm roles:
   ```
   AUTH_PERMISSIONS_CLAIM=realm_access.roles
   ```
   If you use client-scoped roles instead, the path is `resource_access.<client-id>.roles`.

4. Ensure the Keycloak realm is configured to include `preferred_username` in access tokens (it is on by default). The `ID` field in `UserPrincipal` will then be the Keycloak username.

5. Update the frontend's `VITE_AUTH_AUTHORITY` to the same realm URL and `VITE_AUTH_CLIENT_ID` to the frontend-facing Keycloak client.

No code changes are needed in `modules/auth`. The OIDCService reads the new JWKS from the new issuer automatically via discovery.
