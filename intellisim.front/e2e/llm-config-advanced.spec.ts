import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the LLM-config "Advanced" collapse (2026-07-07). The shared
// LlmExecutionConfig picker now shows only model + effort by default and hides
// generation params + the LLM timeout behind a collapsed "Advanced" section
// (PrimeVue toggleable Fieldset). The section auto-opens when the target already
// carries advanced values. Because every extended-config surface renders this one
// component, verifying it in the model editor's LLM tab covers them all.

let token: string
const SHOT_DIR = process.env.E2E_SHOT_DIR || 'test-results'

async function api(page: Page, method: 'get' | 'put', url: string, data?: unknown): Promise<APIResponse> {
  return page.request[method](url, {
    headers: { Authorization: `Bearer ${token}` },
    ...(data !== undefined ? { data } : {}),
  })
}

test('LLM picker hides generation + timeout behind a collapsed Advanced section', async ({ page }) => {
  test.setTimeout(120_000)

  await login(page)
  token = await getToken(page)

  // Create a model, then seed its model-generation LLM target so the picker
  // resolves a real model (Advanced section is gated on a non-empty model_uid).
  const created = await (
    await page.request.post('/api/v1/intellisim/models', {
      headers: { Authorization: `Bearer ${token}` },
      data: { name: 'E2E LLM Advanced', is_spatial: false, llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' } },
    })
  ).json()
  const modelId = created.id

  const model = created.model
  model.llm_config = { ...(model.llm_config ?? {}), target: { provider_uid: 'openai', model_uid: 'gpt-4o' } }
  const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
  expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

  // Open the model editor's LLM tab.
  await page.goto(`/models/${modelId}`)
  await page.getByRole('tab', { name: /llm/i }).click()

  // The model-generation config now has a model, so its "Advanced" section shows —
  // COLLAPSED by default: the timeout input (placeholder 30000) must be hidden.
  const advanced = page.getByText('Advanced', { exact: true }).first()
  await expect(advanced).toBeVisible({ timeout: 20_000 })
  // Collapsed content stays in the DOM (v-show) but must not be visible.
  await expect(page.getByPlaceholder('30000')).not.toBeVisible()
  await page.screenshot({ path: `${SHOT_DIR}/llm-advanced-collapsed.png`, fullPage: false })

  // Expand it → the timeout input appears. Set a value.
  await advanced.click()
  const timeout = page.getByPlaceholder('30000')
  await expect(timeout).toBeVisible({ timeout: 10_000 })
  await timeout.fill('45000')
  await timeout.press('Tab')
  await page.screenshot({ path: `${SHOT_DIR}/llm-advanced-expanded.png`, fullPage: false })

  // Save and confirm the timeout persisted onto the emitted target (regression:
  // model + timeout round-trip through the ExecutionLlmTarget shape).
  await page.getByRole('button', { name: /^save$/i }).first().click()
  await page.waitForTimeout(1_500)
  const after = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
  expect(after.model.llm_config?.target?.timeout_ms, 'timeout must persist on the target').toBe(45000)

  // Reload → because the target now carries an advanced value, the Advanced
  // section must AUTO-OPEN (timeout visible without clicking) and show 45000.
  await page.goto(`/models/${modelId}`)
  await page.getByRole('tab', { name: /llm/i }).click()
  const reopened = page.getByPlaceholder('30000')
  await expect(reopened).toBeVisible({ timeout: 20_000 })
  await expect(reopened).toHaveValue(/45,?000/)
})
