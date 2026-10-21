// Mirror of backend models-module DTO validation
// (../intellisim.core/modules/models/{dtos,responses}.go) — single source of
// truth for UI-side limits + enum value arrays. Backend remains the authority.
import type {
  Model,
  LLMStrategyDTO,
  AgentFootprintDTO,
  DisplayDTO,
  ConditionDTO,
  ActionDTO,
  ShapeDTO,
  ScalarSpecDTO,
  AgentInitializationDTO,
  AgentPropertyDTO,
} from '@/shared/types/model'

// --- field-shape limits ---
export const MODEL_NAME_MAX = 120
export const MODEL_DESCRIPTION_MAX = 2000
export const ENTITY_NAME_MAX = 200
export const UID_MAX = 100
export const MESSAGE_CONTENT_MAX = 10000
/** Show the character counter once the chat input reaches this length. */
export const MESSAGE_COUNTER_THRESHOLD = 9000
export const IMAGE_PROMPT_MAX = 4000
/** Max images a model may hold in its environment-generation library (mirrors MODELS_MAX_MODEL_IMAGES). */
export const MAX_MODEL_IMAGES = 5
export const MODEL_IMAGE_NAME_MAX = 200
export const MODEL_IMAGE_DESCRIPTION_MAX = 2000
/** Per-image upload ceiling (raw bytes, pre-base64); the backend compresses on add. */
export const MAX_IMAGE_BYTES = 6 * 1024 * 1024
/**
 * Soft client-side ceiling for a display sprite upload (raw file bytes). The
 * backend is authoritative and compresses large images on save; we only warn.
 */
export const MAX_DISPLAY_IMAGE_BYTES = 2 * 1024 * 1024

// --- enum value arrays (match the oneof= validate tags) ---
export const MODEL_UPDATE_MODES = ['sequential', 'synchronous'] as const
export const AGENT_TYPES = ['agent', 'archetype'] as const
export const MOVE_TYPES = ['static', 'movable'] as const
export const PROPERTY_TYPES = ['string', 'number', 'enum', 'boolean', 'color'] as const
export const PLACEMENT_TYPES = ['random', 'uniform', 'clustered', 'at_shape', 'manual', 'llm'] as const
export const POPULATION_COUNT_MODES = ['fixed', 'random_range', 'llm'] as const
export const SHAPE_TYPES = ['rectangle', 'circle', 'line', 'polygon'] as const
export const CONDITION_TYPES = [
  'global_property',
  'property',
  'position',
  'proximity',
  'step',
  'composite',
  'count',
  'barrier',
  'state',
  'distance',
  'decision',
  'random',
  'memory',
  'event',
  'custom',
] as const
export const CONDITION_OPERATORS = [
  'eq',
  'neq',
  'gt',
  'lt',
  'gte',
  'lte',
  'contains',
  'in_range',
  'in',
  'not_in',
] as const
export const LOGIC_OPERATORS = ['AND', 'OR', 'NOT'] as const
export const ACTION_TYPES = [
  'set_property',
  'modify_property',
  'move',
  'move_toward',
  'move_away',
  'interact',
  'emit_event',
  'transition',
  'call_llm',
  'spawn_agent',
  'remove_agent',
  'transform',
  'teleport',
  'custom',
] as const
export const ACTION_MODIFIERS = [
  'set',
  'add',
  'subtract',
  'multiply',
  'divide',
  'increment',
  'decrement',
  'toggle',
] as const
export const SPAWN_POSITION_TYPES = ['same', 'random', 'offset', 'nearby', 'at_shape'] as const
export const INTERACTION_TYPES = ['agent_agent', 'agent_shape', 'agent_global'] as const
export const EVENT_SCOPES = ['global', 'local', 'targeted'] as const
export const EVENT_PRODUCERS = ['agent', 'environment', 'llm'] as const
// Agent memory taxonomy (mirrors data/simulations AgentMemoryType) — used by the
// `memory` condition's memory_type parameter select.
export const MEMORY_TYPES = [
  'movement',
  'decision',
  'interaction',
  'property',
  'rule',
  'llm_response',
] as const

// Reserved pseudo-property UIDs (mirror data/models PseudoProperty*): usable as a
// property_uid for the `state`/`position` engine concepts.
export const PseudoPropertyState = 'state'
export const PseudoPropertyPosition = 'position'

// Reserved target UID that routes a set/modify_property action onto the
// environment's global properties (mirrors data/models EnvironmentUID).
export const EnvironmentUID = 'environment'

// Random-condition probability bounds (evalRandom: <=0 never fires, >=1 always).
export const RANDOM_PROBABILITY_MIN = 0
export const RANDOM_PROBABILITY_MAX = 1
export const RANDOM_PROBABILITY_STEP = 0.05

export const LLM_STRATEGY_TYPES = ['none', 'full', 'decision', 'hybrid'] as const
export const LLM_ARCHETYPE_MODES = ['identical', 'distribution'] as const
export const VARIATION_STRENGTH_MIN = 0
export const VARIATION_STRENGTH_MAX = 1
export const VARIATION_STRENGTH_STEP = 0.05
export const FALLBACK_STEP_TYPES = ['retry', 'rules_only', 'last_decision', 'skip'] as const
export const EDIT_SECTIONS = ['agents', 'environment', 'rules', 'interactions', 'events'] as const
export const IMAGE_MIME_TYPES = ['image/png', 'image/jpeg', 'image/webp', 'image/gif'] as const
export const LLM_EFFORTS = ['minimal', 'low', 'medium', 'high', 'max'] as const
export const FOOTPRINT_KINDS = ['point', 'circle', 'rectangle', 'polygon'] as const
export const DISPLAY_KINDS = ['auto', 'image', 'figure'] as const
export const COLLISION_MODES = ['', 'block'] as const
/** A polygon footprint / drawn figure needs at least this many points. */
export const MIN_POLYGON_POINTS = 3

// --- helper predicates ---
export function isValidName(name: string): boolean {
  return name.length <= MODEL_NAME_MAX
}

export function isValidDescription(description: string): boolean {
  return description.length <= MODEL_DESCRIPTION_MAX
}

export function isValidMessage(content: string): boolean {
  const trimmed = content.trim()
  return trimmed.length > 0 && content.length <= MESSAGE_CONTENT_MAX
}

export function isAllowedImageMime(mime: string): boolean {
  return (IMAGE_MIME_TYPES as readonly string[]).includes(mime)
}

// --- conditional LLM-requirement predicates (mirror backend enforcement) ---

// A strategy "uses the LLM" when its type is a real strategy (not none/'') AND
// at least one capability toggle is on. Mirrors the backend's
// 422 sim_execution_target_required precondition.
function strategyUsesLLM(s?: LLMStrategyDTO): boolean {
  if (!s) return false
  if (s.type === 'none' || (s.type as string) === '') return false
  return (
    s.use_for_movement || s.use_for_properties || s.use_for_decisions || s.use_for_interaction
  )
}

// modelUsesLLMForGeneration: true when ANY agent/shape initialization or any
// property generation is configured to use the LLM to seed simulations. When
// true the backend requires a simulation-generation target (422
// sim_generation_target_required); the editor surfaces the same as a UX hint.
export function modelUsesLLMForGeneration(model: Model): boolean {
  const propsUseLLM = (props?: { generation?: { mode?: string } }[]): boolean =>
    (props ?? []).some((p) => p.generation?.mode === 'llm')

  const initUsesLLM = (init?: { count_mode?: string; placement?: { type?: string } }): boolean =>
    init?.count_mode === 'llm' || init?.placement?.type === 'llm'

  for (const a of model.agents ?? []) {
    if (initUsesLLM(a.initialization)) return true
    if (propsUseLLM(a.properties)) return true
  }
  for (const s of model.environment?.shapes ?? []) {
    if (initUsesLLM(s.initialization)) return true
    if (propsUseLLM(s.properties)) return true
  }
  if (propsUseLLM(model.environment?.global_properties)) return true
  return false
}

// modelUsesLLMForExecution: true when the default execution strategy OR any
// agent/shape strategy uses the LLM. When true the backend requires a
// simulation-execution target (422 sim_execution_target_required).
export function modelUsesLLMForExecution(model: Model): boolean {
  if (strategyUsesLLM(model.simulation_config?.llm_simulation_execution_config?.default_strategy)) {
    return true
  }
  for (const a of model.agents ?? []) {
    if (strategyUsesLLM(a.llm_config?.strategy)) return true
  }
  for (const s of model.environment?.shapes ?? []) {
    if (strategyUsesLLM(s.llm_config?.strategy)) return true
  }
  return false
}

// --- footprint / display / collision validation (mirror backend rules) ---

// A single UI-side validation issue (mirrors model.ts ValidationIssue, redefined
// locally to avoid a runtime import cycle through the barrel).
export interface SimpleValidationIssue {
  path: string
  code: string
  message: string
}

/**
 * Validate an agent footprint, mirroring the backend rules:
 *   - circle requires radius > 0
 *   - rectangle requires width > 0 && height > 0
 *   - polygon requires ≥ 3 points
 *   - point requires nothing
 * `pathPrefix` is prepended to each issue path (e.g. "agents[0].footprint").
 */
export function validateFootprint(
  footprint: AgentFootprintDTO | undefined,
  pathPrefix = 'footprint'
): SimpleValidationIssue[] {
  if (!footprint || footprint.kind === 'point') return []
  const issues: SimpleValidationIssue[] = []
  switch (footprint.kind) {
    case 'circle':
      if (!(footprint.radius && footprint.radius > 0)) {
        issues.push({ path: `${pathPrefix}.radius`, code: 'footprint_radius_required', message: 'Circle footprint requires a radius greater than 0.' })
      }
      break
    case 'rectangle':
      if (!(footprint.width && footprint.width > 0)) {
        issues.push({ path: `${pathPrefix}.width`, code: 'footprint_width_required', message: 'Rectangle footprint requires a width greater than 0.' })
      }
      if (!(footprint.height && footprint.height > 0)) {
        issues.push({ path: `${pathPrefix}.height`, code: 'footprint_height_required', message: 'Rectangle footprint requires a height greater than 0.' })
      }
      break
    case 'polygon':
      if ((footprint.points?.length ?? 0) < MIN_POLYGON_POINTS) {
        issues.push({ path: `${pathPrefix}.points`, code: 'footprint_points_required', message: `Polygon footprint requires at least ${MIN_POLYGON_POINTS} points.` })
      }
      break
  }
  return issues
}

/**
 * Validate a display override, mirroring the backend rules:
 *   - image requires non-empty data
 *   - figure requires ≥ 3 points
 *   - auto requires nothing
 */
export function validateDisplay(
  display: DisplayDTO | undefined,
  pathPrefix = 'display'
): SimpleValidationIssue[] {
  if (!display || display.kind === 'auto') return []
  const issues: SimpleValidationIssue[] = []
  if (display.kind === 'image') {
    if (!display.image?.data) {
      issues.push({ path: `${pathPrefix}.image.data`, code: 'display_image_required', message: 'Image display requires an uploaded image.' })
    }
  } else if (display.kind === 'figure') {
    if ((display.figure?.length ?? 0) < MIN_POLYGON_POINTS) {
      issues.push({ path: `${pathPrefix}.figure`, code: 'display_figure_required', message: `Figure display requires at least ${MIN_POLYGON_POINTS} points.` })
    }
  }
  return issues
}

/** collision_mode ∈ {'', 'block'} (undefined treated as ''). */
export function isValidCollisionMode(mode: string | undefined): boolean {
  return (COLLISION_MODES as readonly string[]).includes(mode ?? '')
}

// --- shape geometry / initialization validation (mirror validation.go) ---

/**
 * Validate a ScalarSpec, mirroring validateShapeGeometry.checkSpec: exactly one
 * of `value` or `(min,max)` must be set, and min ≤ max when a range is given.
 */
export function validateScalarSpec(
  spec: ScalarSpecDTO | undefined,
  path: string
): SimpleValidationIssue[] {
  if (!spec) return []
  const issues: SimpleValidationIssue[] = []
  const hasValue = spec.value !== undefined
  const hasRange = spec.min !== undefined && spec.max !== undefined
  if (hasValue === hasRange) {
    issues.push({ path, code: 'scalar_spec', message: 'Set exactly one of a fixed value or a (min, max) range.' })
  }
  if (hasRange && (spec.min as number) > (spec.max as number)) {
    issues.push({ path, code: 'scalar_spec_range', message: 'Min must be ≤ max.' })
  }
  return issues
}

/**
 * Validate a shape's geometry, mirroring validation.go validateShapeGeometry:
 *   - geometry is required unless placement is 'llm';
 *   - per-type geometry: rectangle→width+height, circle→radius, polygon→≥3 points,
 *     line→exactly 2 points (skipped for 'llm' placement, where geometry is a hint);
 *   - each present ScalarSpec is exactly-one-of + min≤max;
 *   - a fixed geometry.position implies population_count === 1.
 * `pathPrefix` is prepended (e.g. "environment.shapes[0]").
 */
export function validateShapeGeometry(
  shape: ShapeDTO,
  pathPrefix = 'shape'
): SimpleValidationIssue[] {
  const issues: SimpleValidationIssue[] = []
  const isLLMPlacement = shape.initialization?.placement?.type === 'llm'
  const g = shape.geometry
  if (!g) {
    if (!isLLMPlacement) {
      issues.push({ path: `${pathPrefix}.geometry`, code: 'geometry_required', message: 'Geometry is required for non-LLM placement.' })
    }
    return issues
  }

  issues.push(...validateScalarSpec(g.width, `${pathPrefix}.geometry.width`))
  issues.push(...validateScalarSpec(g.height, `${pathPrefix}.geometry.height`))
  issues.push(...validateScalarSpec(g.radius, `${pathPrefix}.geometry.radius`))

  if (!isLLMPlacement) {
    switch (shape.type) {
      case 'rectangle':
        if (!g.width || !g.height) {
          issues.push({ path: `${pathPrefix}.geometry`, code: 'geometry_type', message: 'Rectangle geometry requires width and height.' })
        }
        break
      case 'circle':
        if (!g.radius) {
          issues.push({ path: `${pathPrefix}.geometry`, code: 'geometry_type', message: 'Circle geometry requires a radius.' })
        }
        break
      case 'polygon':
        if ((g.points?.length ?? 0) < MIN_POLYGON_POINTS) {
          issues.push({ path: `${pathPrefix}.geometry`, code: 'geometry_type', message: `Polygon geometry requires at least ${MIN_POLYGON_POINTS} points.` })
        }
        break
      case 'line':
        if ((g.points?.length ?? 0) !== 2) {
          issues.push({ path: `${pathPrefix}.geometry`, code: 'geometry_type', message: 'Line geometry requires exactly 2 points.' })
        }
        break
    }
  }

  if (g.position && shape.initialization?.population_count !== 1) {
    issues.push({ path: `${pathPrefix}.geometry.position`, code: 'geometry_position_population', message: 'A shape with a fixed position must have population_count == 1.' })
  }
  return issues
}

/**
 * Validate an initialization block, mirroring validateInitialization:
 *   - count_mode 'random_range' requires min & max population, with min ≤ max;
 *   - count_mode 'llm' with both bounds set still requires min ≤ max;
 *   - clustered placement: clusters ≥ 1 and cluster_radius > 0 when set.
 * `pathPrefix` is prepended (e.g. "environment.shapes[0].initialization").
 */
export function validateInitialization(
  init: AgentInitializationDTO | undefined,
  pathPrefix = 'initialization'
): SimpleValidationIssue[] {
  if (!init) return []
  const issues: SimpleValidationIssue[] = []
  if (init.count_mode === 'random_range') {
    if (init.min_population === undefined || init.max_population === undefined) {
      issues.push({ path: `${pathPrefix}.count_mode`, code: 'count_range_required', message: 'random_range requires both min and max population.' })
    } else if (init.min_population > init.max_population) {
      issues.push({ path: `${pathPrefix}.min_population`, code: 'min_max', message: 'Min population must be ≤ max population.' })
    }
  } else if (init.count_mode === 'llm') {
    if (
      init.min_population !== undefined &&
      init.max_population !== undefined &&
      init.min_population > init.max_population
    ) {
      issues.push({ path: `${pathPrefix}.min_population`, code: 'min_max', message: 'Min population must be ≤ max population.' })
    }
  }
  const pl = init.placement
  if (pl?.type === 'clustered') {
    if (pl.clusters !== undefined && pl.clusters < 1) {
      issues.push({ path: `${pathPrefix}.placement.clusters`, code: 'placement_param', message: 'Clusters must be ≥ 1 when set.' })
    }
    if (pl.cluster_radius !== undefined && pl.cluster_radius <= 0) {
      issues.push({ path: `${pathPrefix}.placement.cluster_radius`, code: 'placement_param', message: 'Cluster radius must be > 0 when set.' })
    }
  }
  return issues
}

/** All client-side issues for one shape (geometry + initialization + display). */
/**
 * Whether v is a valid #RGB / #RGBA / #RRGGBB / #RRGGBBAA hex colour. Mirrors the
 * backend models.IsHexColor so the UI can surface a friendly message before a 422.
 */
export function isHexColor(v: unknown): boolean {
  return typeof v === 'string' && /^#([0-9A-Fa-f]{3,4}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$/.test(v)
}

/**
 * Validate colour-typed properties: a non-empty default and every allowed value
 * must be a hex colour (mirrors validation.go validateProperty). Empty default is
 * fine (the sampler then generates a random hex at t=0).
 */
export function validateProperties(
  properties: AgentPropertyDTO[] | undefined,
  pathPrefix: string
): SimpleValidationIssue[] {
  const issues: SimpleValidationIssue[] = []
  ;(properties ?? []).forEach((p, i) => {
    if (p.type !== 'color') return
    const dv = p.default_value
    if (dv !== undefined && dv !== null && dv !== '' && !isHexColor(dv)) {
      issues.push({
        path: `${pathPrefix}[${i}].default_value`,
        code: 'invalid_color',
        message: 'Colour must be a hex value like #RRGGBB.',
      })
    }
    ;(p.allowed_values ?? []).forEach((av, k) => {
      if (av !== '' && !isHexColor(av)) {
        issues.push({
          path: `${pathPrefix}[${i}].allowed_values[${k}]`,
          code: 'invalid_color',
          message: 'Allowed colour must be a hex value like #RRGGBB.',
        })
      }
    })
  })
  return issues
}

export function validateShape(shape: ShapeDTO, pathPrefix = 'shape'): SimpleValidationIssue[] {
  return [
    ...validateShapeGeometry(shape, pathPrefix),
    ...validateInitialization(shape.initialization, `${pathPrefix}.initialization`),
    ...validateDisplay(shape.display, `${pathPrefix}.display`),
    ...validateProperties(shape.properties, `${pathPrefix}.properties`),
  ]
}

/**
 * Validate the footprint + display of every agent and shape in a model, plus the
 * environment collision_mode. Returns a flat list of issues with model-rooted
 * paths. Backend remains authoritative; this surfaces issues inline pre-save.
 */
export function validateModelDisplayAndFootprint(model: Model): SimpleValidationIssue[] {
  const issues: SimpleValidationIssue[] = []
  ;(model.agents ?? []).forEach((a, i) => {
    issues.push(...validateFootprint(a.footprint, `agents[${i}].footprint`))
    issues.push(...validateDisplay(a.display, `agents[${i}].display`))
    issues.push(...validateProperties(a.properties, `agents[${i}].properties`))
  })
  ;(model.environment?.shapes ?? []).forEach((s, i) => {
    // Geometry + initialization + display for every shape (mirrors validation.go).
    issues.push(...validateShapeGeometry(s, `environment.shapes[${i}]`))
    issues.push(...validateInitialization(s.initialization, `environment.shapes[${i}].initialization`))
    issues.push(...validateDisplay(s.display, `environment.shapes[${i}].display`))
    issues.push(...validateProperties(s.properties, `environment.shapes[${i}].properties`))
  })
  if (!isValidCollisionMode(model.environment?.collision_mode)) {
    issues.push({ path: 'environment.collision_mode', code: 'invalid_collision_mode', message: "collision_mode must be '' or 'block'." })
  }
  return issues
}

// --- condition / action per-type advisory validation ---
//
// These mirror the engine evaluators/appliers (rules.go): a condition/action that
// omits its type-required field simply no-ops at runtime, so the checks are
// ADVISORY (warnings, not hard errors). The backend stays authoritative. Paths are
// relative to the passed condition/action; callers prefix with a rule/interaction
// root path.

function numericValue(v: unknown): number | undefined {
  if (typeof v === 'number' && Number.isFinite(v)) return v
  if (typeof v === 'string' && v.trim() !== '') {
    const n = Number(v)
    if (Number.isFinite(n)) return n
  }
  return undefined
}

/**
 * Validate a single condition against its type's runtime contract (advisory):
 *   - event: requires a target_uid (the event to react to; engine reads target_uid
 *     with a property_uid fallback).
 *   - random: requires a probability value in the open range (0..1) to be useful
 *     (<=0 never fires, >=1 always fires — flagged as a likely mistake).
 *   - property / global_property: property_uid recommended.
 * Other types no-op safely and are not flagged here.
 */
export function validateCondition(
  cond: ConditionDTO,
  pathPrefix = 'condition'
): SimpleValidationIssue[] {
  const issues: SimpleValidationIssue[] = []
  switch (cond.type) {
    case 'event':
      if (!cond.target_uid && !cond.property_uid) {
        issues.push({ path: `${pathPrefix}.target_uid`, code: 'event_target_required', message: 'Event condition needs an event to react to.' })
      }
      break
    case 'random': {
      const p = numericValue(cond.value)
      if (p === undefined) {
        issues.push({ path: `${pathPrefix}.value`, code: 'random_probability_required', message: 'Random condition needs a probability value between 0 and 1.' })
      } else if (p <= RANDOM_PROBABILITY_MIN || p >= RANDOM_PROBABILITY_MAX) {
        issues.push({ path: `${pathPrefix}.value`, code: 'random_probability_range', message: 'Random probability should be between 0 and 1 (≤0 never fires, ≥1 always fires).' })
      }
      break
    }
    case 'property':
    case 'global_property':
      if (!cond.property_uid) {
        issues.push({ path: `${pathPrefix}.property_uid`, code: 'property_uid_recommended', message: 'Select a property for this condition.' })
      }
      break
  }
  return issues
}

/**
 * Validate a single action against its type's runtime contract (advisory):
 *   - set_property / modify_property: property_uid required.
 *   - emit_event: event_uid required.
 *   - transition: new_state required.
 *   - interact: target_uid (interaction id) required.
 *   - spawn_agent / transform: spawn_agent_uid required (transform falls back to
 *     target_uid on the wire, but the editor authors spawn_agent_uid).
 * Other types (move/call_llm/custom/remove_agent/teleport) no-op safely.
 */
export function validateAction(action: ActionDTO, pathPrefix = 'action'): SimpleValidationIssue[] {
  const issues: SimpleValidationIssue[] = []
  switch (action.type) {
    case 'set_property':
    case 'modify_property':
      if (!action.property_uid) {
        issues.push({ path: `${pathPrefix}.property_uid`, code: 'action_property_required', message: 'Select a property for this action.' })
      }
      break
    case 'emit_event':
      if (!action.event_uid) {
        issues.push({ path: `${pathPrefix}.event_uid`, code: 'action_event_required', message: 'Select an event to emit.' })
      }
      break
    case 'transition':
      if (!action.new_state) {
        issues.push({ path: `${pathPrefix}.new_state`, code: 'action_state_required', message: 'Select a target state for this transition.' })
      }
      break
    case 'interact':
      if (!action.target_uid) {
        issues.push({ path: `${pathPrefix}.target_uid`, code: 'action_interaction_required', message: 'Select an interaction to force-fire.' })
      }
      break
    case 'spawn_agent':
    case 'transform':
      if (!action.spawn_agent_uid && !action.target_uid) {
        issues.push({ path: `${pathPrefix}.spawn_agent_uid`, code: 'action_spawn_agent_required', message: 'Select the agent archetype to spawn/transform into.' })
      }
      break
  }
  return issues
}
