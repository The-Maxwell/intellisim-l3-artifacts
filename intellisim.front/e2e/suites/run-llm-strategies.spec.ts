import { test, expect, type APIRequestContext, type Page } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import {
  llmDecision,
  seedModel,
  seedReadySim,
  runToCompletion,
  uniqueName,
  deleteQuietly,
} from '../_helpers/fixtures'
import { pickCheapModel, type LlmTargetRef } from '../_helpers/llm'

// §5.10 RLL — Runs with the REAL LLM provider (Level-2 execution strategies).
//
// Titles carry @llm so the `llm` Playwright project selects them (300 s/test).
//
// ---------------------------------------------------------------------------
// COST DISCIPLINE + BUDGET (≤ 7 LLM-driven runs total):
//   Run 1  RLL-01 (+RLL-07 live tokens)  archetype_decision  nano     8 steps  UI-live
//   Run 2  RLL-02 (+RLL-07 reasoning)    individual_decision o4-mini  5 steps  API
//   Run 3  RLL-03                        individual_full     gpt-4.1  6 steps  API
//   Run 4  RLL-04 identical              archetype_full      gpt-4.1  6 steps  API
//   Run 5  RLL-04 distribution           archetype_full      gpt-4.1  6 steps  API
//   Run 6  RLL-05                        hybrid (dec+full)   gpt-4o   6 steps  API
//   Run 7  RLL-06 + RLL-08               decision + override nano/4o  30 steps API (stopped early)
//
// ---------------------------------------------------------------------------
// FINDINGS discovered while wiring this suite (documented, load-bearing):
//
//  * DECISION-MENU REQUIREMENT (run_validation.go ValidateRunnable): a decision
//    strategy with UseForDecisions requires a NON-EMPTY decision menu, and the
//    menu is built by DecisionMenu() from the Condition.Value strings of
//    `decision`-type conditions on ENABLED RULES that apply to the archetype —
//    NOT from the archetype's states. The llmDecision fixture has NO such rule,
//    so a decision run 422s `not_runnable` unless we add rules with
//    `{type:'decision', value:<label>}` conditions. We add three (calm/alert/
//    fleeing → transition) — `decisionMenuRules()`.
//
//  * AGENT.TYPE = STRATEGY SCOPE: Agent.type 'archetype' ⇒ archetype_* strategy
//    (ONE LLM call per archetype group per step); 'agent' ⇒ individual_* (ONE
//    call per instance per step). Both accept population > 1 (no validation
//    constraint). We keep beacon on an explicit `none` per-archetype override so
//    ONLY the runner archetype spends LLM budget (also sidesteps the beacon
//    needing its own decision menu, since the model DefaultStrategy otherwise
//    applies to every archetype).
//
//  * RUN-START OVERRIDE CAPABILITY (run_service.go Start): POST /run
//    {llm_execution_target} is validated against `simulation_execution`, then
//    ValidateRunnable additionally validates it against the strategy's own
//    capability. Overriding a DECISION model therefore needs a BOTH-capable
//    model (gpt-4o) — a pure-execution model (gpt-4.1, lacks simulation_decisions)
//    would pass Start's check but fail ValidateRunnable. The per-run target is
//    recorded on the SIMULATION as `sim.llm_execution_target` (ExecutionRun rows
//    carry no target).
//
//  * CAPABILITY MAP (probed live 2026-07-12):
//      simulation_decisions : gpt-4o, gpt-4o-mini, gpt-4.1-mini, gpt-4.1-nano, o4-mini
//      simulation_execution : gpt-4o, gpt-4.1, o3
//      BOTH (hybrid/override): gpt-4o, o3    (gpt-4.1 does NOT have decisions)
// ---------------------------------------------------------------------------

const STEPS_QS = 'from=0&to=1000000&page=1&page_size=100000'

type Json = Record<string, any>

// ---- strategy-object + model builders ------------------------------------

const initFixed = (count: number, state: string): Json => ({
  population_count: count,
  count_mode: 'fixed',
  initial_state: state,
  placement: { type: 'random' },
})

const EMPTY_TARGET = { provider_uid: '', model_uid: '' }

type StratType = 'none' | 'full' | 'decision' | 'hybrid'

function strat(type: StratType, extra: Json = {}): Json {
  const flags: Record<StratType, [boolean, boolean, boolean, boolean]> = {
    none: [false, false, false, false],
    full: [true, true, true, true],
    decision: [false, false, false, true],
    hybrid: [false, false, false, false], // per-archetype overrides drive it
  }
  const [m, i, p, d] = flags[type]
  return {
    type,
    batch_size: 0,
    use_for_movement: m,
    use_for_interaction: i,
    use_for_properties: p,
    use_for_decisions: d,
    decision_frequency: 1,
    ...extra,
  }
}

// Three `decision`-condition rules give the runner archetype a non-empty
// decision menu (labels = calm/alert/fleeing), each mapping the LLM's chosen
// label to a state transition.
function decisionMenuRules(): Json[] {
  const mk = (id: string, label: string, state: string): Json => ({
    id,
    name: `decide ${label}`,
    description: '',
    priority: 20,
    applies_to: ['runner'],
    conditions: [{ type: 'decision', value: label }],
    actions: [{ type: 'transition', new_state: state }],
    execute_once: false,
    cooldown_steps: 0,
    enabled: true,
    locked: false,
  })
  return [mk('dc_calm', 'calm', 'calm'), mk('dc_alert', 'alert', 'alert'), mk('dc_flee', 'fleeing', 'fleeing')]
}

interface ModelOpts {
  execTarget: LlmTargetRef
  defaultType: StratType
  archetypeMode?: 'identical' | 'distribution'
  variationStrength?: number
  runnerType?: 'archetype' | 'agent'
  runnerPop?: number
  addDecisionMenu?: boolean
  runnerOverride?: Json
  beaconOverride?: Json
  withDecisionWidget?: boolean
}

function llmModel(name: string, o: ModelOpts): Json {
  const m = llmDecision(name, o.execTarget)
  const exec = m.simulation_config.llm_simulation_execution_config
  exec.target = o.execTarget
  exec.default_strategy = strat(o.defaultType, {
    ...(o.archetypeMode ? { archetype_mode: o.archetypeMode } : {}),
    ...(o.variationStrength !== undefined ? { variation_strength: o.variationStrength } : {}),
  })
  // Drop the spawn rule so the world stays capped at its initial population.
  m.rules = m.rules.filter((r: Json) => r.id !== 'r_spawn')

  // Runner scope + population.
  if (o.runnerType) m.agents[0].type = o.runnerType
  m.agents[0].initialization = initFixed(o.runnerPop ?? 3, 'calm')

  if (o.addDecisionMenu) m.rules.push(...decisionMenuRules())

  // Beacon: explicit per-archetype override (default `none`) so only the runner
  // spends LLM budget; a full override lets it participate (RLL-05 hybrid).
  m.agents[1].llm_config = { target: EMPTY_TARGET, strategy: o.beaconOverride ?? strat('none') }
  if (o.runnerOverride) m.agents[0].llm_config = { target: EMPTY_TARGET, strategy: o.runnerOverride }

  if (o.withDecisionWidget) {
    m.widgets = [
      { id: 'w_dec', type: 'text_log', title: 'Decisions', log_kinds: ['llm_decisions'], position: { x: 0, y: 0, w: 6, h: 4 } },
    ]
  }
  return m
}

// ---- shared API helpers ---------------------------------------------------

async function stepItems(api: APIRequestContext, id: string): Promise<Json[]> {
  const page = await apiJson<{ items: Json[] }>(api, 'get', `/simulations/${id}/steps?${STEPS_QS}`)
  return page.items ?? []
}

// Start a run with retry on the engine run-capacity 429 (contention with other
// agents). Never issues a control right after — callers poll `running` first.
async function startRun(api: APIRequestContext, id: string, body: Json = {}): Promise<void> {
  for (let attempt = 0; attempt < 6; attempt++) {
    const res = await api.post(u(`/simulations/${id}/run`), { data: body })
    if (res.status() === 202) return
    if (res.status() === 429) {
      await new Promise((r) => setTimeout(r, 2000))
      continue
    }
    throw new Error(`POST /run → ${res.status()}: ${await res.text().catch(() => '')}`)
  }
  throw new Error(`POST /run kept returning 429 for sim ${id}`)
}

async function pollStatus(api: APIRequestContext, id: string, wanted: string[], timeoutMs = 60_000): Promise<string> {
  return pollUntil(async () => {
    const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${id}`)
    return wanted.includes(s.status) ? s.status : null
  }, { timeoutMs, intervalMs: 500, label: `sim ${id} status ∈ ${wanted.join('|')}` })
}

interface SimShape {
  id: string
  status: string
  status_text?: string
  token_usage?: { total?: { total_tokens?: number; prompt_tokens?: number; completion_tokens?: number; reasoning_tokens?: number } } | null
  llm_execution_target?: LlmTargetRef | null
  execution_runs?: Array<{ status: string; error?: string; steps_completed?: number }>
}

// Poll a run to terminal, surfacing execution_runs[].error as a FINDING on failure.
async function runToTerminal(api: APIRequestContext, id: string, timeoutMs = 240_000): Promise<SimShape> {
  const terminal = new Set(['finished', 'stopped', 'failed'])
  const sim = await pollUntil(async () => {
    const s = await apiJson<SimShape>(api, 'get', `/simulations/${id}`)
    return terminal.has(s.status) ? s : null
  }, { timeoutMs, intervalMs: 1000, label: `sim ${id} terminal` })
  if (sim.status === 'failed') {
    const runs = sim.execution_runs ?? []
    const err = runs.map((r) => r.error).filter(Boolean).join(' | ')
    console.log(`[RLL FINDING] sim ${id} run FAILED: status_text=${sim.status_text} runErrors=${err}`)
  }
  return sim
}

// Extract decision-field coverage across step frames for the runner archetype.
function decisionCoverage(steps: Json[]): { runnerRows: number; withDecision: number; sample: string[]; perStep: number[] } {
  let runnerRows = 0
  let withDecision = 0
  const sample: string[] = []
  const perStep: number[] = []
  for (const s of steps) {
    let stepDecisions = 0
    for (const a of (s.agents as Json[]) ?? []) {
      if (a.agent_uid !== 'runner') continue
      runnerRows++
      if (typeof a.decision === 'string' && a.decision.length > 0) {
        withDecision++
        stepDecisions++
        if (sample.length < 6) sample.push(a.decision)
      }
    }
    perStep.push(stepDecisions)
  }
  return { runnerRows, withDecision, sample, perStep }
}

// ===========================================================================

test.describe('RLL — runs with LLM strategies', () => {
  let api: APIRequestContext
  let nano: LlmTargetRef // simulation_decisions, cheapest
  let full: LlmTargetRef // simulation_execution, cheapest → gpt-4.1
  let both: LlmTargetRef // simulation_execution + simulation_decisions → gpt-4o
  let reasoning: LlmTargetRef // reasoning + simulation_decisions → o4-mini
  const modelIds: string[] = []
  const simIds: string[] = []

  test.beforeAll(async () => {
    api = await apiContext()

    const decisions = await pickCheapModel('simulation_decisions')
    const execution = await pickCheapModel('simulation_execution')
    if (!decisions || !execution) throw new Error('LLM provider not connected (need simulation_decisions + simulation_execution)')
    nano = decisions
    full = execution

    // Resolve BOTH-capable + reasoning models straight from /llms.
    const llms = await apiJson<{ list: Array<{ uid: string; models: Array<{ uid: string; capabilities: string[]; reasoning: boolean }> }> }>(
      api, 'get', '/llms', undefined, 200,
    )
    const flat = llms.list.flatMap((p) => p.models.map((mm) => ({ provider_uid: p.uid, model_uid: mm.uid, caps: mm.capabilities, reasoning: mm.reasoning })))
    const bothCap = flat.filter((c) => c.caps.includes('simulation_execution') && c.caps.includes('simulation_decisions'))
    const cheapBoth = bothCap.find((c) => c.model_uid === 'gpt-4o') ?? bothCap[0]
    if (!cheapBoth) throw new Error('need a model with BOTH simulation_execution + simulation_decisions (hybrid/override)')
    both = { provider_uid: cheapBoth.provider_uid, model_uid: cheapBoth.model_uid }

    const reasoner = flat.find((c) => c.reasoning && c.caps.includes('simulation_decisions'))
    reasoning = reasoner ? { provider_uid: reasoner.provider_uid, model_uid: reasoner.model_uid } : nano

    console.log('[RLL targets]', JSON.stringify({ nano, full, both, reasoning }))
  })

  test.afterAll(async () => {
    for (const id of simIds) {
      await api.post(u(`/simulations/${id}/control`), { data: { action: 'stop' } }).catch(() => null)
    }
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  async function seed(model: Json, steps: number): Promise<{ id: string }> {
    const m = await seedModel(api, model)
    modelIds.push(m.id)
    const full = await apiJson<any>(api, 'get', `/models/${m.id}`, undefined, 200)
    expect(full.validation?.simulation_ready, `model not runnable: ${JSON.stringify(full.validation?.errors)}`).toBe(true)
    const sim = await seedReadySim(api, m.id, { steps, name: uniqueName('sim-rll') })
    simIds.push(sim.id)
    return { id: sim.id }
  }

  // --- RLL-01 + RLL-07 (live tokens) --------------------------------------

  test('RLL-01 @llm archetype_decision run completes; live token stream + decisions in frames/memory/text_log', async ({ page }) => {
    test.setTimeout(280_000)
    const { id } = await seed(
      llmModel(uniqueName('mdl-rll01'), { execTarget: nano, defaultType: 'decision', addDecisionMenu: true, withDecisionWidget: true }),
      8,
    )

    await login(page)
    await page.goto(`/simulations/${id}/run`)
    await page.getByRole('button', { name: 'Start run' }).click()
    await pollStatus(api, id, ['running', 'finished', 'stopped'], 60_000)

    // LIVE token stream (RLL-07): the run-screen readout reaches > 0 while/after
    // the run streams token_usage frames over the WS.
    await expect.poll(async () => {
      const t = await page.getByTestId('run-token-total').textContent().catch(() => '')
      const n = Number((t ?? '').replace(/[^\d]/g, ''))
      return Number.isFinite(n) ? n : 0
    }, { message: 'run-token-total streams > 0 live', timeout: 120_000, intervals: [1000] }).toBeGreaterThan(0)

    // Per-step delta chip appears at least once with a nonzero value (best-effort:
    // a fast nano run can flush the terminal frame before we sample — annotated).
    let sawStepChip = false
    for (let i = 0; i < 40; i++) {
      const chip = page.getByTestId('run-step-tokens')
      if ((await chip.count()) > 0) {
        const txt = (await chip.textContent().catch(() => '')) ?? ''
        if (/[1-9]/.test(txt)) { sawStepChip = true; break }
      }
      const st = await apiJson<{ status: string }>(api, 'get', `/simulations/${id}`)
      if (['finished', 'stopped', 'failed'].includes(st.status)) break
      await page.waitForTimeout(400)
    }
    test.info().annotations.push({ type: 'RLL-07', description: `run-step-tokens nonzero chip observed live = ${sawStepChip}` })

    // Run finishes.
    const sim = await runToTerminal(api, id)
    expect(sim.status).toBe('finished')
    const total = sim.token_usage?.total?.total_tokens ?? 0
    expect(total, 'token usage recorded').toBeGreaterThan(0)

    // Token panel breakdown visible (expand).
    await page.getByTestId('run-token-panel-toggle').click().catch(() => null)
    await expect(page.getByTestId('run-token-panel')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('run-token-panel-total')).not.toHaveText(/^\s*0 tok\s*$/)

    // Frames carry `decision` strings on runner rows. Document coverage.
    const steps = await stepItems(api, id)
    const cov = decisionCoverage(steps)
    console.log('[RLL-01 decision coverage]', JSON.stringify({
      frames: steps.length, runnerRows: cov.runnerRows, withDecision: cov.withDecision,
      perStepDecisions: cov.perStep, sample: cov.sample,
    }))
    expect(cov.withDecision, 'at least one runner frame carries a decision string').toBeGreaterThan(0)

    // Memory taxonomy WITH an LLM strategy: expect decision and/or llm_response
    // entries (absent from a pure rule run). Document the full type set.
    const runnerId = (steps.at(-1)?.agents as Json[]).find((a) => a.agent_uid === 'runner')?.id
    const agent = await apiJson<Json>(api, 'get', `/simulations/${id}/agents/${runnerId}`)
    const memTypes = [...new Set(((agent.memory as Json[]) ?? []).map((mm) => mm.type))].sort()
    console.log('[RLL-01 memory taxonomy]', JSON.stringify({ agentId: runnerId, memTypes }))
    expect(memTypes.some((t) => t === 'decision' || t === 'llm_response'), `LLM memory type present (got ${memTypes})`).toBe(true)

    // text_log llm_decisions widget lists entries in the (now terminal) run
    // screen. Best-effort: locate the "Decisions" widget and look for its rows.
    let widgetHasEntries = false
    try {
      const decWidget = page.locator('[class*="grid"]').filter({ hasText: 'Decisions' }).first()
      await decWidget.scrollIntoViewIfNeeded({ timeout: 5000 }).catch(() => null)
      // Rendered decision rows carry the "decis" kind tag.
      widgetHasEntries = (await page.getByText('decis', { exact: false }).count()) > 0
    } catch { /* non-fatal */ }
    test.info().annotations.push({ type: 'RLL-01', description: `llm_decisions text_log widget shows entries = ${widgetHasEntries}` })
  })

  // --- RLL-02 + RLL-07 (reasoning) ----------------------------------------

  test('RLL-02 @llm individual_decision run completes; per-instance decisions; reasoning tokens (o4-mini)', async ({ page }) => {
    test.setTimeout(280_000)
    // individual scope (type 'agent') with a reasoning model so we can also
    // assert reasoning_tokens end-to-end (RLL-07 part 2).
    const { id } = await seed(
      llmModel(uniqueName('mdl-rll02'), {
        execTarget: reasoning, defaultType: 'decision', runnerType: 'agent', runnerPop: 2, addDecisionMenu: true,
      }),
      5,
    )
    await startRun(api, id)
    const sim = await runToTerminal(api, id)
    expect(sim.status).toBe('finished')
    const total = sim.token_usage?.total?.total_tokens ?? 0
    expect(total, 'tokens > 0').toBeGreaterThan(0)

    // Individual scope ⇒ per-instance decision rows (one decision per runner
    // instance per decision step). Document call volume vs archetype.
    const steps = await stepItems(api, id)
    const cov = decisionCoverage(steps)
    console.log('[RLL-02 individual coverage]', JSON.stringify({
      frames: steps.length, runnerRows: cov.runnerRows, withDecision: cov.withDecision,
      perStepDecisions: cov.perStep, sample: cov.sample,
    }))
    expect(cov.withDecision, 'individual decisions recorded on frames').toBeGreaterThan(0)

    // RLL-07 reasoning: reasoning_tokens > 0 and ≤ completion_tokens.
    const t = sim.token_usage?.total ?? {}
    const reasoningTok = t.reasoning_tokens ?? 0
    const completionTok = t.completion_tokens ?? 0
    console.log('[RLL-07 reasoning]', JSON.stringify({ model: reasoning.model_uid, reasoning: reasoningTok, completion: completionTok, total }))
    if (reasoning.model_uid === 'o4-mini' || reasoning.model_uid.startsWith('o')) {
      expect(reasoningTok, 'reasoning model spends reasoning tokens').toBeGreaterThan(0)
      expect(reasoningTok, 'reasoning ⊆ completion').toBeLessThanOrEqual(completionTok)
    }

    // RLL-07 UI: the terminal run screen's token panel surfaces reasoning.
    await login(page)
    await page.goto(`/simulations/${id}/run`)
    await page.getByTestId('run-token-panel-toggle').click().catch(() => null)
    await expect(page.getByTestId('run-token-panel')).toBeVisible({ timeout: 15_000 })
    if (reasoningTok > 0) {
      await expect(page.getByTestId('token-reasoning-total')).toBeVisible()
      const rtxt = (await page.getByTestId('token-reasoning-total').textContent().catch(() => '')) ?? ''
      expect(Number(rtxt.replace(/[^\d]/g, '')), 'token-reasoning-total > 0 in panel').toBeGreaterThan(0)
    }
  })

  // --- RLL-03 --------------------------------------------------------------

  test('RLL-03 @llm individual_full run completes; positions/properties change; tokens > 0', async () => {
    test.setTimeout(280_000)
    const { id } = await seed(
      llmModel(uniqueName('mdl-rll03'), { execTarget: full, defaultType: 'full', runnerType: 'agent', runnerPop: 2 }),
      6,
    )
    await startRun(api, id)
    const sim = await runToTerminal(api, id)
    expect(sim.status).toBe('finished')
    expect(sim.token_usage?.total?.total_tokens ?? 0, 'tokens > 0').toBeGreaterThan(0)

    const steps = await stepItems(api, id)
    expect(steps.length).toBeGreaterThan(0)
    // Full control drives movement + properties: some runner position OR property
    // differs between the first and last frame.
    const first = steps[0].agents as Json[]
    const last = steps.at(-1)!.agents as Json[]
    const firstById = new Map(first.filter((a) => a.agent_uid === 'runner').map((a) => [a.id, a]))
    let changed = false
    for (const a of last.filter((x) => x.agent_uid === 'runner')) {
      const b = firstById.get(a.id)
      if (!b) continue
      const posMoved = a.position && b.position && (a.position.x !== b.position.x || a.position.y !== b.position.y)
      const propChanged = JSON.stringify(a.properties) !== JSON.stringify(b.properties)
      const stateChanged = a.state !== b.state
      if (posMoved || propChanged || stateChanged) { changed = true; break }
    }
    console.log('[RLL-03]', JSON.stringify({ frames: steps.length, changed }))
    expect(changed, 'individual_full changed at least one runner position/property/state').toBe(true)
  })

  // --- RLL-04 (identical vs distribution) ----------------------------------

  async function archetypeFullRun(slug: string, mode: 'identical' | 'distribution', variation: number) {
    const { id } = await seed(
      llmModel(uniqueName(slug), { execTarget: full, defaultType: 'full', archetypeMode: mode, variationStrength: variation, runnerPop: 3 }),
      6,
    )
    await startRun(api, id)
    const sim = await runToTerminal(api, id)
    expect(sim.status, `${mode} run finished`).toBe('finished')
    expect(sim.token_usage?.total?.total_tokens ?? 0, 'tokens > 0').toBeGreaterThan(0)
    const steps = await stepItems(api, id)
    const last = (steps.at(-1)?.agents as Json[]).filter((a) => a.agent_uid === 'runner')
    const positions = last.map((a) => `${a.position?.x},${a.position?.y}`)
    const distinctPos = new Set(positions).size
    // Collect distinct values of each mutable property across members.
    const propKeys = ['ticks', 'meetings']
    const distinctProps: Record<string, number> = {}
    for (const k of propKeys) distinctProps[k] = new Set(last.map((a) => a.properties?.[k])).size
    return { id, members: last.length, distinctPos, distinctProps, positions }
  }

  test('RLL-04 @llm archetype_full ArchetypeMode identical vs distribution', async () => {
    test.setTimeout(290_000)
    const ident = await archetypeFullRun('mdl-rll04a', 'identical', 0)
    const dist = await archetypeFullRun('mdl-rll04b', 'distribution', 0.4)
    console.log('[RLL-04 identical]', JSON.stringify(ident))
    console.log('[RLL-04 distribution]', JSON.stringify(dist))

    // Loose assertion per plan: distribution ends with ≥ 2 distinct member states
    // (positions or property values). Members start at random positions, so this
    // is nearly always satisfiable — the real value is the documented comparison.
    const distDistinct = dist.distinctPos >= 2 || Object.values(dist.distinctProps).some((n) => n >= 2)
    expect(distDistinct, 'distribution members are not all uniform').toBe(true)

    test.info().annotations.push({
      type: 'RLL-04',
      description: `identical: distinctPos=${ident.distinctPos}/${ident.members} distinctProps=${JSON.stringify(ident.distinctProps)} | ` +
        `distribution: distinctPos=${dist.distinctPos}/${dist.members} distinctProps=${JSON.stringify(dist.distinctProps)}. ` +
        `Note: random initial placement means absolute positions differ even under 'identical' (uniform DIRECTIVE, not uniform world); ` +
        `heading jitter is scaled by variation_strength via MovementIntent.Resolved.`,
    })
  })

  // --- RLL-05 --------------------------------------------------------------

  test('RLL-05 @llm hybrid (runner decision + beacon full) completes; tokens > 0', async () => {
    test.setTimeout(280_000)
    // Model DefaultStrategy = hybrid ⇒ each archetype runs its OWN override:
    // runner = decision (needs menu + simulation_decisions), beacon = full (needs
    // simulation_execution). The single BOTH-capable target (gpt-4o) satisfies
    // both. A hybrid model with NO per-archetype override degrades every
    // archetype to `none` (resolveArchetypeStrategy), hence the explicit overrides.
    const { id } = await seed(
      llmModel(uniqueName('mdl-rll05'), {
        execTarget: both,
        defaultType: 'hybrid',
        addDecisionMenu: true,
        runnerOverride: strat('decision'),
        beaconOverride: strat('full'),
      }),
      6,
    )
    await startRun(api, id)
    const sim = await runToTerminal(api, id)
    expect(sim.status).toBe('finished')
    expect(sim.token_usage?.total?.total_tokens ?? 0, 'tokens > 0').toBeGreaterThan(0)

    const steps = await stepItems(api, id)
    const cov = decisionCoverage(steps)
    console.log('[RLL-05 hybrid]', JSON.stringify({ frames: steps.length, runnerDecisions: cov.withDecision, sample: cov.sample }))
    expect(cov.withDecision, 'hybrid runner (decision half) recorded decisions').toBeGreaterThan(0)
  })

  // --- RLL-06 + RLL-08 -----------------------------------------------------

  test('RLL-06 + RLL-08 @llm run-start exec-target override honored; pause/resume/stop mid-run is safe', async () => {
    test.setTimeout(200_000)
    // Decision model whose model-level exec target is nano; override at run start
    // to the BOTH-capable model (gpt-4o) — required because the override is
    // validated against simulation_execution AND against the decision strategy's
    // simulation_decisions in ValidateRunnable.
    const { id } = await seed(
      llmModel(uniqueName('mdl-rll06'), { execTarget: nano, defaultType: 'decision', addDecisionMenu: true }),
      30,
    )
    await startRun(api, id, { llm_execution_target: both })
    await pollStatus(api, id, ['running'], 60_000)

    // RLL-06: the per-run override is recorded on the simulation.
    const afterStart = await apiJson<SimShape>(api, 'get', `/simulations/${id}`)
    console.log('[RLL-06]', JSON.stringify({ recorded: afterStart.llm_execution_target, override: both }))
    expect(afterStart.llm_execution_target?.model_uid, 'run-start override recorded on sim.llm_execution_target').toBe(both.model_uid)
    expect(afterStart.llm_execution_target?.provider_uid).toBe(both.provider_uid)

    // RLL-08: pause → resume → stop, each a clean 200, ending terminal `stopped`
    // with partial steps (< 30) recorded and no stuck state.
    const control = async (action: string) => {
      const res = await api.post(u(`/simulations/${id}/control`), { data: { action } })
      expect(res.status(), `control ${action} → ${res.status()}`).toBe(200)
    }
    await control('pause')
    await pollStatus(api, id, ['paused'], 30_000)
    await control('resume')
    await pollStatus(api, id, ['running'], 30_000)
    await control('stop')
    const terminal = await pollStatus(api, id, ['stopped', 'finished', 'failed'], 60_000)
    expect(terminal, 'stop terminates as stopped').toBe('stopped')

    const steps = await stepItems(api, id)
    console.log('[RLL-08]', JSON.stringify({ terminal, stepsRecorded: steps.length }))
    expect(steps.length, 'partial steps recorded').toBeGreaterThan(0)
    expect(steps.length, 'stopped before completing all 30 steps').toBeLessThan(30)
  })
})
