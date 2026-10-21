import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the advanced shape editor (2026-07-07): the Environment tab
// now hosts a shape LIST + full ShapeForm alongside the canvas preview. Every
// shape config the backend supports is editable, and shapes can be authored with
// a placement strategy (no drawing). This exercises: add fixed + strategy shapes
// via the picker, geometry↔placement reconciliation, ScalarSpec range validation,
// bidirectional list selection, and the full model round-trip through PUT.

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

// Set a PrimeVue InputNumber (identified by a data-testid on its root) to a value.
async function setNumber(page: Page, testid: string, value: number): Promise<void> {
  const input = page.getByTestId(testid).locator('input')
  await input.click()
  await input.fill(String(value))
  await input.blur()
}

async function pickOption(page: Page, name: string): Promise<void> {
  await page.getByRole('option', { name, exact: true }).click()
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

test.describe.serial('Advanced shape editor', () => {
  let modelId = ''

  test('setup: spatial model with a 200×200 environment', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Shape Editor',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    modelId = created.id

    // Give the environment concrete dimensions via the API (avoids UI setup flake).
    // A freshly created model may have no environment object yet.
    const full = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
    full.model.environment = {
      uid: 'environment',
      torus: false,
      shapes: [],
      global_properties: [],
      ...(full.model.environment ?? {}),
      width: 200,
      height: 200,
    }
    const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model: full.model })
    expect(put.ok(), `env PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()
  })

  test('add a fixed rectangle + a strategy circle, reconcile, validate ranges', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    await page.goto(`/models/${modelId}`)
    await page.getByRole('tab', { name: /environment/i }).click()

    // The shape manager renders.
    await expect(page.getByText('Manage shapes', { exact: true })).toBeVisible({ timeout: 20_000 })
    const addBtn = page.getByTestId('add-shape')
    await expect(addBtn).toBeVisible()

    // --- Add a FIXED rectangle (default type + Fixed placement) ---
    await addBtn.click()
    const picker = page.getByTestId('add-shape-picker')
    await expect(picker).toBeVisible()
    await page.getByTestId('add-shape-confirm').click()

    // ShapeForm opens in fixed mode → position fields visible.
    await expect(page.getByTestId('geom-pos-x')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('geom-strategy-note')).toBeHidden()
    // Set a fixed width value.
    await setNumber(page, 'geom-width-value', 40)

    let body = await saveModel(page)
    let shapes = body.model.environment.shapes
    expect(shapes).toHaveLength(1)
    expect(shapes[0].type).toBe('rectangle')
    expect(shapes[0].geometry.position).toBeTruthy()
    expect(shapes[0].geometry.width.value).toBe(40)
    expect(shapes[0].initialization.population_count).toBe(1)
    expect(shapes[0].initialization.placement).toBeFalsy()

    // --- Add a STRATEGY circle (random placement) ---
    await addBtn.click()
    await expect(picker).toBeVisible()
    // Type select is the first combobox in the picker, Placement the second.
    await picker.getByRole('combobox').nth(0).click()
    await pickOption(page, 'circle')
    await picker.getByRole('combobox').nth(1).click()
    await pickOption(page, 'Random')
    await page.getByTestId('add-shape-confirm').click()

    // ShapeForm for the new circle: strategy mode → no position, count controls shown.
    await expect(page.getByTestId('geom-strategy-note')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('geom-pos-x')).toBeHidden()
    await expect(page.getByTestId('init-population-count')).toBeVisible()
    await setNumber(page, 'init-population-count', 8)

    body = await saveModel(page)
    shapes = body.model.environment.shapes
    expect(shapes).toHaveLength(2)
    const circle = shapes.find((s: any) => s.type === 'circle')
    expect(circle).toBeTruthy()
    expect(circle.initialization.placement.type).toBe('random')
    expect(circle.geometry.position).toBeFalsy()
    expect(circle.geometry.radius).toBeTruthy()
    expect(circle.initialization.population_count).toBe(8)

    // --- Reconciliation: strategy → fixed (clear placement) ---
    // The circle is still selected. Clear the placement Select → fixed mode.
    const placementSelect = page.getByTestId('placement-type')
    await placementSelect.locator('.p-select-clear-icon').click()
    await expect(page.getByTestId('geom-pos-x')).toBeVisible({ timeout: 10_000 })

    body = await saveModel(page)
    const circle2 = body.model.environment.shapes.find((s: any) => s.type === 'circle')
    expect(circle2.geometry.position).toBeTruthy()
    expect(circle2.initialization.population_count).toBe(1)
    expect(circle2.initialization.placement).toBeFalsy()

    // --- ScalarSpec range validation on the rectangle ---
    // Select the rectangle via the list (bidirectional: list → form).
    await page.locator('button', { hasText: 'Rectangle 1' }).first().click()
    await expect(page.getByTestId('geom-width-value')).toBeVisible({ timeout: 10_000 })
    // Toggle width to Range and enter min > max → inline error + list issue dot.
    await page.getByTestId('geom-width').getByText('Range', { exact: true }).click()
    await setNumber(page, 'geom-width-min', 50)
    await setNumber(page, 'geom-width-max', 10)
    await expect(page.getByTestId('geom-width-error')).toBeVisible()
    await expect(page.getByTestId('shape-issues')).toBeVisible()
    // Fix the range → error clears.
    await setNumber(page, 'geom-width-max', 80)
    await expect(page.getByTestId('geom-width-error')).toBeHidden()

    // Final save is valid (no validation error toast, PUT ok).
    body = await saveModel(page)
    const rect = body.model.environment.shapes.find((s: any) => s.type === 'rectangle')
    expect(rect.geometry.width.min).toBe(50)
    expect(rect.geometry.width.max).toBe(80)
    expect(rect.geometry.width.value).toBeFalsy()
  })

  test('UI-authored strategy shape generates instances and a rules-only run finishes', async ({ page }) => {
    test.setTimeout(180_000)
    await login(page)
    token = await getToken(page)

    // Fresh rules-only spatial model with a concrete environment.
    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Shape Run',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const runModelId: string = created.id
    const model = created.model
    model.is_spatial = true
    model.environment = { uid: 'environment', width: 120, height: 120, torus: false, shapes: [], global_properties: [] }
    // A minimal rules-only agent so the model is simulation-ready (shapes alone
    // don't make a runnable model). The shapes are authored via the UI below.
    model.agents = [
      {
        uid: 'mover', name: 'Mover', type: 'agent', move_type: 'movable',
        properties: [], states: ['idle'],
        initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
      },
    ]
    model.simulation_config.llm_simulation_execution_config.default_strategy.type = 'none'
    const put = await api(page, 'put', `/api/v1/intellisim/models/${runModelId}`, { model })
    expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

    await page.goto(`/models/${runModelId}`)
    await page.getByRole('tab', { name: /environment/i }).click()
    const addBtn = page.getByTestId('add-shape')
    await expect(addBtn).toBeVisible({ timeout: 20_000 })

    // Author a RANDOM-placed circle (population 5) via the form — no drawing.
    await addBtn.click()
    const picker = page.getByTestId('add-shape-picker')
    await picker.getByRole('combobox').nth(0).click()
    await pickOption(page, 'circle')
    await picker.getByRole('combobox').nth(1).click()
    await pickOption(page, 'Random')
    await page.getByTestId('add-shape-confirm').click()
    await expect(page.getByTestId('init-population-count')).toBeVisible({ timeout: 10_000 })
    await setNumber(page, 'init-population-count', 5)

    // Author a FIXED rectangle.
    await addBtn.click()
    await picker.getByRole('combobox').nth(0).click()
    await pickOption(page, 'rectangle')
    await picker.getByRole('combobox').nth(1).click()
    await pickOption(page, 'Fixed (single)')
    await page.getByTestId('add-shape-confirm').click()
    await expect(page.getByTestId('geom-pos-x')).toBeVisible({ timeout: 10_000 })

    const saved = await saveModel(page)
    const savedShapes = saved.model.environment.shapes
    expect(savedShapes).toHaveLength(2)
    const circleDef = savedShapes.find((s: any) => s.type === 'circle')
    const rectDef = savedShapes.find((s: any) => s.type === 'rectangle')
    expect(circleDef.initialization.placement.type).toBe('random')
    expect(circleDef.initialization.population_count).toBe(5)
    expect(circleDef.geometry.position).toBeFalsy()
    expect(rectDef.geometry.position).toBeTruthy()

    // Generate a simulation → the strategy circle expands to 5 placed instances.
    const simResp = await api(page, 'post', '/api/v1/intellisim/simulations', {
      model_id: runModelId, name: 'E2E shape run', steps: 3, seed: 7,
    })
    const sim = await simResp.json()
    expect(simResp.ok(), `sim create failed: ${simResp.status()} ${JSON.stringify(sim)}`).toBeTruthy()
    const simId: string = sim.id
    const ready = await waitForStatus(page, simId, ['ready'], 90_000)

    const simShapes: any[] = ready.environment?.shapes ?? ready.shapes ?? []
    const circles = simShapes.filter((s) => s.shape_uid === circleDef.uid)
    const rects = simShapes.filter((s) => s.shape_uid === rectDef.uid)
    expect(circles, 'random circle expands to 5 instances').toHaveLength(5)
    expect(rects, 'fixed rectangle is a single instance').toHaveLength(1)
    expect(circles.every((s) => !!s.position), 'placed circle instances have positions').toBeTruthy()

    // A short rules-only run completes.
    const run = await api(page, 'post', `/api/v1/intellisim/simulations/${simId}/run`, {})
    expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
    await waitForStatus(page, simId, ['finished'], 90_000)
  })

  test('figure-draw: ShapeForm asks the canvas to draw a display figure', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Shape Figure',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const figModelId: string = created.id
    const model = created.model
    model.is_spatial = true
    model.environment = { uid: 'environment', width: 200, height: 200, torus: false, shapes: [], global_properties: [] }
    const put = await api(page, 'put', `/api/v1/intellisim/models/${figModelId}`, { model })
    expect(put.ok()).toBeTruthy()

    await page.goto(`/models/${figModelId}`)
    await page.getByRole('tab', { name: /environment/i }).click()
    const addBtn = page.getByTestId('add-shape')
    await expect(addBtn).toBeVisible({ timeout: 20_000 })

    // Add a fixed rectangle and open its Display section.
    await addBtn.click()
    await page.getByTestId('add-shape-confirm').click()
    await expect(page.getByTestId('geom-pos-x')).toBeVisible({ timeout: 10_000 })
    await page.getByRole('button', { name: 'Display', exact: true }).click()
    // Switch display kind to Figure, then request a canvas draw.
    await page.getByRole('button', { name: 'Figure', exact: true }).click()
    const drawBtn = page.getByRole('button', { name: 'Draw figure on canvas' })
    await expect(drawBtn).toBeVisible({ timeout: 10_000 })
    await drawBtn.click()

    // The canvas enters figure-draw mode — this is the ShapeForm → EnvironmentForm
    // → canvas.startFigureDraw() wiring added for the advanced editor. The polygon
    // point-capture itself is the existing canvas machinery (covered by
    // polygon-draw.spec) and is not reliably drivable via synthetic pixi events.
    await expect(page.getByText('Drawing figure', { exact: false })).toBeVisible({ timeout: 10_000 })

    // Escape exits draw mode cleanly (canvas keyboard handler).
    await page.locator('canvas').first().click({ position: { x: 5, y: 5 } }).catch(() => {})
    await page.keyboard.press('Escape')
    await expect(page.getByText('Drawing figure', { exact: false })).toBeHidden({ timeout: 10_000 })
  })

  test('regressions: strategy polygon point-edit stays strategy; UID edit keeps selection', async ({ page }) => {
    test.setTimeout(150_000)
    await login(page)
    token = await getToken(page)

    const created = await (
      await api(page, 'post', '/api/v1/intellisim/models', {
        name: 'E2E Shape Regress',
        is_spatial: true,
        llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
      })
    ).json()
    const rModelId: string = created.id
    const model = created.model
    model.is_spatial = true
    model.environment = { uid: 'environment', width: 200, height: 200, torus: false, shapes: [], global_properties: [] }
    expect((await api(page, 'put', `/api/v1/intellisim/models/${rModelId}`, { model })).ok()).toBeTruthy()

    await page.goto(`/models/${rModelId}`)
    await page.getByRole('tab', { name: /environment/i }).click()
    const addBtn = page.getByTestId('add-shape')
    await expect(addBtn).toBeVisible({ timeout: 20_000 })
    const picker = page.getByTestId('add-shape-picker')

    // --- Fix: editing a point on a STRATEGY polygon must NOT inject a position ---
    await addBtn.click()
    await picker.getByRole('combobox').nth(0).click()
    await pickOption(page, 'polygon')
    await picker.getByRole('combobox').nth(1).click()
    await pickOption(page, 'Random')
    await page.getByTestId('add-shape-confirm').click()
    await expect(page.getByTestId('geom-strategy-note')).toBeVisible({ timeout: 10_000 })
    // Edit the first vertex's X — must keep it strategy-placed (no position, no 422).
    const px = page.getByTestId('geom-point-0').locator('input').first()
    await px.click()
    await px.fill('30')
    await px.blur()
    await expect(page.getByTestId('shape-issues')).toBeHidden()

    let body = await saveModel(page)
    const poly = body.model.environment.shapes.find((s: any) => s.type === 'polygon')
    expect(poly.initialization.placement.type).toBe('random')
    expect(poly.geometry.position, 'strategy polygon must not gain a position on point edit').toBeFalsy()

    // --- Fix: editing the UID keeps the selection / detail form open ---
    // The polygon is selected; rename its uid and confirm the form stays.
    const uidInput = page.getByPlaceholder('wall')
    await uidInput.fill('renamed_poly')
    await uidInput.blur()
    // Form still shown for the same (renamed) shape.
    await expect(page.getByTestId('geom-strategy-note')).toBeVisible()
    await expect(uidInput).toHaveValue('renamed_poly')

    body = await saveModel(page)
    expect(body.model.environment.shapes.some((s: any) => s.uid === 'renamed_poly')).toBeTruthy()
  })
})
