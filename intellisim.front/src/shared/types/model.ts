// TypeScript mirror of the backend models module DTOs
// (../intellisim.core/modules/models/{responses,dtos}.go). Keys are snake_case
// to match the JSON wire format exactly. The LLM execution target is reused from
// shared/types/llm.ts (ExecutionLlmTarget) — provider_uid/model_uid/effort?/
// generation?/timeout_ms? — and is NOT duplicated here.
import type { ExecutionLlmTarget, ConfigValue, GenerationParams } from '@/shared/types/llm'
import type { WidgetConfig } from '@/shared/types/widget'
// Token-usage + message-error shapes are identical to the chat-domain ones —
// reuse them rather than redefining (avoids duplicate names in the types barrel).
import type { TokenUsage, TokenUsageSummary, MessageErrorDTO } from '@/shared/types/chat'

// Re-export the shared LLM target/config types under the model namespace so model
// consumers can import them from one place without duplicating the definitions.
export type { ExecutionLlmTarget, ConfigValue, GenerationParams }
export type { TokenUsage, TokenUsageSummary, MessageErrorDTO }

// --- enums (string-literal unions) ---

export type ModelUpdateMode = 'sequential' | 'synchronous'
export type AgentType = 'agent' | 'archetype'
export type MoveType = 'static' | 'movable'
export type PropertyType = 'string' | 'number' | 'enum' | 'boolean' | 'color'
export type PlacementType = 'random' | 'uniform' | 'clustered' | 'at_shape' | 'manual' | 'llm'
export type ShapeType = 'rectangle' | 'circle' | 'line' | 'polygon'
export type ConditionType =
  | 'global_property'
  | 'property'
  | 'position'
  | 'proximity'
  | 'step'
  | 'composite'
  | 'count'
  | 'barrier'
  | 'state'
  | 'distance'
  | 'decision'
  | 'random'
  | 'memory'
  | 'event'
  | 'custom'
export type ConditionOperator =
  | 'eq'
  | 'neq'
  | 'gt'
  | 'lt'
  | 'gte'
  | 'lte'
  | 'contains'
  | 'in_range'
  | 'in'
  | 'not_in'
export type LogicOperator = 'AND' | 'OR' | 'NOT'
export type ActionType =
  | 'set_property'
  | 'modify_property'
  | 'move'
  | 'move_toward'
  | 'move_away'
  | 'interact'
  | 'emit_event'
  | 'transition'
  | 'call_llm'
  | 'spawn_agent'
  | 'remove_agent'
  | 'transform'
  | 'teleport'
  | 'custom'
export type ActionModifier =
  | 'set'
  | 'add'
  | 'subtract'
  | 'multiply'
  | 'divide'
  | 'increment'
  | 'decrement'
  | 'toggle'
export type SpawnPositionType = 'same' | 'random' | 'offset' | 'nearby' | 'at_shape'
export type InteractionType = 'agent_agent' | 'agent_shape' | 'agent_global'
export type EventScope = 'global' | 'local' | 'targeted'
export type EventProducer = 'agent' | 'environment' | 'llm'
export type LLMStrategyType = 'none' | 'full' | 'decision' | 'hybrid'
export type LLMArchetypeMode = 'identical' | 'distribution'
export type FallbackStepType = 'retry' | 'rules_only' | 'last_decision' | 'skip'
export type ImageMimeType = 'image/png' | 'image/jpeg' | 'image/webp' | 'image/gif'
export type EditSection = 'agents' | 'environment' | 'rules' | 'interactions' | 'events'
export type ModelChatStatus = 'idle' | 'thinking'
export type ValidationSeverity = 'error' | 'warning'

// --- agents ---

// PropertyGenerationDTO mirrors the backend PropertyGeneration (data/models/
// generation.go): per-property value-generation strategy (default/random/llm
// mode; uniform/normal/choice distribution + params). Surfaced on snapshot/model
// properties for the simulation editor's generation controls.
export type PropertyGenerationMode = 'default' | 'random' | 'llm'
export type PropertyGenerationDistribution = 'uniform' | 'normal' | 'choice'

export interface ChoiceWeightDTO {
  value: unknown
  weight: number
}

export interface PropertyGenerationDTO {
  mode?: PropertyGenerationMode
  distribution?: PropertyGenerationDistribution
  mean?: number
  std_dev?: number
  min?: number
  max?: number
  choices?: ChoiceWeightDTO[]
  prompt_context?: string
}

export interface AgentPropertyDTO {
  uid: string
  name: string
  type: PropertyType
  description: string
  min_value?: number
  max_value?: number
  default_value?: unknown
  allowed_values?: unknown[]
  immutable: boolean
  llm_prompt_context?: string
  generation?: PropertyGenerationDTO
}

export interface PlacementDTO {
  type: PlacementType
  shape_uid?: string
  prompt_context?: string
  // clustered: number of gaussian cluster centres + spread (world units); nil ⇒ derived.
  clusters?: number
  cluster_radius?: number
  // manual placement carries no coordinates on the model — instances are positioned
  // per-simulation in the editor before the run.
}

// PopulationCountMode mirrors data/models PopulationCountMode (fixed/
// random_range/llm) — how the initial population_count is determined.
export type PopulationCountMode = 'fixed' | 'random_range' | 'llm'

export interface AgentInitializationDTO {
  population_count: number
  min_population?: number
  max_population?: number
  initial_state?: string
  count_mode?: PopulationCountMode
  placement?: PlacementDTO
}

// --- simulation body (footprint) + display (visual) ---

// FootprintKind is the collision/spatial shape the engine uses for an agent.
// `point` (default) keeps the legacy point-agent behaviour; the others give the
// agent a spatial extent for collision/overlap. Mirrors the backend
// AgentFootprintDTO.Kind oneof.
export type FootprintKind = 'point' | 'circle' | 'rectangle' | 'polygon'

// AgentFootprintDTO is the agent's simulation body. radius is used for circle;
// width/height for rectangle; points (anchor-relative, centered on the agent
// position) for polygon. Absent fields default to point. Mirrors the backend
// AgentFootprintDTO.
export interface AgentFootprintDTO {
  kind: FootprintKind
  radius?: number
  width?: number
  height?: number
  points?: PointDTO[]
}

// DisplayKind is the purely-visual representation, INDEPENDENT of the footprint.
// `auto` (default) draws the body in the archetype colour; `image` renders a
// base64 sprite; `figure` renders a user-drawn polygon. Mirrors DisplayDTO.Kind.
export type DisplayKind = 'auto' | 'image' | 'figure'

// DisplayImageDTO carries a base64-encoded sprite (data has NO "data:" prefix).
// width/height are the intrinsic pixel dimensions (optional hints). Mirrors the
// backend DisplayImageDTO.
export interface DisplayImageDTO {
  data: string
  mime_type?: string
  width?: number
  height?: number
}

// DisplayDTO is the agent/shape visual override. figure points are
// anchor-relative (centered on the entity position). scale optionally scales the
// rendered image/figure. Mirrors the backend DisplayDTO.
export interface DisplayDTO {
  kind: DisplayKind
  image?: DisplayImageDTO
  figure?: PointDTO[]
  scale?: number
}

export interface Agent {
  uid: string
  name: string
  description: string
  type: AgentType
  move_type: MoveType
  properties: AgentPropertyDTO[]
  states: string[]
  initialization: AgentInitializationDTO
  // Simulation body (collision/spatial shape). Absent = point (legacy).
  footprint?: AgentFootprintDTO
  // Visual override. Absent = auto (colored body).
  display?: DisplayDTO
  // When the environment collision_mode is 'block', collidable agents
  // participate in overlap resolution. Absent/false = pass-through.
  collidable?: boolean
  llm_config?: LLMAgentConfigDTO
  locked: boolean
  created_at?: string
  updated_at?: string
}

// --- environment + shapes ---

export interface PointDTO {
  x: number
  y: number
}

export interface ScalarSpecDTO {
  value?: number
  min?: number
  max?: number
}

export interface ShapeGeometryDTO {
  position?: PointDTO
  width?: ScalarSpecDTO
  height?: ScalarSpecDTO
  radius?: ScalarSpecDTO
  points?: PointDTO[]
  thickness?: number
  fill?: boolean
  stroke?: boolean
  stroke_width?: number
}

export interface ShapeDTO {
  uid: string
  name: string
  description: string
  type: ShapeType
  layer: number
  visible: boolean
  properties: AgentPropertyDTO[]
  initialization: AgentInitializationDTO
  geometry?: ShapeGeometryDTO
  // Visual override (image sprite / drawn figure). Absent = render geometry.
  display?: DisplayDTO
  llm_config?: LLMShapeConfigDTO
  locked: boolean
}

// CollisionMode is the per-world overlap policy. '' / absent = none (legacy,
// agents may overlap freely); 'block' = the engine resolves agent overlaps.
export type CollisionMode = '' | 'block'

export interface EnvironmentDTO {
  uid: string
  width: number
  height: number
  torus: boolean
  // Overlap policy. Omitted/'' = none; 'block' = engine resolves overlaps.
  collision_mode?: CollisionMode
  shapes: ShapeDTO[]
  global_properties: AgentPropertyDTO[]
  llm_prompt_context?: string
}

// --- rules ---

export interface ConditionDTO {
  type: ConditionType
  property_uid?: string
  operator?: ConditionOperator
  value?: unknown
  min_value?: number
  max_value?: number
  radius?: number
  target_uid?: string
  target_uids?: string[]
  logic_operator?: LogicOperator
  sub_conditions?: ConditionDTO[]
  parameters?: Record<string, unknown>
  description?: string
}

export interface SpawnPositionDTO {
  type: SpawnPositionType
  offset_x?: number
  offset_y?: number
  radius?: number
  shape_uid?: string
}

export interface ActionDTO {
  type: ActionType
  property_uid?: string
  value?: unknown
  modifier?: ActionModifier
  target_uid?: string
  target_uids?: string[]
  speed?: number
  direction?: number
  event_uid?: string
  event_data?: unknown
  new_state?: string
  spawn_agent_uid?: string
  spawn_count?: number
  spawn_properties?: Record<string, unknown>
  spawn_position?: SpawnPositionDTO
  spawn_inherit_from?: string[]
  parameters?: Record<string, unknown>
  description?: string
  llm_prompt_context?: string
}

export interface RuleDTO {
  id: string
  name: string
  description: string
  priority: number
  applies_to: string[]
  conditions: ConditionDTO[]
  actions: ActionDTO[]
  execute_once: boolean
  cooldown_steps: number
  enabled: boolean
  locked: boolean
}

// --- interactions ---

export interface InteractionDTO {
  id: string
  name: string
  description: string
  type: InteractionType
  initiator_uids?: string[]
  target_uids?: string[]
  radius?: number
  conditions: ConditionDTO[]
  initiator_actions: ActionDTO[]
  target_actions: ActionDTO[]
  bidirectional: boolean
  use_llm: boolean
  enabled: boolean
  locked: boolean
  llm_prompt_context?: string
}

// --- events ---

export interface EventDefDTO {
  uid: string
  name: string
  description: string
  scope: EventScope
  radius?: number
  payload: AgentPropertyDTO[]
  producers: EventProducer[]
  producer_uids?: string[]
  consumer_uids?: string[]
  ttl_steps: number
  llm_prompt_context?: string
  llm_target?: ExecutionLlmTarget
  locked: boolean
}

// --- llm config / strategy / fallback ---

// The four use_for_* flags are canonical for `type` (none→all off, full→all on,
// decision→decisions only, hybrid→free); the backend re-normalizes on save.
export interface LLMStrategyDTO {
  type: LLMStrategyType
  batch_size: number
  use_for_movement: boolean
  use_for_interaction: boolean
  use_for_properties: boolean
  use_for_decisions: boolean
  decision_frequency: number
  // Archetype-only: how a single archetype-level LLM result is applied to all
  // members ('identical' = exact clone; 'distribution' = per-agent weighted
  // sampling) and the per-agent jitter amount (0..1). Optional on the wire so
  // existing models keep backend defaults.
  archetype_mode?: LLMArchetypeMode
  variation_strength?: number
}

// Per-agent/per-shape strategy overrides are structurally identical to the base
// strategy (spawn/population are generation concerns and no longer live here;
// population/count + placement guidance is AgentInitialization.Placement.PromptContext).
export type LLMAgentStrategyDTO = LLMStrategyDTO

export type LLMShapeStrategyDTO = LLMStrategyDTO

export interface FallbackStepDTO {
  type: FallbackStepType
  max_retries: number
  retry_delay_ms: number
}

export interface FallbackPolicyDTO {
  steps: FallbackStepDTO[]
}

export interface LLMSimulationAgentConfigDTO {
  perception_radius: number
  interaction_radius: number
  memory_size: number
  fallback_policy?: FallbackPolicyDTO
}

// LLMConfig is the SLIM LLM config: an execution target + a permanent prompt
// context. Used for the model-generation+chat config (model.llm_config) and the
// simulation-generation config (simulation_config.llm_simulation_generation_config).
export interface LLMConfigDTO {
  target: ExecutionLlmTarget
  prompt_context?: string
}

// LLMSimulationExecutionConfig is the FULL execution-engine config: a target +
// prompt context, the inline agent-runtime knobs (perception/interaction/memory/
// fallback — formerly LLMSimulationAgentConfig fields), the runner knobs
// (parallel_workers/retry_timeout_ms/max_retries/stream_batch_size), and the
// default strategy. Lives at simulation_config.llm_simulation_execution_config.
export interface LLMSimulationExecutionConfigDTO {
  target: ExecutionLlmTarget
  prompt_context?: string
  perception_radius: number
  interaction_radius: number
  memory_size: number
  fallback_policy?: FallbackPolicyDTO
  parallel_workers: number
  retry_timeout_ms: number
  max_retries: number
  stream_batch_size: number
  default_strategy: LLMStrategyDTO
}

// SimulationConfig groups the model's simulation-time configuration: the
// update_mode (moved from the top-level Model) plus the two simulation LLM
// configs (generation seeds the world; execution runs the engine).
export interface SimulationConfigDTO {
  update_mode: ModelUpdateMode
  llm_simulation_generation_config: LLMConfigDTO
  llm_simulation_execution_config: LLMSimulationExecutionConfigDTO
}

export interface LLMAgentConfigDTO {
  target: ExecutionLlmTarget
  prompt_context?: string
  strategy: LLMAgentStrategyDTO
  simulation_config?: LLMSimulationAgentConfigDTO
}

export interface LLMShapeConfigDTO {
  target: ExecutionLlmTarget
  prompt_context?: string
  strategy: LLMShapeStrategyDTO
}

// --- images ---

export interface ModelImageDTO {
  uid?: string
  name?: string
  mime_type?: string
  data?: string
  description?: string
}

// --- the editable model body (ModelInputDTO) ---

// Model is the ModelInputDTO shape: the editable model returned on GET and
// accepted on PUT (inside UpdateModelRequest.model).
export interface Model {
  name: string
  description: string
  is_spatial: boolean
  spatial_cell_size: number
  // Model-generation + chat LLM (slim: target + prompt context).
  llm_config: LLMConfigDTO
  // Simulation-time config: update_mode + the simulation-generation and
  // simulation-execution LLM configs (replaces the old top-level update_mode).
  simulation_config: SimulationConfigDTO
  agents: Agent[]
  environment?: EnvironmentDTO
  rules: RuleDTO[]
  interactions: InteractionDTO[]
  events: EventDefDTO[]
  images: ModelImageDTO[]
  // Optional dashboard widget layout (template), mirrors the backend model's
  // widgets field. New (typed) WidgetConfig shape — see shared/types/widget.ts.
  widgets?: WidgetConfig[]
}

// --- normalization ---

// The backend serializes empty Go slices as JSON `null` (nil slices →
// `"agents": null`, etc.), and nested slices the same way. The editor + form
// components iterate these arrays (`v-for`, `.length`), so a null array crashes
// the whole editor body. normalizeModel guarantees every array the UI iterates
// is a real array (`?? []`), shallow-but-sufficient — it does not invent schema,
// only coerces the array fields. Apply it wherever a ModelResponse.model is
// adopted into the store so `current.model` always has arrays.
export function normalizeModel(model: Model): Model {
  return {
    ...model,
    agents: (model.agents ?? []).map((a) => ({
      ...a,
      properties: a.properties ?? [],
      states: a.states ?? [],
    })),
    environment: model.environment
      ? {
          ...model.environment,
          shapes: (model.environment.shapes ?? []).map((s) => ({
            ...s,
            properties: s.properties ?? [],
          })),
          global_properties: model.environment.global_properties ?? [],
        }
      : model.environment,
    rules: (model.rules ?? []).map((r) => ({
      ...r,
      conditions: r.conditions ?? [],
      actions: r.actions ?? [],
    })),
    interactions: (model.interactions ?? []).map((it) => ({
      ...it,
      conditions: it.conditions ?? [],
      initiator_actions: it.initiator_actions ?? [],
      target_actions: it.target_actions ?? [],
    })),
    events: (model.events ?? []).map((e) => ({
      ...e,
      payload: e.payload ?? [],
      producers: e.producers ?? [],
      producer_uids: e.producer_uids ?? [],
      consumer_uids: e.consumer_uids ?? [],
    })),
    images: model.images ?? [],
    // Guarantee the nested simulation_config (and its exec config's fallback
    // policy steps) is a real object so the editor never crashes on a model
    // persisted before the simulation_config split.
    simulation_config: normalizeSimulationConfig(model.simulation_config),
  }
}

// Ensure model.simulation_config and its nested LLM configs/fallback steps are
// real objects (defaults for any missing field), so the editor's deep template
// bindings never read through undefined.
function normalizeSimulationConfig(sc?: SimulationConfigDTO): SimulationConfigDTO {
  const exec = sc?.llm_simulation_execution_config
  return {
    update_mode: sc?.update_mode ?? 'sequential',
    llm_simulation_generation_config: {
      target: sc?.llm_simulation_generation_config?.target ?? emptyTarget(),
      prompt_context: sc?.llm_simulation_generation_config?.prompt_context,
    },
    llm_simulation_execution_config: {
      target: exec?.target ?? emptyTarget(),
      prompt_context: exec?.prompt_context,
      perception_radius: exec?.perception_radius ?? 0,
      interaction_radius: exec?.interaction_radius ?? 0,
      memory_size: exec?.memory_size ?? 0,
      fallback_policy: exec?.fallback_policy
        ? { ...exec.fallback_policy, steps: exec.fallback_policy.steps ?? [] }
        : exec?.fallback_policy,
      parallel_workers: exec?.parallel_workers ?? 0,
      retry_timeout_ms: exec?.retry_timeout_ms ?? 0,
      max_retries: exec?.max_retries ?? 0,
      stream_batch_size: exec?.stream_batch_size ?? 0,
      default_strategy: exec?.default_strategy ?? emptyStrategy(),
    },
  }
}

function emptyTarget(): ExecutionLlmTarget {
  return { provider_uid: '', model_uid: '' }
}

function emptyStrategy(): LLMStrategyDTO {
  return {
    type: 'none',
    batch_size: 0,
    use_for_movement: false,
    use_for_interaction: false,
    use_for_properties: false,
    use_for_decisions: false,
    decision_frequency: 0,
  }
}

// Normalize the editable body of a ModelResponse in place-returning a new
// envelope, leaving every non-`model` field untouched.
export function normalizeModelResponse(resp: ModelResponse): ModelResponse {
  return { ...resp, model: normalizeModel(resp.model) }
}

// --- validation ---

export interface ValidationIssue {
  path: string
  code: string
  message: string
  severity: ValidationSeverity
}

export interface ValidationResult {
  errors: ValidationIssue[]
  warnings: ValidationIssue[]
  simulation_ready: boolean
}

// --- top-level model responses ---

export interface ModelResponse {
  id: string
  chat_id?: string
  model: Model
  validation: ValidationResult
  created_at: string
  updated_at: string
  created_by: string
  updated_by: string
}

export interface ModelSummary {
  id: string
  name: string
  description: string
  is_spatial: boolean
  has_chat: boolean
  agent_count: number
  simulation_ready: boolean
  updated_at: string
  created_by: string
}

export interface ListModelsResponse {
  models: ModelSummary[]
  page: number
  page_size: number
  total: number
  has_more: boolean
}

// Minimal id+name pair for selector UIs (GET /models/options, unpaginated).
export interface ModelOption {
  id: string
  name: string
}

// Body for POST /models/:id/duplicate — optional name override for the copy.
export interface DuplicateModelRequest {
  name?: string
}

export interface ListModelOptionsResponse {
  options: ModelOption[]
}

// --- model chat (lives in shared chats collections) ---

export interface ModelChat {
  model_id: string
  chat_id: string
  llm_target: ExecutionLlmTarget
  model_available: boolean
  status: ModelChatStatus
  token_usage?: TokenUsageSummary
}

export interface ModelChatMessage {
  id: string
  chat_id: string
  role: string
  content: string
  /** Full spec JSON — assistant messages only. */
  raw?: string
  llm_target?: ExecutionLlmTarget
  usage?: TokenUsage
  error?: MessageErrorDTO
  created_at: string
  created_by: string
}

export interface MessagesPage {
  messages: ModelChatMessage[]
  has_more: boolean
}

export interface ModelSendMessageResponse {
  message: ModelChatMessage
  chat_status: ModelChatStatus
}

// --- requests ---

export interface EditScope {
  section?: EditSection
  uid?: string
}

export interface CreateModelRequest {
  name?: string
  description?: string
  is_spatial?: boolean
  llm_target?: ExecutionLlmTarget
  initial_message?: string
}

export interface UpdateModelRequest {
  model: Model
}

export interface SetModelChatTargetRequest {
  llm_target: ExecutionLlmTarget
}

export interface SendModelMessageRequest {
  content: string
  scope?: EditScope
}

// Add one image to a model's environment-generation image library. data is raw
// base64 (no "data:" prefix); the backend compresses it on add.
export interface AddModelImageRequest {
  name?: string
  description?: string
  mime_type: ImageMimeType
  data: string
}

// Start environment generation from the model's STORED image library (images are
// managed separately via the add/delete endpoints).
export interface GenerateEnvironmentRequest {
  /** Must support vision_analysis; omit to use the model chat target. */
  llm_target?: ExecutionLlmTarget
  prompt?: string
}

// --- SSE payloads ---

export interface ModelChatStatusPayload {
  model_id: string
  status: ModelChatStatus
}

export interface ModelUpdatedPayload {
  model_id: string
  model: ModelResponse
}

export interface ModelChatErrorPayload {
  model_id: string
  code: string
  message: string
}

export interface ModelDeletedPayload {
  model_id: string
}
