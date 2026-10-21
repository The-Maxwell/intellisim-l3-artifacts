import { test, expect, type Page, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import { minimalSpatial, seedModel, seedReadySim, uniqueName, deleteQuietly } from '../_helpers/fixtures'

// §5.7 Simulation EDITOR suite: SIM-05..09, SIM-16, SIM-17.
// Deterministic tier — fixture models use NO LLM anywhere; simulation
// generation is local and completes in well under a second.

type Json = Record<string, any>

// F1 + one numeric environment global (SIM-07 needs a typed global editor).
function spatialWithGlobal(name: string): Json {
  const m = minimalSpatial(name)
  m.environment.global_properties = [
    {
      uid: 'temperature',
      name: 'temperature',
      type: 'number',
      description: '',
      min_value: 0,
      max_value: 100,
      default_value: 10,
      immutable: false,
    },
  ]
  return m
}

// Full-body PUT payload built from a GET response (partial PUT ⇒ 400).
function toUpdateBody(sim: Json): Record<string, unknown> {
  return {
    name: sim.name,
    description: sim.description ?? '',
    steps: sim.steps,
    seed: sim.seed,
    llm_generation_target: sim.llm_generation_target ?? null,
    llm_execution_target: sim.llm_execution_target ?? null,
    agents: sim.agents ?? [],
    environment: sim.environment ?? null,
    widgets: sim.widgets ?? [],
  }
}

function positionKeys(sim: Json): string[] {
  return ((sim.agents ?? []) as Json[]).map((a) => `${a.position?.x},${a.position?.y}`).sort()
}

// Rename the SOURCE model's first agent via the API (GET → mutate → wrapped PUT).
async function renameSourceAgent(api: APIRequestContext, modelId: string, newName: string) {
  const resp = await apiJson<Json>(api, 'get', `/models/${modelId}`)
  resp.model.agents[0].name = newName
  await apiJson(api, 'put', `/models/${modelId}`, { model: resp.model }, 200)
}

// Wait until a (re)generation run finished and the sim is back to ready.
async function waitRegenDone(api: APIRequestContext, simId: string, prevRuns: number): Promise<Json> {
  return pollUntil(async () => {
    const s = await apiJson<Json>(api, 'get', `/simulations/${simId}`)
    if (s.status === 'generation_failed') throw new Error(`sim ${simId} generation_failed`)
    return s.status === 'ready' && ((s.generation_runs ?? []) as Json[]).length > prevRuns ? s : null
  }, { timeoutMs: 30_000, label: `sim ${simId} regenerated` })
}

// ---------------------------------------------------------------------------
// PixiJS world-canvas coordinate helpers. The canvas fits the world with
// scale = min(cw/W, ch/H) * 0.9, centered (SimulationWorldCanvas.fitView). We
// re-run "Fit to view" right before computing so the transform matches the
// CURRENT container box, then click at the projected agent position. Canvas
// pixels are opaque to selectors, so selection is asserted via the side panel.
// ---------------------------------------------------------------------------
const HANDLE_PX = 7

interface CanvasXf {
  boxX: number
  boxY: number
  scale: number
  offX: number
  offY: number
}

async function canvasTransform(page: Page, envW: number, envH: number): Promise<CanvasXf> {
  await page.getByTitle('Fit to view').click()
  const box = await page.getByTestId('world-canvas').boundingBox()
  if (!box) throw new Error('world-canvas has no bounding box')
  const scale = Math.min(box.width / envW, box.height / envH) * 0.9
  return {
    boxX: box.x,
    boxY: box.y,
    scale,
    offX: (box.width - envW * scale) / 2,
    offY: (box.height - envH * scale) / 2,
  }
}

async function clickWorld(page: Page, xf: CanvasXf, wx: number, wy: number) {
  await page.mouse.click(xf.boxX + xf.offX + wx * xf.scale, xf.boxY + xf.offY + wy * xf.scale)
}

// Mirror of the canvas hit test (worldCanvasInteractions.hitTestAgent): topmost
// (= last in array order) agent whose center is within rHit of the click point.
function predictSelected(agents: Json[], at: { x: number; y: number }, rHit: number): Json | null {
  for (let i = agents.length - 1; i >= 0; i--) {
    const a = agents[i]
    if (!a.position) continue
    const d = Math.hypot(a.position.x - at.x, a.position.y - at.y)
    if (d <= rHit) return a
  }
  return null
}

// Agent display radius in world units (SimulationWorldCanvas.agentRadius).
function agentDisplayRadius(envW: number, envH: number): number {
  if (envW <= 0 || envH <= 0) return 4
  return Math.max(2, Math.round(Math.min(envW, envH) / 60))
}

// Synthesized HTML5 drag-and-drop for the unplaced tray (Playwright's
// mouse-based dragTo does not reliably trigger native DnD) — proven mechanics
// copied from e2e/manual-placement.spec.ts.
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

async function waitPut(page: Page, simId: string, action: () => Promise<void>) {
  const [resp] = await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes(`/simulations/${simId}`) && r.request().method() === 'PUT'
    ),
    action(),
  ])
  expect(resp.ok(), `PUT /simulations/${simId} → ${resp.status()}`).toBeTruthy()
  return resp
}

test.describe('simulations editor', () => {
  let api: APIRequestContext
  let model: { id: string; name: string } // shared F1+global fixture model
  const simIds: string[] = []
  const modelIds: string[] = []

  test.beforeAll(async () => {
    api = await apiContext()
    model = await seedModel(api, spatialWithGlobal(uniqueName('mdl-simedit')))
    modelIds.push(model.id)
  })

  test.afterAll(async () => {
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  test('SIM-05 General tab: description + steps save & persist; steps=0 is clamped client-side (InputNumber min=1) and rejected by the API', async ({ page }) => {
    test.setTimeout(90_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 11, name: uniqueName('sim-general') })
    simIds.push(sim.id)

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })

    // --- valid edits: description + steps=25 ---
    const desc = page.getByPlaceholder('What this simulation explores')
    await desc.fill('e2e edited description')
    // General tab spinbuttons in order: Steps, Seed (LLM advanced is collapsed).
    const steps = page.getByRole('spinbutton').nth(0)
    await steps.fill('25')
    await steps.press('Tab')
    await expect(page.getByText('unsaved')).toBeVisible()
    await waitPut(page, sim.id, () => page.getByRole('button', { name: 'Save', exact: true }).click())

    // Persisted (API) …
    const after = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    expect(after.description).toBe('e2e edited description')
    expect(after.steps).toBe(25)
    // … and survives a reload (UI).
    await page.reload()
    await expect(page.getByPlaceholder('What this simulation explores')).toHaveValue('e2e edited description', { timeout: 15_000 })
    await expect(page.getByRole('spinbutton').nth(0)).toHaveValue('25')

    // --- invalid steps: type 0 → InputNumber (min=1) clamps ON BLUR to 1.
    // Observed behavior documented: the UI never lets an out-of-range value
    // reach the store, so no 422 banner path is reachable from this field.
    const steps2 = page.getByRole('spinbutton').nth(0)
    await steps2.fill('0')
    await steps2.press('Tab')
    const clamped = await steps2.inputValue()
    expect(clamped, 'InputNumber min=1 clamps 0 client-side').toBe('1')

    // Backend contract probe [API]: a forced full-body PUT with steps=0 must be
    // rejected by request-schema validation → 422 (the UI clamp is not the only
    // guard; validation_failed is 422 everywhere now).
    const cur = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    const res = await api.put(u(`/simulations/${sim.id}`), { data: { ...toUpdateBody(cur), steps: 0 } })
    const bodyText = await res.text()
    expect(res.status(), `PUT steps=0 → ${res.status()}: ${bodyText}`).toBe(422)

    // Restore a sane steps value for cleanliness (page still shows clamped 1).
    await apiJson(api, 'put', `/simulations/${sim.id}`, { ...toUpdateBody(cur), steps: 25 }, 200)
  })

  test('SIM-06 Agents tab: archetype list + instance rows == population; Add instance grows the saved agent count', async ({ page }) => {
    test.setTimeout(90_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 12, name: uniqueName('sim-agents') })
    simIds.push(sim.id)

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })

    await page.getByRole('tab', { name: /Agents/ }).click()

    // Archetype master list shows the fixture archetype with its instance count.
    await expect(page.getByText('Walker').first()).toBeVisible()
    await expect(page.locator('.p-tag').filter({ hasText: '4 instance(s)' })).toBeVisible()

    // Instance table rows == population (F1 spawns 4 walkers).
    const rows = page.locator('tbody tr')
    await expect(rows).toHaveCount(4)

    // Add instance → a 5th row appears (dirty).
    await page.getByRole('button', { name: 'Add instance' }).click()
    await expect(rows).toHaveCount(5)
    await expect(page.locator('.p-tag').filter({ hasText: '5 instance(s)' })).toBeVisible()
    await expect(page.getByText('unsaved')).toBeVisible()

    // Save persists: agent count +1 via API.
    await waitPut(page, sim.id, () => page.getByRole('button', { name: 'Save', exact: true }).click())
    const after = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    expect((after.agents as Json[]).length).toBe(5)
    expect((after.agents as Json[]).every((a) => a.agent_uid === 'walker')).toBe(true)
  })

  test('SIM-07 Environment tab: global property edit + canvas select/edit/delete, saved and verified via API', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 13, name: uniqueName('sim-env') })
    simIds.push(sim.id)
    const before = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    const agents = before.agents as Json[]
    const envW = before.environment.width as number
    const envH = before.environment.height as number

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })
    await page.getByRole('tab', { name: /Environment/ }).click()

    // --- (a) typed global property editor ---
    const tempRow = page.locator('div.flex.items-center.gap-3', { hasText: 'temperature' })
    const tempInput = tempRow.getByRole('spinbutton')
    await expect(tempInput).toHaveValue('10')
    await tempInput.fill('42')
    await tempInput.press('Tab')

    // --- (b) canvas: select an instance, edit a property via the side panel ---
    await expect(page.getByTestId('world-canvas')).toBeVisible({ timeout: 15_000 })
    await page.waitForTimeout(600) // PixiJS mount + first render
    const xf = await canvasTransform(page, envW, envH)
    const rHit = agentDisplayRadius(envW, envH) + HANDLE_PX / xf.scale

    // Deterministic target: the agent closest to the world center (clear of the
    // toolbar/legend overlays); prediction mirrors the canvas topmost-hit rule.
    const byCentrality = [...agents].sort(
      (a, b) =>
        Math.hypot(a.position.x - envW / 2, a.position.y - envH / 2) -
        Math.hypot(b.position.x - envW / 2, b.position.y - envH / 2)
    )
    const targetA = byCentrality[0]!
    const selectedA = predictSelected(agents, targetA.position, rHit)!
    expect(selectedA, 'click point must hit an agent').toBeTruthy()

    await clickWorld(page, xf, targetA.position.x, targetA.position.y)
    const panel = page.locator('div.w-72')
    await expect(panel.locator('.p-tag').filter({ hasText: 'agent' })).toBeVisible({ timeout: 5_000 })

    // Side-panel spinbuttons: X, Y, then the numeric 'energy' property.
    const energyBox = panel.locator('div.space-y-0\\.5', { hasText: 'energy' })
    const energy = energyBox.getByRole('spinbutton')
    await expect(energy).toHaveValue('50') // fixture default
    await energy.fill('77')
    await energy.press('Enter')

    // --- (c) canvas: select a DIFFERENT instance and delete it ---
    const targetB = byCentrality.find((cand) => {
      const sel = predictSelected(agents, cand.position, rHit)
      return sel && sel.id !== selectedA.id
    })
    expect(targetB, 'a second, distinct selectable agent exists').toBeTruthy()
    const selectedB = predictSelected(agents, targetB!.position, rHit)!
    await clickWorld(page, xf, targetB!.position.x, targetB!.position.y)
    await expect(panel.locator('.p-tag').filter({ hasText: 'agent' })).toBeVisible()
    await page.getByTitle('Delete selected').click()
    await expect(panel.getByText('Select an item on the canvas to edit it.')).toBeVisible()

    // --- save everything at once, verify via API ---
    await waitPut(page, sim.id, () => page.getByRole('button', { name: 'Save', exact: true }).click())
    const after = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    expect(after.environment.global_properties.temperature).toBe(42)
    const savedAgents = after.agents as Json[]
    expect(savedAgents.length).toBe(agents.length - 1)
    expect(savedAgents.some((a) => a.id === selectedB.id), 'deleted instance is gone').toBe(false)
    const editedA = savedAgents.find((a) => a.id === selectedA.id)
    expect(editedA, 'edited instance survived').toBeTruthy()
    expect(editedA!.properties.energy).toBe(77)
  })

  test('SIM-07 manual placement: unplaced banner → Place agents deep-link → tray drop decreases the count', async ({ page }) => {
    test.setTimeout(120_000)
    // Model whose single archetype uses manual placement → 4 unplaced instances.
    const manual = spatialWithGlobal(uniqueName('mdl-manual'))
    manual.agents[0].initialization.placement = { type: 'manual' }
    const mdl = await seedModel(api, manual)
    modelIds.push(mdl.id)
    const sim = await seedReadySim(api, mdl.id, { steps: 5, seed: 14, name: uniqueName('sim-manual') })
    simIds.push(sim.id)

    // Backend contract: all 4 manual instances come back unplaced; run blocked.
    const ready = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    expect((ready.agents as Json[]).every((a) => !a.position)).toBe(true)
    expect(ready.validation?.start_ready).toBeFalsy()

    await login(page)
    await page.goto(`/simulations/${sim.id}`)

    // Banner with the unplaced count + disabled Run.
    const banner = page.getByTestId('unplaced-banner')
    await expect(banner).toBeVisible({ timeout: 15_000 })
    await expect(banner).toContainText('4 agent(s)')
    await expect(page.getByRole('button', { name: 'Run', exact: true })).toBeDisabled()

    // "Place agents" deep-links to the Environment tab with the tray open.
    await page.getByRole('button', { name: /place agents/i }).click()
    await expect(page.getByRole('tab', { name: /Environment/ })).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByTestId('world-canvas')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByTestId('unplaced-tray')).toBeVisible()
    const chips = page.getByTestId('unplaced-chip')
    await expect(chips).toHaveCount(4)

    // Drop the first chip near the world center → count decreases everywhere.
    const box = await page.getByTestId('world-canvas').boundingBox()
    if (!box) throw new Error('no canvas box')
    await dropFirstChip(page, box.x + box.width * 0.5, box.y + box.height * 0.5)
    await expect(chips).toHaveCount(3)
    await expect(banner).toContainText('3 agent(s)')
  })

  test('SIM-08 Model snapshot tab: lazy load, 8 read-only inner tabs, hidden action buttons, FROZEN against source-model edits', async ({ page }) => {
    test.setTimeout(120_000)
    // Own model: this test renames the SOURCE agent and must not disturb others.
    const mdl = await seedModel(api, spatialWithGlobal(uniqueName('mdl-snapfrozen')))
    modelIds.push(mdl.id)
    const sim = await seedReadySim(api, mdl.id, { steps: 10, seed: 15, name: uniqueName('sim-snapshot') })
    simIds.push(sim.id)

    // Track /model-snapshot requests to prove laziness.
    const snapshotRequests: string[] = []
    page.on('request', (r) => {
      if (r.url().includes(`/simulations/${sim.id}/model-snapshot`)) snapshotRequests.push(r.url())
    })

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })
    expect(snapshotRequests.length, 'snapshot NOT fetched before the tab is opened').toBe(0)

    // Open the tab → lazy fetch fires and the read-only banner renders.
    await page.getByRole('tab', { name: 'Model snapshot' }).click()
    await expect(page.getByText('Read-only snapshot of the source model')).toBeVisible({ timeout: 15_000 })
    expect(snapshotRequests.length).toBeGreaterThan(0)

    // 8 inner tabs.
    const panel = page.locator('.snapshot-forms')
    const innerTabs = panel.getByRole('tab')
    await expect(innerTabs).toHaveCount(8)
    for (const name of ['General', 'LLM', 'Agents', 'Environment', 'Rules', 'Interactions', 'Events', 'Widgets']) {
      await expect(panel.getByRole('tab', { name: new RegExp(`^${name}`) })).toBeVisible()
    }

    // Agents inner tab shows the frozen source agent; action buttons hidden
    // (snapshot uses scoped `.snapshot-forms :deep(.p-button){display:none}`).
    await panel.getByRole('tab', { name: /^Agents/ }).click()
    await expect(panel.getByText('Walker').first()).toBeVisible()
    await expect(panel.locator('.p-button:visible')).toHaveCount(0)

    // Edit the SOURCE model (rename the agent) → the snapshot must stay frozen.
    await renameSourceAgent(api, mdl.id, 'Walker Mk2')
    await page.reload()
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })
    await page.getByRole('tab', { name: 'Model snapshot' }).click()
    await expect(page.getByText('Read-only snapshot of the source model')).toBeVisible({ timeout: 15_000 })
    await panel.getByRole('tab', { name: /^Agents/ }).click()
    await expect(panel.getByText('Walker').first()).toBeVisible()
    await expect(panel.getByText('Walker Mk2')).toHaveCount(0)

    // API cross-check: the snapshot endpoint still serves the OLD name while the
    // live model already carries the new one.
    const snap = await apiJson<Json>(api, 'get', `/simulations/${sim.id}/model-snapshot`)
    expect(snap.model.agents[0].name).toBe('Walker')
    const live = await apiJson<Json>(api, 'get', `/models/${mdl.id}`)
    expect(live.model.agents[0].name).toBe('Walker Mk2')
  })

  test('SIM-09 regenerate: partial (placement) keeps the frozen snapshot; FULL refreshes it to the current model', async ({ page }) => {
    test.setTimeout(150_000)
    // Own model (renamed mid-test).
    const mdl = await seedModel(api, spatialWithGlobal(uniqueName('mdl-regen')))
    modelIds.push(mdl.id)
    const sim = await seedReadySim(api, mdl.id, { steps: 10, seed: 16, name: uniqueName('sim-regen') })
    simIds.push(sim.id)

    const snap0 = await apiJson<Json>(api, 'get', `/simulations/${sim.id}/model-snapshot`)
    expect(snap0.model.agents[0].name).toBe('Walker')

    // Rename the SOURCE agent now — a frozen snapshot must keep 'Walker' until
    // a FULL regenerate refreshes it.
    await renameSourceAgent(api, mdl.id, 'Walker Mk2')

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })

    // ---- PARTIAL regenerate (placement only) via the SplitButton popover ----
    // Regeneration rolls a fresh seed itself now (per GenerationRun), so a
    // placement regen re-scatters positions without any manual seed bump.
    let cur = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    const posBeforePartial = positionKeys(cur)

    let runsBefore = ((cur.generation_runs ?? []) as Json[]).length
    await page.locator('.p-splitbutton-dropdown').click()
    await page.getByRole('menuitem', { name: 'Partial regenerate…' }).click()
    const popover = page.locator('.p-popover')
    await expect(popover).toBeVisible()
    // Archetype box still shows the FROZEN name (inline snapshot): 'Walker'.
    const archBox = popover.locator('.rounded-md.border', { hasText: 'Walker' })
    await expect(archBox).toBeVisible()
    await archBox.getByText('placement', { exact: true }).click()
    await archBox.getByRole('button', { name: 'Regenerate', exact: true }).click()

    const afterPartial = await waitRegenDone(api, sim.id, runsBefore)
    const partialRun = (afterPartial.generation_runs as Json[]).at(-1)!
    expect(partialRun.scope.kind).toBe('agents')
    expect(partialRun.scope.parts).toContain('placement')

    // Placement changed…
    expect(positionKeys(afterPartial)).not.toEqual(posBeforePartial)
    // …but the model snapshot was NOT refreshed (still the old agent name).
    const snapAfterPartial = await apiJson<Json>(api, 'get', `/simulations/${sim.id}/model-snapshot`)
    expect(snapAfterPartial.model.agents[0].name).toBe('Walker')

    // ---- FULL regenerate via the SplitButton main action + confirm dialog ----
    // A full regen likewise rolls a fresh seed, so positions change on their own.
    // Reload to a fresh `ready` editor first so the Generate control is
    // deterministically enabled (the sim is ready server-side per waitRegenDone;
    // this avoids depending on the partial-regen completion SSE arriving in-page).
    // Wait on the Generate button being ENABLED (the actual precondition) rather
    // than a status tag — the status .p-tag can live in a collapsed/hidden panel.
    await page.reload()
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })
    await expect(page.getByRole('button', { name: 'Generate', exact: true })).toBeEnabled({ timeout: 15_000 })

    cur = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    const posBeforeFull = positionKeys(cur)
    runsBefore = ((cur.generation_runs ?? []) as Json[]).length

    await page.getByRole('button', { name: 'Generate', exact: true }).click()
    const confirm = page.locator('.p-confirmdialog')
    await expect(confirm).toBeVisible()
    await expect(confirm).toContainText('This overwrites the current generated state')
    await confirm.getByRole('button', { name: 'Regenerate', exact: true }).click()

    const afterFull = await waitRegenDone(api, sim.id, runsBefore)
    expect((afterFull.generation_runs as Json[]).at(-1)!.scope.kind).toBe('full')

    // The world regenerated (new positions) …
    expect(positionKeys(afterFull)).not.toEqual(posBeforeFull)
    // … and the snapshot was REFRESHED to the current model (new agent name).
    const snapAfterFull = await apiJson<Json>(api, 'get', `/simulations/${sim.id}/model-snapshot`)
    expect(snapAfterFull.model.agents[0].name).toBe('Walker Mk2')

    // Editor is back to ready in the UI too (SSE-driven).
    await expect(page.locator('.p-tag').filter({ hasText: 'ready' }).first()).toBeVisible({ timeout: 15_000 })
  })

  test('SIM-16 header: inline rename persists, back arrow navigates to the list, Analysis disabled for a ready sim (tooltip)', async ({ page }) => {
    test.setTimeout(90_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 17, name: uniqueName('sim-header') })
    simIds.push(sim.id)

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    const nameInput = page.getByPlaceholder('Untitled simulation')
    await expect(nameInput).toHaveValue(sim.name, { timeout: 15_000 })

    // --- inline rename + Save → persists across reload + API ---
    const newName = `${sim.name}-renamed`
    await nameInput.fill(newName)
    await expect(page.getByText('unsaved')).toBeVisible()
    await waitPut(page, sim.id, () => page.getByRole('button', { name: 'Save', exact: true }).click())
    await page.reload()
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(newName, { timeout: 15_000 })
    const after = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    expect(after.name).toBe(newName)

    // --- Analysis is disabled for a `ready` sim; hover the wrapper for the hint ---
    const analysisBtn = page.getByRole('button', { name: 'Analysis' })
    await expect(analysisBtn).toBeDisabled()
    // The tooltip lives on an enabled wrapper span (disabled buttons swallow
    // hover events). Capture the text when reachable; non-fatal otherwise.
    const wrapper = page.locator('span.inline-flex', { has: analysisBtn })
    await wrapper.hover()
    const tooltip = page.locator('.p-tooltip')
    try {
      await expect(tooltip).toBeVisible({ timeout: 4_000 })
      const text = (await tooltip.textContent())?.trim() ?? ''
      test.info().annotations.push({ type: 'tooltip', description: `Analysis hint: "${text}"` })
      expect(text).toContain('finished')
    } catch {
      test.info().annotations.push({
        type: 'tooltip',
        description: 'Analysis tooltip did not appear on wrapper hover (disabled inner button suppresses events)',
      })
    }

    // --- back arrow → /simulations list ---
    await page.locator('a[title="Back to simulations"]').click()
    await expect(page).toHaveURL(/\/simulations$/)
  })

  test('SIM-17 auto-save parity: dirty General tab is silently persisted on switching to Agents (no Save click)', async ({ page }) => {
    test.setTimeout(90_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 18, name: uniqueName('sim-autosave') })
    simIds.push(sim.id)

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })

    // Dirty the General tab.
    await page.getByPlaceholder('What this simulation explores').fill('autosaved via tab switch')
    await expect(page.getByText('unsaved')).toBeVisible()

    // Switching tabs triggers the silent auto-save PUT — no Save click.
    await waitPut(page, sim.id, () => page.getByRole('tab', { name: /Agents/ }).click())

    // Silently saved: dirty chip gone, API carries the new description.
    await expect(page.getByText('unsaved')).toHaveCount(0)
    const after = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
    expect(after.description).toBe('autosaved via tab switch')
  })
})
