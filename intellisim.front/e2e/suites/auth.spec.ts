import { test, expect, request } from '@playwright/test'
import { login } from '../_helpers/login'
import { apiContext, apiJson, u } from '../_helpers/api'

// AUTH suite — §5.1 of the e2e comprehensive test plan.
// Every test gets a fresh browser context (Playwright default), so each starts
// COLD (no stored OIDC session). Tests needing a warm session call login(page).
//
// SPEC MISMATCH vs the plan: the plan writes the login redirect as
// `?redirect=%2Fmodels`, but vue-router leaves `/` unencoded in query values
// (legal per RFC 3986), so the real URL is `/login?redirect=/models`. We assert
// the DECODED redirect value instead (accepts either encoding).

// Wait for /login and assert the decoded redirect query equals `path`.
async function expectLoginRedirect(page: import('@playwright/test').Page, path: string): Promise<void> {
  await page.waitForURL((url) => url.pathname === '/login' && url.searchParams.get('redirect') === path)
}

test.describe('AUTH — authentication, shell & guards', () => {
  test('AUTH-01 cold visit to /models redirects to login with redirect query', async ({ page }) => {
    await page.goto('/models')
    await expectLoginRedirect(page, '/models')
    expect(new URL(page.url()).searchParams.get('redirect')).toBe('/models')
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()
  })

  test('AUTH-02 sign in lands back on the requested route with the app shell', async ({ page }) => {
    await page.goto('/models')
    await expectLoginRedirect(page, '/models')
    const signIn = page.getByRole('button', { name: /sign in/i })
    await expect(signIn).toBeVisible()
    await Promise.all([
      page.waitForURL((url) => url.pathname === '/models', { timeout: 60_000 }),
      signIn.click(),
    ])
    // Sidebar app shell rendered (scoped to the <aside> — the models list page
    // has its own "New model" button that would trip strict mode otherwise)
    const sidebar = page.locator('aside')
    await expect(sidebar.getByRole('button', { name: 'New Model' })).toBeVisible({ timeout: 30_000 })
    await expect(sidebar.getByRole('link', { name: 'Simulations' })).toBeVisible()
  })

  test('AUTH-03 avatar popover shows dev persona without Admin tag; /auth/me is_admin:false', async ({ page }) => {
    await login(page)
    // Sidebar footer avatar button carries the display name
    await page.getByRole('button', { name: 'Dev User' }).click()
    const popover = page.locator('.p-popover')
    await expect(popover).toBeVisible()
    await expect(popover.getByText('Dev User')).toBeVisible()
    await expect(popover.getByText('dev@intellisim.local')).toBeVisible()
    await expect(popover.getByText('Admin', { exact: true })).toHaveCount(0)
    await expect(popover.getByRole('button', { name: 'Log out' })).toBeVisible()

    // [API] authoritative backend profile
    const api = await apiContext()
    try {
      const me = await apiJson<{ is_admin: boolean; email: string; name: string }>(api, 'get', '/auth/me', undefined, 200)
      expect(me.is_admin).toBe(false)
      expect(me.email).toBe('dev@intellisim.local')
      expect(me.name).toBe('Dev User')
    } finally {
      await api.dispose()
    }
  })

  test('AUTH-04 log out returns to login; protected routes redirect again', async ({ page }) => {
    await login(page)
    await page.getByRole('button', { name: 'Dev User' }).click()
    const popover = page.locator('.p-popover')
    await expect(popover).toBeVisible()
    await popover.getByRole('button', { name: 'Log out' }).click()
    // signoutRedirect round-trips through the mock's end_session back to /login
    await page.waitForURL(/\/login/, { timeout: 30_000 })
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()

    // Session is gone: a protected route redirects to login again
    await page.goto('/simulations')
    await expectLoginRedirect(page, '/simulations')
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()
  })

  test('AUTH-05 reload keeps the warm session (no sign-in prompt)', async ({ page }) => {
    await login(page)
    await page.reload()
    // App shell loads directly from the stored session
    await expect(page.locator('aside').getByRole('button', { name: 'New Model' })).toBeVisible({ timeout: 30_000 })
    await expect(page.getByRole('button', { name: /sign in/i })).toHaveCount(0)
    expect(new URL(page.url()).pathname).not.toBe('/login')
  })

  test('AUTH-06 direct /auth/callback visit fails gracefully with a working back-link', async ({ page }) => {
    await page.goto('/auth/callback')
    await expect(page.getByText(/sign-in failed/i)).toBeVisible({ timeout: 15_000 })
    const back = page.getByRole('link', { name: /back to login/i })
    await expect(back).toBeVisible()
    await back.click()
    await page.waitForURL(/\/login/)
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible()
  })

  test('AUTH-07 sidebar collapse/expand and navigation links work', async ({ page }) => {
    await login(page)

    // Collapse → expanded-only controls disappear, expand toggle appears
    await page.getByTitle('Collapse sidebar').click()
    await expect(page.getByTitle('Expand sidebar')).toBeVisible()
    await expect(page.getByRole('link', { name: 'Models' })).toHaveCount(0)
    // Expand back
    await page.getByTitle('Expand sidebar').click()
    await expect(page.getByTitle('Collapse sidebar')).toBeVisible()

    // Nav links
    await page.getByRole('link', { name: 'Models' }).click()
    await page.waitForURL((url) => url.pathname === '/models')
    await page.getByRole('link', { name: 'Simulations' }).click()
    await page.waitForURL((url) => url.pathname === '/simulations')
    await page.getByRole('button', { name: 'New General Chat' }).click()
    await page.waitForURL((url) => url.pathname === '/chat/new')
  })

  test('AUTH-08 quick-chat launcher visible on /models, hidden on /chat/new', async ({ page }) => {
    await login(page)
    await page.goto('/models')
    const launcher = page.locator('button[title="Quick Chat"]')
    await expect(launcher).toBeVisible()
    await page.goto('/chat/new')
    await expect(launcher).toHaveCount(0)
  })

  test('AUTH-09 [API] unauthenticated request gets a 401 JSON error envelope', async () => {
    const anon = await request.newContext()
    try {
      const res = await anon.get(u('/models'))
      expect(res.status()).toBe(401)
      expect(res.headers()['content-type']).toContain('application/json')
      const body = await res.json()
      expect(body.code).toBe('unauthorized')
      expect(typeof body.message).toBe('string')
    } finally {
      await anon.dispose()
    }
  })
})
