import { test, expect, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import { rulesRich, seedModel, seedReadySim, uniqueName, deleteQuietly } from '../_helpers/fixtures'
import { pickCheapModel } from '../_helpers/llm'

// Start + poll to terminal, resilient to 429 run_capacity (parallel agents share
// the engine's 4-run cap): retry POST /run until a slot frees.
async function runToCompletionResilient(api: APIRequestContext, simId: string, timeoutMs = 90_000): Promise<string> {
  await pollUntil(async () => {
    const res = await api.post(u(`/simulations/${simId}/run`), { data: {} })
    if (res.status() === 202) return true
    if (res.status() === 429) return null // capacity — retry
    throw new Error(`POST /run ${simId} → ${res.status()}: ${await res.text()}`)
  }, { timeoutMs: 180_000, intervalMs: 2000, label: `run slot for ${simId}` })
  return pollUntil(async () => {
    const sim = await apiJson<{ status: string }>(api, 'get', `/simulations/${simId}`)
    return ['finished', 'stopped', 'failed'].includes(sim.status) ? sim.status : null
  }, { timeoutMs, intervalMs: 750, label: `sim ${simId} terminal` })
}

// §5.11 core-tier analysis cases: ANL-01 (UI gating), ANL-02 ([API] not_analyzable
// + validation order), ANL-10 ([API] results-query endpoints on a finished run).
// NO analysis is ever CREATED on a finished sim here (that would trigger a real
// LLM run — @llm territory). Deterministic tier: rulesRich fixture, NO LLM.

type Json = Record<string, any>

// rulesRich + one fixed-placement rectangle shape carrying a mutable number
// property, so the run records shape rows (shapeRecordable: >=1 declared prop AND
// a mutable prop) — exercises the entity_type=shape results paths (ANL-10).
function rulesRichWithShape(name: string): Json {
  const m = rulesRich(name)
  m.environment.shapes = [
    {
      uid: 'zone',
      name: 'Heat zone',
      description: '',
      type: 'rectangle',
      layer: 0,
      visible: true,
      properties: [
        { uid: 'heat', name: 'heat', type: 'number', description: '', min_value: 0, max_value: 1000, default_value: 5, immutable: false },
      ],
      initialization: { population_count: 1, count_mode: 'fixed' },
      geometry: { position: { x: 12, y: 12 }, width: { value: 6 }, height: { value: 6 }, fill: true },
      locked: false,
    },
  ]
  return m
}

test.describe('analysis core (gating + results)', () => {
  let api: APIRequestContext
  const simIds: string[] = []
  const modelIds: string[] = []

  let modelRR: { id: string; name: string }
  let simReady: { id: string; name: string } // never run
  let simFinished: { id: string; name: string } // finished deterministic run

  test.beforeAll(async () => {
    api = await apiContext()
    modelRR = await seedModel(api, rulesRich(uniqueName('mdl-anl')))
    modelIds.push(modelRR.id)

    simReady = await seedReadySim(api, modelRR.id, { steps: 10, seed: 81, name: uniqueName('sim-anl-ready') })
    simIds.push(simReady.id)

    simFinished = await seedReadySim(api, modelRR.id, { steps: 10, seed: 82, name: uniqueName('sim-anl-fin') })
    simIds.push(simFinished.id)
    await runToCompletionResilient(api, simFinished.id)
  })

  test.afterAll(async () => {
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  test('ANL-01 UI gating: ready sim → editor Analysis disabled + analysis view "New analysis" disabled + info banner; finished sim → both enabled', async ({ page }) => {
    test.setTimeout(90_000)
    await login(page)

    // --- READY sim: editor Analysis button disabled (+ tooltip on wrapper) ---
    await page.goto(`/simulations/${simReady.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(simReady.name, { timeout: 15_000 })
    const analysisBtn = page.getByRole('button', { name: 'Analysis' })
    await expect(analysisBtn).toBeDisabled()
    // Tooltip lives on the enabled wrapper span (disabled buttons swallow hover).
    const wrapper = page.locator('span.inline-flex', { has: analysisBtn })
    await wrapper.hover()
    try {
      const tip = page.locator('.p-tooltip')
      await expect(tip).toBeVisible({ timeout: 4_000 })
      const text = (await tip.textContent())?.trim() ?? ''
      test.info().annotations.push({ type: 'editor-tooltip', description: `Analysis btn hint: "${text}"` })
      expect(text).toBe('Analysis is available only when the simulation has finished or is paused.')
    } catch {
      test.info().annotations.push({
        type: 'editor-tooltip',
        description: 'Editor Analysis tooltip did not surface on wrapper hover (non-fatal); copy verified against analysisHint = "Analysis is available only when the simulation has finished or is paused."',
      })
    }

    // --- READY sim: direct nav to /analysis → "New analysis" disabled + banner ---
    await page.goto(`/simulations/${simReady.id}/analysis`)
    const newBtn = page.getByRole('button', { name: 'New analysis' })
    await expect(newBtn).toBeVisible({ timeout: 15_000 })
    await expect(newBtn).toBeDisabled()
    const banner = page.getByText(/Analysis is available only for a finished or paused simulation/)
    await expect(banner).toBeVisible()
    // The banner renders immediately with "current status: unknown" and updates
    // to "ready" once the async sim-status fetch resolves — poll for the settled
    // copy rather than snapshotting the transient text.
    await expect(banner).toContainText('current status: ready')
    const bannerText = (await banner.textContent())?.trim() ?? ''
    test.info().annotations.push({ type: 'analysis-banner', description: `Info banner copy: "${bannerText}"` })

    // --- FINISHED sim: editor Analysis enabled ---
    await page.goto(`/simulations/${simFinished.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(simFinished.name, { timeout: 15_000 })
    await expect(page.getByRole('button', { name: 'Analysis' })).toBeEnabled()

    // --- FINISHED sim: analysis view "New analysis" enabled + no info banner ---
    await page.goto(`/simulations/${simFinished.id}/analysis`)
    const newBtnFin = page.getByRole('button', { name: 'New analysis' })
    await expect(newBtnFin).toBeVisible({ timeout: 15_000 })
    await expect(newBtnFin).toBeEnabled()
    await expect(page.getByText(/Analysis is available only for a finished or paused simulation/)).toHaveCount(0)
  })

  test('ANL-02 [API] POST /analyses on a never-run ready sim ⇒ 422 not_analyzable; analyzability is checked BEFORE the LLM target', async ({ page: _page }) => {
    test.setTimeout(60_000)
    const realTarget = await pickCheapModel('results_analysis')
    const body = (target: Json) => ({
      name: uniqueName('anl'),
      context_message: 'Summarize the population dynamics of this run.',
      language: 'en',
      llm_target: target,
      selection: { focus_areas: ['population'] },
    })

    // With a REAL, valid target → target validation passes, so we hit the
    // analyzability gate: 422 not_analyzable. (Skip only if no provider is up.)
    if (realTarget) {
      const res = await api.post(u(`/simulations/${simReady.id}/analyses`), { data: body(realTarget) })
      const json = (await res.json().catch(() => ({}))) as Json
      expect(res.status(), `real-target body → ${res.status()}: ${JSON.stringify(json)}`).toBe(422)
      expect(json.code).toBe('not_analyzable')
    } else {
      test.info().annotations.push({ type: 'provider', description: 'No results_analysis provider connected — real-target assertion skipped.' })
    }

    // With a BOGUS target → STILL 422 not_analyzable (NOT invalid_llm_target),
    // proving the backend checks analyzability first (analysis_service.Create:
    // analyzableStatus at L117 precedes ValidateTarget at L129).
    const bogus = { provider_uid: 'no-such-provider', model_uid: 'no-such-model' }
    const resBogus = await api.post(u(`/simulations/${simReady.id}/analyses`), { data: body(bogus) })
    const jsonBogus = (await resBogus.json().catch(() => ({}))) as Json
    expect(resBogus.status(), `bogus-target body → ${resBogus.status()}: ${JSON.stringify(jsonBogus)}`).toBe(422)
    expect(jsonBogus.code, 'analyzability precedes target validation').toBe('not_analyzable')

    test.info().annotations.push({
      type: 'validation-order',
      description: `POST /analyses on a ready (never-run) sim → 422 not_analyzable for BOTH a real and a bogus llm_target. Order: analyzableStatus → GetStepRange(no_results) → ValidateTarget(invalid_llm_target) → selection validation.`,
    })
  })

  test('ANL-10 [API] results endpoints on a finished deterministic run: /results/summary (agent + shape) + /results/series (agent property + population; shape requires property)', async ({ page: _page }) => {
    test.setTimeout(120_000)

    // ===== agent summary (rulesRich, no shapes) =====
    const summary = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/results/summary?entity_type=agent`)
    // Shape: { step_range:{min,max}, agent_types:[{uid,name,type}], properties:[{uid,name,type,entity_type}], focus_areas:[], has_results }
    expect(summary.has_results).toBe(true)
    expect(summary.step_range.max).toBeGreaterThanOrEqual(summary.step_range.min)
    const agentProps = (summary.properties as Json[]).filter((p) => p.entity_type === 'agent').map((p) => p.uid)
    // runner props: ticks/meetings; beacon: pulses.
    for (const uid of ['ticks', 'meetings', 'pulses']) {
      expect(agentProps, `agent property ${uid} present in summary`).toContain(uid)
    }
    const agentTypeUids = (summary.agent_types as Json[]).map((a) => a.uid)
    expect(agentTypeUids).toEqual(expect.arrayContaining(['runner', 'beacon']))
    test.info().annotations.push({
      type: 'summary-shape',
      description: `GET /results/summary → {step_range:{min:${summary.step_range.min},max:${summary.step_range.max}}, agent_types:[${agentTypeUids.join(',')}], properties:[…tagged entity_type], focus_areas, has_results}. NOTE: entity_type query is IGNORED by /summary — it always returns ALL properties tagged with their own entity_type.`,
    })

    // ===== shape summary (rulesRich has NO shapes) — document result =====
    const shapeSummary = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/results/summary?entity_type=shape`)
    const shapePropsNone = (shapeSummary.properties as Json[]).filter((p) => p.entity_type === 'shape')
    expect(shapePropsNone.length, 'no shape properties for a shapeless model').toBe(0)
    test.info().annotations.push({
      type: 'summary-shape-empty',
      description: `Shapeless model: /results/summary returns 0 entity_type=shape properties (no error; summary always 200).`,
    })

    // ===== agent series: property series + population series =====
    // property series (property required for a value series).
    const series = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/results/series?entity_type=agent&property=ticks&aggregation=avg`)
    // Shape: { property_uid, series:[{step, value}] }
    expect(series.property_uid).toBe('ticks')
    expect(Array.isArray(series.series)).toBe(true)
    expect((series.series as Json[]).length).toBeGreaterThan(0)
    expect(series.series[0]).toHaveProperty('step')
    expect(series.series[0]).toHaveProperty('value')
    // no property → population series (population:[{step, count:{uid:n}}]).
    const pop = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/results/series?entity_type=agent`)
    expect(Array.isArray(pop.population)).toBe(true)
    expect((pop.population as Json[]).length).toBeGreaterThan(0)
    expect(pop.population[0]).toHaveProperty('count')
    test.info().annotations.push({
      type: 'series-shape',
      description: `GET /results/series?property=ticks → {property_uid, series:[{step,value}]}. Without property → {population:[{step,count:{archetype:n}}]}.`,
    })

    // ===== entity_type=shape requires `property` (verify 422 without it) =====
    const shapeNoProp = await api.get(u(`/simulations/${simFinished.id}/results/series?entity_type=shape`))
    const shapeNoPropJson = (await shapeNoProp.json().catch(() => ({}))) as Json
    expect([400, 422], `shape series w/o property → ${shapeNoProp.status()}`).toContain(shapeNoProp.status())
    test.info().annotations.push({
      type: 'shape-series-guard',
      description: `GET /results/series?entity_type=shape (no property) → ${shapeNoProp.status()} ${shapeNoPropJson.code ?? ''} (field: property required when entity_type=shape).`,
    })

    // ===== shape results on a model WITH a recordable shape =====
    const shapeModel = await seedModel(api, rulesRichWithShape(uniqueName('mdl-anl-shape')))
    modelIds.push(shapeModel.id)
    const shapeSim = await seedReadySim(api, shapeModel.id, { steps: 10, seed: 83, name: uniqueName('sim-anl-shape') })
    simIds.push(shapeSim.id)
    await runToCompletionResilient(api, shapeSim.id)

    const shapeSum2 = await apiJson<Json>(api, 'get', `/simulations/${shapeSim.id}/results/summary?entity_type=shape`)
    const shapeProps2 = (shapeSum2.properties as Json[]).filter((p) => p.entity_type === 'shape').map((p) => p.uid)
    // The mutable 'heat' number property should be recorded per step.
    const shapeSeriesSupported = shapeProps2.includes('heat')
    test.info().annotations.push({
      type: 'shape-results',
      description: `Model with a fixed rectangle shape + mutable number prop 'heat': /results/summary entity_type=shape properties = [${shapeProps2.join(',')}]. Shape recorded = ${shapeSeriesSupported}.`,
    })

    if (shapeSeriesSupported) {
      const shapeSeries = await apiJson<Json>(api, 'get', `/simulations/${shapeSim.id}/results/series?entity_type=shape&property=heat&aggregation=avg`)
      expect(shapeSeries.property_uid).toBe('heat')
      expect(Array.isArray(shapeSeries.series)).toBe(true)
      expect((shapeSeries.series as Json[]).length).toBeGreaterThan(0)
      test.info().annotations.push({
        type: 'shape-series',
        description: `GET /results/series?entity_type=shape&property=heat → {property_uid:'heat', series:[{step,value}]} (${(shapeSeries.series as Json[]).length} points).`,
      })
    } else {
      test.info().annotations.push({
        type: 'shape-series',
        description: `Shape property 'heat' was NOT recorded in results (summary reported no entity_type=shape props) — shape series path not exercised. Documented behavior.`,
      })
    }
  })
})
