# intellisim.auth.mock

Dev-only OIDC/OAuth2 provider for the IntelliSim workspace, wrapping
[oauth2-mock-server](https://github.com/axa-group/oauth2-mock-server).
Serves discovery (`/.well-known/openid-configuration`), JWKS, `/authorize`
(auto-redirects — no login form, by design, for fast deterministic dev/e2e),
`/token` (authorization_code + refresh_token grants) and `/userinfo`.

Production target is Keycloak; the core backend only knows an issuer URL
(`AUTH_SERVER`), so swapping later is config-only.

## Run

    npm install
    npm start                       # http://localhost:9090, user: dev-user

## Env

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `9090` | Listen port; issuer = `http://localhost:<PORT>` |
| `MOCK_ACTIVE_USER` | `dev-user` | Key into `users.json`; the user whose claims get signed into tokens |
| `MOCK_TOKEN_TTL` | `3600` | Access-token lifetime (seconds). Use `60` to exercise frontend silent renew |

## Users (users.json)

- `dev-user` — preferred_username `dev.user`, no permissions
- `dev-admin` — preferred_username `dev.admin`, permissions `["intellisim:admin"]`

Switching users = restart with `MOCK_ACTIVE_USER=dev-admin`.

## Testing admin mode end-to-end

1. `$env:MOCK_ACTIVE_USER='dev-admin'; npm start`
2. Core `.env`: `AUTH_ADMIN_ENABLED=true` (and restart core)
3. Log in on the frontend — `GET /auth/me` returns `"is_admin": true`; `GET /chats` lists all users' chats.

> **Note:** `$env:MOCK_ACTIVE_USER='dev-admin'` persists for the entire PowerShell session.
> Clear it when done: `Remove-Item Env:\MOCK_ACTIVE_USER`

## Getting a token from a script (for curl)

> **Note:** `client_id` is accepted without validation — any string works here (unlike the future Keycloak setup which enforces registered clients).

    $body = @{ grant_type = 'client_credentials'; client_id = 'cli'; scope = 'openid' }
    $tok = (Invoke-RestMethod -Method Post -Uri http://localhost:9090/token -Body $body).access_token

## License

Copyright (c) 2026 Vladyslav Pryshchepa.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE) — free to use, modify and
share for **noncommercial purposes** such as scientific research, academic study and teaching.

**Commercial use requires a separate license.** Contact vlad.prishhepa@gmail.com.
