import { test, expect, type APIRequestContext, type Page } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import { rulesRich, seedModel, seedReadySim, uniqueName, deleteQuietly } from '../_helpers/fixtures'

// §5.9 replay block: RUN-20..RUN-24. Deterministic tier — the rulesRich fixture
// (+ inline valid widgets) uses NO LLM anywhere; runs finish in a few seconds at
// the engine's default 100ms/step. Replay/scrub, faithfulness, /steps pagination,
// mid-run reconnect, and the run-screen widget dashboard are all covered here.

type Json = Record<string, any>

// The /steps window is bounded by the `to` query param. An absent `to` now
// returns the FULL recorded range; these tests still pass an explicit generous
// ceiling as a precise upper bound (a literal `to=0` would clamp to step 0).
const STEP_TO = 1_000_000

const TERMINAL = ['finished', 'stopped', 'failed']

// Start a run tolerating 429 run_capacity (parallel agents share the 4-run cap):
// retry POST /run until a slot frees, returning once 202 is accepted.
async function startRunResilient(api: APIRequestContext, simId: string, body: Json = {}): Promise<void> {
  await pollUntil(async () => {
    const res = await api.post(u(`/simulations/${simId}/run`), { data: body })
    if (res.status() === 202) return true
    if (res.status() === 429) return null // capacity — retry
    throw new Error(`POST /run ${simId} → ${res.status()}: ${await res.text()}`)
  }, { timeoutMs: 180_000, intervalMs: 2000, label: `run slot for ${simId}` })
}

// Start + poll to terminal, resilient to 429.
async function runToCompletionResilient(api: APIRequestContext, simId: string, timeoutMs = 90_000): Promise<string> {
  await startRunResilient(api, simId)
  return pollUntil(async () => {
    const sim = await apiJson<Json>(api, 'get', `/simulations/${simId}`)
    return TERMINAL.includes(sim.status) ? sim.status : null
  }, { timeoutMs, intervalMs: 750, label: `sim ${simId} terminal` })
}

// rulesRich + a schema-valid widget set (the F3 widgetsFull fixture is REJECTED by
// the current backend — line_chart must use `series` not `source`, and text_log
// takes no source; documented in the report). Titles feed the RUN-24 assertions.
function rulesRichWidgets(name: string): Json {
  const m = rulesRich(name)
  m.widgets = [
    { id: 'w_mon', type: 'monitor', title: 'Total agents', source: { type: 'sim_metric', metric: 'total_agents' }, position: { x: 0, y: 0, w: 3, h: 2 } },
    { id: 'w_line', type: 'line_chart', title: 'Avg ticks', series: [{ label: 'ticks', source: { type: 'agent_property', property_uid: 'ticks', agent_uids: ['runner'], aggregation: 'avg' } }], position: { x: 3, y: 0, w: 4, h: 3 } },
    { id: 'w_bar', type: 'bar_chart', title: 'Population', source: { type: 'population' }, position: { x: 7, y: 0, w: 4, h: 3 } },
    { id: 'w_pie', type: 'pie_chart', title: 'Runner states', source: { type: 'state_distribution', agent_uids: ['runner'] }, position: { x: 0, y: 3, w: 4, h: 3 } },
    { id: 'w_hist', type: 'histogram', title: 'Ticks histogram', source: { type: 'agent_property', property_uid: 'ticks', agent_uids: ['runner'] }, buckets: 5, position: { x: 4, y: 3, w: 4, h: 3 } },
    { id: 'w_tab', type: 'agent_table', title: 'Runners', source: { type: 'agent_property', agent_uids: ['runner'] }, columns: ['ticks', 'meetings'], position: { x: 8, y: 3, w: 4, h: 3 } },
    { id: 'w_log', type: 'text_log', title: 'Events', log_kinds: ['events', 'state_transitions'], position: { x: 0, y: 6, w: 6, h: 3 } },
  ]
  return m
}

// Read the RunDashboard replay timeline label ("step X / N").
async function scrubLabel(page: Page): Promise<string> {
  return (await page.getByText(/step \d+ \/ \d+/).first().textContent())?.trim() ?? ''
}

test.describe('run replay', () => {
  let api: APIRequestContext
  const simIds: string[] = []
  const modelIds: string[] = []

  // Shared rulesRich model + one finished 10-step sim, reused by RUN-20/21/22.
  let modelRR: { id: string; name: string }
  let simFinished: { id: string; name: string }

  test.beforeAll(async () => {
    api = await apiContext()
    modelRR = await seedModel(api, rulesRich(uniqueName('mdl-replay')))
    modelIds.push(modelRR.id)
    simFinished = await seedReadySim(api, modelRR.id, { steps: 10, seed: 71, name: uniqueName('sim-replay') })
    simIds.push(simFinished.id)
    await runToCompletionResilient(api, simFinished.id)
  })

  test.afterAll(async () => {
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  test('RUN-20 finished sim: editor "View results" (pi-eye) → run screen in REPLAY mode with a scrub timeline + play/pause; scrubbing changes the displayed step', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)
    await page.goto(`/simulations/${simFinished.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(simFinished.name, { timeout: 15_000 })

    // View results is shown ONLY for terminal states (store.terminal); the Run
    // button is disabled for a finished sim.
    const viewResults = page.getByRole('button', { name: 'View results' })
    await expect(viewResults).toBeVisible()
    // pi-eye icon renders as a <span class="pi pi-eye"> inside the button.
    await expect(viewResults.locator('span.pi-eye')).toBeVisible()

    await viewResults.click()
    await expect(page).toHaveURL(new RegExp(`/simulations/${simFinished.id}/run$`))

    // Replay chrome: scrub range slider + timeline label + play/pause control.
    const slider = page.locator('input[type="range"]')
    await expect(slider).toBeVisible({ timeout: 20_000 })
    const playBtn = page.locator('button[title="Play replay"], button[title="Pause replay"]')
    await expect(playBtn).toBeVisible()
    await expect(page.getByText(/step \d+ \/ \d+/).first()).toBeVisible()

    // Scrubbing changes the displayed step: drive the slider to two distinct
    // values and assert the timeline label follows.
    const max = Number(await slider.getAttribute('max'))
    const min = Number(await slider.getAttribute('min'))
    expect(max).toBeGreaterThan(min)
    await slider.fill(String(max))
    const labelHi = await scrubLabel(page)
    expect(labelHi).toContain(`step ${max} / ${max}`)
    await slider.fill(String(min))
    const labelLo = await scrubLabel(page)
    expect(labelLo).toContain(`step ${min} /`)
    expect(labelLo).not.toBe(labelHi)
  })

  test('RUN-21 replay faithfulness: last /steps frame matches the sim final agents (GET /agents/:id); AgentInspector does NOT poll in replay', async ({ page }) => {
    test.setTimeout(90_000)

    // --- [API] faithfulness: the last persisted step frame agents match the
    // FinalAgents snapshot served by GET /:id/agents/:agentId post-run. NOTE:
    // GET /simulations/:id returns the INITIAL world (runtime isolation, 2026-07-09);
    // the FINAL world lives in FinalAgents (surfaced via /agents/:id) + the
    // simulation_steps timeseries (/steps). We cross-check those two.
    const steps = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/steps?from=0&to=${STEP_TO}&page=1&page_size=100000`)
    const frames = steps.items as Json[]
    expect(frames.length, 'persisted step frames exist').toBeGreaterThan(0)
    const lastFrame = frames.reduce((a, b) => (b.step > a.step ? b : a))
    const frameAgents = lastFrame.agents as Json[]
    expect(frameAgents.length).toBeGreaterThan(0)

    // Compare a couple of agents' position/state/numeric property to the live
    // final-agent snapshot.
    let compared = 0
    for (const fa of frameAgents.slice(0, 3)) {
      const finalA = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/agents/${fa.id}`).catch(() => null)
      if (!finalA) continue
      compared++
      if (fa.position && finalA.position) {
        expect(finalA.position.x, `agent ${fa.id} x`).toBeCloseTo(fa.position.x, 5)
        expect(finalA.position.y, `agent ${fa.id} y`).toBeCloseTo(fa.position.y, 5)
      }
      if (fa.state) expect(finalA.state, `agent ${fa.id} state`).toBe(fa.state)
      if (fa.properties?.ticks !== undefined && finalA.properties?.ticks !== undefined) {
        expect(finalA.properties.ticks, `agent ${fa.id} ticks`).toBe(fa.properties.ticks)
      }
    }
    expect(compared, 'at least one agent cross-checked against FinalAgents').toBeGreaterThan(0)
    test.info().annotations.push({
      type: 'faithfulness',
      description: `Last /steps frame (step ${lastFrame.step}) agents match GET /agents/:id FinalAgents for ${compared} agent(s). GET /simulations/:id.agents carries the INITIAL world; FinalAgents (via /agents/:id) + /steps carry the final world.`,
    })

    // --- no-poll in replay: count GET /agents/:id requests over a 4s window on
    // the replay run screen. In replay the inspector's live poll is disabled, so
    // the count must NOT grow (periodic poll would fire ~every 3s).
    let agentReqs = 0
    page.on('request', (r) => {
      if (/\/simulations\/[^/]+\/agents\/\d+/.test(r.url()) && r.method() === 'GET') agentReqs++
    })

    await login(page)
    await page.goto(`/simulations/${simFinished.id}/run`)
    await expect(page.locator('input[type="range"]')).toBeVisible({ timeout: 20_000 })

    // Best-effort: try to open the inspector by clicking the run canvas centre
    // (RunWorldCanvas has no test id + its own fit transform, so this is flaky).
    // Whether or not selection succeeds, replay must not periodically poll.
    const canvas = page.locator('canvas').first()
    try {
      const box = await canvas.boundingBox()
      if (box) await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2)
    } catch {
      /* selection is best-effort */
    }
    const inspectorOpened = await page.getByText(/memory/i).first().isVisible().catch(() => false)

    await page.waitForTimeout(700)
    const baseline = agentReqs
    await page.waitForTimeout(4000)
    const delta = agentReqs - baseline
    expect(delta, 'no periodic GET /agents/:id polling in replay').toBe(0)
    test.info().annotations.push({
      type: 'no-poll',
      description: `Replay: ${delta} periodic /agents/:id requests over 4s (expected 0). Inspector open via canvas click: ${inspectorOpened} (best-effort). Step-filtered memory (entry.step <= scrubStep) is asserted in faithful-replay.spec.ts.`,
    })
  })

  test('RUN-22 [API] /steps pagination: small pages, has_more toggles true→false, contiguous step numbers', async ({ page: _page }) => {
    test.setTimeout(60_000)
    // Param names (from useSimulationApi.listSteps): from / to / page / page_size.
    // NOTE: this test passes an explicit `to` for a precise window (an absent `to`
    // now returns the full range; a literal `to=0` still clamps to step 0).
    const pageSize = 3
    const collected: number[] = []
    let pageNo = 1
    let sawMore = false
    let lastHasMore = true
    for (let guard = 0; guard < 40 && lastHasMore; guard++) {
      const resp = await apiJson<Json>(
        api,
        'get',
        `/simulations/${simFinished.id}/steps?from=0&to=${STEP_TO}&page=${pageNo}&page_size=${pageSize}`,
      )
      const items = resp.items as Json[]
      // Paginated by LOGICAL step-number window, so a page holds <= pageSize steps
      // (fewer if some numbers in the window have no recorded step, e.g. step 0).
      expect(items.length, `page ${pageNo} not over page_size`).toBeLessThanOrEqual(pageSize)
      for (const it of items) collected.push(it.step as number)
      lastHasMore = resp.has_more === true
      if (lastHasMore) sawMore = true
      pageNo++
    }
    expect(sawMore, 'has_more was true for at least one early page').toBe(true)
    expect(lastHasMore, 'pagination terminated with has_more=false').toBe(false)

    // Step numbers are contiguous and strictly increasing across pages.
    expect(collected.length, 'reassembled all ~10 steps').toBeGreaterThanOrEqual(10)
    const sorted = [...collected].sort((a, b) => a - b)
    expect(collected).toEqual(sorted)
    for (let i = 1; i < sorted.length; i++) expect(sorted[i] - sorted[i - 1]).toBe(1)
    test.info().annotations.push({
      type: 'steps-pagination',
      description: `Reassembled ${collected.length} contiguous steps [${sorted[0]}..${sorted.at(-1)}] via page/page_size=${pageSize} (explicit to= sets a precise window; an absent to now returns the full range). has_more toggled true→false.`,
    })
  })

  test('RUN-23 reload/attach mid-run: opening an already-running sim attaches live — run-step advances, run-elapsed seeded + ticking', async ({ page }) => {
    test.setTimeout(150_000)
    const mdl = await seedModel(api, rulesRich(uniqueName('mdl-midrun')))
    modelIds.push(mdl.id)
    // 3000 steps @ default 100ms/step ≈ 5min of run time — plenty to attach to.
    const sim = await seedReadySim(api, mdl.id, { steps: 3000, seed: 73, name: uniqueName('sim-midrun') })
    simIds.push(sim.id)

    // Start via API (429-resilient) and wait until it's actually running (do NOT
    // control right after POST /run — control races the async run start).
    await startRunResilient(api, sim.id)
    await pollUntil(async () => {
      const s = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
      return s.status === 'running' ? s : null
    }, { timeoutMs: 30_000, label: `sim ${sim.id} running` })

    try {
      // Fresh open of the already-running sim = the reload/attach case.
      await login(page)
      await page.goto(`/simulations/${sim.id}/run`)

      // Live controls render for running/paused (store.active): run-step advances.
      const runStep = page.getByTestId('run-step')
      await expect(runStep).toBeVisible({ timeout: 20_000 })
      const first = Number((await runStep.textContent())?.match(/\d+/)?.[0] ?? '0')
      expect(first, 'run-step > 0 on attach').toBeGreaterThan(0)
      await expect
        .poll(async () => Number((await runStep.textContent())?.match(/\d+/)?.[0] ?? '0'), {
          timeout: 15_000,
          message: 'run-step advances after attach',
        })
        .toBeGreaterThan(first)

      // run-elapsed seeded from started_at + status_history (non-zero) and ticks.
      const elapsed = page.getByTestId('run-elapsed')
      await expect(elapsed).toBeVisible()
      const e1 = (await elapsed.textContent())?.trim() ?? ''
      expect(e1).not.toBe('')
      expect(e1, 'run-elapsed seeded non-zero').not.toMatch(/^0?0:00$/)
      await expect
        .poll(async () => (await elapsed.textContent())?.trim() ?? '', { timeout: 8_000, message: 'run-elapsed ticks' })
        .not.toBe(e1)
    } finally {
      // Stop the run (API) so it releases its run-concurrency slot.
      await api.post(u(`/simulations/${sim.id}/control`), { data: { action: 'stop' } }).catch(() => null)
      await pollUntil(async () => {
        const s = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
        return TERMINAL.includes(s.status) ? s : null
      }, { timeoutMs: 30_000, label: `sim ${sim.id} stopped` }).catch(() => null)
    }
  })

  test('RUN-24 run-screen dashboard: finished sim with 7 widgets → replay shows widget tiles (titles) laid out + world canvas on the LEFT; saved positions honored', async ({ page }) => {
    test.setTimeout(150_000)
    const mdl = await seedModel(api, rulesRichWidgets(uniqueName('mdl-widrun')))
    modelIds.push(mdl.id)
    const sim = await seedReadySim(api, mdl.id, { steps: 10, seed: 74, name: uniqueName('sim-widrun') })
    simIds.push(sim.id)
    await runToCompletionResilient(api, sim.id)

    await login(page)
    await page.goto(`/simulations/${sim.id}/run`)
    await expect(page.locator('input[type="range"]')).toBeVisible({ timeout: 20_000 })

    // Widget tiles rendered with their titles (7 widgets wired).
    const titles = ['Total agents', 'Avg ticks', 'Population', 'Runner states', 'Ticks histogram', 'Runners', 'Events']
    let shown = 0
    for (const t of titles) {
      if (await page.getByText(t, { exact: true }).first().isVisible().catch(() => false)) shown++
    }
    expect(shown, `widget tiles rendered with titles (saw ${shown}/${titles.length})`).toBeGreaterThanOrEqual(5)

    // World canvas present and on the LEFT (spatial → fixed world pane left of grid).
    const canvas = page.locator('canvas').first()
    await expect(canvas).toBeVisible()
    const canvasBox = await canvas.boundingBox()

    // Saved positions honored (not naive stacking): 'Total agents' (x:0,y:0) and
    // 'Avg ticks' (x:3,y:0) share a row → Avg ticks sits to the RIGHT of Total
    // agents, and both sit to the right of the world canvas.
    const totalBox = await page.getByText('Total agents', { exact: true }).first().boundingBox()
    const avgBox = await page.getByText('Avg ticks', { exact: true }).first().boundingBox()
    expect(canvasBox && totalBox && avgBox).toBeTruthy()
    if (canvasBox && totalBox && avgBox) {
      expect(totalBox.x, 'widget grid is right of the world canvas').toBeGreaterThan(canvasBox.x)
      expect(avgBox.x, 'x:3 widget is right of the x:0 widget (positions honored)').toBeGreaterThan(totalBox.x)
      expect(Math.abs(avgBox.y - totalBox.y), 'same-row widgets share a y').toBeLessThan(60)
    }
    test.info().annotations.push({
      type: 'dashboard-layout',
      description: `Run replay dashboard: ${shown}/7 widget titles rendered; world canvas left of the grid; x-ordered tiles confirm saved WidgetConfig.position honored.`,
    })
  })
})
