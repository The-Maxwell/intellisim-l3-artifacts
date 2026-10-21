import { test, expect } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// Regression coverage for archetype LLM-result application modes (identical |
// distribution). The full distribution sampling behaviour is proven by the Go unit
// tests (modules/simulations); here we verify the two cross-stack contracts:
//   1. backend — a newly created model defaults its execution strategy to
//      `distribution` (StampDefaultArchetypeMode at create time);
//   2. frontend — the model editor renders the new `archetype_mode` /
//      `variation_strength` controls bound to the persisted value.
// (A live distribution run + per-agent divergence is exercised manually / via the
// engine tests; it is intentionally excluded here to keep the spec fast and free of
// LLM-dependent flakiness within the 30s default timeout.)
test.describe('Archetype result application', () => {
  test('new model defaults to distribution and the editor renders the controls', async ({ page }) => {
    test.setTimeout(120_000)

    await login(page)
    const token = await getToken(page)

    // 1. Backend default: POST /models stamps distribution on the execution default strategy.
    const res = await page.request.post('/api/v1/intellisim/models', {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        name: 'E2E ArchDist Spec',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      },
    })
    expect(res.ok(), `create model failed: ${res.status()}`).toBeTruthy()
    const created = await res.json()
    const mode =
      created.model.simulation_config.llm_simulation_execution_config.default_strategy.archetype_mode
    expect(mode, 'new model should default archetype_mode to distribution').toBe('distribution')

    // 2. Frontend controls: the editor shows the archetype-mode select (= distribution)
    //    and the variation-strength input. Since the tabbed-editor refactor they
    //    live in the lazy "LLM" tab (execution section), so open it first.
    await page.goto(`/models/${created.id}`)
    await page.getByRole('tab', { name: /llm/i }).click()
    await expect(page.locator('[data-testid="archetype-mode"]')).toContainText('distribution', {
      timeout: 20_000,
    })
    await expect(page.locator('[data-testid="variation-strength"]')).toBeAttached()
  })
})
