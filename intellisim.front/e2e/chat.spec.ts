import { test, expect } from '@playwright/test'
import { login } from './_helpers/login'

// Legacy chat smoke, repaired 2026-07-12 (INF-06). The original asserted a UI
// that no longer exists ("New Model Chat" sidebar button, "Model Builder"
// view, Quick Chat launcher on `/`) and never logged in. Today's reality:
// `/` redirects to /chat/new, the sidebar offers "New Model" (→ /models) and
// "New General Chat" (→ /chat/new), and the Quick Chat launcher is hidden on
// chat pages by design. Deep chat coverage lives in e2e/suites/chats.spec.ts.
test.describe('IntelliSim Chat (shell smoke)', () => {
  test('shows welcome screen on /chat/new', async ({ page }) => {
    await login(page)
    await page.goto('/chat/new')
    await expect(page.getByText('IntelliSim Chat')).toBeVisible()
    await expect(page.getByText('Ask questions about agent-based modeling')).toBeVisible()
  })

  test('root path redirects to /chat/new', async ({ page }) => {
    await login(page)
    await page.goto('/')
    await page.waitForURL('**/chat/new')
    await expect(page.getByText('IntelliSim Chat')).toBeVisible()
  })

  test('sidebar shows navigation buttons and New General Chat navigates', async ({ page }) => {
    await login(page) // lands on /simulations
    await expect(page.getByRole('button', { name: 'New Model' })).toBeVisible()
    const newChat = page.getByRole('button', { name: 'New General Chat' })
    await expect(newChat).toBeVisible()
    await newChat.click()
    await page.waitForURL('**/chat/new')
  })

  test('Quick Chat launcher shows off chat pages and hides on them', async ({ page }) => {
    await login(page) // /simulations — launcher visible here
    const launcher = page.locator('button[title="Quick Chat"]')
    await expect(launcher).toBeVisible()
    await page.getByRole('button', { name: 'New General Chat' }).click()
    await page.waitForURL('**/chat/new')
    await expect(launcher).toHaveCount(0)
  })
})
