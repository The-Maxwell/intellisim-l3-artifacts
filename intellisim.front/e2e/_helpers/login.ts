import { type Page, expect } from '@playwright/test'

// login drives the dev OIDC redirect flow. The dev auth-mock (localhost:9090)
// auto-approves the authorization request, so clicking "Sign in" round-trips back
// to the app already authenticated. If the session is already present (warm
// context) the Sign in button is absent and we just wait for the app shell.
export async function login(page: Page): Promise<void> {
  await page.goto('/simulations')
  const signIn = page.getByRole('button', { name: /sign in/i })
  if (await signIn.isVisible().catch(() => false)) {
    await Promise.all([
      page.waitForURL(/\/(simulations|models|chat)/, { timeout: 60_000 }),
      signIn.click(),
    ])
  }
  // App shell is up once the sidebar "New Model" action is rendered.
  await expect(page.getByText('New Model', { exact: false }).first()).toBeVisible({ timeout: 30_000 })
}

// getToken returns the bearer access token oidc-client-ts persisted in localStorage,
// so page.request calls can hit the API as the logged-in user.
export async function getToken(page: Page): Promise<string> {
  const token = await page.evaluate(() => {
    const k = Object.keys(localStorage).find((x) => x.startsWith('oidc.user:'))
    return k ? (JSON.parse(localStorage.getItem(k) as string).access_token as string) : null
  })
  if (!token) throw new Error('no access token in localStorage after login')
  return token
}
