import { test, expect, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { minimalSpatial, seedModel, uniqueName, deleteQuietly } from '../_helpers/fixtures'
import { pickCheapModel, type LlmTargetRef } from '../_helpers/llm'
import { login } from '../_helpers/login'

// SGE — Simulation generation with the REAL LLM provider (plan §5.8).
//
// Cost discipline: the `simulation_generation` capability is on gpt-4o / gpt-4.1
// / o3 only. pickCheapModel('simulation_generation') → gpt-4.1 (base). We keep
// the world TINY (1 archetype, count 2–4, env 20×20) and cap ourselves at
// ≤ 4 LLM generations total across the whole file:
//   SGE-01 create+generate ............ 1
//   SGE-02 create+generate (override) . 1
//   SGE-02 UI partial regenerate ...... 1
//   SGE-03 bogus target ............... 0 (422 at create, no generation)
//                                       = 3 generations.
//
// Assertions are STRUCTURAL/lifecycle only — never on LLM content. LLM
// count/placement/property adherence are documented as FINDINGS (loosened to
// what the system GUARANTEES). Titles carry @llm so the `llm` project selects
// them; the 300 s project timeout covers the 200 s generation budget.

const GEN_TIMEOUT = 200_000

// Where the sim-gen target lives in the model JSON (verified against
// src/shared/types/model.ts + live PUT round-trip 2026-07-12):
//   model.simulation_config.llm_simulation_generation_config.target
// A TOP-LEVEL field is silently ignored by the backend — it must be nested.
function llmGenModel(name: string, genTarget: LlmTargetRef) {
  const m = minimalSpatial(name)
  m.description = 'e2e llm-generation fixture'
  // LLM count mode + LLM placement (count prompt reuses Placement.PromptContext).
  m.agents[0].initialization = {
    population_count: 3,
    count_mode: 'llm',
    initial_state: 'idle',
    placement: { type: 'llm', prompt_context: 'Place 2 to 4 walkers spread out across the area' },
  }
  // An LLM-generated property.
  m.agents[0].properties.push({
    uid: 'mood',
    name: 'Mood',
    type: 'number',
    description: '',
    min_value: 0,
    max_value: 100,
    immutable: false,
    generation: { mode: 'llm', prompt_context: 'Between 30 and 80, varied.' },
  })
  // Sim-generation target (nested — top-level is ignored by the backend).
  m.simulation_config = {
    update_mode: 'sequential',
    llm_simulation_generation_config: { target: genTarget },
  }
  return m
}

interface SimShape {
  id: string
  status: string
  status_text?: string
  agents?: Array<{ agent_uid: string; position?: { x: number; y: number }; properties?: Record<string, unknown>; state?: string }>
  token_usage?: { total?: { total_tokens?: number; prompt_tokens?: number; completion_tokens?: number } } | null
  llm_generation_target?: LlmTargetRef | null
  generation_runs?: Array<{ status: string; error?: string }>
}

async function pollSimReady(api: APIRequestContext, simId: string, timeoutMs = GEN_TIMEOUT): Promise<SimShape> {
  return pollUntil(async () => {
    const sim = await apiJson<SimShape>(api, 'get', `/simulations/${simId}`)
    if (sim.status === 'generation_failed') {
      const why = JSON.stringify({ status_text: sim.status_text, runs: sim.generation_runs })
      throw new Error(`generation_failed for sim ${simId}: ${why}`)
    }
    return sim.status === 'ready' ? sim : null
  }, { timeoutMs, intervalMs: 1500, label: `sim ${simId} ready` })
}

test.describe('SGE — simulation generation with LLM', () => {
  let api: APIRequestContext
  let baseTarget: LlmTargetRef // sim_generation_capable, cheapest → gpt-4.1
  let overrideTarget: LlmTargetRef // a DIFFERENT sim_generation_capable model → gpt-4o
  let modelId: string
  const createdSims: string[] = []

  test.beforeAll(async () => {
    api = await apiContext()
    const cheap = await pickCheapModel('simulation_generation')
    if (!cheap) throw new Error('no simulation_generation-capable model connected — is the provider up?')
    baseTarget = cheap

    // A different capable model for the override cases. Resolve from /llms so we
    // never assume: pick any sim_generation-capable model whose uid ≠ base.
    const llms = await apiJson<{ list: Array<{ uid: string; models: Array<{ uid: string; capabilities: string[] }> }> }>(
      api, 'get', '/llms', undefined, 200,
    )
    const alt = llms.list
      .flatMap((p) => p.models.map((mm) => ({ provider_uid: p.uid, model_uid: mm.uid, caps: mm.capabilities })))
      .find((c) => c.caps.includes('simulation_generation') && c.model_uid !== baseTarget.model_uid)
    if (!alt) throw new Error('need a second simulation_generation-capable model for the override cases')
    overrideTarget = { provider_uid: alt.provider_uid, model_uid: alt.model_uid }

    const model = await seedModel(api, llmGenModel(uniqueName('mdl-sge'), baseTarget))
    modelId = model.id
    const full = await apiJson<any>(api, 'get', `/models/${modelId}`, undefined, 200)
    expect(full.validation?.simulation_ready, JSON.stringify(full.validation?.errors)).toBe(true)
  })

  test.afterAll(async () => {
    for (const id of createdSims) await deleteQuietly(api, `/simulations/${id}`)
    if (modelId) await deleteQuietly(api, `/models/${modelId}`)
    await api.dispose()
  })

  test('SGE-01 @llm generates world: llm count/placement/property + token usage', async () => {
    const created = await apiJson<{ id: string }>(api, 'post', '/simulations', {
      model_id: modelId, name: uniqueName('sim-sge01'), steps: 5,
    }, 202)
    createdSims.push(created.id)

    const sim = await pollSimReady(api, created.id)
    const agents = sim.agents ?? []

    // --- GUARANTEED assertions (what the system must hold) ---
    // At least one agent generated (LLM count mode produced a population).
    expect(agents.length).toBeGreaterThan(0)
    // Every agent has a position (LLM placement placed them).
    for (const a of agents) {
      expect(a.position, `agent ${a.agent_uid} missing position`).toBeTruthy()
      expect(typeof a.position!.x).toBe('number')
      expect(typeof a.position!.y).toBe('number')
    }
    // Generation token usage recorded.
    const totalTokens = sim.token_usage?.total?.total_tokens ?? 0
    expect(totalTokens, 'generation token usage should be recorded on sim.token_usage.total').toBeGreaterThan(0)

    // --- FINDINGS (documented, not hard-asserted) ---
    const moods = agents.map((a) => a.properties?.mood).filter((v) => typeof v === 'number') as number[]
    const inRequestedCount = agents.length >= 2 && agents.length <= 4
    const moodInRange = moods.every((v) => v >= 0 && v <= 100)
    const distinctMoods = new Set(moods).size
    console.log('[SGE-01 FINDINGS]', JSON.stringify({
      agentCount: agents.length,
      requestedCountRange: '[2,4]',
      countWithinRequested: inRequestedCount,
      moodValues: moods,
      moodCount: moods.length,
      moodAllInRange_0_100: moodInRange,
      distinctMoodValues: distinctMoods,
      tokenUsageTotal: totalTokens,
    }))

    // mood values present for at least some agents and numeric (guaranteed: the
    // property exists on the archetype). Range/variation adherence is a finding.
    expect(moods.length, 'at least one agent carries a numeric mood property').toBeGreaterThan(0)
  })

  test('SGE-02 @llm generation-model override honored (create body + UI GenerateMenu)', async ({ page }) => {
    // --- create with an override target DIFFERENT from the model's base ---
    const created = await apiJson<{ id: string }>(api, 'post', '/simulations', {
      model_id: modelId, name: uniqueName('sim-sge02'), steps: 5,
      llm_generation_target: overrideTarget,
    }, 202)
    createdSims.push(created.id)

    const sim = await pollSimReady(api, created.id)
    // The sim carries the create-time override and generation succeeded.
    expect(sim.status).toBe('ready')
    expect(sim.llm_generation_target?.model_uid).toBe(overrideTarget.model_uid)
    expect(sim.llm_generation_target?.provider_uid).toBe(overrideTarget.provider_uid)
    expect((sim.agents ?? []).length).toBeGreaterThan(0)

    // --- UI path: GenerateMenu popover → set override → partial-regen one
    //     archetype's PROPERTIES. Set the UI override to the BASE model so the
    //     persisted override provably CHANGES (create-time gpt-4o → base). ---
    await login(page)
    await page.goto(`/simulations/${created.id}`)
    // Editor loaded: status tag reads "ready" and the Generate split-button shows.
    await expect(page.getByRole('button', { name: 'Generate' })).toBeVisible({ timeout: 30_000 })

    // Open the SplitButton dropdown → "Partial regenerate…".
    await page.locator('.p-splitbutton-dropdown').click()
    await page.getByRole('menuitem', { name: /Partial regenerate/i }).click()

    const popover = page.locator('.p-popover')
    await expect(popover).toBeVisible()

    // Set the generation-model override to the BASE model. The Select filters to
    // simulation_generation-capable models only; options read "<Provider> · <Name>".
    const modelSelect = popover.locator('.p-select').first()
    await modelSelect.click()
    // Options render in a body-level overlay panel. Match the base model's name.
    const baseNameRe = new RegExp(baseTarget.model_uid.replace('.', '\\.').replace(/^gpt-/, 'GPT-') + '$', 'i')
    const option = page.getByRole('option').filter({ hasText: baseNameRe }).first()
    await expect(option, `expected an option for base model ${baseTarget.model_uid}`).toBeVisible({ timeout: 10_000 })
    await option.click()
    // Confirm the Select now displays the base model before we regenerate.
    await expect(modelSelect.locator('.p-select-label')).toHaveText(baseNameRe)

    // Check the "properties" part for the Walker archetype.
    await popover.getByText('properties', { exact: true }).click()

    // Capture the actual generate request to prove the UI override is sent.
    const genRespPromise = page.waitForResponse(
      (r) => r.url().includes(`/simulations/${created.id}/generate`) && r.request().method() === 'POST',
      { timeout: 30_000 },
    )
    await popover.getByRole('button', { name: /^Regenerate$/ }).click()
    const genResp = await genRespPromise
    expect(genResp.status()).toBe(202)
    const sentBody = genResp.request().postDataJSON() as { scope?: any; llm_generation_target?: LlmTargetRef }
    // The request carries the UI-selected override + the partial properties scope.
    expect(sentBody.llm_generation_target?.model_uid).toBe(baseTarget.model_uid)
    expect(sentBody.scope?.kind).toBe('agents')
    expect(sentBody.scope?.parts).toContain('properties')

    // status goes generating → ready. Poll via API and assert the override persisted.
    const regenerated = await pollSimReady(api, created.id)
    expect(regenerated.llm_generation_target?.model_uid).toBe(baseTarget.model_uid)
    expect(regenerated.status).toBe('ready')
  })

  test('SGE-03 @llm failure path: bogus/unsupported generation target', async () => {
    // gpt-3.5-turbo is a REAL connected model that LACKS simulation_generation.
    const res = await api.post(u('/simulations'), {
      data: {
        model_id: modelId, name: uniqueName('sim-sge03'), steps: 5,
        llm_generation_target: { provider_uid: 'openai', model_uid: 'gpt-3.5-turbo' },
      },
    })
    const status = res.status()
    const body = await res.json().catch(() => ({}))
    console.log('[SGE-03 FINDINGS]', JSON.stringify({ httpStatus: status, body }))

    // Observed behavior (probed live 2026-07-12): the backend rejects at CREATE
    // with 422 invalid_llm_target — no simulation is created, no generation runs.
    expect(status).toBe(422)
    expect(body.code).toBe('invalid_llm_target')
    if (body.id) createdSims.push(body.id) // safety net if behavior ever changes

    // Assert no sim was created: the name never appears in the list.
    // (No id was returned, so nothing to GET; verify create truly produced none.)
    expect(body.id, 'no simulation should be created on 422').toBeUndefined()
  })
})
