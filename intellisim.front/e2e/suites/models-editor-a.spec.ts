import { test, expect, type APIRequestContext, type Page, type Locator } from '@playwright/test'
import { login } from '../_helpers/login'
import { apiContext, apiJson } from '../_helpers/api'
import { deleteQuietly, minimalSpatial, nonSpatial, seedModel, uniqueName } from '../_helpers/fixtures'

// MDE-A — Model editor part A (plan §5.4, MDE-01…MDE-11). Deterministic, no LLM.
// Models are seeded via API; only the editor UI under test is driven. Every model
// is tracked and deleted in afterAll.
//
// Editor facts these tests rely on (verified against the sources):
//  - readiness chip text "Ready"/"Not ready" in the top bar; dirty Tag "unsaved";
//    header Save disabled unless dirty; tab SWITCH auto-saves (useEditorAutoSave)
//    but summary-issue deep-links set the tab programmatically → NO auto-save.
//  - PrimeVue Select options render in a body-level overlay (role=option).
//  - PrimeVue Fieldset toggles by clicking its legend text.

let api: APIRequestContext
const createdModelIds: string[] = []

function trackId(id: string) {
  if (id && !createdModelIds.includes(id)) createdModelIds.push(id)
}

test.beforeAll(async () => {
  api = await apiContext()
})

test.afterAll(async () => {
  for (const id of createdModelIds) await deleteQuietly(api, `/models/${id}`)
  await api.dispose()
})

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// The editor's forms use <div><label>Label</label><input/select/…></div> rows
// without label[for]; `field` returns the row container for a label.
function field(scope: Page | Locator, label: string): Locator {
  return scope.locator(`label:text-is("${label}")`).first().locator('..')
}

// Fill a text/number input identified by its row label (InputNumber commits on blur).
async function fillField(scope: Page | Locator, label: string, value: string | number): Promise<void> {
  const input = field(scope, label).locator('input').first()
  await input.click()
  await input.fill(String(value))
  await input.press('Tab')
}

async function pickOption(page: Page, name: string): Promise<void> {
  // Restrict to real overlay options — PrimeVue InputChips chips ALSO carry
  // role=option, so a bare getByRole('option') can hit a chip (strict violation).
  await page
    .getByRole('option', { name, exact: true })
    .and(page.locator('.p-select-option, .p-multiselect-option'))
    .click()
}

// Open the PrimeVue Select in the labelled row and pick an option from the overlay.
async function selectInField(page: Page, scope: Page | Locator, label: string, option: string): Promise<void> {
  await field(scope, label).getByRole('combobox').first().click()
  await pickOption(page, option)
}

// PrimeVue Fieldset located by its legend text (legend also toggles collapse).
function fieldsetByLegend(page: Page, legend: string): Locator {
  return page.locator('fieldset').filter({ has: page.getByText(legend, { exact: true }) })
}

// Expand/collapse a Fieldset by clicking its legend text. Scoped INSIDE
// <fieldset> — a bare getByText('Properties') would also match the Summary
// panel's "Properties" count tile (strict-mode collision).
async function toggleFieldset(page: Page, legend: string): Promise<void> {
  await page.locator('fieldset').getByText(legend, { exact: true }).click()
}

// Set a PrimeVue InputNumber wrapped in a data-testid container.
async function setNumberByTestId(page: Page, testid: string, value: number): Promise<void> {
  const input = page.getByTestId(testid).locator('input')
  await input.click()
  await input.fill(String(value))
  await input.press('Tab')
}

// Click the header Save button and return the PUT /models/:id response JSON.
async function saveModel(page: Page): Promise<any> {
  const [resp] = await Promise.all([
    page.waitForResponse(
      (r) => /\/api\/v1\/intellisim\/models\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT',
    ),
    page.getByRole('button', { name: 'Save', exact: true }).click(),
  ])
  expect(resp.ok(), `save PUT failed: ${resp.status()} ${await resp.text()}`).toBeTruthy()
  return resp.json()
}

async function openModelEditor(page: Page, id: string): Promise<void> {
  await page.goto(`/models/${id}`)
  await expect(page.getByRole('tab', { name: /general/i })).toBeVisible({ timeout: 20_000 })
}

async function openTab(page: Page, name: RegExp): Promise<void> {
  await page.getByRole('tab', { name }).click()
}

// Add a vertex on the inline polygon-draw SVG by dispatching a native pointerdown
// (synthetic mouse.click doesn't reliably reach the SVG — same approach as
// e2e/polygon-draw.spec.ts). One point per call + awaited count between calls.
async function clickPolygonCanvas(page: Page, canvasIndex: number, fx: number, fy: number): Promise<void> {
  await page.evaluate(
    ({ canvasIndex, fx, fy }) => {
      const canvas = document.querySelectorAll('[data-testid="polygon-draw"]')[canvasIndex]
      const svg = canvas?.querySelector('svg')
      if (!svg) throw new Error('no polygon-draw svg')
      const r = svg.getBoundingClientRect()
      svg.dispatchEvent(
        new PointerEvent('pointerdown', {
          bubbles: true,
          cancelable: true,
          clientX: r.left + r.width * fx,
          clientY: r.top + r.height * fy,
        }),
      )
    },
    { canvasIndex, fx, fy },
  )
}

async function drawPolygon(page: Page, canvasIndex: number, corners: [number, number][]): Promise<void> {
  const vertices = page
    .locator('[data-testid="polygon-draw"]')
    .nth(canvasIndex)
    .locator('[data-testid="polygon-vertex"]')
  for (let i = 0; i < corners.length; i++) {
    await clickPolygonCanvas(page, canvasIndex, corners[i]![0], corners[i]![1])
    await expect(vertices).toHaveCount(i + 1)
  }
}

// ---------------------------------------------------------------------------
// MDE-01 — General tab
// ---------------------------------------------------------------------------

test('MDE-01 General tab: name/description/spatial+cell size/update mode; Save persists; unsaved indicator lifecycle', async ({ page }) => {
  test.setTimeout(90_000)
  const created = await apiJson<{ id: string }>(api, 'post', '/models', { name: uniqueName('mde01') }, 201)
  trackId(created.id)

  await login(page)
  await openModelEditor(page, created.id)

  // Pristine editor: not dirty, Save disabled.
  await expect(page.getByText('unsaved')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled()

  // Edit name → dirty indicator appears + Save enables.
  const newName = uniqueName('mde01-renamed')
  await page.getByPlaceholder('Model name').fill(newName)
  await expect(page.getByText('unsaved')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeEnabled()

  // Description, Spatial toggle (+ cell size appears only when spatial), update mode.
  await field(page, 'Description').locator('textarea').fill('edited by MDE-01')
  await expect(field(page, 'Spatial cell size')).toHaveCount(0)
  await page.getByText('Spatial model', { exact: true }).click()
  await expect(field(page, 'Spatial cell size').locator('input')).toBeVisible()
  await fillField(page, 'Spatial cell size', 2)
  const simCfg = fieldsetByLegend(page, 'Simulation configuration')
  await selectInField(page, simCfg, 'Update mode', 'synchronous')

  // Manual Save (same tab — tab switches would auto-save) → dirty clears.
  const body = await saveModel(page)
  await expect(page.getByText('unsaved')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled()
  expect(body.model.name).toBe(newName)
  expect(body.model.is_spatial).toBe(true)
  expect(body.model.spatial_cell_size).toBe(2)
  expect(body.model.simulation_config.update_mode).toBe('synchronous')

  // Reload → everything persisted into the form.
  await page.reload()
  await expect(page.getByPlaceholder('Model name')).toHaveValue(newName, { timeout: 20_000 })
  await expect(field(page, 'Description').locator('textarea')).toHaveValue('edited by MDE-01')
  await expect(field(page, 'Spatial cell size').locator('input')).toHaveValue('2')
  await expect(fieldsetByLegend(page, 'Simulation configuration').getByRole('combobox').first()).toContainText('synchronous')
})

// ---------------------------------------------------------------------------
// MDE-02 — Readiness + Summary issues deep-link
// ---------------------------------------------------------------------------

test('MDE-02 readiness: blank model Not ready with Summary issues; issue click deep-links; authoring agent+env dims flips to Ready', async ({ page }) => {
  test.setTimeout(120_000)
  const created = await apiJson<{ id: string }>(
    api, 'post', '/models', { name: uniqueName('mde02'), is_spatial: true }, 201,
  )
  trackId(created.id)

  await login(page)
  await openModelEditor(page, created.id)

  // Header chip + summary verdict.
  await expect(page.getByText('Not ready', { exact: true })).toBeVisible()
  await expect(page.getByText('Not ready to run')).toBeVisible()

  // The two blockers of a blank spatial model (verified against live backend).
  const agentsIssue = page.getByRole('button').filter({ hasText: 'model has no agents' })
  const envIssue = page.getByRole('button').filter({ hasText: 'width & height' })
  await expect(agentsIssue).toBeVisible()
  await expect(envIssue).toBeVisible()

  // Issue click deep-links to the offending tab.
  await agentsIssue.click()
  await expect(page.getByRole('tab', { name: /agents/i })).toHaveAttribute('aria-selected', 'true')

  // Author a valid agent.
  await page.getByRole('button', { name: 'Add agent' }).click()
  await page.getByPlaceholder('wolf').fill('walker')
  await fillField(page, 'Name', 'Walker')
  await toggleFieldset(page, 'Initialization')
  await setNumberByTestId(page, 'init-population-count', 2)
  await page.getByTestId('placement-type').click()
  await pickOption(page, 'random')

  // Deep-link to Environment via the env issue, author dims.
  await envIssue.click()
  await expect(page.getByRole('tab', { name: /environment/i })).toHaveAttribute('aria-selected', 'true')
  await fillField(page, 'Width', 20)
  await fillField(page, 'Height', 20)

  await saveModel(page)

  // Ready now — header chip + summary verdict + zero error issues + API agrees.
  await expect(page.getByText('Ready', { exact: true })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText('Ready to run')).toBeVisible()
  await expect(page.getByText('model has no agents')).toHaveCount(0)
  const full = await apiJson<any>(api, 'get', `/models/${created.id}`, undefined, 200)
  expect(full.validation?.simulation_ready, JSON.stringify(full.validation?.errors)).toBe(true)
})

// ---------------------------------------------------------------------------
// MDE-03 — Agents tab basics
// ---------------------------------------------------------------------------

test('MDE-03 Agents tab: add archetype, set name/type/move type/description/states chips, remove agent', async ({ page }) => {
  test.setTimeout(120_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde03')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /agents/i)

  // Add a new agent (auto-selected).
  await page.getByRole('button', { name: 'Add agent' }).click()
  await page.getByPlaceholder('wolf').fill('critter')
  await fillField(page, 'Name', 'Critter')
  await selectInField(page, page, 'Type', 'archetype')
  await selectInField(page, page, 'Move type', 'movable')
  await field(page, 'Description').locator('textarea').fill('an e2e archetype')

  // States chips: type + Enter per state.
  const chips = field(page, 'States').locator('input')
  await chips.click()
  await chips.fill('idle')
  await chips.press('Enter')
  await chips.fill('roam')
  await chips.press('Enter')

  // The master list marks archetypes with an "arch" badge.
  await expect(
    page.getByRole('button').filter({ hasText: 'Critter' }).filter({ hasText: 'arch' }),
  ).toBeVisible()

  // Save → the new agent round-trips.
  let body = await saveModel(page)
  expect(body.model.agents).toHaveLength(2)
  const critter = body.model.agents.find((a: any) => a.uid === 'critter')
  expect(critter, 'critter agent saved').toBeTruthy()
  expect(critter.name).toBe('Critter')
  expect(critter.type).toBe('archetype')
  expect(critter.move_type).toBe('movable')
  expect(critter.description).toBe('an e2e archetype')
  expect(critter.states).toEqual(['idle', 'roam'])

  // Remove the (still selected) agent → save → gone.
  await page.getByRole('button', { name: 'Remove agent' }).click()
  body = await saveModel(page)
  expect(body.model.agents).toHaveLength(1)
  expect(body.model.agents[0].uid).toBe('walker')
})

// ---------------------------------------------------------------------------
// MDE-04 — Properties of every type
// ---------------------------------------------------------------------------

test('MDE-04 properties: add one of each type (string/number/enum/boolean/color), remove one, save round-trips', async ({ page }) => {
  test.setTimeout(150_000)
  const fixture = minimalSpatial(uniqueName('mde04'))
  fixture.agents[0].properties = [] // start from a clean property list
  const m = await seedModel(api, fixture)
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /agents/i)

  // Expand the Properties fieldset (collapsed by default).
  await toggleFieldset(page, 'Properties')
  const propsFS = fieldsetByLegend(page, 'Properties')
  const addProp = propsFS.getByRole('button', { name: 'Add property' })
  const rows = propsFS.locator('div.rounded-lg.bg-white')

  for (let i = 0; i < 5; i++) await addProp.click()
  await expect(rows).toHaveCount(5)

  // Row 0 — string with a default.
  await selectInField(page, rows.nth(0), 'Type', 'string')
  await fillField(rows.nth(0), 'UID', 'p_str')
  await fillField(rows.nth(0), 'Default value', 'hello')

  // Row 1 — number with min/max/default (new rows default to number already).
  await fillField(rows.nth(1), 'UID', 'p_num')
  await fillField(rows.nth(1), 'Min value', 0)
  await fillField(rows.nth(1), 'Max value', 10)
  await fillField(rows.nth(1), 'Default value', 5)

  // Row 2 — enum with allowed values + enum default.
  await selectInField(page, rows.nth(2), 'Type', 'enum')
  await fillField(rows.nth(2), 'UID', 'p_enum')
  const allowed = field(rows.nth(2), 'Allowed values').locator('input')
  await allowed.click()
  await allowed.fill('red')
  await allowed.press('Enter')
  await allowed.fill('blue')
  await allowed.press('Enter')
  await selectInField(page, rows.nth(2), 'Default value', 'red')

  // Row 3 — boolean with default true.
  await selectInField(page, rows.nth(3), 'Type', 'boolean')
  await fillField(rows.nth(3), 'UID', 'p_bool')
  await field(rows.nth(3), 'Default value').locator('.p-toggleswitch').click()

  // Row 4 — color (default left empty, which is valid).
  await selectInField(page, rows.nth(4), 'Type', 'color')
  await fillField(rows.nth(4), 'UID', 'p_col')

  // Remove the string property.
  await rows.nth(0).getByRole('button', { name: 'Remove property' }).click()
  await expect(rows).toHaveCount(4)

  await saveModel(page)

  // [API] round-trip check via GET /models/:id.
  const full = await apiJson<any>(api, 'get', `/models/${m.id}`, undefined, 200)
  const props = full.model.agents[0].properties
  expect(props.map((p: any) => p.uid)).toEqual(['p_num', 'p_enum', 'p_bool', 'p_col'])
  const num = props.find((p: any) => p.uid === 'p_num')
  expect(num.type).toBe('number')
  expect(num.min_value).toBe(0)
  expect(num.max_value).toBe(10)
  expect(num.default_value).toBe(5)
  const en = props.find((p: any) => p.uid === 'p_enum')
  expect(en.type).toBe('enum')
  expect(en.allowed_values).toEqual(['red', 'blue'])
  expect(en.default_value).toBe('red')
  const bo = props.find((p: any) => p.uid === 'p_bool')
  expect(bo.type).toBe('boolean')
  expect(bo.default_value).toBe(true)
  expect(props.find((p: any) => p.uid === 'p_col').type).toBe('color')
})

// ---------------------------------------------------------------------------
// MDE-05 — Colour property (seed / hint / restore / invalid hex 422)
// ---------------------------------------------------------------------------

test('MDE-05 colour property: auto-seeded on new spatial agent; delete → hint + Add colour restores; invalid hex → 422', async ({ page }) => {
  test.setTimeout(120_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde05')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /agents/i)

  // A NEW agent on a spatial model is auto-seeded with a colour property.
  await page.getByRole('button', { name: 'Add agent' }).click()
  await page.getByPlaceholder('wolf').fill('tinted')
  const colorInput = page.getByLabel('Agent color')
  await expect(colorInput).toBeVisible()
  await expect(colorInput).toHaveValue(/^#[0-9a-f]{6}$/i)
  await expect(page.getByText(/No colour set/i)).toHaveCount(0)

  // Delete the colour property → hint appears.
  await toggleFieldset(page, 'Properties')
  const propsFS = fieldsetByLegend(page, 'Properties')
  await expect(propsFS.locator('div.rounded-lg.bg-white')).toHaveCount(1)
  await propsFS.getByRole('button', { name: 'Remove property' }).click()
  await expect(page.getByText(/No colour set/i)).toBeVisible()

  // One-click "Add colour" restores the property → hint clears.
  await page.getByRole('button', { name: 'Add colour' }).click()
  await expect(page.getByText(/No colour set/i)).toHaveCount(0)
  await expect(colorInput).toHaveValue(/^#[0-9a-f]{6}$/i)
  await expect(propsFS.locator('div.rounded-lg.bg-white')).toHaveCount(1)

  // [API] PUT an invalid hex default → 422. NOTE (SPEC-MISMATCH): the backend
  // returns code "validation_failed" with the invalid-colour detail in `fields`
  // (path names the property), not a top-level "invalid_color" code.
  const full = await apiJson<any>(api, 'get', `/models/${m.id}`, undefined, 200)
  const colorProp = full.model.agents[0].properties.find((p: any) => p.type === 'color')
  expect(colorProp).toBeTruthy()
  colorProp.default_value = 'red'
  const bad = await apiJson<any>(api, 'put', `/models/${m.id}`, { model: full.model }, 422)
  expect(bad.code).toBe('validation_failed')
  const fieldKeys = Object.keys(bad.fields ?? {})
  expect(fieldKeys.some((k) => /agents\[0\]\.properties\[\d+\]/.test(k)), JSON.stringify(bad)).toBe(true)
  expect(JSON.stringify(bad.fields)).toMatch(/hex colour/i)

  // Fixing the hex saves fine again.
  colorProp.default_value = '#3366cc'
  await apiJson(api, 'put', `/models/${m.id}`, { model: full.model }, 200)
})

// ---------------------------------------------------------------------------
// MDE-06 — Initialization conditional fields + placement types
// ---------------------------------------------------------------------------

test('MDE-06 initialization: count-mode conditional fields; placement offers all 6 types; manual shows note', async ({ page }) => {
  test.setTimeout(90_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde06')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /agents/i)
  await toggleFieldset(page, 'Initialization')

  // Fixture agent is count_mode fixed → count only.
  await expect(page.getByTestId('init-count-mode')).toBeVisible()
  await expect(page.getByTestId('init-population-count')).toBeVisible()
  await expect(page.getByTestId('init-min-population')).toHaveCount(0)
  await expect(page.getByTestId('init-max-population')).toHaveCount(0)

  // random_range → min/max only.
  await page.getByTestId('init-count-mode').click()
  await pickOption(page, 'random_range')
  await expect(page.getByTestId('init-population-count')).toHaveCount(0)
  await expect(page.getByTestId('init-min-population')).toBeVisible()
  await expect(page.getByTestId('init-max-population')).toBeVisible()

  // Placement select offers all 6 types.
  await page.getByTestId('placement-type').click()
  for (const t of ['random', 'uniform', 'clustered', 'at_shape', 'manual', 'llm']) {
    await expect(page.getByRole('option', { name: t, exact: true })).toBeVisible()
  }

  // manual → advisory note.
  await pickOption(page, 'manual')
  await expect(page.getByTestId('manual-placement-note')).toBeVisible()

  // Leave the form in a saveable state (route-leave triggers an auto-save attempt).
  await page.getByTestId('placement-type').click()
  await pickOption(page, 'random')
  await page.getByTestId('init-count-mode').click()
  await pickOption(page, 'fixed')
  await setNumberByTestId(page, 'init-population-count', 4)
})

// ---------------------------------------------------------------------------
// MDE-07 — Body & display footprints + collidable gating
// ---------------------------------------------------------------------------

test('MDE-07 body & display: footprint point→circle→rectangle→polygon (draw canvas); collidable only when collision=block', async ({ page }) => {
  test.setTimeout(150_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde07')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /agents/i)
  await toggleFieldset(page, 'Body & display')
  const bodyFS = fieldsetByLegend(page, 'Body & display')

  // collision_mode is unset on the fixture → NO collidable toggle.
  await expect(page.getByText(/Collidable \(participates/)).toHaveCount(0)

  // point (default) → circle: radius input appears.
  const kindSelect = field(bodyFS, 'Simulation body').getByRole('combobox').first()
  await expect(kindSelect).toContainText('point')
  await kindSelect.click()
  await pickOption(page, 'circle')
  await expect(field(bodyFS, 'Radius').locator('input')).toBeVisible()
  await fillField(bodyFS, 'Radius', 1)

  // circle → rectangle: width/height inputs replace radius.
  await kindSelect.click()
  await pickOption(page, 'rectangle')
  await expect(bodyFS.locator('label:text-is("Radius")')).toHaveCount(0)
  await fillField(bodyFS, 'Width', 2)
  await fillField(bodyFS, 'Height', 3)

  // rectangle → polygon: draw canvas; click 3 vertices.
  await kindSelect.click()
  await pickOption(page, 'polygon')
  const drawCanvas = page.locator('[data-testid="polygon-draw"]').first()
  await expect(drawCanvas).toBeVisible()
  await drawPolygon(page, 0, [
    [0.5, 0.25],
    [0.75, 0.7],
    [0.25, 0.7],
  ])

  let body = await saveModel(page)
  expect(body.model.agents[0].footprint.kind).toBe('polygon')
  expect(body.model.agents[0].footprint.points).toHaveLength(3)

  // Set the environment collision mode to block…
  await openTab(page, /environment/i)
  await selectInField(page, page, 'Collision mode', 'Block (no overlap)')

  // …then back on Agents the collidable toggle exists; enable it.
  await openTab(page, /agents/i)
  await toggleFieldset(page, 'Body & display')
  const collidable = page.getByText(/Collidable \(participates/)
  await expect(collidable).toBeVisible()
  await collidable.locator('..').locator('.p-toggleswitch').click()

  body = await saveModel(page)
  expect(body.model.environment.collision_mode).toBe('block')
  expect(body.model.agents[0].collidable).toBe(true)
  expect(body.model.agents[0].footprint.points).toHaveLength(3) // survived
})

// ---------------------------------------------------------------------------
// MDE-08 — Environment tab (spatial)
// ---------------------------------------------------------------------------

test('MDE-08 environment (spatial): dims/torus/collision editable; global property CRUD; prompt context persists', async ({ page }) => {
  test.setTimeout(120_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde08')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /environment/i)

  await fillField(page, 'Width', 44)
  await fillField(page, 'Height', 33)
  await page.getByText('Torus (wrap edges)', { exact: true }).click()
  await selectInField(page, page, 'Collision mode', 'Block (no overlap)')

  // Environment-level LLM prompt context (first such label on the tab).
  const promptCtx = field(page, 'LLM prompt context').locator('textarea').first()
  await promptCtx.fill('the world is a mossy forest')

  // Add a global property (fieldset collapsed for spatial models).
  await toggleFieldset(page, 'Global properties')
  const globalsFS = fieldsetByLegend(page, 'Global properties')
  await globalsFS.getByRole('button', { name: 'Add property' }).click()
  const row = globalsFS.locator('div.rounded-lg.bg-white').first()
  await fillField(row, 'UID', 'g_temp')
  await fillField(row, 'Name', 'Temperature')
  await fillField(row, 'Default value', 7)

  const body = await saveModel(page)
  expect(body.model.environment.width).toBe(44)
  expect(body.model.environment.height).toBe(33)
  expect(body.model.environment.torus).toBe(true)
  expect(body.model.environment.collision_mode).toBe('block')
  expect(body.model.environment.llm_prompt_context).toBe('the world is a mossy forest')
  const g = body.model.environment.global_properties.find((p: any) => p.uid === 'g_temp')
  expect(g).toBeTruthy()
  expect(g.default_value).toBe(7)

  // Reload → the form reflects the persisted values.
  await page.reload()
  await expect(page.getByRole('tab', { name: /environment/i })).toBeVisible({ timeout: 20_000 })
  await openTab(page, /environment/i)
  await expect(field(page, 'Width').locator('input')).toHaveValue('44')
  await expect(field(page, 'Height').locator('input')).toHaveValue('33')
  await expect(field(page, 'Collision mode').getByRole('combobox').first()).toContainText('Block')
  await expect(field(page, 'LLM prompt context').locator('textarea').first()).toHaveValue('the world is a mossy forest')
})

// ---------------------------------------------------------------------------
// MDE-09 — Environment (non-spatial) + non-destructive spatial toggle
// ---------------------------------------------------------------------------

test('MDE-09 environment (non-spatial): globals+context only; spatial toggle shows geometry; toggling back preserves it', async ({ page }) => {
  test.setTimeout(150_000)
  const m = await seedModel(api, nonSpatial(uniqueName('mde09')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /environment/i)

  // Non-spatial: NO geometry UI (width/height/torus/collision/shapes/canvas)…
  await expect(page.locator('label:text-is("Width")')).toHaveCount(0)
  await expect(page.locator('label:text-is("Collision mode")')).toHaveCount(0)
  await expect(page.getByTestId('add-shape')).toHaveCount(0)
  // …but global properties + prompt context ARE editable (fieldset starts expanded).
  await expect(field(page, 'LLM prompt context').locator('textarea').first()).toBeVisible()
  const globalsFS = fieldsetByLegend(page, 'Global properties')
  await expect(globalsFS.locator('div.rounded-lg.bg-white')).toHaveCount(1) // fixture "temperature"

  // Toggle spatial ON in General → Environment gains the geometry UI
  // (the tab switch auto-saves is_spatial).
  await openTab(page, /general/i)
  await page.getByText('Spatial model', { exact: true }).click()
  await openTab(page, /environment/i)
  await expect(field(page, 'Width').locator('input')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('add-shape')).toBeVisible()

  // Author geometry: dims + one fixed rectangle shape.
  await fillField(page, 'Width', 25)
  await fillField(page, 'Height', 15)
  await page.getByTestId('add-shape').click()
  await page.getByTestId('add-shape-confirm').click()
  await expect(page.getByTestId('geom-pos-x')).toBeVisible({ timeout: 10_000 })
  await saveModel(page)

  // Toggle spatial back OFF and save.
  await openTab(page, /general/i)
  await page.getByText('Spatial model', { exact: true }).click()
  await saveModel(page)

  // Environment shows only globals + context again.
  await openTab(page, /environment/i)
  await expect(page.locator('label:text-is("Width")')).toHaveCount(0)
  await expect(page.getByTestId('add-shape')).toHaveCount(0)

  // [API] geometry data survived the OFF toggle (non-destructive).
  const full = await apiJson<any>(api, 'get', `/models/${m.id}`, undefined, 200)
  expect(full.model.is_spatial).toBe(false)
  expect(full.model.environment.width).toBe(25)
  expect(full.model.environment.height).toBe(15)
  expect(full.model.environment.shapes).toHaveLength(1)
  expect(full.model.environment.shapes[0].type).toBe('rectangle')
})

// ---------------------------------------------------------------------------
// MDE-10 — Shapes: fixed vs strategy, ScalarSpec, validation, delete
// ---------------------------------------------------------------------------

test('MDE-10 shapes: fixed rectangle (position, pop 1) + strategy circle (count, radius range), validation issue, delete', async ({ page }) => {
  test.setTimeout(150_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde10')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /environment/i)
  const addBtn = page.getByTestId('add-shape')
  await expect(addBtn).toBeVisible({ timeout: 20_000 })

  // --- FIXED rectangle (picker defaults: rectangle + Fixed (single)) ---
  await addBtn.click()
  const picker = page.getByTestId('add-shape-picker')
  await expect(picker).toBeVisible()
  await page.getByTestId('add-shape-confirm').click()

  await expect(page.getByTestId('shape-type')).toContainText('rectangle')
  await expect(page.getByTestId('geom-pos-x')).toBeVisible()
  await expect(page.getByTestId('geom-pos-y')).toBeVisible()
  await expect(page.getByTestId('geom-strategy-note')).toBeHidden()
  // Fixed placement → the count controls are hidden (population forced to 1).
  await expect(page.getByTestId('init-count-mode')).toHaveCount(0)
  await expect(page.getByTestId('init-population-count')).toHaveCount(0)
  await setNumberByTestId(page, 'geom-pos-x', 5)
  await setNumberByTestId(page, 'geom-pos-y', 6)
  await setNumberByTestId(page, 'geom-width-value', 8) // ScalarSpec FIXED
  await setNumberByTestId(page, 'geom-height-value', 4)

  // --- STRATEGY circle (random) ---
  await addBtn.click()
  await picker.getByRole('combobox').nth(0).click()
  await pickOption(page, 'circle')
  await picker.getByRole('combobox').nth(1).click()
  await pickOption(page, 'Random')
  await page.getByTestId('add-shape-confirm').click()

  await expect(page.getByTestId('shape-type')).toContainText('circle')
  await expect(page.getByTestId('geom-strategy-note')).toBeVisible()
  await expect(page.getByTestId('geom-pos-x')).toHaveCount(0)
  await expect(page.getByTestId('init-population-count')).toBeVisible()
  await setNumberByTestId(page, 'init-population-count', 5)

  // ScalarSpec RANGE on the radius — invalid (min > max) → inline error + shape-issues.
  await page.getByTestId('geom-radius').getByText('Range', { exact: true }).click()
  await setNumberByTestId(page, 'geom-radius-min', 3)
  await setNumberByTestId(page, 'geom-radius-max', 1)
  await expect(page.getByTestId('geom-radius-error')).toBeVisible()
  await expect(page.getByTestId('shape-issues')).toBeVisible()
  // Fix the range → both clear.
  await setNumberByTestId(page, 'geom-radius-max', 4)
  await expect(page.getByTestId('geom-radius-error')).toHaveCount(0)
  await expect(page.getByTestId('shape-issues')).toHaveCount(0)

  let body = await saveModel(page)
  let shapes = body.model.environment.shapes
  expect(shapes).toHaveLength(2)
  const rect = shapes.find((s: any) => s.type === 'rectangle')
  expect(rect.geometry.position).toEqual(expect.objectContaining({ x: 5, y: 6 }))
  expect(rect.geometry.width.value).toBe(8)
  expect(rect.geometry.height.value).toBe(4)
  expect(rect.initialization.population_count).toBe(1)
  expect(rect.initialization.placement).toBeFalsy()
  const circle = shapes.find((s: any) => s.type === 'circle')
  expect(circle.initialization.placement.type).toBe('random')
  expect(circle.initialization.population_count).toBe(5)
  expect(circle.geometry.position).toBeFalsy()
  expect(circle.geometry.radius.min).toBe(3)
  expect(circle.geometry.radius.max).toBe(4)
  expect(circle.geometry.radius.value).toBeFalsy()

  // --- delete the circle (still selected) ---
  await page.getByRole('button', { name: 'Remove shape' }).click()
  body = await saveModel(page)
  shapes = body.model.environment.shapes
  expect(shapes).toHaveLength(1)
  expect(shapes[0].type).toBe('rectangle')
})

// ---------------------------------------------------------------------------
// MDE-11 — Rules: build one rule fully via UI, edit, remove
// ---------------------------------------------------------------------------

test('MDE-11 rules: build step-gte-2 ⇒ set_property rule with priority/cooldown; save round-trips; edit priority; remove', async ({ page }) => {
  test.setTimeout(150_000)
  const m = await seedModel(api, minimalSpatial(uniqueName('mde11')))
  trackId(m.id)

  await login(page)
  await openModelEditor(page, m.id)
  await openTab(page, /rules/i)

  await page.getByRole('button', { name: 'Add rule' }).click()
  await fillField(page, 'ID', 'r_e2e')
  await fillField(page, 'Name', 'e2e step rule')
  await fillField(page, 'Priority', 7)
  await fillField(page, 'Cooldown steps', 2)

  // Applies to → the fixture's Walker archetype.
  await field(page, 'Applies to').locator('.p-multiselect').click()
  await pickOption(page, 'Walker')
  await page.keyboard.press('Escape')

  // Condition: step gte 2 (new conditions default to type "property").
  await page.getByRole('button', { name: 'Add condition' }).click()
  const condCard = page.locator('div.bg-zinc-50.rounded-lg').filter({
    has: page.locator('label:text-is("Condition type")'),
  })
  await selectInField(page, condCard, 'Condition type', 'step')
  await selectInField(page, condCard, 'Operator', 'gte')
  await fillField(condCard, 'Step', 2)

  // Action: set_property (the default) on the agent's `energy` property.
  await page.getByRole('button', { name: 'Add action' }).click()
  const actionCard = page.locator('div.bg-zinc-50.rounded-lg').filter({
    has: page.locator('label:text-is("Action type")'),
  })
  await expect(field(actionCard, 'Action type').getByRole('combobox').first()).toContainText('set_property')
  // Property UID is an EDITABLE Select — clicking its input does not open the
  // overlay, so type the uid directly (the editable input emits on input).
  const propInput = field(actionCard, 'Property UID').locator('input').first()
  await propInput.click()
  await propInput.fill('energy')
  await propInput.press('Tab')
  await selectInField(page, actionCard, 'Modifier', 'set')
  await fillField(actionCard, 'Value', 10)

  let body = await saveModel(page)
  expect(body.model.rules).toHaveLength(1)
  let rule = body.model.rules[0]
  expect(rule.id).toBe('r_e2e')
  expect(rule.name).toBe('e2e step rule')
  expect(rule.priority).toBe(7)
  expect(rule.cooldown_steps).toBe(2)
  expect(rule.applies_to).toEqual(['walker'])
  expect(rule.conditions).toHaveLength(1)
  expect(rule.conditions[0].type).toBe('step')
  expect(rule.conditions[0].operator).toBe('gte')
  expect(Number(rule.conditions[0].value)).toBe(2)
  expect(rule.actions).toHaveLength(1)
  expect(rule.actions[0].type).toBe('set_property')
  expect(rule.actions[0].property_uid).toBe('energy')
  expect(rule.actions[0].modifier).toBe('set')
  expect(String(rule.actions[0].value)).toBe('10')

  // [API] the persisted rule matches too.
  const full = await apiJson<any>(api, 'get', `/models/${m.id}`, undefined, 200)
  expect(full.model.rules[0].id).toBe('r_e2e')
  expect(full.model.rules[0].priority).toBe(7)

  // Edit: change the priority → save → persisted.
  await fillField(page, 'Priority', 9)
  body = await saveModel(page)
  expect(body.model.rules[0].priority).toBe(9)

  // Remove the rule → list empties → save → no rules persisted.
  await page.getByRole('button', { name: 'Remove rule' }).click()
  await expect(page.getByText('No rules yet.')).toBeVisible()
  body = await saveModel(page)
  expect(body.model.rules ?? []).toHaveLength(0)
})
