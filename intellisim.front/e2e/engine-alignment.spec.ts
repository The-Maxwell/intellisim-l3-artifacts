import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the engine ↔ data-structure alignment batch (2026-07-02):
//   1. a full model with env-scoped global-property rules + a forced interaction
//      persists and validates;
//   2. the rebuilt condition/action editors author the new per-type fields
//      (round-trip through the API);
//   3. a rules-only run actually mutates environment global properties each step
//      (previously runtime-constant), fires forced interactions (previously a
//      no-op action), and serves the live-agent endpoint;
//   4. a paused run reopened in the browser shows the world (join keyframe +
//      history catch-up) instead of an empty canvas.
// call_llm's off-schedule LLM call is proven by the Go unit tests
// (strategy_call_llm_test.go); it is excluded here to keep the spec free of
// LLM-provider flakiness.

let token: string
let modelId: string

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
    const res = await api(page, 'get', `/api/v1/intellisim/simulations/${simId}`)
    expect(res.ok(), `get simulation failed: ${res.status()}`).toBeTruthy()
    last = await res.json()
    if (want.includes(last.status)) return last
    if (['generation_failed', 'failed'].includes(last.status) && !want.includes(last.status)) {
      throw new Error(`simulation entered ${last.status}: ${last.status_text}`)
    }
    await page.waitForTimeout(500)
  }
  throw new Error(`timeout waiting for status ${want}; last=${last?.status}`)
}

// createAlignedSim creates + waits for a ready simulation over the shared model.
async function createAlignedSim(page: Page, name: string, steps: number, seed: number): Promise<string> {
  const res = await api(page, 'post', '/api/v1/intellisim/simulations', {
    model_id: modelId,
    name,
    steps,
    seed,
  })
  expect(res.ok(), `create simulation failed: ${res.status()} ${await res.text()}`).toBeTruthy()
  const sim = await res.json()
  await waitForStatus(page, sim.id, ['ready'], 60_000)
  return sim.id
}

test.describe.serial('Engine-data alignment', () => {
  test('model with env global-property rules + forced interaction persists', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Engine Alignment',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    modelId = created.id

    // Mutate the created model JSON and PUT it back (full replace, {model:{...}}).
    const model = created.model
    model.is_spatial = true
    model.spatial_cell_size = 10
    model.environment = {
      uid: 'environment',
      width: 60,
      height: 40,
      torus: false,
      shapes: [],
      global_properties: [
        { uid: 'temperature', name: 'Temperature', type: 'number', min_value: 0, default_value: 20 },
      ],
    }
    model.agents = [
      {
        uid: 'wolf', name: 'Wolf', type: 'agent', move_type: 'movable',
        properties: [{ uid: 'energy', name: 'Energy', type: 'number', default_value: 50 }],
        states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
      {
        uid: 'sheep', name: 'Sheep', type: 'agent', move_type: 'movable',
        properties: [{ uid: 'energy', name: 'Energy', type: 'number', default_value: 40 }],
        states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
    ]
    model.rules = [
      {
        id: 'env-cool', name: 'Cooldown', enabled: true, priority: 10,
        applies_to: ['environment'],
        conditions: [],
        actions: [{ type: 'modify_property', property_uid: 'temperature', modifier: 'subtract', value: 1 }],
      },
      {
        id: 'attack', name: 'Attack', enabled: true, priority: 5,
        applies_to: ['wolf'],
        conditions: [],
        actions: [{ type: 'interact', target_uid: 'hunt' }],
      },
    ]
    model.interactions = [
      {
        id: 'hunt', name: 'Hunt', type: 'agent_agent', enabled: true,
        initiator_uids: ['wolf'], target_uids: ['sheep'], radius: 100,
        // Conditions that can never pass on their own — a fire proves the FORCED path.
        conditions: [{ type: 'step', operator: 'gt', value: 1e9 }],
        initiator_actions: [{ type: 'modify_property', property_uid: 'energy', modifier: 'add', value: 1 }],
        target_actions: [],
        bidirectional: false, use_llm: false,
      },
    ]

    const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
    expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

    const validation = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}/validation`)).json()
    expect(validation.simulation_ready, `model not simulation-ready: ${JSON.stringify(validation.errors)}`).toBeTruthy()
  })

  test('editor UI authors the new condition/action fields (round-trip)', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)

    await page.goto(`/models/${modelId}`)
    await page.getByRole('tab', { name: /rules/i }).click()

    // Select the wolf "Attack" rule in the master list.
    await page.getByText('Attack', { exact: false }).first().click()

    // Add a condition and switch its type to `random` — the new probability
    // field must appear and persist through Save.
    await page.getByRole('button', { name: /add condition/i }).click()
    // Each condition renders as a rounded card starting with a "Condition type"
    // labeled Select (options are the raw type strings).
    const card = page.locator('div.rounded-lg', { hasText: 'Condition type' }).last()
    await expect(card).toBeVisible({ timeout: 10_000 })
    await card.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'random', exact: true }).click()

    // The probability InputNumber (role=spinbutton) is the new random-type field.
    await expect(card.getByText(/Probability/), 'random condition must render a probability field').toBeVisible({ timeout: 10_000 })
    const probability = card.getByRole('spinbutton').first()
    await probability.fill('0.5')
    await probability.press('Tab')

    // Save via the editor's save action.
    await page.getByRole('button', { name: /^save$/i }).first().click()
    await page.waitForTimeout(1_500)

    const after = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
    const attack = after.model.rules.find((r: any) => r.id === 'attack')
    const rnd = (attack?.conditions ?? []).find((c: any) => c.type === 'random')
    expect(rnd, 'random condition must persist').toBeTruthy()
    expect(Number(rnd.value)).toBeCloseTo(0.5, 5)

    // Restore the always-fire rule (remove the random gate) for the run tests.
    const model = after.model
    model.rules = model.rules.map((r: any) => (r.id === 'attack' ? { ...r, conditions: [] } : r))
    const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
    expect(put.ok()).toBeTruthy()
  })

  test('run mutates globals, fires forced interactions, serves live agents', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    const simId = await createAlignedSim(page, 'E2E alignment run', 12, 5)

    const run = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
    const done = await waitForStatus(page, simId, ['finished'], 120_000)

    // G3 — the env rule decays temperature by 1 per step: 20 - 12 = 8.
    // Since the 2026-07-09 runtime-isolation change the persisted simulation
    // document keeps the INITIAL world; runtime global values live in the step
    // frames — read the final frame (steps window needs an explicit `to`).
    const stepsRes = await api(page, 'get', `/api/v1/intellisim/simulations/${simId}/steps?from=0&to=1000000&page=1&page_size=100000`)
    expect(stepsRes.ok(), `get steps failed: ${stepsRes.status()}`).toBeTruthy()
    const frames = (await stepsRes.json()).items ?? []
    expect(frames.length, 'run must persist step frames').toBeGreaterThan(0)
    const lastGlobals = frames[frames.length - 1].global_properties ?? {}
    expect(lastGlobals.temperature, 'global property must decay at runtime').toBe(8)
    expect(done.environment.global_properties.temperature, 'persisted sim doc keeps the INITIAL world').toBe(20)

    // G2 — forced interactions fired despite impossible interaction conditions:
    // 2 wolves × 12 steps = 24 forced fires.
    const hunt = (done.statistics.by_interaction ?? []).find((x: any) => x.interaction_id === 'hunt')
    expect(hunt?.count, 'forced interaction must fire and be counted').toBe(24)

    // Env rule counted in rule stats.
    const envRule = (done.statistics.by_rule ?? []).find((x: any) => x.rule_id === 'env-cool')
    expect(envRule?.executions).toBe(12)

    // B9 — live-agent endpoint (post-run: persisted fallback) returns memory.
    const firstAgent = done.agents[0]
    const agentRes = await api(page, 'get', `/api/v1/intellisim/simulations/${simId}/agents/${firstAgent.id}`)
    expect(agentRes.ok(), `get agent failed: ${agentRes.status()}`).toBeTruthy()
    const agent = await agentRes.json()
    expect(agent.id).toBe(firstAgent.id)
    expect((agent.memory ?? []).length, 'agent memory must be populated').toBeGreaterThan(0)

    // 404 contract for unknown ids.
    const missing = await api(page, 'get', `/api/v1/intellisim/simulations/${simId}/agents/99999`)
    expect(missing.status()).toBe(404)
  })

  test('paused run reopened in the browser shows the world', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    const simId = await createAlignedSim(page, 'E2E paused world', 500, 6)

    const run = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(run.status()).toBe(202)
    // Let a few steps land, then pause.
    await page.waitForTimeout(1_500)
    const pause = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/control`, { action: 'pause' })
    expect(pause.ok(), `pause failed: ${pause.status()}`).toBeTruthy()
    await waitForStatus(page, simId, ['paused'], 15_000)

    // Cold-open the run view for the PAUSED sim: the join keyframe must
    // populate the canvas and the catch-up path the widgets.
    await page.goto(`/simulations/${simId}/run`)
    await expect(page.getByText(/paused/i).first()).toBeVisible({ timeout: 20_000 })
    // The PixiJS world canvas is mounted and sized (not the empty no-run state).
    await expect(page.locator('canvas').first()).toBeVisible({ timeout: 20_000 })

    // GET agent while PAUSED exercises the live ctrlAgentQuery path (answered
    // in the paused control loop).
    const sim = await (await api(page, 'get', `/api/v1/intellisim/simulations/${simId}`)).json()
    const live = await api(page, 'get', `/api/v1/intellisim/simulations/${simId}/agents/${sim.agents[0].id}`)
    expect(live.ok(), 'live agent query while paused must answer').toBeTruthy()

    // Cleanup: stop the run.
    const stop = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/control`, { action: 'stop' })
    expect(stop.ok()).toBeTruthy()
    await waitForStatus(page, simId, ['stopped'], 20_000)
  })
})
