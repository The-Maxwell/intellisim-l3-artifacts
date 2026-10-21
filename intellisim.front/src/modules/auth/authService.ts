import { UserManager, WebStorageStateStore } from 'oidc-client-ts'

// Single UserManager for the app: Authorization Code + PKCE against the
// configured OIDC authority (dev: intellisim.auth.mock; later: Keycloak).
// localStorage store → session shared across tabs/popup and survives reload.
// automaticSilentRenew uses the refresh-token grant when the server issues
// refresh tokens (the mock and Keycloak both do in code flow).
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
