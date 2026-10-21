import { test, expect, type APIRequestContext, type Locator, type Page } from '@playwright/test'
import { login } from '../_helpers/login'
import { apiContext, apiJson, pollUntil, u, type ApiErrorBody } from '../_helpers/api'
import { deleteQuietly, rulesRich, seedModel, uniqueName } from '../_helpers/fixtures'

// MWD — Widgets authoring & validation (plan §5.5). Deterministic, no LLM.
// Each test gets its own rulesRich model (2 archetypes runner/beacon with
// number props ticks/meetings/pulses + env global alarm_count) so widget
// sources have real targets. Everything is uniqueName()-seeded and deleted
// in afterAll.
//
// Adjacent legacy specs (run separately, see report):
//   • e2e/widget-agent-labels.spec.ts  (MWD-03 wording/labels)
//   • e2e/dashboard-layout.spec.ts     (MWD-05 arrange/no-overlap/run-screen)

let api: APIRequestContext
const createdModelIds: string[] = []

let modelA: { id: string; name: string } // MWD-01: 7 widget types via UI
let modelB: { id: string; name: string } // MWD-02/03: validation matrix + sim_metric depth (no persisted widgets)
let modelBBody: Record<string, any> // kept for the API-negative PUT
let modelC: { id: string; name: string } // MWD-04: series editor
let modelD: { id: string; name: string } // MWD-05: arrange layout drag

// MWD-05 seed widgets: valid monitors at known, non-overlapping positions so
// needsAutoArrange() is false and any position change is attributable to the drag.
const MWD5_POSITIONS = [
  { x: 0, y: 0, w: 3, h: 2 },
  { x: 3, y: 0, w: 3, h: 2 },
  { x: 6, y: 0, w: 3, h: 2 },
]
function mwd5Widgets() {
  return MWD5_POSITIONS.map((position, i) => ({
    id: `w_p${i + 1}`,
    type: 'monitor',
    title: `P${i + 1}`,
    source: { type: 'population' },
    position,
  }))
}

test.beforeAll(async () => {
  api = await apiContext()
  modelA = await seedModel(api, rulesRich(uniqueName('mwd-add')))
  createdModelIds.push(modelA.id)
  modelBBody = rulesRich(uniqueName('mwd-neg'))
  modelB = await seedModel(api, modelBBody)
  createdModelIds.push(modelB.id)
  modelC = await seedModel(api, rulesRich(uniqueName('mwd-series')))
  createdModelIds.push(modelC.id)
  const dBody = rulesRich(uniqueName('mwd-arrange'))
  dBody.widgets = mwd5Widgets()
  modelD = await seedModel(api, dBody)
  createdModelIds.push(modelD.id)
})

test.afterAll(async () => {
  for (const id of createdModelIds) await deleteQuietly(api, `/models/${id}`)
  await api.dispose()
})

// ---------------------------------------------------------------------------
// UI helpers
// ---------------------------------------------------------------------------

async function openWidgetsTab(page: Page, modelId: string, modelName: string) {
  await login(page)
  await page.goto(`/models/${modelId}`)
  await expect(page.getByPlaceholder('Model name')).toHaveValue(modelName, { timeout: 20_000 })
  await page.getByRole('tab', { name: /widgets/i }).click()
  await expect(page.getByRole('button', { name: 'Add widget' })).toBeVisible({ timeout: 15_000 })
}

async function openAddWidget(page: Page): Promise<Locator> {
  await page.getByRole('button', { name: 'Add widget' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('Add widget', { exact: true })).toBeVisible({ timeout: 15_000 })
  return dialog
}

// The widget-type picker buttons carry "<icon glyph> <label> <description>" as
// their accessible name — the PrimeIcons ::before glyph char LEADS the name, so
// an anchored /^Label/ regex never matches. Use an unanchored regex (the 7
// labels are mutually non-substrings, so each stays unique in the dialog).
function typeButton(dialog: Locator, label: string): Locator {
  return dialog.getByRole('button', { name: new RegExp(label) })
}

// A labelled PrimeVue Select in the editor/source builder: the wrapper div holds
// <label>…</label> followed by the .p-select root.
function fieldSelect(scope: Locator, label: string): Locator {
  return scope.locator(`div:has(> label:text-is("${label}")) > .p-select`).last()
}

// The Agents / Columns MultiSelects (the Agents label carries a qualifier span,
// so match on prefix).
function agentsMultiSelect(dialog: Locator): Locator {
  return dialog.locator('div:has(> label:has-text("Agents")) > .p-multiselect').last()
}
function columnsMultiSelect(dialog: Locator): Locator {
  return dialog.locator('div:has(> label:text-is("Columns")) > .p-multiselect').last()
}

// PrimeVue overlay roots for Select / MultiSelect dropdown panels. A closing
// overlay keeps its options visible during the fade-out transition, so every
// interaction below scopes to the CURRENT (last-mounted) overlay and waits for
// full detach before returning — otherwise option reads bleed across pickers.
const OVERLAY = '.p-select-overlay, .p-multiselect-overlay'

async function openOverlay(page: Page, trigger: Locator): Promise<Locator> {
  await trigger.click()
  const overlay = page.locator(OVERLAY).last()
  await expect(overlay).toBeVisible({ timeout: 10_000 })
  return overlay
}

async function overlaysGone(page: Page) {
  await expect(page.locator(OVERLAY)).toHaveCount(0)
}

// Pick one option in a single-select (its overlay auto-closes on pick).
async function pickOption(page: Page, trigger: Locator, option: string) {
  const overlay = await openOverlay(page, trigger)
  await overlay.getByRole('option', { name: option, exact: true }).click()
  await overlaysGone(page)
}

// Overlay-close strategies. Hard-won notes (each alternative below flaked at
// least once under parallel-agent CPU load):
//   • Escape can bubble past the overlay to the Dialog → closes it UNSAVED.
//   • Re-clicking the MultiSelect trigger can land on a selected chip / its
//     remove icon → deselects instead of toggling.
//   • Outside-clicks (dialog title) intermittently miss the overlay's
//     outside-click listener; option clicks can hit-test to <html> while the
//     overlay repositions.
// So: MultiSelect option toggles + trigger close go through dispatchEvent
// (real component handlers, no hit-testing); single Selects close by
// re-selecting the currently selected option (a no-op that hides the overlay).

// Toggle options in a MultiSelect (its overlay stays open), then close it by
// dispatching a click on the trigger (root handler toggles → hides).
async function pickMultiOptions(page: Page, trigger: Locator, options: string[]) {
  const overlay = await openOverlay(page, trigger)
  for (const o of options) {
    const opt = overlay.getByRole('option', { name: o, exact: true })
    await expect(opt).toBeVisible({ timeout: 10_000 })
    await opt.dispatchEvent('click')
  }
  await trigger.dispatchEvent('click')
  await overlaysGone(page)
}

// Open a single-Select overlay, read all option labels, then close it by
// re-selecting the CURRENT value (PrimeVue hides the overlay, state unchanged).
// NOTE: PrimeVue 4 Select options select on MOUSEDOWN (their onClick is a
// stop-propagation no-op), so the dismiss dispatches 'mousedown'.
// Callers only use this on Selects that already have a selected value.
async function listOptions(page: Page, trigger: Locator): Promise<string[]> {
  const current = (await trigger.innerText()).trim()
  const overlay = await openOverlay(page, trigger)
  const opts = overlay.getByRole('option')
  await expect(opts.first()).toBeVisible({ timeout: 10_000 })
  const texts = (await opts.allInnerTexts()).map((t) => t.trim())
  const dismissVia = texts.includes(current) ? current : texts[0]!
  await overlay.getByRole('option', { name: dismissVia, exact: true }).dispatchEvent('mousedown')
  await overlaysGone(page)
  return texts
}

async function saveWidget(page: Page, dialog: Locator) {
  const save = dialog.getByRole('button', { name: 'Save widget' })
  await expect(save).toBeEnabled()
  try {
    await save.click({ timeout: 10_000 })
  } catch {
    // Rare under parallel-agent CPU load: the footer button reports "not
    // stable"/detaches mid-retry while the dialog re-renders. The button is
    // asserted enabled above, so fall back to a programmatic click.
    if (await dialog.isVisible().catch(() => false)) await save.dispatchEvent('click')
  }
  await expect(dialog).toBeHidden()
}

async function saveModel(page: Page) {
  await page.getByRole('button', { name: 'Save', exact: true }).click()
}

async function fetchWidgets(modelId: string): Promise<any[]> {
  const res = await apiJson<any>(api, 'get', `/models/${modelId}`, undefined, 200)
  return res.model?.widgets ?? []
}

// ---------------------------------------------------------------------------
// MWD-01 — add each of the 7 widget types via the UI WidgetEditor
// ---------------------------------------------------------------------------

test('MWD-01 add all 7 widget types via WidgetEditor; tiles appear; API-verifies types + sources', async ({ page }) => {
  test.setTimeout(180_000)
  await openWidgetsTab(page, modelA.id, modelA.name)

  // 1. monitor + sim_metric total_agents ------------------------------------
  let dialog = await openAddWidget(page)
  await dialog.getByPlaceholder('My widget').fill('MWD monitor')
  await pickOption(page, fieldSelect(dialog, 'Source type'), 'Simulation metric')
  // onTypeChange auto-seeds metric=total_agents; the Metric select reflects it.
  await expect(fieldSelect(dialog, 'Metric')).toContainText('Total agents')
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD monitor', { exact: true })).toBeVisible()

  // 2. line_chart + agent_property avg ticks series --------------------------
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Line chart').click()
  await dialog.getByPlaceholder('My widget').fill('MWD line')
  await dialog.getByRole('button', { name: 'Add series' }).click()
  await dialog.getByPlaceholder('Series label').fill('Avg ticks')
  await pickOption(page, fieldSelect(dialog, 'Source type'), 'Agent property')
  await pickOption(page, fieldSelect(dialog, 'Property'), 'ticks')
  await expect(fieldSelect(dialog, 'Aggregation')).toContainText('avg')
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD line', { exact: true })).toBeVisible()

  // 3. bar_chart + population (single-source mode, the default) --------------
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Bar chart').click()
  await dialog.getByPlaceholder('My widget').fill('MWD bar')
  await expect(fieldSelect(dialog, 'Source type')).toContainText('Population')
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD bar', { exact: true })).toBeVisible()

  // 4. pie_chart + state_distribution (runner, exactly one agent) ------------
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Pie chart').click()
  await dialog.getByPlaceholder('My widget').fill('MWD pie')
  await pickOption(page, fieldSelect(dialog, 'Source type'), 'State distribution')
  await pickMultiOptions(page, agentsMultiSelect(dialog), ['Runner'])
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD pie', { exact: true })).toBeVisible()

  // 5. histogram + agent_property ticks + buckets -----------------------------
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Histogram').click()
  await dialog.getByPlaceholder('My widget').fill('MWD histogram')
  await pickOption(page, fieldSelect(dialog, 'Property'), 'ticks')
  const buckets = dialog.locator('div:has(> label:has-text("Buckets")) input')
  await buckets.fill('5')
  await buckets.press('Tab')
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD histogram', { exact: true })).toBeVisible()

  // 6. agent_table + exactly one agent + columns ------------------------------
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Agent table').click()
  await dialog.getByPlaceholder('My widget').fill('MWD table')
  await pickMultiOptions(page, agentsMultiSelect(dialog), ['Runner'])
  await pickMultiOptions(page, columnsMultiSelect(dialog), ['ticks', 'meetings'])
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD table', { exact: true })).toBeVisible()

  // 7. text_log + events kind --------------------------------------------------
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Text log').click()
  await dialog.getByPlaceholder('My widget').fill('MWD log')
  await dialog.getByText('Events', { exact: true }).click()
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD log', { exact: true })).toBeVisible()

  // All 7 tiles listed in the preview grid.
  await expect(page.getByText('7 widget(s)', { exact: true })).toBeVisible()

  // Persist + API-verify types and source types.
  await saveModel(page)
  const widgets = await pollUntil(async () => {
    const w = await fetchWidgets(modelA.id)
    return w.length === 7 ? w : null
  }, { timeoutMs: 15_000, label: '7 widgets persisted' })

  const byTitle = new Map(widgets.map((w: any) => [w.title, w]))
  const mon = byTitle.get('MWD monitor')
  expect(mon.type).toBe('monitor')
  expect(mon.source?.type).toBe('sim_metric')
  expect(mon.source?.metric).toBe('total_agents')

  const line = byTitle.get('MWD line')
  expect(line.type).toBe('line_chart')
  expect(line.series?.length).toBe(1)
  expect(line.series[0].label).toBe('Avg ticks')
  expect(line.series[0].source?.type).toBe('agent_property')
  expect(line.series[0].source?.property_uid).toBe('ticks')
  expect(line.series[0].source?.aggregation).toBe('avg')

  const bar = byTitle.get('MWD bar')
  expect(bar.type).toBe('bar_chart')
  expect(bar.source?.type).toBe('population')

  const pie = byTitle.get('MWD pie')
  expect(pie.type).toBe('pie_chart')
  expect(pie.source?.type).toBe('state_distribution')
  expect(pie.source?.agent_uids).toEqual(['runner'])

  const hist = byTitle.get('MWD histogram')
  expect(hist.type).toBe('histogram')
  expect(hist.source?.type).toBe('agent_property')
  expect(hist.source?.property_uid).toBe('ticks')
  expect(hist.buckets).toBe(5)

  const table = byTitle.get('MWD table')
  expect(table.type).toBe('agent_table')
  expect(table.source?.type).toBe('agent_property')
  expect(table.source?.agent_uids).toEqual(['runner'])
  expect(table.columns).toEqual(['ticks', 'meetings'])

  const log = byTitle.get('MWD log')
  expect(log.type).toBe('text_log')
  expect(log.source ?? null).toBeNull()
  expect(log.log_kinds).toEqual(['events'])
})

// ---------------------------------------------------------------------------
// MWD-02 — validation matrix (negative)
// ---------------------------------------------------------------------------

test('MWD-02 validation matrix: sim_metric hidden off monitor/line-series; per-type issues disable Save; API PUT 422', async ({ page }) => {
  test.setTimeout(120_000)
  await openWidgetsTab(page, modelB.id, modelB.name)
  const dialog = await openAddWidget(page)
  const issuesPanel = dialog.locator('.bg-red-50')
  const saveBtn = dialog.getByRole('button', { name: 'Save widget' })

  // (a) bar_chart never offers sim_metric — neither in single-source mode nor
  //     for series sources (metric is allowed only on monitor + line series).
  await typeButton(dialog, 'Bar chart').click()
  const singleOpts = await listOptions(page, fieldSelect(dialog, 'Source type'))
  expect(singleOpts).toEqual(['Population', 'State distribution'])
  // Switch to Series mode (SelectButton: 'Population / states' | 'Series').
  await dialog.getByRole('button', { name: 'Series', exact: true }).click()
  await dialog.getByRole('button', { name: 'Add series' }).click()
  const barSeriesOpts = await listOptions(page, fieldSelect(dialog, 'Source type'))
  expect(barSeriesOpts).toEqual(['Agent property', 'Global property', 'Population'])
  expect(barSeriesOpts).not.toContain('Simulation metric')

  // (b1) histogram: SPEC-MISMATCH vs plan wording — histogram does NOT require
  //      an agent selection; its blocking requirement is a property. Assert the
  //      real matrix: no property ⇒ Save disabled + issue listed.
  await typeButton(dialog, 'Histogram').click()
  await dialog.getByPlaceholder('My widget').fill('neg histogram')
  await expect(issuesPanel).toContainText('histogram requires a property')
  await expect(saveBtn).toBeDisabled()

  // (b2) state_distribution (pie) requires exactly one agent.
  await typeButton(dialog, 'Pie chart').click()
  await dialog.getByPlaceholder('My widget').fill('neg pie')
  await pickOption(page, fieldSelect(dialog, 'Source type'), 'State distribution')
  await expect(issuesPanel).toContainText('state_distribution requires exactly one agent')
  await expect(saveBtn).toBeDisabled()

  // (b3) agent_table requires exactly one agent (+ at least one column).
  await typeButton(dialog, 'Agent table').click()
  await dialog.getByPlaceholder('My widget').fill('neg table')
  await expect(issuesPanel).toContainText('agent_table requires exactly one agent')
  await expect(issuesPanel).toContainText('agent_table requires at least one column')
  await expect(saveBtn).toBeDisabled()

  // (c) text_log with no kinds ⇒ blocked.
  await typeButton(dialog, 'Text log').click()
  await dialog.getByPlaceholder('My widget').fill('neg log')
  await expect(issuesPanel).toContainText('text_log requires at least one log kind')
  await expect(saveBtn).toBeDisabled()
  // Checking a kind clears the issue and enables Save (we cancel — nothing persists).
  await dialog.getByText('Events', { exact: true }).click()
  await expect(saveBtn).toBeEnabled()
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(dialog).toBeHidden()

  // (d) [API] PUT a pie_chart carrying a sim_metric source ⇒ 422 validation_failed
  //     with the exact widget-matrix field error.
  const bad = JSON.parse(JSON.stringify(modelBBody))
  bad.widgets = [
    {
      id: 'w_bad',
      type: 'pie_chart',
      title: 'Bad pie',
      source: { type: 'sim_metric', metric: 'total_agents' },
      position: { x: 0, y: 0, w: 4, h: 3 },
    },
  ]
  const res = await api.put(u(`/models/${modelB.id}`), { data: { model: bad } })
  expect(res.status(), await res.text()).toBe(422)
  const body = (await res.json()) as ApiErrorBody
  expect(body.code).toBe('validation_failed')
  expect(body.message).toBe('model failed field-shape validation')
  expect(body.fields?.['widgets[0].source.type']).toBe(
    'source type "sim_metric" is not allowed for this widget',
  )
  // The rejected PUT persisted nothing.
  expect((await fetchWidgets(modelB.id)).length).toBe(0)
})

// ---------------------------------------------------------------------------
// MWD-03 — sim_metric depth: 7 counters; offered only on monitor + line series
// ---------------------------------------------------------------------------

const ALL_METRICS = [
  'Total agents',
  'Agents spawned',
  'Agents removed',
  'Interactions',
  'Events',
  'Rule executions',
  'Movement distance',
]

test('MWD-03 sim_metric: metric dropdown lists all 7 counters on monitor and line-chart series only', async ({ page }) => {
  test.setTimeout(120_000)
  await openWidgetsTab(page, modelB.id, modelB.name)
  const dialog = await openAddWidget(page)

  // monitor (default type): source types include Simulation metric.
  const monitorSourceOpts = await listOptions(page, fieldSelect(dialog, 'Source type'))
  expect(monitorSourceOpts).toEqual(['Agent property', 'Global property', 'Population', 'Simulation metric'])
  await pickOption(page, fieldSelect(dialog, 'Source type'), 'Simulation metric')
  const monitorMetrics = await listOptions(page, fieldSelect(dialog, 'Metric'))
  expect(monitorMetrics).toEqual(ALL_METRICS)

  // line_chart series: sim_metric offered; metric dropdown lists the same 7.
  await typeButton(dialog, 'Line chart').click()
  await dialog.getByRole('button', { name: 'Add series' }).click()
  const lineSeriesOpts = await listOptions(page, fieldSelect(dialog, 'Source type'))
  expect(lineSeriesOpts).toEqual(['Agent property', 'Global property', 'Population', 'Simulation metric'])
  await pickOption(page, fieldSelect(dialog, 'Source type'), 'Simulation metric')
  const lineMetrics = await listOptions(page, fieldSelect(dialog, 'Metric'))
  expect(lineMetrics).toEqual(ALL_METRICS)

  // pie_chart / histogram / agent_table never offer it.
  await typeButton(dialog, 'Pie chart').click()
  expect(await listOptions(page, fieldSelect(dialog, 'Source type'))).toEqual(['Population', 'State distribution'])
  await typeButton(dialog, 'Histogram').click()
  expect(await listOptions(page, fieldSelect(dialog, 'Source type'))).toEqual(['Agent property'])
  await typeButton(dialog, 'Agent table').click()
  expect(await listOptions(page, fieldSelect(dialog, 'Source type'))).toEqual(['Agent property'])

  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(dialog).toBeHidden()
})

// ---------------------------------------------------------------------------
// MWD-04 — series editor: add/label/color/remove; bar_chart mode toggle
// ---------------------------------------------------------------------------

test('MWD-04 series editor: 2 labelled+coloured series, remove one; bar_chart mode toggle; API-verifies colors', async ({ page }) => {
  test.setTimeout(150_000)
  await openWidgetsTab(page, modelC.id, modelC.name)

  // line_chart with 3 series → remove the 3rd → 2 persist.
  let dialog = await openAddWidget(page)
  await typeButton(dialog, 'Line chart').click()
  await dialog.getByPlaceholder('My widget').fill('MWD series line')
  for (let i = 0; i < 3; i++) await dialog.getByRole('button', { name: 'Add series' }).click()
  await expect(dialog.getByPlaceholder('Series label')).toHaveCount(3)
  await dialog.getByPlaceholder('Series label').nth(0).fill('S1')
  await dialog.getByPlaceholder('Series label').nth(1).fill('S2')
  await dialog.getByPlaceholder('Series label').nth(2).fill('S3')

  // Change series 2's color via the ColorPicker panel (top-left ≈ white; the
  // exact hex is irrelevant — it must differ from the default #8B5CF6).
  const s2Preview = dialog.locator('.p-colorpicker').nth(1)
  await s2Preview.click()
  const colorArea = page.locator('.p-colorpicker-color-selector')
  await expect(colorArea).toBeVisible()
  await colorArea.click({ position: { x: 2, y: 2 } })
  await s2Preview.click() // toggle the panel closed
  await expect(colorArea).toBeHidden()

  // Remove the 3rd series.
  await dialog.getByRole('button', { name: 'Remove series' }).nth(2).click()
  await expect(dialog.getByPlaceholder('Series label')).toHaveCount(2)
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD series line', { exact: true })).toBeVisible()

  // bar_chart mode toggle: exactly two modes — 'Population / states' (single
  // source) and 'Series' (series editor, no sim_metric). Save one in series mode.
  dialog = await openAddWidget(page)
  await typeButton(dialog, 'Bar chart').click()
  await dialog.getByPlaceholder('My widget').fill('MWD series bar')
  const modeButtons = dialog.locator('.p-selectbutton button')
  await expect(modeButtons).toHaveCount(2)
  await expect(modeButtons.nth(0)).toContainText('Population / states')
  await expect(modeButtons.nth(1)).toContainText('Series')
  await dialog.getByRole('button', { name: 'Series', exact: true }).click()
  await expect(dialog.getByText('Add at least one series.')).toBeVisible()
  await dialog.getByRole('button', { name: 'Add series' }).click()
  await dialog.getByPlaceholder('Series label').fill('B1')
  await saveWidget(page, dialog)
  await expect(page.getByText('MWD series bar', { exact: true })).toBeVisible()

  // Persist + API-verify the series with colors.
  await saveModel(page)
  const widgets = await pollUntil(async () => {
    const w = await fetchWidgets(modelC.id)
    return w.length === 2 ? w : null
  }, { timeoutMs: 15_000, label: '2 series widgets persisted' })

  const line = widgets.find((w: any) => w.title === 'MWD series line')
  expect(line.type).toBe('line_chart')
  expect(line.series?.length).toBe(2)
  expect(line.series[0].label).toBe('S1')
  expect(line.series[0].color).toBe('#8B5CF6') // editor default
  expect(line.series[1].label).toBe('S2')
  expect(line.series[1].color).toMatch(/^#[0-9a-fA-F]{6}$/)
  expect(line.series[1].color.toUpperCase()).not.toBe('#8B5CF6')

  const bar = widgets.find((w: any) => w.title === 'MWD series bar')
  expect(bar.type).toBe('bar_chart')
  expect(bar.source ?? null).toBeNull()
  expect(bar.series?.length).toBe(1)
  expect(bar.series[0].label).toBe('B1')
  expect(bar.series[0].color).toBe('#8B5CF6')
})

// ---------------------------------------------------------------------------
// MWD-05 — arrange layout: drag a tile, save, position.x/y round-trips
// ---------------------------------------------------------------------------

test('MWD-05 Arrange layout: drag a tile to a new position; Save persists position.x/y (API round-trip)', async ({ page }) => {
  test.setTimeout(120_000)
  await openWidgetsTab(page, modelD.id, modelD.name)

  // Sanity: the seeded layout is exactly what we PUT (no auto-arrange rewrite).
  const before = await fetchWidgets(modelD.id)
  expect(before.map((w: any) => w.position)).toEqual(MWD5_POSITIONS)

  await page.getByRole('button', { name: 'Arrange layout' }).click()
  const dialog = page.getByRole('dialog')
  // Spatial model → fixed world pane placeholder + 3 draggable tiles.
  await expect(dialog.getByText('Environment / World')).toBeVisible({ timeout: 20_000 })
  const tiles = dialog.locator('.vgl-item:not(.vgl-item--placeholder)')
  await expect(tiles).toHaveCount(3)

  // Drag the P1 tile right+down (grid-layout-plus mouse drag). Vertical
  // compaction may pull y back up, so the reliable delta is horizontal.
  const p1 = tiles.filter({ hasText: 'P1' })
  const box = await p1.boundingBox()
  expect(box).toBeTruthy()
  await page.mouse.move(box!.x + box!.width / 2, box!.y + 10)
  await page.mouse.down()
  await page.mouse.move(box!.x + box!.width / 2 + 180, box!.y + 130, { steps: 15 })
  await page.mouse.up()
  await page.waitForTimeout(400)

  // Close the dialog and persist through the editor's Save.
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  await saveModel(page)

  const after = await pollUntil(async () => {
    const w = await fetchWidgets(modelD.id)
    const p = w.find((x: any) => x.title === 'P1')
    return p && (p.position.x !== 0 || p.position.y !== 0) ? w : null
  }, { timeoutMs: 15_000, label: 'dragged P1 position persisted' })

  const p1After = after.find((w: any) => w.title === 'P1')
  expect(p1After.position.x !== 0 || p1After.position.y !== 0, `P1 moved: ${JSON.stringify(p1After.position)}`).toBe(true)
  // Sizes are untouched by a pure drag.
  expect(p1After.position.w).toBe(3)
  expect(p1After.position.h).toBe(2)

  // The saved layout stays non-overlapping (grid-layout-plus guarantee).
  const overlaps = (a: any, b: any) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h
  for (let i = 0; i < after.length; i++) {
    for (let j = i + 1; j < after.length; j++) {
      expect(
        overlaps(after[i].position, after[j].position),
        `widgets ${after[i].title}/${after[j].title} overlap after drag`,
      ).toBe(false)
    }
  }
})
