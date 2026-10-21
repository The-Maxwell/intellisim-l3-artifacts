import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { User } from 'oidc-client-ts'
import { userManager } from '../authService'

export interface MeResponse {
  id: string
  sub: string
  email: string
  name: string
  permissions: string[]
  is_admin: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  // Authoritative backend profile (GET /auth/me) — source of is_admin.
  const me = ref<MeResponse | null>(null)

  let initPromise: Promise<void> | null = null
  let renewPromise: Promise<User | null> | null = null

  const isAuthenticated = computed(() => !!user.value && !user.value.expired)
  const accessToken = computed(() => user.value?.access_token ?? null)
  const displayName = computed(
    () =>
      me.value?.name ||
      (user.value?.profile?.name as string | undefined) ||
      (user.value?.profile?.preferred_username as string | undefined) ||
      (user.value?.profile?.email as string | undefined) ||
      ''
  )
  const email = computed(
    () => me.value?.email || (user.value?.profile?.email as string | undefined) || ''
  )
  const isAdmin = computed(() => me.value?.is_admin ?? false)

  function init(): Promise<void> {
    if (!initPromise) {
      initPromise = (async () => {
        userManager.events.addUserLoaded((u) => {
          user.value = u
        })
        userManager.events.addUserUnloaded(() => {
          user.value = null
          me.value = null
        })
        user.value = await userManager.getUser()
      })()
    }
    return initPromise
  }

  function login(redirectPath?: string): Promise<void> {
    return userManager.signinRedirect({ state: { redirect: redirectPath || '/' } })
  }

  // Completes the code flow on /auth/callback; returns the post-login path.
  async function handleCallback(): Promise<string> {
    const u = await userManager.signinCallback()
    user.value = u ?? (await userManager.getUser())
    const state = (u?.state ?? {}) as { redirect?: string }
    return state.redirect || '/'
  }

  // Single-flight silent renew shared by router guard, axios and SSE.
  function renew(): Promise<User | null> {
    if (!renewPromise) {
      renewPromise = userManager
        .signinSilent()
        .then((u) => {
          user.value = u
          return u
        })
        .catch(() => null)
        .finally(() => {
          renewPromise = null
        })
    }
    return renewPromise
  }

  // Token for an outgoing request; proactively renews when <30s to expiry.
  async function getValidAccessToken(): Promise<string | null> {
    await init()
    if (!user.value) return null
    if (user.value.expired || (user.value.expires_in !== undefined && user.value.expires_in < 30)) {
      await renew()
    }
    return user.value && !user.value.expired ? user.value.access_token : null
  }

  async function loadMe(): Promise<void> {
    try {
      const { default: apiClient } = await import('@/shared/api/client')
      me.value = (await apiClient.get<MeResponse>('/auth/me')).data
    } catch {
      me.value = null
    }
  }

  async function logout(): Promise<void> {
    me.value = null
    await userManager.signoutRedirect()
  }

  return {
    user,
    me,
    isAuthenticated,
    accessToken,
    displayName,
    email,
    isAdmin,
    init,
    login,
    handleCallback,
    renew,
    getValidAccessToken,
    loadMe,
    logout,
  }
})
