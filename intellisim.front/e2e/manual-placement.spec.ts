import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for Change 2 (2026-07-05):
//   2a — the Placement "Prompt context" box shows ONLY for placement type = llm
//        (not for count_mode = llm); manual placement shows a note, no coords.
//   2b — "manual" placement means place-before-run: the generator emits the
//        instances unplaced, the run is blocked, a tray lets the user drag each
//        into the world, and once placed + saved the run is unblocked.

let token = ''
async function api(page: Page, method: 'get' | 'post' | 'put', url: string, data?: unknown): Promise<APIResponse> {
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

// Dispatch a full HTML5 drag-and-drop sequence (dragstart → dragover → drop →
// dragend) with a shared DataTransfer. Playwright's mouse-based dragTo does not
// reliably trigger native DnD, so we synthesize the events on the real elements.
async function dropFirstChip(page: Page, clientX: number, clientY: number): Promise<void> {
  await page.evaluate(
    ({ clientX, clientY }) => {
      const chip = document.querySelector('[data-testid="unplaced-chip"]') as HTMLElement
      const canvas = document.querySelector('[data-testid="world-canvas"]') as HTMLElement
      const dt = new DataTransfer()
      const opts = (extra: Record<string, unknown> = {}) =>
        ({ bubbles: true, cancelable: true, dataTransfer: dt, ...extra }) as DragEventInit
      chip.dispatchEvent(new DragEvent('dragstart', opts()))
      canvas.dispatchEvent(new DragEvent('dragover', opts({ clientX, clientY })))
      canvas.dispatchEvent(new DragEvent('drop', opts({ clientX, clientY })))
      chip.dispatchEvent(new DragEvent('dragend', opts()))
    },
    { clientX, clientY }
  )
}

test.describe.serial('Manual placement — place before run', () => {
  // ---- 2a: model-editor Placement UI ----
  test('AgentForm: manual shows a note; Prompt context only for placement=llm', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Placement UI',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()

    await page.goto(`/models/${created.id}`)
    await page.getByRole('tab', { name: /agents/i }).click()
    await page.getByRole('button', { name: /add agent/i }).click()
    const uid = page.getByPlaceholder('wolf')
    await expect(uid).toBeVisible({ timeout: 20_000 })
    await uid.fill('critter')
    await page.getByText('Initialization', { exact: true }).click()

    // Placement = manual → note visible, no coordinate inputs.
    // The Placement Select lives inside the bordered Placement box.
    const placement = page.locator('.rounded-md.border', { hasText: 'Placement' }).getByRole('combobox').first()
    await placement.click()
    await page.getByRole('option', { name: 'manual', exact: true }).click()
    await expect(page.getByTestId('manual-placement-note')).toBeVisible()

    // Placement = llm → Prompt context appears.
    await placement.click()
    await page.getByRole('option', { name: 'llm', exact: true }).click()
    await expect(page.getByText('Prompt context', { exact: true })).toBeVisible()

    // Placement = random + count_mode = llm → Prompt context must NOT appear.
    await placement.click()
    await page.getByRole('option', { name: 'random', exact: true }).click()
    await page.getByTestId('init-count-mode').click()
    await page.getByRole('option', { name: 'llm', exact: true }).click()
    await expect(page.getByText('Prompt context', { exact: true })).toHaveCount(0)
  })

  // ---- 2b: full-stack place-before-run ----
  test('manual agents generate unplaced, block the run, and are placed via the tray', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    // Seed a small spatial, rules-only (no-LLM) model: a manual-placement agent
    // (count 3) + a random-placement agent (count 2).
    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Manual Placement',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const modelId: string = created.id
    const model = created.model
    model.is_spatial = true
    model.environment = { uid: 'environment', width: 80, height: 60, torus: false, shapes: [], global_properties: [] }
    model.agents = [
      {
        uid: 'hand', name: 'Hand', type: 'agent', move_type: 'movable',
        properties: [], states: ['idle'],
        initialization: { population_count: 3, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'manual' } },
      },
      {
        uid: 'auto', name: 'Auto', type: 'agent', move_type: 'movable',
        properties: [], states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
    ]
    // Rules-only execution so the run needs no LLM provider.
    model.simulation_config.llm_simulation_execution_config.default_strategy.type = 'none'
    const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
    expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

    // Create a simulation and wait for it to be generated.
    const sim = await (
      await api(page, 'post', '/api/v1/intellisim/simulations', {
        model_id: modelId, name: 'E2E manual run', steps: 3, seed: 7,
      })
    ).json()
    const simId: string = sim.id
    const ready = await waitForStatus(page, simId, ['ready'], 60_000)

    // Backend contract: manual agents come back unplaced; random agents placed.
    const hands = ready.agents.filter((a: any) => a.agent_uid === 'hand')
    const autos = ready.agents.filter((a: any) => a.agent_uid === 'auto')
    expect(hands).toHaveLength(3)
    expect(autos).toHaveLength(2)
    expect(hands.every((a: any) => !a.position), 'manual agents must be unplaced').toBeTruthy()
    expect(autos.every((a: any) => !!a.position), 'random agents must be placed').toBeTruthy()

    // The simulation must not be start-ready (unplaced agents block the run).
    expect(ready.validation?.start_ready, 'unplaced agents must block start').toBeFalsy()

    // The run gate rejects a run while agents are unplaced.
    const blocked = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(blocked.status(), 'run must be blocked while unplaced').not.toBe(202)

    // Open the editor → banner + disabled Run.
    await page.goto(`/simulations/${simId}`)
    await expect(page.getByTestId('unplaced-banner')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('unplaced-banner')).toContainText('3 agent(s)')
    const runBtn = page.getByRole('button', { name: 'Run', exact: true })
    await expect(runBtn).toBeDisabled()

    // Go to the Environment tab and place all three via the tray.
    await page.getByRole('button', { name: /place agents/i }).click()
    await expect(page.getByTestId('world-canvas')).toBeVisible({ timeout: 20_000 })
    const chips = page.getByTestId('unplaced-chip')
    await expect(chips).toHaveCount(3)

    const box = await page.getByTestId('world-canvas').boundingBox()
    if (!box) throw new Error('no canvas box')
    for (let i = 0; i < 3; i++) {
      const remaining = 3 - i
      await expect(chips).toHaveCount(remaining)
      await dropFirstChip(page, box.x + box.width * (0.3 + i * 0.15), box.y + box.height * 0.5)
      await expect(chips).toHaveCount(remaining - 1)
    }
    await expect(page.getByTestId('unplaced-tray')).toHaveCount(0)

    // The editor banner clears once all are placed (still needs a Save to run).
    await expect(page.getByTestId('unplaced-banner')).toHaveCount(0)

    // Save → backend revalidates → start-ready → Run enabled.
    const [saveResp] = await Promise.all([
      page.waitForResponse(
        (r) => /\/api\/v1\/intellisim\/simulations\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'
      ),
      page.getByRole('button', { name: 'Save', exact: true }).click(),
    ])
    expect(saveResp.ok(), `save failed: ${saveResp.status()}`).toBeTruthy()
    const saved = await saveResp.json()
    expect(saved.validation?.start_ready, 'placed + saved sim must be start-ready').toBeTruthy()
    expect(saved.agents.every((a: any) => !!a.position), 'all agents placed after save').toBeTruthy()
    await expect(runBtn).toBeEnabled({ timeout: 10_000 })

    // A short rules-only run now succeeds and every agent keeps a position.
    const run = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
    const done = await waitForStatus(page, simId, ['finished'], 60_000)
    expect(done.agents.every((a: any) => !!a.position), 'agents keep positions through the run').toBeTruthy()
  })
})
