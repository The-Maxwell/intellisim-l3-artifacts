import { test, expect, type Page } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the "conditional agent-initialization fields by Count mode"
// UI change (2026-07-05):
//   - fixed (or unset) shows ONLY Population count;
//   - random_range shows ONLY Min/Max population;
//   - llm shows NONE of the numeric inputs (the LLM decides the count);
//   - switching mode CLEARS the now-hidden fields (not merely hides them), so a
//     round-trip back to a mode shows empty inputs and the saved model carries
//     no stale bounds.
// Data-model + validation are covered by the Go tests; here we prove the
// cross-stack UI contract and the clear-on-switch persistence.

const testId = (page: Page, id: string) => page.getByTestId(id)

// selectCountMode opens the PrimeVue Count-mode Select and picks an option.
async function selectCountMode(page: Page, value: string): Promise<void> {
  await testId(page, 'init-count-mode').click()
  await page.getByRole('option', { name: value, exact: true }).click()
}

// saveAndGetModel clicks Save and returns the model from the PUT response —
// waiting on the actual network round-trip (not the toast, which lingers from a
// prior save and would let a follow-up GET race ahead of the real write).
async function saveAndGetModel(page: Page): Promise<any> {
  const [resp] = await Promise.all([
    page.waitForResponse(
      (r) =>
        /\/api\/v1\/intellisim\/models\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'
    ),
    page.getByRole('button', { name: 'Save', exact: true }).click(),
  ])
  expect(resp.ok(), `save PUT failed: ${resp.status()} ${await resp.text()}`).toBeTruthy()
  return resp.json()
}

test.describe('Agent initialization — conditional fields by Count mode', () => {
  test('shows the right numeric inputs per mode and clears hidden fields on switch', async ({
    page,
  }) => {
    test.setTimeout(120_000)

    await login(page)
    const token = await getToken(page)

    // Bare model to edit.
    const res = await page.request.post('/api/v1/intellisim/models', {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        name: 'E2E CountMode Spec',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      },
    })
    expect(res.ok(), `create model failed: ${res.status()} ${await res.text()}`).toBeTruthy()
    const created = await res.json()
    const modelId: string = created.id

    // Open editor → Agents tab → add one agent → open its Initialization section.
    await page.goto(`/models/${modelId}`)
    await page.getByRole('tab', { name: /agents/i }).click()
    await page.getByRole('button', { name: /add agent/i }).click()

    // Give the agent a uid so the model saves cleanly later.
    const uid = page.getByPlaceholder('wolf')
    await expect(uid).toBeVisible({ timeout: 20_000 })
    await uid.fill('critter')

    // Expand the (collapsed-by-default) Initialization fieldset.
    await page.getByText('Initialization', { exact: true }).click()
    await expect(testId(page, 'init-count-mode')).toBeVisible()

    // 1. Default (unset ⇒ fixed): only Population count is present.
    await expect(testId(page, 'init-population-count')).toBeVisible()
    await expect(testId(page, 'init-min-population')).toHaveCount(0)
    await expect(testId(page, 'init-max-population')).toHaveCount(0)

    // 2. random_range: only Min/Max population; the advisory hint appears.
    await selectCountMode(page, 'random_range')
    await expect(testId(page, 'init-population-count')).toHaveCount(0)
    await expect(testId(page, 'init-min-population')).toBeVisible()
    await expect(testId(page, 'init-max-population')).toBeVisible()
    await expect(page.getByText(/random_range draws the population/i)).toBeVisible()

    // Fill the bounds so we can later prove they get cleared.
    await testId(page, 'init-min-population').locator('input').fill('3')
    await testId(page, 'init-min-population').locator('input').press('Tab')
    await testId(page, 'init-max-population').locator('input').fill('8')
    await testId(page, 'init-max-population').locator('input').press('Tab')

    // 3. Switch to fixed → bounds hidden, Population count back.
    await selectCountMode(page, 'fixed')
    await expect(testId(page, 'init-population-count')).toBeVisible()
    await expect(testId(page, 'init-min-population')).toHaveCount(0)
    await expect(testId(page, 'init-max-population')).toHaveCount(0)

    // 4. Round-trip back to random_range: bounds must be EMPTY (cleared, not hidden).
    await selectCountMode(page, 'random_range')
    await expect(testId(page, 'init-min-population').locator('input')).toHaveValue('')
    await expect(testId(page, 'init-max-population').locator('input')).toHaveValue('')

    // 5. llm: no numeric inputs at all; the llm-count hint appears. (The Prompt
    //    context box belongs to placement=llm only, so it must NOT appear here —
    //    placement is unset in this test.)
    await selectCountMode(page, 'llm')
    await expect(testId(page, 'init-population-count')).toHaveCount(0)
    await expect(testId(page, 'init-min-population')).toHaveCount(0)
    await expect(testId(page, 'init-max-population')).toHaveCount(0)
    await expect(page.getByText(/llm count mode lets the LLM decide/i)).toBeVisible()
    await expect(page.getByText('Prompt context', { exact: true })).toHaveCount(0)

    // 6. Persistence: end on random_range with fresh bounds, Save, and confirm the
    //    saved model carries the range bounds and NO stale fixed count leakage.
    await selectCountMode(page, 'random_range')
    await testId(page, 'init-min-population').locator('input').fill('2')
    await testId(page, 'init-min-population').locator('input').press('Tab')
    await testId(page, 'init-max-population').locator('input').fill('5')
    await testId(page, 'init-max-population').locator('input').press('Tab')

    const saveBtn = page.getByRole('button', { name: 'Save', exact: true })
    await expect(saveBtn).toBeEnabled()
    const body = await saveAndGetModel(page)
    const init = body.model.agents[0].initialization
    expect(init.count_mode).toBe('random_range')
    expect(init.min_population).toBe(2)
    expect(init.max_population).toBe(5)

    // Now flip to fixed via the UI and save → bounds must be cleared server-side.
    await selectCountMode(page, 'fixed')
    await testId(page, 'init-population-count').locator('input').fill('4')
    await testId(page, 'init-population-count').locator('input').press('Tab')
    await expect(saveBtn).toBeEnabled()
    const body2 = await saveAndGetModel(page)
    const init2 = body2.model.agents[0].initialization
    expect(init2.count_mode).toBe('fixed')
    expect(init2.population_count).toBe(4)
    // Cleared bounds must not survive (absent ⇒ undefined, or explicit 0/null).
    expect(init2.min_population ?? 0).toBe(0)
    expect(init2.max_population ?? 0).toBe(0)
  })
})
