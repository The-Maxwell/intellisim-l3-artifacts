import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the dashboard widget layout feature (2026-07-08):
//   • A full-screen "Arrange layout" dialog (opened from the Widgets tab of both
//     the model and simulation editors) where the user drags/resizes widgets on a
//     grid with guaranteed no-overlap (grid-layout-plus).
//   • The world/environment occupies a FIXED pane on the LEFT (spatial models);
//     widgets live in a free-placement grid on the RIGHT. Non-spatial models have
//     no world pane — widgets fill the full area.
//   • The live run screen is rewritten to render the world + widgets from their
//     saved positions (replacing the old fixed Splitpanes).
// The feature is frontend-only: widget positions reuse the existing (already
// persisted) WidgetConfig.position; the world region is a fixed client-side pane.

const SHOTS = process.env.E2E_SHOT_DIR || 'test-results'

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
    const res = await api(page, 'get', `/api/v1/intellisim/simulations/${simId}`)
    expect(res.ok(), `get simulation failed: ${res.status()}`).toBeTruthy()
    last = await res.json()
    if (want.includes(last.status)) return last
    if (['generation_failed', 'failed'].includes(last.status) && !want.includes(last.status)) {
      throw new Error(`simulation entered ${last.status}: ${last.status_text}`)
    }
    await page.waitForTimeout(500)
  }
  throw new Error(`timeout waiting for status ${want.join('|')}; last=${last?.status}`)
}

type Pos = { x: number; y: number; w: number; h: number }
function overlaps(a: Pos, b: Pos): boolean {
  return a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h
}
// Assert no two widget positions overlap (the core "no-overlay" guarantee).
function assertNoOverlap(widgets: { position: Pos }[]) {
  for (let i = 0; i < widgets.length; i++) {
    for (let j = i + 1; j < widgets.length; j++) {
      expect(
        overlaps(widgets[i]!.position, widgets[j]!.position),
        `widgets ${i} and ${j} overlap: ${JSON.stringify(widgets[i]!.position)} vs ${JSON.stringify(widgets[j]!.position)}`,
      ).toBeFalsy()
    }
  }
}

// Four population/log widgets — all valid without a property_uid — deliberately
// seeded at the origin so the arrange dialog's auto-arrange must flow them.
function seedWidgets() {
  return [
    { id: 'w-mon', type: 'monitor', title: 'Population', source: { type: 'population' }, position: { x: 0, y: 0, w: 4, h: 3 } },
    { id: 'w-pie', type: 'pie_chart', title: 'Share', source: { type: 'population' }, position: { x: 0, y: 0, w: 4, h: 3 } },
    { id: 'w-bar', type: 'bar_chart', title: 'By type', source: { type: 'population' }, position: { x: 0, y: 0, w: 4, h: 3 } },
    { id: 'w-log', type: 'text_log', title: 'Events', log_kinds: ['events'], position: { x: 0, y: 0, w: 4, h: 4 } },
  ]
}

test('spatial: arrange dialog (world pane + no-overlap widgets) → run screen honors layout', async ({ page }) => {
  test.setTimeout(240_000)
  await login(page)
  token = await getToken(page)

  // 1. Spatial model with one movable agent + the seeded widgets (all at 0,0).
  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Dashboard Layout (spatial)',
      is_spatial: true,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const modelId: string = created.id
  const model = created.model
  model.is_spatial = true
  model.spatial_cell_size = 10
  model.environment = { uid: 'environment', width: 100, height: 80, torus: false, global_properties: [], shapes: [] }
  model.agents = [
    {
      uid: 'wolf', name: 'Wolf', type: 'agent', move_type: 'movable',
      properties: [{ uid: 'energy', name: 'Energy', type: 'number', default_value: 10 }],
      states: ['hunting', 'resting'],
      initialization: { population_count: 4, count_mode: 'fixed', initial_state: 'hunting', placement: { type: 'random' } },
    },
  ]
  model.widgets = seedWidgets()
  const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
  expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

  // 2. Open model editor → Widgets tab → Arrange layout.
  await page.goto(`/models/${modelId}`)
  await page.getByRole('tab', { name: /widgets/i }).click()
  await page.getByRole('button', { name: /arrange layout/i }).click()

  const dialog = page.getByRole('dialog')
  // Spatial → the fixed world pane placeholder is shown.
  await expect(dialog.getByText('Environment / World')).toBeVisible({ timeout: 20_000 })
  // All four widget placeholders render (by title).
  for (const t of ['Population', 'Share', 'By type', 'Events']) {
    await expect(dialog.getByText(t, { exact: true })).toBeVisible()
  }
  // grid-layout-plus mounted four draggable items.
  await expect(dialog.locator('.vgl-item:not(.vgl-item--placeholder)')).toHaveCount(4)
  await page.waitForTimeout(400)
  await dialog.screenshot({ path: `${SHOTS}/arrange-spatial.png` })
  // eslint-disable-next-line no-console
  console.log(`ARRANGE_SPATIAL_SHOT=${SHOTS}/arrange-spatial.png`)

  // 3. Drag the first widget item a bit (exercise interactivity + reflow).
  const firstItem = dialog.locator('.vgl-item:not(.vgl-item--placeholder)').first()
  const box = await firstItem.boundingBox()
  if (box) {
    await page.mouse.move(box.x + 30, box.y + 12)
    await page.mouse.down()
    await page.mouse.move(box.x + 220, box.y + 160, { steps: 12 })
    await page.mouse.up()
    await page.waitForTimeout(300)
  }

  // 4. Close dialog, Save the model, then refetch: positions must be flowed
  //    (auto-arrange fired on open) and NON-OVERLAPPING.
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await page.waitForTimeout(1500)

  const saved = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
  const savedWidgets = saved.model.widgets as { position: Pos }[]
  expect(savedWidgets.length).toBe(4)
  const allAtOrigin = savedWidgets.every((w) => w.position.x === 0 && w.position.y === 0)
  expect(allAtOrigin, 'positions should no longer all be at the origin').toBeFalsy()
  assertNoOverlap(savedWidgets)

  // 5. Create a simulation from the model → widgets (with positions) are copied.
  const simRes = await api(page, 'post', '/api/v1/intellisim/simulations', {
    model_id: modelId, name: 'E2E dashboard layout run', steps: 6, seed: 3,
  })
  expect(simRes.ok(), `create sim failed: ${simRes.status()} ${await simRes.text()}`).toBeTruthy()
  const sim = await simRes.json()
  await waitForStatus(page, sim.id, ['ready'], 90_000)

  // 5b. The SIMULATION editor exposes the SAME arrange entry point (shared
  //     WidgetsSection). Open it, confirm the dialog + world pane, then close.
  await page.goto(`/simulations/${sim.id}`)
  await page.getByRole('tab', { name: /widgets/i }).click()
  await page.getByRole('button', { name: /arrange layout/i }).click()
  const simDialog = page.getByRole('dialog')
  await expect(simDialog.getByText('Environment / World')).toBeVisible({ timeout: 20_000 })
  await expect(simDialog.locator('.vgl-item:not(.vgl-item--placeholder)')).toHaveCount(4)
  await page.keyboard.press('Escape')

  // 6. Run to completion (strategy none → no LLM), then open the run screen.
  const run = await api(page, 'post', `/api/v1/intellisim/simulations/${sim.id}/run`, {})
  expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
  await waitForStatus(page, sim.id, ['finished'], 120_000)

  await page.goto(`/simulations/${sim.id}/run`)
  // World canvas present + sized (not 0×0 — the PixiJS-in-flex fit gotcha).
  const canvas = page.locator('canvas').first()
  await expect(canvas).toBeVisible({ timeout: 20_000 })
  await page.waitForTimeout(1500)
  const cbox = await canvas.boundingBox()
  expect(cbox && cbox.width > 50 && cbox.height > 50, `canvas mounted with size ${JSON.stringify(cbox)}`).toBeTruthy()
  // The world pane rendered (its unique view controls are present).
  await expect(page.locator('button[title="Fit to view"]')).toBeVisible()
  // Widget tiles render in the grid alongside the world pane.
  await expect(page.locator('.vgl-item:not(.vgl-item--placeholder)')).toHaveCount(4)
  await page.screenshot({ path: `${SHOTS}/run-spatial.png`, fullPage: false })
  // eslint-disable-next-line no-console
  console.log(`RUN_SPATIAL_SHOT=${SHOTS}/run-spatial.png`)
})

test('run screen auto-flows never-arranged widgets (no single-column stacking)', async ({ page }) => {
  test.setTimeout(200_000)
  await login(page)
  token = await getToken(page)

  // Spatial model whose widgets are all at (0,0) — NEVER opened in the arrange
  // dialog. The run screen must flow them for display so they don't stack into one
  // column (the old CSS grid auto-flowed; grid-layout-plus would otherwise compact).
  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Layout Legacy Flow', is_spatial: true, llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const model = created.model
  model.is_spatial = true
  model.spatial_cell_size = 10
  model.environment = { uid: 'environment', width: 80, height: 60, torus: false, global_properties: [], shapes: [] }
  model.agents = [
    {
      uid: 'a', name: 'A', type: 'agent', move_type: 'movable',
      properties: [{ uid: 'e', name: 'E', type: 'number', default_value: 1 }],
      states: ['s'], initialization: { population_count: 3, count_mode: 'fixed', initial_state: 's', placement: { type: 'random' } },
    },
  ]
  model.widgets = seedWidgets() // all at (0,0)
  expect((await api(page, 'put', `/api/v1/intellisim/models/${created.id}`, { model })).ok()).toBeTruthy()

  const simRes = await api(page, 'post', '/api/v1/intellisim/simulations', {
    model_id: created.id, name: 'E2E legacy flow run', steps: 5, seed: 1,
  })
  const sim = await simRes.json()
  await waitForStatus(page, sim.id, ['ready'], 90_000)
  // Sanity: the sim inherited the widgets still at the origin (never arranged).
  expect((sim.widgets ?? []).every((w: any) => w.position.x === 0 && w.position.y === 0)).toBeTruthy()
  expect((await api(page, 'post', `/api/v1/intellisim/simulations/${sim.id}/run`, {})).status()).toBe(202)
  await waitForStatus(page, sim.id, ['finished'], 120_000)

  await page.goto(`/simulations/${sim.id}/run`)
  const items = page.locator('.vgl-item:not(.vgl-item--placeholder)')
  await expect(items).toHaveCount(4)
  await page.waitForTimeout(800)
  // The four tiles must NOT all share one left edge — flow spread them across columns.
  const xs: number[] = []
  for (let i = 0; i < 4; i++) {
    const b = await items.nth(i).boundingBox()
    if (b) xs.push(Math.round(b.x))
  }
  const distinctX = new Set(xs)
  expect(distinctX.size, `widget tiles should occupy multiple columns, got x=${JSON.stringify(xs)}`).toBeGreaterThan(1)
})

test('non-spatial: arrange dialog has no world pane; run screen is widgets-only', async ({ page }) => {
  test.setTimeout(200_000)
  await login(page)
  token = await getToken(page)

  // 1. Non-spatial model with an agent + widgets.
  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Dashboard Layout (non-spatial)',
      is_spatial: false,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const modelId: string = created.id
  const model = created.model
  model.agents = [
    {
      uid: 'cell', name: 'Cell', type: 'agent',
      properties: [{ uid: 'value', name: 'Value', type: 'number', default_value: 1 }],
      states: ['active', 'idle'],
      initialization: { population_count: 4, count_mode: 'fixed', initial_state: 'active' },
    },
  ]
  model.widgets = seedWidgets()
  const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
  expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

  // 2. Arrange dialog: NO world pane; widgets present + auto-arranged (no overlap).
  await page.goto(`/models/${modelId}`)
  await page.getByRole('tab', { name: /widgets/i }).click()
  await page.getByRole('button', { name: /arrange layout/i }).click()

  const dialog = page.getByRole('dialog')
  await expect(dialog.locator('.vgl-item:not(.vgl-item--placeholder)')).toHaveCount(4)
  await expect(dialog.getByText('Environment / World')).toHaveCount(0)
  await expect(dialog.getByText(/Non-spatial model/i)).toBeVisible()
  await page.waitForTimeout(400)
  await dialog.screenshot({ path: `${SHOTS}/arrange-nonspatial.png` })
  // eslint-disable-next-line no-console
  console.log(`ARRANGE_NONSPATIAL_SHOT=${SHOTS}/arrange-nonspatial.png`)

  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await page.waitForTimeout(1200)
  const saved = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}`)).json()
  assertNoOverlap(saved.model.widgets as { position: Pos }[])

  // 3. Run a non-spatial sim → run screen shows the widgets-only grid (no canvas).
  const simRes = await api(page, 'post', '/api/v1/intellisim/simulations', {
    model_id: modelId, name: 'E2E non-spatial run', steps: 5, seed: 2,
  })
  expect(simRes.ok(), `create sim failed: ${simRes.status()} ${await simRes.text()}`).toBeTruthy()
  const sim = await simRes.json()
  await waitForStatus(page, sim.id, ['ready'], 90_000)
  const run = await api(page, 'post', `/api/v1/intellisim/simulations/${sim.id}/run`, {})
  expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
  await waitForStatus(page, sim.id, ['finished'], 120_000)

  await page.goto(`/simulations/${sim.id}/run`)
  await expect(page.locator('.vgl-item:not(.vgl-item--placeholder)').first()).toBeVisible({ timeout: 20_000 })
  await expect(page.locator('.vgl-item:not(.vgl-item--placeholder)')).toHaveCount(4)
  // Non-spatial → no PixiJS world pane (its unique view controls are absent).
  // (Chart widgets still render their own <canvas>, so we key off the world controls.)
  await expect(page.locator('button[title="Fit to view"]')).toHaveCount(0)
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${SHOTS}/run-nonspatial.png`, fullPage: false })
  // eslint-disable-next-line no-console
  console.log(`RUN_NONSPATIAL_SHOT=${SHOTS}/run-nonspatial.png`)
})
