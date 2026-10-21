# Frontend Authentication Reference

IntelliSim's frontend authenticates users via OIDC Authorization Code + PKCE using the `oidc-client-ts` library. In development the auth server is `intellisim.auth.mock` (port 9090). In production the target is Keycloak. The frontend only knows the issuer URL — switching providers is a `.env` change.

---

## oidc-client-ts Setup

**File:** `src/modules/auth/authService.ts`

A single `UserManager` instance is created at module load:

```typescript
export const userManager = new UserManager({
  authority: import.meta.env.VITE_AUTH_AUTHORITY,
  client_id: import.meta.env.VITE_AUTH_CLIENT_ID,
  redirect_uri: `${window.location.origin}/auth/callback`,
  post_logout_redirect_uri: `${window.location.origin}/login`,
  response_type: 'code',
  scope: 'openid profile email',
  userStore: new WebStorageStateStore({ store: window.localStorage }),
  automaticSilentRenew: true,
  accessTokenExpiringNotificationTimeInSeconds: 60,
})
```

Key choices:
- **PKCE** (`response_type: 'code'`) — the only supported grant type. No implicit flow.
- **localStorage store** — the session is shared across browser tabs and survives page reload. The floating popup chat and the main chat view use the same token.
- **`automaticSilentRenew: true`** — `oidc-client-ts` will attempt a silent renew (via the refresh token grant for the dev mock and Keycloak) 60 seconds before access token expiry. This happens automatically without any application code driving it.

### Environment Variables

| Variable | Development default | Description |
|---|---|---|
| `VITE_AUTH_AUTHORITY` | `http://localhost:9090` | OIDC issuer URL. Must match `AUTH_SERVER` in core. |
| `VITE_AUTH_CLIENT_ID` | `intellisim-front` | Registered OIDC client ID. |
| `VITE_API_BASE_URL` | `/api/v1/intellisim` | Backend API prefix used by Axios and SSE. |
| `VITE_WS_BASE_URL` | `ws://localhost:8080/api/v1/intellisim` | WebSocket base for simulation streaming. |

---

## Login Flow and Router Guard

**Files:** `src/app/router.ts`, `src/modules/auth/views/LoginView.vue`, `src/modules/auth/views/AuthCallbackView.vue`

Two routes are public (no auth required): `/login` and `/auth/callback`. Every other route — including the root `/` — is protected by a global `beforeEach` guard.

### Guard Logic

```
1. If the route has `meta.public` → allow immediately.
2. Call authStore.init() to load any stored session from localStorage.
3. If not authenticated:
   a. If a stored (possibly expired) user exists → attempt a silent renew.
   b. If no stored user → redirect to /login immediately (never attempt silent renew).
   c. If silent renew returns null → redirect to /login.
4. If authenticated → allow; fire-and-forget authStore.loadMe() for /auth/me data.
```

The guard **never attempts silent sign-in on a cold visit** (no stored user in localStorage). This is deliberate: the dev mock auto-approves `prompt=none` requests, which would silently log users in and make the login page unreachable on fresh sessions. Cold visits always land on `/login`.

### Login Redirect

The `LoginView` calls `authStore.login(redirectPath)`, which calls `userManager.signinRedirect`. The current path is stored in the OIDC `state` parameter so the callback can restore it:

```
/chat/new → /login → mock /authorize (auto-redirect) → /auth/callback → /chat/new
```

`AuthCallbackView` calls `authStore.handleCallback()`, which completes the code exchange, sets `user.value`, and navigates to the stored redirect path (or `/` as fallback).

---

## Token Lifecycle

### Automatic Silent Renew

`oidc-client-ts` handles the primary renewal loop via `automaticSilentRenew`. It fires `userLoaded` on success and the `authStore` updates `user.value` accordingly. No application code triggers this directly.

### 60-Second Expiry Notification

`accessTokenExpiringNotificationTimeInSeconds: 60` causes `oidc-client-ts` to fire an internal event when the token has 60 seconds left. The `automaticSilentRenew` machinery responds to this event and begins the refresh. The application is not directly involved.

### Proactive Axios Renew (< 30 seconds)

`authStore.getValidAccessToken()` is called by the Axios request interceptor before every API request:

```typescript
if (user.value.expired || user.value.expires_in < 30) {
  await renew()
}
return user.value?.access_token ?? null
```

If the token expires within 30 seconds, a renew is triggered synchronously before the request is sent. This closes the gap between the `oidc-client-ts` 60-second notification and actual expiry, ensuring no request ever uses a token that will expire mid-flight.

### Single-Flight Renew

`authStore.renew()` uses a single-flight pattern:

```typescript
function renew(): Promise<User | null> {
  if (!renewPromise) {
    renewPromew = userManager.signinSilent()
      .then(u => { user.value = u; return u })
      .catch(() => null)
      .finally(() => { renewPromise = null })
  }
  return renewPromise
}
```

Multiple simultaneous callers (Axios, SSE, router guard) all await the same `Promise`. Only one `signinSilent` network request is made regardless of concurrency.

### 401 Retry (Axios)

If an API call returns 401 (e.g. the token expired between the proactive check and the server's validation), the Axios response interceptor performs one silent renew and retries the original request. If the renew also fails, the user is redirected to `/login`:

```
Request → 401 → renew() → retry original → success  (normal path)
                        → renew fails → redirect /login
```

A request is retried at most once (`_retried` flag prevents loops).

### 403 Forbidden (Authorization, not Authentication)

A **403** is distinct from a 401: the token is valid, but the caller is not allowed to touch that resource (e.g. opening another user's chat by URL while not an admin). The interceptor never renews or retries a 403 — renewing the same identity would not change the outcome. It is surfaced as an `ApiError` with `status: 403`, `code: 'forbidden'`.

In the chat UI this is handled distinctly from a deleted chat (404): `useChatSession.open()` catches the 403, sets a `forbidden` flag (no SSE subscription is opened), and the view shows an inline **"Access denied"** state instead of a generic error toast. The floating quick-chat reports it via toast and resets to a fresh chat. Other modules can do the same by inspecting `ApiError.status === 403`.

### SSE Reconnect Token Behavior

The SSE client (`useChatEvents` / `subscribeChatEvents`) uses `@microsoft/fetch-event-source` rather than the browser's native `EventSource`, because `EventSource` cannot send custom headers. On every reconnect attempt, the `fetch` override calls `auth.getValidAccessToken()` to obtain the current token:

```typescript
fetch: async (input, init) => {
  const token = await auth.getValidAccessToken()
  const headers = new Headers(init?.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return window.fetch(input, { ...init, headers })
}
```

This means the SSE stream automatically reconnects with a fresh token after a silent renew, without any special handling.

### Logout

`authStore.logout()` clears `me.value` and calls `userManager.signoutRedirect()`. After the redirect, the browser lands on `post_logout_redirect_uri` (`/login`). `userManager` fires `userUnloaded` which clears `user.value` and `me.value`. Subsequent navigation to any protected route redirects to `/login`.

---

## `/auth/me` and `is_admin`

After login the router guard calls `authStore.loadMe()` (fire-and-forget):

```typescript
const { default: apiClient } = await import('@/shared/api/client')
me.value = (await apiClient.get<MeResponse>('/auth/me')).data
```

The `MeResponse` shape mirrors the backend `UserPrincipal`:
```typescript
interface MeResponse {
  id: string
  sub: string
  email: string
  name: string
  permissions: string[]
  is_admin: boolean
}
```

`isAdmin` is a computed store property derived from `me.value?.is_admin`. The `UserAvatar` component uses `isAdmin` to display the "Admin" tag in the profile popover.

Display name resolution order (first non-empty): `me.name` → `user.profile.name` → `user.profile.preferred_username` → `user.profile.email`.

---

## Troubleshooting

**Guard keeps redirecting to `/login` on every navigation:**
The mock server is probably not running. Without `AUTH_SERVER` returning a valid OIDC discovery document, the backend rejects every token with 401. The Axios interceptor redirects to `/login`. The guard then tries a silent renew, which also fails because the mock is down, and redirects again.
Fix: Start the mock (`npm start` in `intellisim.auth.mock`).

**Silent renew is never triggered (session always expires at full TTL):**
The default `MOCK_TOKEN_TTL` is 3600 seconds. The automatic renew fires 60 seconds before expiry, which is too long to wait during development. Set `MOCK_TOKEN_TTL=60` to see the renewal cycle in about 60 seconds:
```powershell
$env:MOCK_TOKEN_TTL = '60'
npm start
```

**Frontend goes to `/login` instead of auto-renewing on a page refresh:**
This is correct behavior on a cold visit. If there is no stored user in localStorage, the guard does not attempt silent renew (by design — see above). After logging in once, the session is stored and future refreshes will attempt renew automatically.

**`is_admin: false` even though the mock user has the admin permission:**
`AUTH_ADMIN_ENABLED` in core `.env` must be `true`. When it is `false` (the default), the backend always returns `is_admin: false` regardless of the token's permissions.
