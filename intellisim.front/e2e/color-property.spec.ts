import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the predefined `color` property (2026-07-08):
//   1. adding an agent to a SPATIAL model in the editor seeds a `color` property
//      (palette default) and shows a ColorPicker; editing it round-trips through PUT.
//   2. removing the colour surfaces a hint + one-click "Add colour" restore.
//   3. shapes still seed + edit colour (regression).
//   4. strict hex validation rejects a bad colour default with 422.
//   5. a simulation's agent instances carry the archetype colour default, a
//      manually placed instance inherits it, and a small rules-only run keeps it.

let token = ''
async function api(page: Page, method: 'get' | 'post' | 'put', url: string, data?: unknown): Promise<APIResponse> {
  return page.request[method](url, {
    headers: { Authorization: `Bearer ${token}` },
    ...(data !== undefined ? { data } : {}),
  })
}

async function saveModel(page: Page): Promise<any> {
  const [resp] = await Promise.all([
    page.waitForResponse(
      (r) => /\/api\/v1\/intellisim\/models\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'
    ),
    page.getByRole('button', { name: 'Save', exact: true }).click(),
  ])
  expect(resp.ok(), `save PUT failed: ${resp.status()} ${await resp.text()}`).toBeTruthy()
  return resp.json()
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

// Set a native <input type=color> value and fire the input event the form listens on.
async function setColor(page: Page, label: string, hex: string): Promise<void> {
  await page.getByLabel(label).evaluate((el, hex) => {
    const input = el as HTMLInputElement
    input.value = hex
    input.dispatchEvent(new Event('input', { bubbles: true }))
  }, hex)
}

// Colour-typed property finder on a saved entity.
function colorProp(entity: any): any {
  return (entity?.properties ?? []).find((p: any) => p.type === 'color')
}

async function createSpatialModel(page: Page, name: string, width = 120, height = 120): Promise<string> {
  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name,
      is_spatial: true,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const id = created.id as string
  const full = await (await api(page, 'get', `/api/v1/intellisim/models/${id}`)).json()
  full.model.is_spatial = true
  full.model.environment = {
    uid: 'environment',
    torus: false,
    shapes: [],
    global_properties: [],
    ...(full.model.environment ?? {}),
    width,
    height,
  }
  const put = await api(page, 'put', `/api/v1/intellisim/models/${id}`, { model: full.model })
  expect(put.ok(), `env PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()
  return id
}

test.describe.serial('Predefined color property', () => {
  test('editor: adding an agent to a spatial model seeds a color property (palette default)', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)
    const modelId = await createSpatialModel(page, 'E2E Color Agent Seed')

    await page.goto(`/models/${modelId}`)
    await page.getByRole('tab', { name: /agents/i }).click()
    await page.getByRole('button', { name: /add agent/i }).click()

    // The colour ColorPicker is present for a spatial agent, seeded with palette[0].
    const colorInput = page.getByLabel('Agent color')
    await expect(colorInput).toBeVisible({ timeout: 20_000 })
    await expect(colorInput).toHaveValue('#8b5cf6')

    // No missing-colour hint while the seeded property is present.
    await expect(page.getByText(/No colour set/i)).toHaveCount(0)
  })

  test('editor: missing-colour hint + one-click add + PUT round-trip', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)

    // Seed a valid spatial agent WITHOUT a colour (a generic PUT does not stamp).
    const modelId = await createSpatialModel(page, 'E2E Color Hint')
    const full = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
    full.model.agents = [
      {
        uid: 'wolf', name: 'Wolf', type: 'agent', move_type: 'movable',
        properties: [], states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
    ]
    full.model.simulation_config.llm_simulation_execution_config.default_strategy.type = 'none'
    expect((await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model: full.model })).ok()).toBeTruthy()

    await page.goto(`/models/${modelId}`)
    await page.getByRole('tab', { name: /agents/i }).click()

    // Hint appears (spatial agent with no colour property); the header ColorPicker
    // still shows the palette fallback so the user can pick directly.
    await expect(page.getByText(/No colour set/i)).toBeVisible({ timeout: 20_000 })
    const colorInput = page.getByLabel('Agent color')
    await expect(colorInput).toHaveValue('#8b5cf6')

    // One-click add → the colour property is created, so the hint clears.
    await page.getByRole('button', { name: 'Add colour' }).click()
    await expect(page.getByText(/No colour set/i)).toHaveCount(0)
    await expect(colorInput).toHaveValue('#8b5cf6')

    // Edit the colour and save → the colour property persists with the new hex.
    await setColor(page, 'Agent color', '#ff0000')
    await expect(colorInput).toHaveValue('#ff0000')
    const body = await saveModel(page)
    const cp = colorProp(body.model.agents.find((a: any) => a.uid === 'wolf'))
    expect(cp, 'agent has a colour-typed property after save').toBeTruthy()
    expect(cp.uid).toBe('color')
    expect(String(cp.default_value).toLowerCase()).toBe('#ff0000')
  })

  test('editor: adding a shape seeds + edits its colour (regression)', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)
    const modelId = await createSpatialModel(page, 'E2E Color Shape', 200, 200)

    await page.goto(`/models/${modelId}`)
    await page.getByRole('tab', { name: /environment/i }).click()
    const addBtn = page.getByTestId('add-shape')
    await expect(addBtn).toBeVisible({ timeout: 20_000 })
    await addBtn.click()
    await page.getByTestId('add-shape-confirm').click()

    // ShapeForm shows a seeded colour; no missing-colour hint.
    const colorInput = page.getByLabel('Shape color')
    await expect(colorInput).toBeVisible({ timeout: 10_000 })
    await expect(colorInput).toHaveValue(/^#[0-9a-f]{6}$/)
    await expect(page.getByText(/No colour set/i)).toHaveCount(0)

    await setColor(page, 'Shape color', '#00ff00')
    const body = await saveModel(page)
    const shape = body.model.environment.shapes[0]
    const cp = colorProp(shape)
    expect(cp, 'shape has a colour-typed property').toBeTruthy()
    expect(String(cp.default_value).toLowerCase()).toBe('#00ff00')
  })

  test('validation: a non-hex colour default is rejected with 422', async ({ page }) => {
    test.setTimeout(60_000)
    await login(page)
    token = await getToken(page)
    const modelId = await createSpatialModel(page, 'E2E Color Validation')

    const full = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
    const agent = {
      uid: 'blob', name: 'Blob', type: 'agent', move_type: 'static',
      states: ['idle'],
      initialization: { population_count: 1, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      properties: [{ uid: 'color', name: 'color', type: 'color', description: '', immutable: false, default_value: 'red' }],
    }
    full.model.agents = [agent]

    // Bad hex → 422.
    const bad = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model: full.model })
    expect(bad.status(), 'non-hex colour default must be rejected').toBe(422)
    expect((await bad.text()).toLowerCase()).toContain('color')

    // Fix to a valid hex → ok.
    agent.properties[0].default_value = '#3366cc'
    const good = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model: full.model })
    expect(good.ok(), `valid colour PUT failed: ${good.status()} ${await good.text()}`).toBeTruthy()
  })

  test('simulation: instances inherit the archetype colour, placement seeds it, run keeps it', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    // Rules-only spatial model: one archetype with an explicit colour default.
    const modelId = await createSpatialModel(page, 'E2E Color Run', 100, 100)
    const full = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
    full.model.agents = [
      {
        uid: 'dot', name: 'Dot', type: 'agent', move_type: 'movable',
        states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
        properties: [
          { uid: 'color', name: 'color', type: 'color', description: '', immutable: false, default_value: '#22C55E' },
        ],
      },
    ]
    full.model.simulation_config.llm_simulation_execution_config.default_strategy.type = 'none'
    expect((await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model: full.model })).ok()).toBeTruthy()

    // Generate a simulation → both instances carry the colour default.
    const sim = await (
      await api(page, 'post', '/api/v1/intellisim/simulations', {
        model_id: modelId, name: 'E2E colour run', steps: 3, seed: 7,
      })
    ).json()
    const simId: string = sim.id
    const ready = await waitForStatus(page, simId, ['ready'], 90_000)
    const dots = ready.agents.filter((a: any) => a.agent_uid === 'dot')
    expect(dots).toHaveLength(2)
    expect(dots.every((a: any) => String(a.properties?.color).toLowerCase() === '#22c55e'),
      'generated instances inherit the colour default').toBeTruthy()

    // Manually place a third instance via the sim editor world canvas → it inherits
    // the archetype colour default (makeAgentFromArchetype seeding).
    await page.goto(`/simulations/${simId}`)
    await page.getByRole('tab', { name: /environment/i }).click()
    const canvas = page.getByTestId('world-canvas')
    await expect(canvas).toBeVisible({ timeout: 20_000 })
    await page.getByRole('button', { name: 'Agent', exact: true }).click()
    await page.getByRole('menuitem', { name: /dot/i }).click()
    const box = await canvas.boundingBox()
    if (!box) throw new Error('no canvas box')
    await canvas.click({ position: { x: box.width * 0.5, y: box.height * 0.5 } })

    const saved = await (async () => {
      const [resp] = await Promise.all([
        page.waitForResponse(
          (r) => /\/api\/v1\/intellisim\/simulations\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'
        ),
        page.getByRole('button', { name: 'Save', exact: true }).click(),
      ])
      expect(resp.ok(), `sim save failed: ${resp.status()} ${await resp.text()}`).toBeTruthy()
      return resp.json()
    })()
    const savedDots = saved.agents.filter((a: any) => a.agent_uid === 'dot')
    expect(savedDots.length, 'a third instance was placed').toBe(3)
    expect(savedDots.every((a: any) => String(a.properties?.color).toLowerCase() === '#22c55e'),
      'placed instance inherits the archetype colour').toBeTruthy()

    // A short rules-only run finishes and instances keep their colour.
    const run = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
    const done = await waitForStatus(page, simId, ['finished'], 90_000)
    const doneDots = done.agents.filter((a: any) => a.agent_uid === 'dot')
    expect(doneDots.every((a: any) => String(a.properties?.color).toLowerCase() === '#22c55e'),
      'instances keep their colour through the run').toBeTruthy()
  })
})
