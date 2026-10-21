import { test, expect, type Page, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import { minimalSpatial, seedModel, seedReadySim, uniqueName, deleteQuietly } from '../_helpers/fixtures'

// §5.7 Simulations CRUD / list / lifecycle-status suite.
// Deterministic tier: the fixture model uses NO LLM anywhere — simulation
// generation is local and completes in well under a second.

function rx(s: string): RegExp {
  return new RegExp(s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
}

function idFromUrl(url: string): string {
  return url.replace(/[?#].*$/, '').split('/').pop() ?? ''
}

// Sorted "x,y" position fingerprints of a simulation's initial agents.
function positionKeys(sim: any): string[] {
  return ((sim.agents ?? []) as any[])
    .map((a) => `${a.position?.x},${a.position?.y}`)
    .sort()
}

// Full-body update request built from a GET response (the backend PUT expects
// the complete editable subset — a partial body fails validation before the
// editable-state check, masking the 409 this suite asserts on).
function toUpdateBody(sim: any): Record<string, unknown> {
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

test.describe('simulations CRUD', () => {
  let api: APIRequestContext
  let model: { id: string; name: string } // main ready fixture model (F1)
  const simIds: string[] = []
  const modelIds: string[] = []

  test.beforeAll(async () => {
    api = await apiContext()
    model = await seedModel(api, minimalSpatial(uniqueName('mdl-simcrud')))
    modelIds.push(model.id)
  })

  test.afterAll(async () => {
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  // Open the create-simulation dialog from the simulations list header.
  async function openCreateDialogFromList(page: Page) {
    await page.goto('/simulations')
    await page.getByRole('button', { name: 'Create simulation' }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    return dialog
  }

  test('SIM-01 create from list dialog → editor reaches ready via SSE without reload', async ({ page }) => {
    await login(page)
    const dialog = await openCreateDialogFromList(page)

    // Model picker: the first Select in the dialog (the two LLM override
    // pickers below also render Selects).
    await dialog.locator('.p-select').first().click()
    await page.getByRole('option', { name: model.name, exact: true }).click()

    const simName = uniqueName('sim')
    await dialog.getByPlaceholder('Untitled simulation').fill(simName)
    await dialog.getByPlaceholder('100').fill('10')
    await dialog.getByPlaceholder('random').fill('42')
    await dialog.getByRole('button', { name: 'Create', exact: true }).click()

    // 202 → navigate to the new simulation's editor.
    await page.waitForURL(/\/simulations\/[0-9a-f]{24}$/, { timeout: 20_000 })
    simIds.push(idFromUrl(page.url()))

    // A "Generating…" banner may flash; the status Tag must reach `ready`
    // WITHOUT any manual reload (SSE simulation.updated drives the store).
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(simName)
    // (header tag + summary panel tag both flip — assert the first)
    await expect(page.locator('.p-tag').filter({ hasText: 'ready' }).first()).toBeVisible({ timeout: 30_000 })
    // Sanity: the run gate opened too.
    await expect(page.getByRole('button', { name: 'Run', exact: true })).toBeEnabled()
  })

  test('SIM-02 create-sim entry points preselect and lock the model', async ({ page }) => {
    await login(page)

    // (a) Models list row action (aria-label="Create simulation").
    await page.goto('/models')
    const row = page.getByRole('row', { name: rx(model.name) })
    await expect(row).toBeVisible({ timeout: 15_000 })
    await row.getByRole('button', { name: 'Create simulation' }).click()

    let dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    // Preset model renders as a locked (disabled) InputText — no picker at all.
    const presetInput = dialog.locator('input').first()
    await expect(presetInput).toHaveValue(model.name)
    await expect(presetInput).toBeDisabled()
    await expect(dialog.getByText('Select a model')).toHaveCount(0)
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(dialog).toBeHidden()

    // (b) Model editor header "New simulation".
    await page.goto(`/models/${model.id}`)
    const newSimBtn = page.getByRole('button', { name: 'New simulation' })
    await expect(newSimBtn).toBeEnabled({ timeout: 15_000 })
    await newSimBtn.click()

    dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    const presetInput2 = dialog.locator('input').first()
    await expect(presetInput2).toHaveValue(model.name)
    await expect(presetInput2).toBeDisabled()
    await dialog.getByRole('button', { name: 'Cancel' }).click()
  })

  test('SIM-03 draft (not-ready) model is disabled in the create dialog', async ({ page }) => {
    // A bare POST /models draft has no agents → simulation_ready=false.
    const draftName = uniqueName('mdl-draft')
    const draft = await apiJson<{ id: string }>(api, 'post', '/models', { name: draftName }, 201)
    modelIds.push(draft.id)

    await login(page)
    const dialog = await openCreateDialogFromList(page)
    await dialog.locator('.p-select').first().click()

    // Draft models are labelled "(not ready)" and rendered disabled.
    const option = page.getByRole('option', { name: `${draftName} (not ready)` })
    await expect(option).toBeVisible({ timeout: 15_000 })
    await expect(option).toHaveAttribute('aria-disabled', 'true')
    // Clicking a disabled option must not select it (Create stays disabled —
    // no model chosen).
    await option.click({ force: true })
    await expect(dialog.getByRole('button', { name: 'Create', exact: true })).toBeDisabled()
  })

  test('SIM-04 list model filter mirrors ?model_id=, deep-links, sorts; columns show Status/Steps/Agents', async ({ page }) => {
    test.setTimeout(120_000)
    // Second model + three sims: two on the main model with a known
    // alphabetical order, one on model B (must be filtered out).
    const modelB = await seedModel(api, minimalSpatial(uniqueName('mdl-simfilter')))
    modelIds.push(modelB.id)
    const simAA = await seedReadySim(api, model.id, { steps: 10, seed: 1, name: uniqueName('sim-aa') })
    const simZZ = await seedReadySim(api, model.id, { steps: 10, seed: 2, name: uniqueName('sim-zz') })
    const simB = await seedReadySim(api, modelB.id, { steps: 10, seed: 3, name: uniqueName('sim-bb') })
    simIds.push(simAA.id, simZZ.id, simB.id)

    await login(page)
    await page.goto('/simulations')

    // Filter to the main model via the header MultiSelect (mirrors ?model_id=).
    await page.locator('.p-multiselect').click()
    await page.getByRole('searchbox').fill(model.name)
    await page.getByRole('option', { name: model.name, exact: true }).click()
    await page.keyboard.press('Escape')

    await expect(page).toHaveURL(new RegExp(`model_id=${model.id}`))
    await expect(page.getByRole('row', { name: rx(simAA.name) })).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('row', { name: rx(simZZ.name) })).toBeVisible()
    await expect(page.getByRole('row', { name: rx(simB.name) })).toBeHidden()

    // Columns: Status / Steps / Agents (+ status tag and values on our row).
    for (const header of ['Status', 'Steps', 'Agents']) {
      await expect(page.getByRole('columnheader', { name: header })).toBeVisible()
    }
    const rowAA = page.getByRole('row', { name: rx(simAA.name) })
    await expect(rowAA.locator('.p-tag')).toHaveText('ready')
    await expect(rowAA).toContainText('10') // steps
    await expect(rowAA).toContainText('4') // agents (F1 spawns 4 walkers)

    // Sort by Name asc → our aa row precedes our zz row; desc → reversed.
    // Only the RELATIVE order of our own rows is asserted (foreign e2e rows
    // are tolerated, though the model filter already scopes to ours).
    const relativeOrder = async () => {
      const texts = await page.locator('tbody tr').allTextContents()
      const ia = texts.findIndex((t) => t.includes(simAA.name))
      const iz = texts.findIndex((t) => t.includes(simZZ.name))
      return { ia, iz }
    }
    await page.getByRole('columnheader', { name: 'Name' }).click()
    await expect
      .poll(async () => {
        const { ia, iz } = await relativeOrder()
        return ia >= 0 && iz >= 0 && ia < iz
      }, { message: 'name asc: sim-aa before sim-zz' })
      .toBe(true)
    await page.getByRole('columnheader', { name: 'Name' }).click()
    await expect
      .poll(async () => {
        const { ia, iz } = await relativeOrder()
        return ia >= 0 && iz >= 0 && ia > iz
      }, { message: 'name desc: sim-zz before sim-aa' })
      .toBe(true)

    // Pagination: the paginator is always visible with a page report.
    await expect(page.locator('.p-paginator')).toBeVisible()
    await expect(page.getByText(/Showing \d+–\d+ of \d+/)).toBeVisible()

    // Deep-link: ?model_id=<B> pre-applies the filter (control + rows).
    await page.goto(`/simulations?model_id=${modelB.id}`)
    await expect(page.getByRole('row', { name: rx(simB.name) })).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('row', { name: rx(simAA.name) })).toBeHidden()
    await expect(page.locator('.p-multiselect')).toContainText(modelB.name)
  })

  test('SIM-10 list row actions: Run enabled, Duplicate redirects to copy, Delete removes', async ({ page }) => {
    test.setTimeout(90_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 5, name: uniqueName('sim-actions') })
    simIds.push(sim.id)

    await login(page)
    await page.goto(`/simulations?model_id=${model.id}`)
    const row = page.getByRole('row', { name: rx(sim.name) })
    await expect(row).toBeVisible({ timeout: 15_000 })

    // Run action is enabled for a ready simulation.
    await expect(row.getByRole('button', { name: 'Run' })).toBeEnabled()

    // Duplicate: inline name dialog (pre-filled "<name> (copy)") → redirects
    // to the copy's editor.
    await row.getByRole('button', { name: 'Duplicate' }).click()
    const dupDialog = page.getByRole('dialog')
    await expect(dupDialog).toBeVisible()
    await expect(dupDialog.getByPlaceholder('Copy name')).toHaveValue(`${sim.name} (copy)`)
    await dupDialog.getByRole('button', { name: 'Duplicate', exact: true }).click()
    await page.waitForURL(/\/simulations\/[0-9a-f]{24}$/, { timeout: 20_000 })
    const copyId = idFromUrl(page.url())
    expect(copyId, 'redirected to a different simulation id').not.toBe(sim.id)
    simIds.push(copyId)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(`${sim.name} (copy)`, { timeout: 15_000 })

    // Delete the original from the list: confirm dialog → row gone (+ API 404).
    await page.goto(`/simulations?model_id=${model.id}`)
    const rowAgain = page.getByRole('row', { name: rx(sim.name), exact: false }).filter({ hasNotText: '(copy)' })
    await expect(rowAgain).toBeVisible({ timeout: 15_000 })
    await rowAgain.getByRole('button', { name: 'Delete' }).click()
    const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(confirm.getByText('Delete simulation')).toBeVisible()
    await confirm.getByRole('button', { name: 'Delete', exact: true }).click()
    await expect(rowAgain).toBeHidden({ timeout: 15_000 })
    const gone = await api.get(u(`/simulations/${sim.id}`))
    expect(gone.status()).toBe(404)
  })

  test('SIM-11 status gating: generating locks the editor; PUT while running → 409 not_editable', async ({ page }) => {
    test.setTimeout(120_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 6, name: uniqueName('sim-gating') })
    simIds.push(sim.id)

    // --- (a) UI gating for status=generating. Local (no-LLM) generation
    // completes in <1s, far too fast to observe live, so the GET response is
    // stubbed to `generating` — this deterministically exercises the exact
    // UI gating path (store.generating derives purely from status).
    const real = await apiJson<any>(api, 'get', `/simulations/${sim.id}`)
    const simUrl = `**/api/v1/intellisim/simulations/${sim.id}`
    await page.route(simUrl, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback()
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ...real, status: 'generating' }),
      })
    })

    await login(page)
    await page.goto(`/simulations/${sim.id}`)
    // Progress banner + read-only editor + Run disabled + Generate (the
    // regenerate control) visible-but-disabled while a generation is in flight.
    await expect(page.getByRole('alert').getByText('Generating…')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('button', { name: 'Run', exact: true })).toBeDisabled()
    await expect(page.getByPlaceholder('Untitled simulation')).toBeDisabled()
    await expect(page.getByRole('button', { name: 'Generate', exact: true })).toBeDisabled()
    await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled()
    await page.unroute(simUrl)

    // --- (b) Real full regenerate via API: 202 → back to ready → Run enabled.
    // A full regen rolls a FRESH seed (stored per GenerationRun); the new
    // generation_runs[last].seed becomes the sim's seed.
    const before = ((real.generation_runs ?? []) as any[]).length
    const seedBefore = real.seed
    await apiJson(api, 'post', `/simulations/${sim.id}/generate`, { scope: { kind: 'full' } }, 202)
    const after = await pollUntil(async () => {
      const s = await apiJson<any>(api, 'get', `/simulations/${sim.id}`)
      if (s.status === 'generation_failed') throw new Error('regenerate failed')
      return s.status === 'ready' && ((s.generation_runs ?? []) as any[]).length > before ? s : null
    }, { timeoutMs: 30_000, label: 'regenerated sim ready' })
    expect(((after.generation_runs ?? []) as any[]).length).toBeGreaterThan(before)

    // Fresh seed rolled and recorded on the latest generation run (JS-safe int).
    expect(after.seed, 'a full regen rolls a fresh seed').not.toBe(seedBefore)
    expect(after.seed).toBeLessThan(Number.MAX_SAFE_INTEGER)
    const lastRun = ((after.generation_runs ?? []) as any[]).at(-1)
    expect(lastRun.seed, 'generation_runs[last].seed === sim.seed').toBe(after.seed)

    await page.goto(`/simulations/${sim.id}`)
    await expect(page.locator('.p-tag').filter({ hasText: 'ready' }).first()).toBeVisible({ timeout: 15_000 })
    await expect(page.getByRole('button', { name: 'Run', exact: true })).toBeEnabled()

    // --- (c) [API] PUT while running → 409 not_editable. Steps=200 keeps the
    // run alive long enough; never send control right after POST /run —
    // validate `running` via polling first (async start race).
    const runSim = await seedReadySim(api, model.id, { steps: 200, seed: 8, name: uniqueName('sim-running') })
    simIds.push(runSim.id)
    await apiJson(api, 'post', `/simulations/${runSim.id}/run`, {}, 202)
    const running = await pollUntil(async () => {
      const s = await apiJson<any>(api, 'get', `/simulations/${runSim.id}`)
      return s.status !== 'ready' && s.status !== 'queued' ? s : null
    }, { timeoutMs: 20_000, intervalMs: 250, label: 'run started' })
    expect(running.status).toBe('running')

    const res = await api.put(u(`/simulations/${runSim.id}`), { data: toUpdateBody(running) })
    expect(res.status(), await res.text()).toBe(409)
    expect((await res.json()).code).toBe('not_editable')

    await apiJson(api, 'post', `/simulations/${runSim.id}/control`, { action: 'stop' }, 200)
    const final = await pollUntil(async () => {
      const s = await apiJson<any>(api, 'get', `/simulations/${runSim.id}`)
      return ['stopped', 'finished', 'failed'].includes(s.status) ? s.status : null
    }, { timeoutMs: 20_000, label: 'run stopped' })
    expect(final).toBe('stopped')
  })

  test('SIM-12 [API] create with bogus llm_generation_target → immediate 422 invalid_llm_target', async () => {
    // OBSERVED BEHAVIOR (documented per plan): the backend validates the LLM
    // target capability synchronously at create time and rejects with
    // 422 invalid_llm_target — NO simulation is created, so there is no
    // `generation_failed` editor state to assert (that branch is unreachable
    // via a bogus target on this endpoint).
    const name = uniqueName('sim-badllm')
    const res = await api.post(u('/simulations'), {
      data: {
        model_id: model.id,
        name,
        steps: 5,
        llm_generation_target: { provider_uid: 'nope', model_uid: 'nope' },
      },
    })
    expect(res.status(), await res.text().catch(() => '')).toBe(422)
    const body = await res.json()
    expect(body.code).toBe('invalid_llm_target')

    // And no orphan simulation was persisted under that name.
    const list = await apiJson<any>(api, 'get', '/simulations?page=1&page_size=100')
    expect(((list.items ?? []) as any[]).some((s) => s.name === name)).toBe(false)
  })

  test('SIM-13 [API] seed determinism: same seed ⇒ identical initial positions, different seed differs', async () => {
    test.setTimeout(90_000)
    const a = await seedReadySim(api, model.id, { steps: 10, seed: 4242, name: uniqueName('sim-seed-a') })
    const b = await seedReadySim(api, model.id, { steps: 10, seed: 4242, name: uniqueName('sim-seed-b') })
    const c = await seedReadySim(api, model.id, { steps: 10, seed: 7, name: uniqueName('sim-seed-c') })
    simIds.push(a.id, b.id, c.id)

    const [simA, simB, simC] = await Promise.all([
      apiJson<any>(api, 'get', `/simulations/${a.id}`),
      apiJson<any>(api, 'get', `/simulations/${b.id}`),
      apiJson<any>(api, 'get', `/simulations/${c.id}`),
    ])
    expect(simA.agents?.length).toBe(4)
    expect(positionKeys(simA)).toEqual(positionKeys(simB))
    expect(positionKeys(simC)).not.toEqual(positionKeys(simA))
  })
})
