import type { APIRequestContext } from '@playwright/test'
import { apiJson, pollUntil, u } from './api'

// All e2e entities carry this prefix; sweepE2EEntities deletes only these.
export const E2E_PREFIX = 'e2e-'

export function uniqueName(slug: string): string {
  return `${E2E_PREFIX}${slug}-${Math.random().toString(36).slice(2, 7)}`
}

// ---------------------------------------------------------------------------
// Fixture model factories. Plain JSON matching the backend model PUT body
// (wrapped as {model: ...} — see seedModel). Shapes proven against the live
// API (mirrors the retired tmp_e2e/seed_model.py payload).
// ---------------------------------------------------------------------------

type Json = Record<string, any>

const colorProp = (hex: string): Json => ({
  uid: 'color',
  name: 'Colour',
  type: 'color',
  description: '',
  default_value: hex,
  immutable: false,
})

const numberProp = (uid: string, def: number, min = 0, max = 100): Json => ({
  uid,
  name: uid,
  type: 'number',
  description: '',
  min_value: min,
  max_value: max,
  default_value: def,
  immutable: false,
})

const baseInit = (count: number, state?: string): Json => ({
  population_count: count,
  count_mode: 'fixed',
  ...(state ? { initial_state: state } : {}),
  placement: { type: 'random' },
})

// F1 minimalSpatial — 1 movable archetype, 4 agents, no rules, strategy none.
export function minimalSpatial(name: string): Json {
  return {
    name,
    description: 'e2e minimal spatial fixture',
    update_mode: 'sequential',
    is_spatial: true,
    spatial_cell_size: 0,
    environment: { uid: 'env', width: 20, height: 20, torus: false, global_properties: [], shapes: [] },
    agents: [
      {
        uid: 'walker',
        name: 'Walker',
        description: '',
        type: 'archetype',
        move_type: 'movable',
        states: ['idle'],
        properties: [colorProp('#7c3aed'), numberProp('energy', 50)],
        initialization: baseInit(4, 'idle'),
        locked: false,
      },
    ],
    rules: [],
    interactions: [],
    events: [],
    widgets: [],
  }
}

// F2 rulesRich — 2 archetypes exercising the rule engine end-to-end:
// movement, property mutation, proximity+interact, events (agent + env
// producers), state transition, spawn/remove, cooldown+priority.
export function rulesRich(name: string): Json {
  return {
    name,
    description: 'e2e rules-rich fixture',
    update_mode: 'sequential',
    is_spatial: true,
    spatial_cell_size: 0,
    environment: {
      uid: 'env',
      width: 30,
      height: 30,
      torus: false,
      global_properties: [numberProp('alarm_count', 0, 0, 100000)],
      shapes: [],
    },
    agents: [
      {
        uid: 'runner',
        name: 'Runner',
        description: '',
        type: 'archetype',
        move_type: 'movable',
        states: ['calm', 'alert'],
        properties: [colorProp('#0ea5e9'), numberProp('ticks', 0, 0, 100000), numberProp('meetings', 0, 0, 100000)],
        initialization: baseInit(4, 'calm'),
        locked: false,
      },
      {
        uid: 'beacon',
        name: 'Beacon',
        description: '',
        type: 'archetype',
        move_type: 'static',
        states: ['on'],
        properties: [colorProp('#f59e0b'), numberProp('pulses', 0, 0, 100000)],
        initialization: baseInit(2, 'on'),
        locked: false,
      },
    ],
    rules: [
      {
        id: 'r_move',
        name: 'random walk',
        description: '',
        priority: 10,
        applies_to: ['runner'],
        conditions: [{ type: 'step', operator: 'gte', value: 0 }],
        actions: [{ type: 'move', speed: 1 }],
        execute_once: false,
        cooldown_steps: 0,
        enabled: true,
        locked: false,
      },
      {
        id: 'r_tick',
        name: 'count ticks each step',
        description: '',
        priority: 9,
        applies_to: ['runner'],
        conditions: [{ type: 'step', operator: 'gte', value: 0 }],
        actions: [{ type: 'modify_property', property_uid: 'ticks', modifier: 'add', value: 1 }],
        execute_once: false,
        cooldown_steps: 0,
        enabled: true,
        locked: false,
      },
      {
        id: 'r_pulse',
        name: 'beacon pulses with cooldown',
        description: '',
        priority: 5,
        applies_to: ['beacon'],
        conditions: [{ type: 'step', operator: 'gte', value: 0 }],
        actions: [
          { type: 'modify_property', property_uid: 'pulses', modifier: 'add', value: 1 },
          { type: 'emit_event', event_uid: 'pulse' },
        ],
        execute_once: false,
        cooldown_steps: 3,
        enabled: true,
        locked: false,
      },
      {
        id: 'r_alert',
        name: 'go alert near beacon',
        description: '',
        priority: 8,
        applies_to: ['runner'],
        conditions: [{ type: 'proximity', target_uid: 'beacon', radius: 6 }],
        actions: [
          { type: 'transition', new_state: 'alert' },
          { type: 'interact', target_uid: 'beacon' },
          { type: 'modify_property', property_uid: 'meetings', modifier: 'add', value: 1 },
        ],
        execute_once: false,
        cooldown_steps: 1,
        enabled: true,
        locked: false,
      },
      {
        id: 'r_spawn',
        name: 'spawn one extra runner at step 3',
        description: '',
        priority: 4,
        applies_to: ['beacon'],
        conditions: [{ type: 'step', operator: 'eq', value: 3 }],
        actions: [{ type: 'spawn_agent', spawn_agent_uid: 'runner', spawn_count: 1, spawn_position: { type: 'nearby', radius: 3 } }],
        execute_once: true,
        cooldown_steps: 0,
        enabled: true,
        locked: false,
      },
      {
        id: 'r_env_alarm',
        name: 'env counts pulses',
        description: '',
        priority: 3,
        applies_to: ['environment'],
        conditions: [{ type: 'event', target_uid: 'pulse' }],
        actions: [{ type: 'modify_property', property_uid: 'alarm_count', modifier: 'add', value: 1 }],
        execute_once: false,
        cooldown_steps: 0,
        enabled: true,
        locked: false,
      },
    ],
    interactions: [
      {
        id: 'i_greet',
        name: 'runner greets runner',
        description: '',
        type: 'agent_agent',
        initiator_uids: ['runner'],
        target_uids: ['runner'],
        radius: 4,
        conditions: [],
        initiator_actions: [{ type: 'modify_property', property_uid: 'meetings', modifier: 'add', value: 1 }],
        target_actions: [],
        bidirectional: false,
        use_llm: false,
        enabled: true,
        locked: false,
      },
    ],
    events: [
      {
        uid: 'pulse',
        name: 'Pulse',
        description: '',
        scope: 'global',
        payload: [],
        producers: ['agent', 'environment'],
        ttl_steps: 2,
        locked: false,
      },
    ],
    widgets: [],
  }
}

// F3 widgetsFull — F1 world + all 7 widget types with valid sources.
export function widgetsFull(name: string): Json {
  const m = rulesRich(name)
  m.description = 'e2e widgets-full fixture'
  // Per backend ValidateWidget matrix: line_chart uses series[] (no single
  // source); text_log uses log_kinds only (a source is rejected).
  m.widgets = [
    { id: 'w_mon', type: 'monitor', title: 'Total agents', source: { type: 'sim_metric', metric: 'total_agents' }, position: { x: 0, y: 0, w: 3, h: 2 } },
    { id: 'w_line', type: 'line_chart', title: 'Avg ticks', series: [{ label: 'ticks', color: '#0ea5e9', source: { type: 'agent_property', property_uid: 'ticks', agent_uids: ['runner'], aggregation: 'avg' } }], position: { x: 3, y: 0, w: 4, h: 3 } },
    { id: 'w_bar', type: 'bar_chart', title: 'Population', source: { type: 'population' }, position: { x: 7, y: 0, w: 4, h: 3 } },
    { id: 'w_pie', type: 'pie_chart', title: 'Runner states', source: { type: 'state_distribution', agent_uids: ['runner'] }, position: { x: 0, y: 3, w: 4, h: 3 } },
    { id: 'w_hist', type: 'histogram', title: 'Ticks histogram', source: { type: 'agent_property', property_uid: 'ticks', agent_uids: ['runner'] }, buckets: 5, position: { x: 4, y: 3, w: 4, h: 3 } },
    { id: 'w_tab', type: 'agent_table', title: 'Runners', source: { type: 'agent_property', agent_uids: ['runner'] }, columns: ['ticks', 'meetings'], position: { x: 8, y: 3, w: 4, h: 3 } },
    { id: 'w_log', type: 'text_log', title: 'Events', log_kinds: ['events', 'state_transitions'], position: { x: 0, y: 6, w: 6, h: 3 } },
  ]
  return m
}

// F4 llmDecision — decision-strategy model (per-test strategy overrides).
export function llmDecision(name: string, exec: { provider_uid: string; model_uid: string }): Json {
  const m = rulesRich(name)
  m.description = 'e2e llm decision fixture'
  m.agents[0].initialization = baseInit(3, 'calm')
  m.agents[1].initialization = baseInit(1, 'on')
  // NOTE: the exec config MUST live under simulation_config — a top-level
  // llm_simulation_execution_config is silently ignored by the backend
  // (probed 2026-07-12: top-level ⇒ 200 + no gating; nested ⇒ gating fires).
  m.simulation_config = {
    update_mode: 'sequential',
    llm_simulation_execution_config: {
      target: exec,
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
  // decision menus via states — decision strategies need decision options
  m.agents[0].states = ['calm', 'alert', 'fleeing']
  return m
}

// F5 nonSpatial — global-property world, one archetype, one global rule.
export function nonSpatial(name: string): Json {
  return {
    name,
    description: 'e2e non-spatial fixture',
    update_mode: 'sequential',
    is_spatial: false,
    spatial_cell_size: 0,
    environment: {
      uid: 'env',
      width: 0,
      height: 0,
      torus: false,
      global_properties: [numberProp('temperature', 10, -100, 1000)],
      shapes: [],
    },
    agents: [
      {
        uid: 'sensor',
        name: 'Sensor',
        description: '',
        type: 'archetype',
        move_type: 'static',
        states: ['watching'],
        properties: [numberProp('readings', 0, 0, 100000)],
        initialization: baseInit(3, 'watching'),
        locked: false,
      },
    ],
    rules: [
      {
        id: 'r_warm',
        name: 'temperature rises',
        description: '',
        priority: 5,
        applies_to: ['environment'],
        conditions: [{ type: 'step', operator: 'gte', value: 0 }],
        actions: [{ type: 'modify_property', property_uid: 'temperature', modifier: 'add', value: 1 }],
        execute_once: false,
        cooldown_steps: 0,
        enabled: true,
        locked: false,
      },
      {
        id: 'r_read',
        name: 'sensors read',
        description: '',
        priority: 4,
        applies_to: ['sensor'],
        conditions: [{ type: 'global_property', property_uid: 'temperature', operator: 'gte', value: 0 }],
        actions: [{ type: 'modify_property', property_uid: 'readings', modifier: 'add', value: 1 }],
        execute_once: false,
        cooldown_steps: 0,
        enabled: true,
        locked: false,
      },
    ],
    interactions: [],
    events: [],
    widgets: [],
  }
}

// F6 collisionTorus — blocking rectangle + torus wraparound world.
export function collisionTorus(name: string): Json {
  const m = minimalSpatial(name)
  m.description = 'e2e collision+torus fixture'
  m.environment.torus = true
  m.environment.collision_mode = 'block'
  m.environment.shapes = [
    {
      uid: 'wall',
      name: 'Wall',
      description: '',
      type: 'rectangle',
      layer: 0,
      visible: true,
      properties: [],
      initialization: { population_count: 1, count_mode: 'fixed' },
      geometry: { position: { x: 10, y: 10 }, width: { value: 6 }, height: { value: 6 }, fill: true },
      locked: false,
    },
  ]
  m.agents[0].footprint = { kind: 'circle', radius: 0.5 }
  m.agents[0].collidable = true
  m.rules = [
    {
      id: 'r_move',
      name: 'random walk',
      description: '',
      priority: 10,
      applies_to: ['walker'],
      conditions: [{ type: 'step', operator: 'gte', value: 0 }],
      actions: [{ type: 'move', speed: 2 }],
      execute_once: false,
      cooldown_steps: 0,
      enabled: true,
      locked: false,
    },
  ]
  return m
}

// ---------------------------------------------------------------------------
// Seeding / lifecycle helpers
// ---------------------------------------------------------------------------

export async function seedModel(api: APIRequestContext, body: Json): Promise<{ id: string; name: string }> {
  const created = await apiJson<{ id: string }>(api, 'post', '/models', {
    name: body.name,
    description: body.description,
    is_spatial: body.is_spatial,
  }, 201)
  await apiJson(api, 'put', `/models/${created.id}`, { model: body }, 200)
  return { id: created.id, name: body.name }
}

export interface SeedSimOpts {
  steps?: number
  seed?: number
  name?: string
  llm_generation_target?: Json | null
  llm_execution_target?: Json | null
  timeoutMs?: number
}

export async function seedReadySim(
  api: APIRequestContext,
  modelId: string,
  opts: SeedSimOpts = {},
): Promise<{ id: string; name: string }> {
  const name = opts.name ?? uniqueName('sim')
  const created = await apiJson<{ id: string }>(api, 'post', '/simulations', {
    model_id: modelId,
    name,
    steps: opts.steps ?? 10,
    ...(opts.seed !== undefined ? { seed: opts.seed } : {}),
    ...(opts.llm_generation_target ? { llm_generation_target: opts.llm_generation_target } : {}),
    ...(opts.llm_execution_target ? { llm_execution_target: opts.llm_execution_target } : {}),
  }, 202)
  await pollUntil(async () => {
    const sim = await apiJson<{ status: string }>(api, 'get', `/simulations/${created.id}`)
    if (sim.status === 'generation_failed') throw new Error(`simulation ${created.id} generation_failed`)
    return sim.status === 'ready'
  }, { timeoutMs: opts.timeoutMs ?? 30_000, label: `sim ${created.id} ready` })
  return { id: created.id, name }
}

const TERMINAL = new Set(['finished', 'stopped', 'failed'])

// Start a run and poll to a terminal status. Default engine speed finishes
// small deterministic runs in seconds — do NOT try to pause/resume for speed
// here (control races the async run start and can strand the sim paused).
export async function runToCompletion(
  api: APIRequestContext,
  simId: string,
  opts: { timeoutMs?: number; expect?: 'finished' | 'stopped' | 'failed'; runBody?: Record<string, unknown> } = {},
): Promise<string> {
  await apiJson(api, 'post', `/simulations/${simId}/run`, opts.runBody ?? {}, 202)
  const status = await pollUntil(async () => {
    const sim = await apiJson<{ status: string }>(api, 'get', `/simulations/${simId}`)
    return TERMINAL.has(sim.status) ? sim.status : null
  }, { timeoutMs: opts.timeoutMs ?? 120_000, intervalMs: 750, label: `sim ${simId} terminal` })
  if (opts.expect && status !== opts.expect) throw new Error(`run ended ${status}, expected ${opts.expect}`)
  return status
}

// ---------------------------------------------------------------------------
// Cleanup
// ---------------------------------------------------------------------------

export async function deleteQuietly(api: APIRequestContext, path: string): Promise<void> {
  await api.delete(u(path)).catch(() => null)
}

// Remove every e2e-prefixed chat/model/simulation (leftovers from crashes).
// List envelopes are per-resource: simulations → `items`, models → `models`,
// chats → `chats` (verified live 2026-07-12 — a generic `items ?? list` reader
// silently sweeps nothing for models/chats).
export async function sweepE2EEntities(api: APIRequestContext): Promise<{ chats: number; models: number; sims: number }> {
  const removed = { chats: 0, models: 0, sims: 0 }

  const sweep = async (resource: string, key: string, nameOf: (x: any) => string, onRemove: () => void) => {
    for (let guard = 0; guard < 50; guard++) {
      const page = await apiJson<any>(api, 'get', `/${resource}?page=1&page_size=100`)
      const rows: any[] = page[key] ?? []
      const mine = rows.filter((x) => (nameOf(x) ?? '').startsWith(E2E_PREFIX))
      if (!mine.length) break
      for (const x of mine) { await deleteQuietly(api, `/${resource}/${x.id}`); onRemove() }
      if (rows.length < 100) break
    }
  }

  // simulations first (they reference models)
  await sweep('simulations', 'items', (s) => s.name, () => removed.sims++)
  await sweep('models', 'models', (m) => m.name, () => removed.models++)
  await sweep('chats', 'chats', (c) => c.title ?? c.name, () => removed.chats++)
  return removed
}
