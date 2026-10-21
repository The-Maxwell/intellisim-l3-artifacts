import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the editor auto-save + generate-environment clarity change:
//  - editing then switching tabs auto-saves (model & simulation editors)
//  - toggling Spatial then opening the Environment tab persists is_spatial,
//    which is what used to make "Generate environment" 409 ("not spatial")
//  - a rejected auto-save (422) still switches the tab, shows the inline banner,
//    and keeps the edits "unsaved" (never traps the user)
//  - leaving the editor with unsaveable edits confirms before discarding
//  - the manual Save button still confirms with a toast
//  - a small rules-only run still succeeds through the save path

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

const PUT_MODEL = (r: any) =>
  /\/api\/v1\/intellisim\/models\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'
const PUT_SIM = (r: any) =>
  /\/api\/v1\/intellisim\/simulations\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'

test.describe.serial('Editor auto-save + generate-environment clarity', () => {
  // ---- 1: model editor — editing a field then switching tabs auto-saves ----
  test('model editor auto-saves on tab switch; manual Save toasts', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E AutoSave Model',
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const modelId: string = created.id

    await page.goto(`/models/${modelId}`)
    await expect(page.getByRole('tab', { name: /general/i })).toBeVisible({ timeout: 20_000 })

    // Edit the name → the editor goes dirty ("unsaved").
    const newName = 'Renamed via autosave'
    await page.getByPlaceholder('Model name').fill(newName)
    await expect(page.getByText('unsaved')).toBeVisible()

    // Switch tabs → auto-save fires a PUT; "unsaved" clears without a manual Save.
    const [put] = await Promise.all([
      page.waitForResponse(PUT_MODEL),
      page.getByRole('tab', { name: /llm/i }).click(),
    ])
    expect(put.ok(), `auto-save PUT failed: ${put.status()}`).toBeTruthy()
    await expect(page.getByText('unsaved')).toHaveCount(0)

    // Reload → the rename persisted.
    await page.reload()
    await expect(page.getByPlaceholder('Model name')).toHaveValue(newName, { timeout: 20_000 })

    // Manual Save still shows its success toast (edit again, click Save).
    await page.getByPlaceholder('Model name').fill(newName + '!')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(page.getByText('Model saved')).toBeVisible({ timeout: 10_000 })
  })

  // ---- 2: toggling Spatial then opening Environment tab persists is_spatial ----
  // This is the root-cause fix for the confusing "environment operations require
  // a spatial model" 409: the flag is now persisted before the user can generate.
  test('toggling Spatial then opening Environment auto-saves is_spatial', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Spatial Persist',
        is_spatial: false,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const modelId: string = created.id
    expect(created.model.is_spatial).toBeFalsy()

    await page.goto(`/models/${modelId}`)
    await expect(page.getByRole('tab', { name: /general/i })).toBeVisible({ timeout: 20_000 })

    // Toggle "Spatial model" ON (dirty, but not yet persisted).
    await page.getByText('Spatial model', { exact: true }).click()
    await expect(page.getByText('unsaved')).toBeVisible()

    // Open the Environment tab → auto-save persists is_spatial.
    const [put] = await Promise.all([
      page.waitForResponse(PUT_MODEL),
      page.getByRole('tab', { name: /environment/i }).click(),
    ])
    expect(put.ok()).toBeTruthy()
    await expect(page.getByText('unsaved')).toHaveCount(0)

    // The persisted model is now spatial — the generate-environment gate is satisfied.
    const after = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
    expect(after.model.is_spatial, 'is_spatial must be persisted by the tab-switch auto-save').toBeTruthy()

    // The image→environment panel (spatial-only) is now visible.
    await expect(page.getByTestId('generate-environment')).toBeVisible({ timeout: 10_000 })
  })

  // ---- 3: a rejected auto-save (422) still switches tab, banners, stays unsaved ----
  test('rejected auto-save keeps edits and switches tab (non-blocking)', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Reject AutoSave',
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const modelId: string = created.id

    await page.goto(`/models/${modelId}`)
    await expect(page.getByRole('tab', { name: /general/i })).toBeVisible({ timeout: 20_000 })

    // Force every model PUT to fail hard-validation, deterministically.
    await page.route('**/api/v1/intellisim/models/*', async (route) => {
      if (route.request().method() === 'PUT') {
        await route.fulfill({
          status: 422,
          contentType: 'application/json',
          body: JSON.stringify({
            code: 'validation_failed',
            message: 'Validation failed',
            fields: { 'agents[0].uid': 'agent uid is required' },
          }),
        })
      } else {
        await route.continue()
      }
    })

    await page.getByPlaceholder('Model name').fill('will not save')
    await expect(page.getByText('unsaved')).toBeVisible()

    // Switch tab → the switch is ALLOWED, the banner shows the reason, edits stay.
    await page.getByRole('tab', { name: /agents/i }).click()
    await expect(page.getByRole('tab', { name: /agents/i })).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByText('Save failed — validation:')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('agent uid is required')).toBeVisible()
    await expect(page.getByText('unsaved')).toBeVisible()

    await page.unroute('**/api/v1/intellisim/models/*')
  })

  // ---- 4: leaving the editor with an unsaveable edit confirms before discarding ----
  test('route-leave with an unsaveable edit prompts a confirm; Cancel stays put', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Leave Guard',
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const modelId: string = created.id

    await page.goto(`/models/${modelId}`)
    await expect(page.getByRole('tab', { name: /general/i })).toBeVisible({ timeout: 20_000 })

    await page.route('**/api/v1/intellisim/models/*', async (route) => {
      if (route.request().method() === 'PUT') {
        await route.fulfill({
          status: 422,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'validation_failed', message: 'Validation failed', fields: {} }),
        })
      } else {
        await route.continue()
      }
    })

    await page.getByPlaceholder('Model name').fill('dirty and invalid')
    await expect(page.getByText('unsaved')).toBeVisible()

    // Dismiss the confirm → we must STAY on the editor route.
    let sawDialog = false
    page.once('dialog', (d) => {
      sawDialog = true
      void d.dismiss()
    })
    await page.getByRole('button', { name: 'Simulations', exact: true }).click()
    await page.waitForTimeout(500)
    expect(sawDialog, 'a confirm dialog must appear on route-leave with an unsaveable edit').toBeTruthy()
    await expect(page).toHaveURL(new RegExp(`/models/${modelId}`))

    await page.unroute('**/api/v1/intellisim/models/*')
  })

  // ---- 5: simulation editor — tab-switch auto-saves and re-enables Run ----
  test('simulation editor auto-saves on tab switch and re-enables Run', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    // Rules-only spatial model with a random-placement agent → generated sims are
    // start-ready (all agents placed) and runs need no LLM provider.
    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Sim AutoSave',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const modelId: string = created.id
    const model = created.model
    model.is_spatial = true
    model.environment = { uid: 'environment', width: 60, height: 40, torus: false, shapes: [], global_properties: [] }
    model.agents = [
      {
        uid: 'mover', name: 'Mover', type: 'agent', move_type: 'movable',
        properties: [], states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
    ]
    model.simulation_config.llm_simulation_execution_config.default_strategy.type = 'none'
    const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
    expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

    const sim = await (
      await api(page, 'post', '/api/v1/intellisim/simulations', {
        model_id: modelId, name: 'E2E sim autosave', steps: 3, seed: 7,
      })
    ).json()
    const simId: string = sim.id
    const ready = await waitForStatus(page, simId, ['ready'], 90_000)
    expect(ready.validation?.start_ready, 'seeded sim must be start-ready').toBeTruthy()

    await page.goto(`/simulations/${simId}`)
    const runBtn = page.getByRole('button', { name: 'Run', exact: true })
    await expect(runBtn).toBeEnabled({ timeout: 20_000 })

    // Edit the name → dirty → Run disabled (a run executes the persisted sim).
    await page.getByPlaceholder('Untitled simulation').fill('E2E sim autosave edited')
    await expect(page.getByText('unsaved')).toBeVisible()
    await expect(runBtn).toBeDisabled()

    // Switch to Agents → auto-save PUT → dirty clears → Run re-enabled.
    const [simPut] = await Promise.all([
      page.waitForResponse(PUT_SIM),
      page.getByRole('tab', { name: /agents/i }).click(),
    ])
    expect(simPut.ok()).toBeTruthy()
    await expect(page.getByText('unsaved')).toHaveCount(0)
    await expect(runBtn).toBeEnabled({ timeout: 10_000 })

    // Regression: a small rules-only run still completes through the save path.
    const run = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
    const done = await waitForStatus(page, simId, ['finished'], 60_000)
    expect(done.agents.every((a: any) => !!a.position), 'agents keep positions through the run').toBeTruthy()
  })
})
