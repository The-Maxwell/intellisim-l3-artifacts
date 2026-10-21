import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for "duplicating a model / simulation redirects to the new copy".
// The editors already redirected; this verifies the two LIST views now do too:
//  - models list row → Duplicate → lands on /models/<newId> (editor)
//  - simulations list row → Duplicate → lands on /simulations/<newId> (editor)

let token = ''
async function api(
  page: Page,
  method: 'get' | 'post' | 'put',
  url: string,
  data?: unknown
): Promise<APIResponse> {
  return page.request[method](url, {
    headers: { Authorization: `Bearer ${token}` },
    ...(data !== undefined ? { data } : {}),
  })
}

async function waitForStatus(page: Page, simId: string, want: string[], timeoutMs: number): Promise<any> {
  const deadline = Date.now() + timeoutMs
  let last: any
  while (Date.now() < deadline) {
    last = await (await api(page, 'get', `/api/v1/intellisim/simulations/${simId}`)).json()
    if (want.includes(last.status)) return last
    if (['generation_failed', 'failed'].includes(last.status) && !want.includes(last.status)) {
      throw new Error(`simulation entered ${last.status}: ${last.status_text}`)
    }
    await page.waitForTimeout(400)
  }
  throw new Error(`timeout waiting for ${want}; last=${last?.status}`)
}

function idFromUrl(url: string): string {
  return url.replace(/[?#].*$/, '').split('/').pop() ?? ''
}

test.describe.serial('Duplicate → redirect to the copy', () => {
  // ---- models list ----
  test('duplicating from the models list opens the new model editor', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)
    token = await getToken(page)

    const name = `E2E DupRedirect Model ${Date.now()}`
    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const originalId: string = created.id

    await page.goto('/models')
    const row = page.getByRole('row', { name: new RegExp(name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) })
    await expect(row).toBeVisible({ timeout: 20_000 })

    // Row Duplicate (aria-label="Duplicate", @click.stop) → shared dialog.
    await row.getByRole('button', { name: 'Duplicate' }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await dialog.getByRole('button', { name: 'Duplicate', exact: true }).click()

    // Redirect to the NEW model's editor (a different id).
    await page.waitForURL(/\/models\/[^/]+$/, { timeout: 20_000 })
    const newId = idFromUrl(page.url())
    expect(newId, 'redirected to a different model id').not.toBe(originalId)
    await expect(page.getByPlaceholder('Model name')).toBeVisible({ timeout: 20_000 })
    // The copy carries the "(copy)" name the dialog pre-filled.
    await expect(page.getByPlaceholder('Model name')).toHaveValue(`${name} (copy)`)
  })

  // ---- simulations list ----
  test('duplicating from the simulations list opens the new simulation editor', async ({ page }) => {
    test.setTimeout(150_000)
    await login(page)
    token = await getToken(page)

    // Seed a rules-only spatial model + a simulation, waited to `ready`.
    const model = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: `E2E DupRedirect Sim Model ${Date.now()}`,
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const m = model.model
    m.is_spatial = true
    m.environment = { uid: 'environment', width: 40, height: 40, torus: false, shapes: [], global_properties: [] }
    m.agents = [
      {
        uid: 'mover', name: 'Mover', type: 'agent', move_type: 'movable',
        properties: [], states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
    ]
    m.simulation_config.llm_simulation_execution_config.default_strategy.type = 'none'
    const put = await api(page, 'put', `/api/v1/intellisim/models/${model.id}`, { model: m })
    expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

    const simName = `E2E DupRedirect Sim ${Date.now()}`
    const sim = await (
      await api(page, 'post', '/api/v1/intellisim/simulations', {
        model_id: model.id, name: simName, steps: 3, seed: 7,
      })
    ).json()
    const originalId: string = sim.id
    await waitForStatus(page, originalId, ['ready'], 90_000)

    await page.goto('/simulations')
    const row = page.getByRole('row', { name: new RegExp(simName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) })
    await expect(row).toBeVisible({ timeout: 20_000 })

    await row.getByRole('button', { name: 'Duplicate' }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await dialog.getByRole('button', { name: 'Duplicate', exact: true }).click()

    // Redirect to the NEW simulation's editor (a different id).
    await page.waitForURL(/\/simulations\/[^/]+$/, { timeout: 20_000 })
    const newId = idFromUrl(page.url())
    expect(newId, 'redirected to a different simulation id').not.toBe(originalId)
    await expect(page.getByRole('button', { name: 'Run', exact: true })).toBeVisible({ timeout: 20_000 })
  })
})
