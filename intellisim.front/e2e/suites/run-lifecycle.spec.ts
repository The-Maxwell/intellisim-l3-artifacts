import { test, expect, type Page, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import {
  minimalSpatial,
  seedModel,
  seedReadySim,
  runToCompletion,
  uniqueName,
  deleteQuietly,
} from '../_helpers/fixtures'

// §5.9 RUN lifecycle suite (RUN-01…RUN-07). Deterministic tier: F1 minimalSpatial
// uses strategy `none` and no LLM anywhere — the engine runs on rules alone, so
// no provider is touched and token totals stay zero.
//
// GOTCHAs honoured here (from prior waves):
//  - Never send a control right after POST /run: poll status=running first
//    (async start race). We start via the UI Start button, then confirm via API.
//  - Default engine speed is FAST; small runs finish in seconds. Pause/speed
//    cases use steps=3000 (fast per-step, plenty of wall time) and are stopped
//    in cleanup so no run is left live.
//  - The sim status Tag has no testid → scope to `.p-tag` (first).

function rx(s: string): RegExp {
  return new RegExp(s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
}

function idFromUrl(url: string): string {
  return url.replace(/[?#].*$/, '').split('/').pop() ?? ''
}

test.describe('run lifecycle', () => {
  let api: APIRequestContext
  let model: { id: string; name: string } // F1 ready fixture model
  const simIds: string[] = []
  const modelIds: string[] = []

  test.beforeAll(async () => {
    api = await apiContext()
    model = await seedModel(api, minimalSpatial(uniqueName('mdl-runlife')))
    modelIds.push(model.id)
  })

  test.afterAll(async () => {
    // Stop anything still running, then delete sims + models.
    for (const id of simIds) {
      await api.post(u(`/simulations/${id}/control`), { data: { action: 'stop' } }).catch(() => null)
    }
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  // --- helpers -------------------------------------------------------------

  async function seedSim(steps: number, seed: number, slug: string) {
    const sim = await seedReadySim(api, model.id, { steps, seed, name: uniqueName(slug) })
    simIds.push(sim.id)
    return sim
  }

  // Open the run screen for a READY sim and click Start; leaves the run live.
  async function startRunUI(page: Page, simId: string) {
    await page.goto(`/simulations/${simId}/run`)
    const startBtn = page.getByRole('button', { name: 'Start run' })
    await expect(startBtn).toBeVisible({ timeout: 15_000 })
    await startBtn.click()
  }

  // Generous default: under parallel load the core may keep a fresh run `queued`
  // for a while before it reaches `running` (run-capacity contention with other
  // agents), so allow ample time to transition.
  async function pollStatus(simId: string, wanted: string[], timeoutMs = 45_000): Promise<string> {
    return pollUntil(async () => {
      const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${simId}`)
      return wanted.includes(s.status) ? s.status : null
    }, { timeoutMs, intervalMs: 250, label: `sim ${simId} status ∈ ${wanted.join('|')}` })
  }

  async function runStepValue(page: Page): Promise<number> {
    const t = await page.getByTestId('run-step').textContent().catch(() => null)
    const m = /(\d+)/.exec(t ?? '')
    return m ? Number(m[1]) : -1
  }

  // --- RUN-01 --------------------------------------------------------------

  test('RUN-01 ready → Start → live steps → terminal banner + finished status', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedSim(10, 42, 'sim-run01')

    await login(page)
    await page.goto(`/simulations/${sim.id}/run`)

    // Ready state: execution-target override picker + Start button.
    await expect(page.getByText('Execution model override (optional)')).toBeVisible({ timeout: 15_000 })
    const startBtn = page.getByRole('button', { name: 'Start run' })
    await expect(startBtn).toBeVisible()
    await startBtn.click()

    // Live: the step counter renders (RunControls appear once store.active) and
    // the world canvas mounts.
    await expect(page.getByTestId('run-step')).toBeVisible({ timeout: 15_000 })
    await expect(page.locator('canvas').first()).toBeVisible()

    // Run finishes → terminal banner with the active-run total + steps completed.
    await pollStatus(sim.id, ['finished', 'stopped', 'failed'], 60_000)
    await expect(page.getByTestId('run-elapsed-total')).toBeVisible({ timeout: 15_000 })
    const banner = page.getByText('Run finished')
    await expect(banner).toBeVisible()
    // Banner reads "· <n> step(s) completed" (n = final store.currentStep).
    await expect(page.getByText(/step\(s\) completed/)).toBeVisible()

    // API confirms finished.
    const finalSim = await apiJson<{ status: string }>(api, 'get', `/simulations/${sim.id}`)
    expect(finalSim.status).toBe('finished')

    // Editor status chip finished (fresh load; SSE already flipped the sim).
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.locator('.p-tag').filter({ hasText: 'finished' }).first()).toBeVisible({ timeout: 15_000 })

    // List row status finished.
    await page.goto(`/simulations?model_id=${model.id}`)
    const row = page.getByRole('row', { name: rx(sim.name) })
    await expect(row).toBeVisible({ timeout: 15_000 })
    await expect(row.locator('.p-tag')).toHaveText('finished')
  })

  // --- RUN-02 --------------------------------------------------------------

  test('RUN-02 pause freezes elapsed, resume advances, stop terminates', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedSim(3000, 7, 'sim-run02')

    await login(page)
    await startRunUI(page, sim.id)

    // Confirm the run is actually running (async start race) before controlling.
    await pollStatus(sim.id, ['running'])
    await expect(page.getByRole('button', { name: 'Pause' })).toBeVisible({ timeout: 15_000 })

    // Let it accrue a little active time so the frozen value is meaningful.
    await page.waitForTimeout(2000)

    // Pause → server-confirmed paused, then elapsed must freeze. Wait for the UI
    // to settle in the paused state (Resume button shown) + a short grace so any
    // in-flight run.progress SSE (which can briefly re-flip runStatus to running
    // and re-tick the client clock) has passed before sampling.
    await page.getByRole('button', { name: 'Pause' }).click()
    await pollStatus(sim.id, ['paused'])
    await expect(page.getByRole('button', { name: 'Resume' })).toBeVisible({ timeout: 15_000 })
    await page.waitForTimeout(1200)
    const elapsedA = await page.getByTestId('run-elapsed').textContent()
    await page.waitForTimeout(1500)
    const elapsedB = await page.getByTestId('run-elapsed').textContent()
    expect(elapsedB, 'run-elapsed freezes while paused').toBe(elapsedA)

    // Resume → running again; the step counter advances.
    const stepBeforeResume = await runStepValue(page)
    await page.getByRole('button', { name: 'Resume' }).click()
    await pollStatus(sim.id, ['running'])
    await expect
      .poll(async () => runStepValue(page), { message: 'run-step advances after resume', timeout: 15_000 })
      .toBeGreaterThan(stepBeforeResume)

    // Stop (danger button) → terminal stopped.
    await page.getByRole('button', { name: 'Stop' }).click()
    const terminal = await pollStatus(sim.id, ['stopped', 'finished', 'failed'])
    expect(terminal).toBe('stopped')
    await expect(page.getByText('Run stopped')).toBeVisible({ timeout: 15_000 })
  })

  // --- RUN-03 --------------------------------------------------------------

  test('RUN-03 speed slider issues POST /control {action:set_speed, speed_ms} while running → 200', async ({ page }) => {
    // Run-speed control (2026-07-14): the slider now sends the dedicated
    // { action: 'set_speed', speed_ms } control action, which the backend accepts
    // while RUNNING (200) — no more masked 409 from the old action:'resume' path.
    test.setTimeout(120_000)
    const sim = await seedSim(3000, 11, 'sim-run03')

    await login(page)
    await startRunUI(page, sim.id)
    await pollStatus(sim.id, ['running'])
    await expect(page.getByRole('button', { name: 'Pause' })).toBeVisible({ timeout: 15_000 })

    // Arm a wait for the control request carrying speed_ms, then nudge the slider.
    const controlReq = page.waitForRequest((r) => {
      if (r.method() !== 'POST' || !r.url().includes(`/simulations/${sim.id}/control`)) return false
      try {
        return r.postDataJSON()?.speed_ms !== undefined
      } catch {
        return false
      }
    }, { timeout: 15_000 })

    // Focus the slider handle and step it down (slower). The frontend maps this to
    // store.setSpeed, which sends action:'set_speed' + speed_ms.
    const handle = page.locator('.p-slider-handle').first()
    await handle.focus()
    await page.keyboard.press('ArrowLeft')

    const req = await controlReq
    const body = req.postDataJSON()
    expect(body.speed_ms, 'control body carries speed_ms').toBeDefined()
    // The accompanying action is the dedicated set_speed action.
    expect(body.action).toBe('set_speed')
    const resp = await req.response()
    expect(resp, 'control response present').not.toBeNull()
    // A speed change while running is accepted (200) — the sim stays running.
    expect(
      resp!.status(),
      `set_speed while running should be accepted; got ${resp!.status()} ${await resp!.text().catch(() => '')}`,
    ).toBe(200)

    // Stop so nothing is left live (also tracked in simIds for afterAll).
    await page.getByRole('button', { name: 'Stop' }).click()
    await pollStatus(sim.id, ['stopped', 'finished', 'failed'])
  })

  // --- RUN-04 --------------------------------------------------------------

  test('RUN-04 Run entry points (list row + editor) both land on /run', async ({ page }) => {
    test.setTimeout(60_000)
    const sim = await seedSim(10, 13, 'sim-run04')

    await login(page)

    // (a) Simulations-list row Run button.
    await page.goto(`/simulations?model_id=${model.id}`)
    const row = page.getByRole('row', { name: rx(sim.name) })
    await expect(row).toBeVisible({ timeout: 15_000 })
    await row.getByRole('button', { name: 'Run' }).click()
    await expect(page).toHaveURL(new RegExp(`/simulations/${sim.id}/run$`), { timeout: 15_000 })

    // (b) Editor header Run button.
    await page.goto(`/simulations/${sim.id}`)
    const editorRun = page.getByRole('button', { name: 'Run', exact: true })
    await expect(editorRun).toBeEnabled({ timeout: 15_000 })
    await editorRun.click()
    await expect(page).toHaveURL(new RegExp(`/simulations/${sim.id}/run$`), { timeout: 15_000 })
  })

  // --- RUN-05 --------------------------------------------------------------

  test('RUN-05 finished sim: Run disabled (editor + list); View results opens replay', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedSim(10, 17, 'sim-run05')
    await runToCompletion(api, sim.id, { expect: 'finished' })

    await login(page)

    // Editor: the Run button is DISABLED for a terminal sim (store.terminal gate);
    // "View results" (pi-eye) is offered instead and opens the run screen in replay.
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.locator('.p-tag').filter({ hasText: 'finished' }).first()).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('button', { name: 'Run', exact: true })).toBeDisabled()
    const viewResults = page.getByRole('button', { name: 'View results' })
    await expect(viewResults).toBeVisible()
    await viewResults.click()
    await expect(page).toHaveURL(new RegExp(`/simulations/${sim.id}/run$`), { timeout: 15_000 })

    // List row: Run is disabled for a finished sim (canRun excludes terminal
    // statuses); the "View results" eye action (aria-label) is present and opens
    // the run screen in replay.
    await page.goto(`/simulations?model_id=${model.id}`)
    const row = page.getByRole('row', { name: rx(sim.name) })
    await expect(row).toBeVisible({ timeout: 15_000 })
    await expect(row.locator('.p-tag')).toHaveText('finished')
    await expect(row.getByRole('button', { name: 'Run' })).toBeDisabled()
    const listViewResults = row.getByRole('button', { name: 'View results' })
    await expect(listViewResults).toBeVisible()
    await listViewResults.click()
    await expect(page).toHaveURL(new RegExp(`/simulations/${sim.id}/run$`), { timeout: 15_000 })
  })

  // --- RUN-06 --------------------------------------------------------------

  test('RUN-06 strategy none: token totals zero, per-step token chip absent', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedSim(3000, 19, 'sim-run06')

    await login(page)
    await startRunUI(page, sim.id)
    await pollStatus(sim.id, ['running'])
    await expect(page.getByTestId('run-step')).toBeVisible({ timeout: 15_000 })

    // Collapsible run-screen token panel shows zero totals (trailing space in the
    // template, so match loosely).
    await expect(page.getByTestId('run-token-panel-total')).toHaveText(/^\s*0 tok\s*$/)
    // RunControls inline readout also zero.
    await expect(page.getByTestId('run-token-total')).toContainText('0 tok')
    // Per-step delta chip: rendered `v-if="hasStepTokens"`, where hasStepTokens
    // is true only for a NON-ZERO delta (total_tokens>0 || calls>0). A `none` run
    // sends an all-zero step_tokens delta every step, so the chip is ABSENT
    // throughout (no meaningless "0 tok this step" chip).
    await expect(page.getByTestId('run-step-tokens')).toHaveCount(0)

    // Expand the panel — still zero.
    await page.getByTestId('run-token-panel-toggle').click()
    await expect(page.getByTestId('run-token-panel')).toBeVisible()

    // Stop so nothing is left live.
    await page.getByRole('button', { name: 'Stop' }).click()
    await pollStatus(sim.id, ['stopped', 'finished', 'failed'])
  })

  // --- RUN-07 --------------------------------------------------------------

  test('RUN-07 active-duration excludes paused time; summary + banner populated', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedSim(3000, 23, 'sim-run07')

    await login(page)
    await startRunUI(page, sim.id)
    await pollStatus(sim.id, ['running'])
    await expect(page.getByRole('button', { name: 'Pause' })).toBeVisible({ timeout: 15_000 })

    // Run a bit, then pause ~2.2s (this paused interval must be EXCLUDED from the
    // backend active_duration_ms), resume briefly, then stop to finalize.
    await page.waitForTimeout(1200)
    await page.getByRole('button', { name: 'Pause' }).click()
    await pollStatus(sim.id, ['paused'])
    await page.waitForTimeout(2200)
    await page.getByRole('button', { name: 'Resume' }).click()
    await pollStatus(sim.id, ['running'])
    await page.waitForTimeout(600)
    await page.getByRole('button', { name: 'Stop' }).click()
    const terminal = await pollStatus(sim.id, ['stopped', 'finished', 'failed'])
    expect(terminal).toBe('stopped')

    // Run screen: terminal banner shows the active-run total.
    await expect(page.getByTestId('run-elapsed-total')).toBeVisible({ timeout: 15_000 })

    // Editor Summary panel: Started/Finished/Active block populated.
    await page.goto(`/simulations/${sim.id}`)
    const durationCell = page.getByTestId('summary-run-duration')
    await expect(durationCell).toBeVisible({ timeout: 15_000 })
    await expect(durationCell).not.toHaveText('—')

    // [API] active_duration_ms > 0 and, because we paused ~2.2s, wall − active is
    // clearly positive (loose ≥ 1000ms assertion).
    const final = await apiJson<any>(api, 'get', `/simulations/${sim.id}`)
    const runs = (final.execution_runs ?? []) as any[]
    expect(runs.length, 'has an execution run').toBeGreaterThan(0)
    const last = runs[runs.length - 1]
    const active = Number(last.active_duration_ms ?? 0)
    expect(active, 'active_duration_ms > 0').toBeGreaterThan(0)
    const wall = Date.parse(last.finished_at) - Date.parse(last.started_at)
    expect(wall - active, 'wall − active reflects the ~2.2s pause').toBeGreaterThanOrEqual(1000)
  })
})
