import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the fallback-policy explanation hint (2026-07-05): the model
// editor's LLM tab → Simulation execution → Simulation config section shows a
// short explanation of what the fallback chain does (retry / rules_only /
// last_decision / skip) and the empty-default behaviour. Frontend-only, static.

let token = ''
async function api(page: Page, method: 'post', url: string, data?: unknown): Promise<APIResponse> {
  return page.request[method](url, {
    headers: { Authorization: `Bearer ${token}` },
    ...(data !== undefined ? { data } : {}),
  })
}

test('fallback policy editor shows an explanation hint', async ({ page }) => {
  test.setTimeout(120_000)
  await login(page)
  token = await getToken(page)

  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Fallback Hint',
      is_spatial: false,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()

  await page.goto(`/models/${created.id}`)
  await page.getByRole('tab', { name: /llm/i }).click()

  // Expand the collapsible "Simulation config" section inside the execution editor.
  const section = page.getByText('Simulation config', { exact: true })
  await expect(section).toBeVisible({ timeout: 20_000 })
  await section.click()

  // The explanation hint + the empty-default note are visible.
  await expect(page.getByText(/What to do when an LLM call fails/)).toBeVisible()
  await expect(page.getByText(/defaults to\s+rules_only/)).toBeVisible()
})
