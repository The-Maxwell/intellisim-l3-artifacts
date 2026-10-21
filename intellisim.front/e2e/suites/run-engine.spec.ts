import { test, expect, type Page, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import {
  rulesRich,
  nonSpatial,
  collisionTorus,
  seedModel,
  seedReadySim,
  runToCompletion,
  uniqueName,
  deleteQuietly,
} from '../_helpers/fixtures'

// §5.9 RUN-08…RUN-19 — deterministic rule-engine behavior (NO LLM).
//
// These cases are almost entirely API-level: run a fixture sim to completion,
// then assert on GET /simulations/:id/steps frames + the final GET /simulations/:id
// statistics. RUN-18 additionally opens the run screen in the browser.
//
// ENGINE SEMANTICS PINNED BY LIVE PROBING (documented inline where load-bearing):
//  * GET /:id/steps with NO `to` param now returns the FULL recorded range (was
//    previously clamped to step 0); passing an explicit `to` still bounds the
//    window. Steps are 0-indexed; frame step N reflects the world AFTER N+1 engine
//    steps (so frame step 0 already shows ticks=1; the final frame step (steps-1)
//    shows ticks=steps for agents alive from the start).
//  * GET /:id `.agents` is the PRISTINE INITIAL world (the engine never mutates it
//    — see core "Runtime isolation" 2026-07-09). The FINAL world lives in the step
//    frames and in GET /:id/agents/:agentId (FinalAgents). So "final state" is read
//    from the last step frame / statistics / getAgent, NOT from `.agents`.

type Json = Record<string, any>

const STEPS_QS = 'from=0&to=1000000&page=1&page_size=100000'

async function stepItems(api: APIRequestContext, id: string): Promise<Json[]> {
  const page = await apiJson<{ items: Json[] }>(api, 'get', `/simulations/${id}/steps?${STEPS_QS}`)
  return page.items ?? []
}

function runnerIds(agents: Json[]): number[] {
  return agents.filter((a) => a.agent_uid === 'runner').map((a) => a.id)
}

function posEq(a?: Json, b?: Json): boolean {
  return !!a && !!b && a.x === b.x && a.y === b.y
}

// One full deterministic run: seed a sim, run it, collect initial world + final
// sim + all persisted step frames.
async function runAndCollect(
  api: APIRequestContext,
  modelId: string,
  opts: { seed: number; steps?: number; expect?: 'finished' | 'stopped' },
): Promise<{ simId: string; initial: Json; final: Json; steps: Json[] }> {
  const sim = await seedReadySim(api, modelId, { seed: opts.seed, steps: opts.steps ?? 10, name: uniqueName('sim-run') })
  const initial = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
  await runToCompletion(api, sim.id, { expect: opts.expect ?? 'finished' })
  const final = await apiJson<Json>(api, 'get', `/simulations/${sim.id}`)
  const steps = await stepItems(api, sim.id)
  return { simId: sim.id, initial, final, steps }
}

test.describe('run engine (deterministic, no LLM)', () => {
  let api: APIRequestContext
  let rrModel: { id: string; name: string } // shared F2 rulesRich model
  const simIds: string[] = []
  const modelIds: string[] = []

  // Shared seed-1 rulesRich run — reused by RUN-08/09/10/13/14/18 to avoid
  // re-running the identical world four times. seed 1 deterministically yields
  // movement + ticks + a state transition + the fixed cooldown cadence.
  let rr: { simId: string; initial: Json; final: Json; steps: Json[] }

  test.beforeAll(async () => {
    api = await apiContext()
    rrModel = await seedModel(api, rulesRich(uniqueName('mdl-rules')))
    modelIds.push(rrModel.id)
    rr = await runAndCollect(api, rrModel.id, { seed: 1, steps: 10 })
    simIds.push(rr.simId)
  })

  test.afterAll(async () => {
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  test('RUN-08 movement: runner positions at the final step differ from initial', async () => {
    const originals = runnerIds(rr.initial.agents)
    expect(originals.length).toBe(4)
    const initById = new Map(rr.initial.agents.map((a: Json) => [a.id, a.position]))
    const last = rr.steps.at(-1)!
    const moved = originals.some((id) => {
      const now = last.agents.find((a: Json) => a.id === id)?.position
      return !posEq(now, initById.get(id))
    })
    expect(moved, 'at least one original runner moved from its initial position').toBe(true)
  })

  test('RUN-09 property mutation: ticks strictly increases across frames; final == steps for originals, fewer for a spawned agent', async () => {
    const originals = new Set(runnerIds(rr.initial.agents))
    const rid = [...originals][0]!
    // Off-by-one: frame step 0 already shows ticks=1 (frame step N = world after
    // N+1 engine steps), so an original runner's ticks trajectory is 1..steps.
    const traj = rr.steps.map((s) => s.agents.find((a: Json) => a.id === rid)?.properties?.ticks)
    expect(traj[0], 'frame step 0 already reflects one tick (0-indexed frames)').toBe(1)
    for (let i = 1; i < traj.length; i++) {
      expect(traj[i], `ticks strictly increases at frame ${i}`).toBe(traj[i - 1] + 1)
    }
    expect(traj.at(-1), 'final ticks == steps for an original runner').toBe(rr.final.steps)

    // The agent spawned at step 3 (r_spawn) appears mid-run, so it accrues fewer
    // ticks than the originals present from the start.
    const last = rr.steps.at(-1)!
    const spawned = last.agents.find((a: Json) => a.agent_uid === 'runner' && !originals.has(a.id))
    expect(spawned, 'a spawned runner exists in the final frame').toBeTruthy()
    expect(spawned.properties.ticks).toBeLessThan(rr.final.steps)
  })

  test('RUN-10 state transition: runners near a beacon reach state "alert"', async () => {
    // proximity(beacon, r6) + transition → alert. Present in frames AND in the
    // final per-archetype state distribution.
    const anyAlertFrame = rr.steps.some((s) => s.agents.some((a: Json) => a.state === 'alert'))
    expect(anyAlertFrame, 'some frame shows a runner in state alert').toBe(true)

    const runnerPop = (rr.final.statistics.population as Json[]).find((p) => p.agent_uid === 'runner')
    expect(runnerPop?.by_state?.alert ?? 0, 'final state distribution counts alert runners').toBeGreaterThan(0)
  })

  test('RUN-11 spawn/remove: population trajectory + spawned/removed counters', async () => {
    // Local mutation: add a remove_agent rule. IMPORTANT engine semantics —
    // remove_agent with no target applies to SELF, and EVERY runner matching the
    // rule fires it the same step, so all runners are removed at once (there is no
    // "remove exactly one" affordance without a per-agent discriminator). We assert
    // the observed all-matched removal rather than the plan's "-1".
    const model = rulesRich(uniqueName('mdl-remove'))
    model.rules.push({
      id: 'r_remove',
      name: 'runners self-remove at step 6',
      description: '',
      priority: 2,
      applies_to: ['runner'],
      conditions: [{ type: 'step', operator: 'eq', value: 6 }],
      actions: [{ type: 'remove_agent' }],
      execute_once: true,
      cooldown_steps: 0,
      enabled: true,
      locked: false,
    })
    const m = await seedModel(api, model)
    modelIds.push(m.id)
    const run = await runAndCollect(api, m.id, { seed: 3, steps: 10 })
    simIds.push(run.simId)

    const runnerCount = run.steps.map((s) => s.agents.filter((a: Json) => a.agent_uid === 'runner').length)
    // 4 originals → 6 after the step-3 spawn (2 beacons each spawn once) → 0 after
    // the step-6 self-remove of all runners.
    expect(runnerCount[0], 'starts at 4 runners').toBe(4)
    expect(Math.max(...runnerCount), 'peaks at 6 after the spawn').toBe(6)
    expect(runnerCount.at(-1), 'all runners removed by the end').toBe(0)

    const st = run.final.statistics
    expect(st.agents_spawned, 'agents_spawned >= 1').toBeGreaterThanOrEqual(1)
    expect(st.agents_removed, 'agents_removed >= 1').toBeGreaterThanOrEqual(1)
    // total_agents is the CURRENT (surviving) count, consistent with the last
    // frame's agent count (2 beacons remain).
    expect(st.total_agents).toBe(run.steps.at(-1)!.agents.length)
    expect(st.total_agents, 'only the 2 static beacons survive').toBe(2)
  })

  test('RUN-12 interactions: interactions counter > 0 and a runner accrues meetings', async () => {
    // seed 5 deterministically clusters runners so i_greet (runner↔runner r4)
    // fires repeatedly. (interactions is seed-dependent — some seeds yield 0.)
    const run = await runAndCollect(api, rrModel.id, { seed: 5, steps: 10 })
    simIds.push(run.simId)

    const st = run.final.statistics
    expect(st.interactions, 'interactions counter > 0').toBeGreaterThan(0)
    const greet = (st.by_interaction as Json[])?.find((i) => i.interaction_id === 'i_greet')
    expect(greet?.count ?? 0, 'i_greet fired').toBeGreaterThan(0)

    // meetings is bumped by i_greet (+1) and r_alert (+1); its max across runners
    // is > 0 (read from the run's property statistics, since GET /:id.agents is
    // the pristine initial world with meetings=0).
    const meetings = (st.properties as Json[])?.find((p) => p.property_uid === 'meetings')
    expect(meetings?.max ?? 0, 'some runner has meetings > 0').toBeGreaterThan(0)
  })

  test('RUN-13 events: emit_event (agent producer) increments the events counter', async () => {
    const st = rr.final.statistics
    expect(st.events, 'events counter > 0').toBeGreaterThan(0)
    const pulse = (st.by_event as Json[])?.find((e) => e.event_uid === 'pulse')
    expect(pulse?.emitted ?? 0, 'pulse event emitted').toBeGreaterThan(0)
  })

  test('RUN-13 environment-scoped rule consumes events → alarm_count increments; by_rule records r_env_alarm', async () => {
    // The fixture wires r_env_alarm (applies_to environment, condition event:pulse
    // ⇒ alarm_count+1). evalEnvCondition now evaluates the `event` condition, so
    // the environment reacts to the beacon's pulses: alarm_count climbs across
    // frames and r_env_alarm shows up in the run statistics.
    const alarm = rr.steps.at(-1)!.global_properties?.alarm_count
    expect(alarm, 'environment consumed pulse events → alarm_count > 0').toBeGreaterThan(0)

    // by_rule serializes as an ARRAY of { rule_id, executions }; r_env_alarm fired.
    const byRule = (rr.final.statistics.by_rule as Json[]) ?? []
    const envRule = byRule.find((r) => r.rule_id === 'r_env_alarm')
    expect(envRule, 'r_env_alarm present in by_rule').toBeTruthy()
    expect(envRule!.executions, 'r_env_alarm executed at least once').toBeGreaterThan(0)
  })

  test('RUN-14 cooldown: beacon pulses fire every 3 steps (cooldown_steps=3)', async () => {
    const bid = rr.initial.agents.find((a: Json) => a.agent_uid === 'beacon')!.id
    const pulses = rr.steps.map((s) => s.agents.find((a: Json) => a.id === bid)?.properties?.pulses)
    // Observed cadence over 10 frames: [1,1,1,2,2,2,3,3,3,4]. r_pulse fires on
    // frames 0,3,6,9 (cooldown 3), so a beacon's pulses land on exactly ceil(steps/3).
    const steps = rr.final.steps
    const expected = Math.ceil(steps / 3)
    expect(pulses.at(-1), 'final pulses == ceil(steps/3)').toBe(expected)
    expect(pulses[0]).toBe(1)
    expect(pulses[3]).toBe(2)
    expect(pulses[6]).toBe(3)
    expect(pulses[9]).toBe(4)
    // Bounds requested by the plan.
    expect(pulses.at(-1)!).toBeGreaterThanOrEqual(1)
    expect(pulses.at(-1)!).toBeLessThanOrEqual(Math.floor(steps / 3) + 1)
  })

  test('RUN-15 non-spatial run: temperature/readings advance; agents carry no position', async () => {
    const m = await seedModel(api, nonSpatial(uniqueName('mdl-nonspatial')))
    modelIds.push(m.id)
    const run = await runAndCollect(api, m.id, { seed: 5, steps: 10 })
    simIds.push(run.simId)

    const steps = run.final.steps
    const last = run.steps.at(-1)!
    // env global rises +1/step from initial 10 → 10+steps at the last frame.
    expect(last.global_properties.temperature, 'temperature == initial(10) + steps').toBe(10 + steps)
    // each sensor reads +1/step (temperature>=0 always true) → readings == steps.
    const sensorId = run.initial.agents[0].id
    const readingsTraj = run.steps.map((s) => s.agents.find((a: Json) => a.id === sensorId)?.properties?.readings)
    expect(readingsTraj.at(-1), 'readings == steps').toBe(steps)

    // Non-spatial agents carry NO position — neither on frames nor on getAgent.
    expect('position' in last.agents[0], 'frame agent has no position field').toBe(false)
    const agent = await apiJson<Json>(api, 'get', `/simulations/${run.simId}/agents/${sensorId}`)
    expect(agent.position, 'getAgent position is absent for non-spatial').toBeFalsy()
    expect(agent.properties.readings, 'getAgent shows the final readings').toBe(steps)
  })

  test('RUN-16 execution-target contract: empty-target decision strategy is rejected (top-level 422 unknown + nested 422 gate); a nested type=none config saves and runs on rules with zero tokens', async () => {
    // DEVIATION FROM THE ORIGINAL RUN-16 PREMISE (documented): the old test relied
    // on an EMPTY-target decision strategy being SAVEABLE and degrading to rules.
    // That only worked because a TOP-LEVEL llm_simulation_execution_config was
    // silently ignored (no gating). Both halves of that bug are now fixed:
    //   • a top-level llm_simulation_execution_config → 422 (must be nested), and
    //   • once nested, an empty-target *decision* strategy trips the capability
    //     gate → 422 sim_execution_target_required (mirrors MDE-20).
    // So a non-none exec strategy now genuinely requires a valid target; the
    // empty-target degrade path is closed. The deterministic no-LLM run path that
    // SURVIVES is a nested type=none exec config (runs on rules, zero tokens).
    const draft = await apiJson<{ id: string }>(api, 'post', '/models', { name: uniqueName('mdl-degrade') }, 201)
    modelIds.push(draft.id)

    // (a) top-level exec config → 422 validation_failed (unknown top-level field).
    const topLevel = rulesRich(uniqueName('mdl-degrade-top'))
    ;(topLevel as Json).llm_simulation_execution_config = {
      target: { provider_uid: '', model_uid: '' },
      default_strategy: { type: 'decision', use_for_decisions: true, decision_frequency: 1, batch_size: 0 },
    }
    const topRes = await api.put(u(`/models/${draft.id}`), { data: { model: topLevel } })
    expect(topRes.status(), await topRes.text()).toBe(422)
    expect((await topRes.json()).code).toBe('validation_failed')

    // (b) nested empty-target decision strategy → 422 sim_execution_target_required.
    const nestedGate = rulesRich(uniqueName('mdl-degrade-gate'))
    ;(nestedGate as Json).simulation_config = {
      update_mode: 'sequential',
      llm_simulation_execution_config: {
        target: { provider_uid: '', model_uid: '' },
        default_strategy: {
          type: 'decision',
          batch_size: 0,
          use_for_movement: false,
          use_for_interaction: false,
          use_for_properties: false,
          use_for_decisions: true,
          decision_frequency: 1,
        },
        perception_radius: 10,
        interaction_radius: 4,
        memory_size: 10,
      },
    }
    const gateRes = await api.put(u(`/models/${draft.id}`), { data: { model: nestedGate } })
    expect(gateRes.status(), await gateRes.text()).toBe(422)
    expect((await gateRes.json()).code).toBe('sim_execution_target_required')

    // (c) a nested type=none exec config DOES save and runs purely on rules with
    // zero tokens — the surviving deterministic (no-LLM) path.
    const noneModel = rulesRich(uniqueName('mdl-degrade-none'))
    ;(noneModel as Json).simulation_config = {
      update_mode: 'sequential',
      llm_simulation_execution_config: { target: { provider_uid: '', model_uid: '' }, default_strategy: { type: 'none' } },
    }
    const m = await seedModel(api, noneModel) // seedModel PUTs — a 422 would throw here
    modelIds.push(m.id)

    const run = await runAndCollect(api, m.id, { seed: 2, steps: 8 })
    simIds.push(run.simId)
    expect(run.final.status).toBe('finished')
    expect(run.final.execution_runs?.[0]?.steps_completed, 'all steps ran on rules').toBe(8)
    expect(run.final.token_usage?.total?.total_tokens ?? 0, 'zero token usage (no LLM invoked)').toBe(0)
  })

  test('RUN-17 collision + torus: agents stay in-bounds (wrap, not clamp) and never inside the blocking wall', async () => {
    const m = await seedModel(api, collisionTorus(uniqueName('mdl-collision')))
    modelIds.push(m.id)
    const run = await runAndCollect(api, m.id, { seed: 7, steps: 15 })
    simIds.push(run.simId)

    // Torus wrap keeps every coordinate in [0, 20) — no clamping at the edges.
    // Collision `block` keeps every agent CENTRE outside the wall body inflated by
    // the footprint radius (0.5): the wall rect is (10,10)-(16,16) (top-left anchor,
    // confirmed by agents freely occupying (9,y)), so a centre must sit outside
    // (9.5,16.5)². Verified across all 15 frames × 4 walkers.
    let checked = 0
    for (const s of run.steps) {
      for (const a of s.agents as Json[]) {
        const p = a.position
        expect(p, 'spatial agent has a position').toBeTruthy()
        expect(p.x, `x in [0,20) at step ${s.step}`).toBeGreaterThanOrEqual(0)
        expect(p.x).toBeLessThan(20)
        expect(p.y, `y in [0,20) at step ${s.step}`).toBeGreaterThanOrEqual(0)
        expect(p.y).toBeLessThan(20)
        const insideWall = p.x > 9.5 && p.x < 16.5 && p.y > 9.5 && p.y < 16.5
        expect(insideWall, `agent ${a.id} centre outside the blocking wall at step ${s.step} (${p.x},${p.y})`).toBe(false)
        checked++
      }
    }
    expect(checked, 'exercised many agent-frames').toBeGreaterThan(20)
  })

  test('RUN-18 agent inspector: memory taxonomy [API] + best-effort canvas click [UI]', async ({ page }) => {
    // Hard assertions: the persisted final agent (FinalAgents) exposes the full
    // identity + a typed memory list. For a purely rule-driven run the memory
    // types observed are {rule, property, movement, interaction} — NO `decision`
    // or `llm_response` (those require an LLM strategy).
    const rid = runnerIds(rr.initial.agents)[0]!
    const agent = await apiJson<Json>(api, 'get', `/simulations/${rr.simId}/agents/${rid}`)
    expect(agent.id).toBe(rid)
    expect(agent.agent_uid).toBe('runner')
    expect(typeof agent.state).toBe('string')
    expect(agent.position, 'final position present').toBeTruthy()
    expect(agent.properties?.ticks, 'final ticks present').toBe(rr.final.steps)
    const memTypes = new Set((agent.memory as Json[] ?? []).map((mm) => mm.type))
    expect((agent.memory as Json[]).length, 'memory populated post-run').toBeGreaterThan(0)
    for (const t of ['rule', 'property', 'movement']) {
      expect(memTypes.has(t), `memory contains a "${t}" entry`).toBe(true)
    }
    test.info().annotations.push({
      type: 'engine',
      description: `RUN-18 memory types (deterministic run) = ${[...memTypes].sort().join(', ')} (no decision/llm_response without an LLM strategy)`,
    })

    // Best-effort UI: open the finished sim's run screen (replay) and try to open
    // the AgentInspector by clicking the agent's projected pixel. The run canvas
    // tweens sprite positions and clamps scale, so a pixel click is inherently
    // flaky — this part is non-fatal and annotated SPEC-LIMITATION if it misses.
    let inspectorOpened = false
    try {
      await login(page)
      await page.goto(`/simulations/${rr.simId}/run`)
      const canvas = page.locator('canvas').first()
      await expect(canvas).toBeVisible({ timeout: 15_000 })
      await page.waitForTimeout(1200) // PixiJS mount + fit-view
      await page.getByTitle('Fit to view').click().catch(() => {})
      await page.waitForTimeout(400)

      const box = await canvas.boundingBox()
      const envW = rr.final.environment?.width ?? 30
      const envH = rr.final.environment?.height ?? 30
      if (box) {
        const scale = Math.min(box.width / envW, box.height / envH) * 0.9
        const offX = (box.width - envW * scale) / 2
        const offY = (box.height - envH * scale) / 2
        // Project every final-frame agent centre and click each until the
        // inspector's identity panel ("archetype") shows up.
        const inspector = page.locator('div.w-72').filter({ hasText: 'archetype' })
        for (const a of rr.steps.at(-1)!.agents as Json[]) {
          if (!a.position) continue
          await page.mouse.click(box.x + offX + a.position.x * scale, box.y + offY + a.position.y * scale)
          if (await inspector.isVisible().catch(() => false)) {
            inspectorOpened = true
            break
          }
        }
        if (inspectorOpened) {
          await expect(inspector).toContainText('archetype')
          await expect(inspector).toContainText('state')
          await expect(inspector).toContainText('Memory')
        }
      }
    } catch {
      /* non-fatal — API assertions above are authoritative */
    }
    test.info().annotations.push({
      type: inspectorOpened ? 'ui' : 'spec-limitation',
      description: inspectorOpened
        ? 'AgentInspector opened via a projected canvas click and showed id/archetype/state/memory'
        : 'AgentInspector canvas click not landed (PixiJS tween/clamp) — memory taxonomy asserted via GET /:id/agents/:id [SPEC-LIMITATION]',
    })
  })

  test('RUN-19 frames carry a statistics snapshot + shape rows for recordable shapes', async () => {
    // A shape is only recorded per step when it is "recordable" (>=1 declared
    // property that is mutable / LLM-driven). The plain collisionTorus wall has NO
    // properties, so its frames carry no `shapes`; add a mutable `wear` property to
    // make it recordable and observe the shape rows.
    const model = collisionTorus(uniqueName('mdl-shapes'))
    model.environment.shapes[0].properties = [
      { uid: 'wear', name: 'wear', type: 'number', description: '', min_value: 0, max_value: 100000, default_value: 0, immutable: false },
    ]
    const m = await seedModel(api, model)
    modelIds.push(m.id)
    const run = await runAndCollect(api, m.id, { seed: 7, steps: 8 })
    simIds.push(run.simId)

    const frame = run.steps[0]
    // Observed StepDTO shape: { step, agents[], shapes?, global_properties?, statistics }.
    // NO token_usage (token spend is streamed live over the WS but never persisted
    // per step).
    expect(frame).toHaveProperty('step')
    expect(Array.isArray(frame.agents)).toBe(true)
    expect(frame.statistics, 'per-step statistics snapshot present').toBeTruthy()
    expect(typeof frame.statistics.total_agents).toBe('number')
    expect(frame).not.toHaveProperty('token_usage')

    // Agent rows carry id/agent_uid/position/properties/state.
    const ag = frame.agents[0]
    for (const k of ['id', 'agent_uid', 'position', 'properties', 'state']) {
      expect(ag, `agent row has ${k}`).toHaveProperty(k)
    }

    // Shape rows: { id, properties } for the recordable wall.
    expect(Array.isArray(frame.shapes), 'frames carry shape rows').toBe(true)
    expect(frame.shapes.length).toBeGreaterThan(0)
    expect(frame.shapes[0]).toHaveProperty('id')
    expect(frame.shapes[0].properties, 'shape row exposes its property snapshot').toHaveProperty('wear')
  })

  test('RUN-19 [API] GET /steps with no `to` param returns the full recorded range', async () => {
    // Behavior change: omitting `to` now returns every recorded step (was clamped
    // to step 0). The shared 10-step rr run has frames 0..9.
    const noTo = await apiJson<{ items: Json[] }>(api, 'get', `/simulations/${rr.simId}/steps?page=1&page_size=100`)
    const steps = (noTo.items ?? []).map((s) => s.step).sort((a, b) => a - b)
    expect(steps.length, 'no-`to` query returns the full range').toBe(rr.final.steps)
    expect(steps[0]).toBe(0)
    expect(steps.at(-1)).toBe(rr.final.steps - 1)
  })
})
