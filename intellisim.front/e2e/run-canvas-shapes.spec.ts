import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the "render environment shapes in the live run canvas" change
// (2026-07-07). Previously RunWorldCanvas drew only the grid + agent sprites, so
// environment shapes (and their layer/visible/fill/stroke/stroke_width/thickness
// fields) had no effect on the running-world view. Now the run canvas draws
// shapes (shared displayRender helpers) on a layer beneath agents.
//
// The PixiJS canvas renders to a WebGL surface that Playwright can't introspect
// by DOM, so the visual assertion is a screenshot the operator inspects. The
// spec proves the cross-stack wiring: a model with distinctive fixed shapes
// (outline-only rect, filled rect + overlapping filled circle on a higher layer,
// a thick line, and a visible=false rect that must NOT appear) generates a
// runnable simulation, a short strategy-none run finishes, and the finished
// replay mounts the world canvas with shapes present.

let token: string

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

test('run canvas renders environment shapes (finished replay)', async ({ page }) => {
  test.setTimeout(240_000)

  await login(page)
  token = await getToken(page)

  // 1. Create a spatial model, then PUT a full model JSON with distinctive fixed
  //    shapes exercising every visual field + two movable agents.
  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Run Canvas Shapes',
      is_spatial: true,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const modelId = created.id

  const model = created.model
  model.is_spatial = true
  model.spatial_cell_size = 10
  model.environment = {
    uid: 'environment',
    width: 120,
    height: 80,
    torus: false,
    global_properties: [],
    shapes: [
      // outline-only rectangle (fill=false, thick stroke) — bottom layer
      {
        uid: 'wall', name: 'Wall', type: 'rectangle', layer: 1, visible: true,
        geometry: { position: { x: 10, y: 10 }, width: { value: 45 }, height: { value: 15 }, fill: false, stroke: true, stroke_width: 3 },
      },
      // filled rectangle UNDER the circle (layer 0 → painted first)
      {
        uid: 'under_zone', name: 'UnderZone', type: 'rectangle', layer: 0, visible: true,
        geometry: { position: { x: 48, y: 28 }, width: { value: 34 }, height: { value: 28 }, fill: true, stroke: false },
      },
      // filled circle on a HIGHER layer (5) → must paint over the under-zone rect
      {
        uid: 'zone', name: 'Zone', type: 'circle', layer: 5, visible: true,
        geometry: { position: { x: 62, y: 42 }, radius: { value: 16 }, fill: true, stroke: true, stroke_width: 2 },
      },
      // thick line (thickness=4)
      {
        uid: 'barrier', name: 'Barrier', type: 'line', layer: 2, visible: true,
        geometry: { position: { x: 12, y: 62 }, points: [{ x: 0, y: 0 }, { x: 85, y: 0 }], thickness: 4 },
      },
      // hidden rectangle (visible=false) → must NOT render
      {
        uid: 'hidden', name: 'Hidden', type: 'rectangle', layer: 9, visible: false,
        geometry: { position: { x: 95, y: 8 }, width: { value: 18 }, height: { value: 14 }, fill: true },
      },
    ],
  }
  model.agents = [
    {
      uid: 'walker', name: 'Walker', type: 'agent', move_type: 'movable',
      properties: [{ uid: 'energy', name: 'Energy', type: 'number', default_value: 50 }],
      states: ['idle'],
      initialization: { population_count: 3, count_mode: 'fixed', initial_state: 'idle', placement: { type: 'random' } },
    },
  ]

  const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
  expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

  const validation = await (await api(page, 'get', `/api/v1/intellisim/models/${modelId}/validation`)).json()
  expect(validation.simulation_ready, `model not simulation-ready: ${JSON.stringify(validation.errors)}`).toBeTruthy()

  // 2. Create a simulation (strategy none by default → no LLM) with few steps.
  const simRes = await api(page, 'post', '/api/v1/intellisim/simulations', {
    model_id: modelId,
    name: 'E2E run canvas shapes',
    steps: 6,
    seed: 3,
  })
  expect(simRes.ok(), `create simulation failed: ${simRes.status()} ${await simRes.text()}`).toBeTruthy()
  const sim = await simRes.json()
  const ready = await waitForStatus(page, sim.id, ['ready'], 90_000)

  // Sanity: the generated simulation carries all five shapes with the visual fields.
  const genShapes = ready.environment?.shapes ?? []
  expect(genShapes.length, 'all five shapes generated').toBe(5)
  const wall = genShapes.find((s: any) => s.name === 'Wall' || s.shape_uid === 'wall')
  expect(wall?.fill, 'wall fill=false preserved into runtime shape').toBe(false)

  // 3. Run to completion (small, strategy none).
  const run = await api(page, 'post', `/api/v1/intellisim/simulations/${sim.id}/run`, {})
  expect(run.status(), `run failed: ${await run.text()}`).toBe(202)
  await waitForStatus(page, sim.id, ['finished'], 120_000)

  // 4. Open the finished run: the world canvas mounts in replay mode.
  await page.goto(`/simulations/${sim.id}/run`)
  await expect(page.locator('canvas').first()).toBeVisible({ timeout: 20_000 })
  // Give the canvas a moment to build + fit + render shapes.
  await page.waitForTimeout(1_500)

  // 5. Screenshot the canvas for visual inspection of the rendered shapes.
  const shot = `${process.env.E2E_SHOT_DIR || 'test-results'}/run-canvas-shapes.png`
  await page.locator('canvas').first().screenshot({ path: shot })
  // eslint-disable-next-line no-console
  console.log(`RUN_CANVAS_SHOT=${shot}`)

  // 6. Regression: the simulation ENVIRONMENT editor (SimulationWorldCanvas) — which
  //    was refactored to the same shared displayRender helpers — still renders the
  //    shapes. Open the sim editor's Environment tab and screenshot its canvas.
  await page.goto(`/simulations/${sim.id}`)
  await page.getByRole('tab', { name: /environment/i }).click()
  await expect(page.locator('canvas').first()).toBeVisible({ timeout: 20_000 })
  await page.waitForTimeout(1_500)
  const editorShot = `${process.env.E2E_SHOT_DIR || 'test-results'}/env-editor-shapes.png`
  await page.locator('canvas').first().screenshot({ path: editorShot })
  // eslint-disable-next-line no-console
  console.log(`ENV_EDITOR_SHOT=${editorShot}`)
})
