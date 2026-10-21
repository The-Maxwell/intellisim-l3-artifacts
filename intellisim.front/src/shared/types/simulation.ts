// TypeScript mirror of the backend simulations module DTOs
// (../intellisim.core/modules/simulations/{responses,dtos}.go). Keys are
// snake_case to match the JSON wire format exactly. The LLM execution target is
// reused from shared/types/llm.ts (ExecutionLlmTarget) rather than redefined.

import type { ExecutionLlmTarget } from '@/shared/types/llm'
import type { WidgetConfig } from '@/shared/types/widget'
// Footprint/display are wire-identical to the model-domain DTOs; reuse them
// rather than redefining (avoids duplicate names in the types barrel). They are
// NOT re-exported here — import them from model.ts / the barrel directly.
import type { AgentFootprintDTO, DisplayDTO } from '@/shared/types/model'

// Re-export the execution target so simulation consumers can import it from one
// place (mirrors model.ts re-exporting it under the model namespace).
export type { ExecutionLlmTarget }

// --- status (mirrors data/simulations status values) ---

export type SimulationStatus =
  | 'generating'
  | 'ready'
  | 'generation_failed'
  | 'queued'
  | 'running'
  | 'paused'
  | 'finished'
  | 'failed'
  | 'stopped'

// --- shared geometry / llm target (mirror PointDTO / ConfigValueDTO / LLMTargetDTO) ---

export interface PointXY {
  x: number
  y: number
}

// LLMTarget is wire-identical to the shared llm.ts ExecutionLlmTarget (which
// itself carries the dynamic-config `generation` map of ConfigValue). Aliased
// here so simulation consumers have a local name without re-declaring it.
export type LLMTarget = ExecutionLlmTarget

// --- simulation state (mirror SimulationAgentDTO / SimulationEnvironmentDTO / SimulationShapeDTO) ---

// MemoryEntry is the compact, API-facing form of one AgentMemory entry (mirrors
// AgentMemoryDTO). Details and Timestamp are excluded server-side to keep the
// payload small. type is one of: decision, interaction, movement, property, rule,
// llm_response.
export interface MemoryEntry {
  step: number
  type: string
  summary: string
}

export interface SimulationAgent {
  id: number
  agent_uid: string
  name: string
  position?: PointXY
  properties?: Record<string, unknown>
  state: string
  // memory is populated after a run; absent during the create/edit phase or
  // when the simulation has never been run.
  memory?: MemoryEntry[]
  // decision is the per-step LLM decision summary when this agent shape is
  // synthesized from a run/replay frame (widget-compute context only; never
  // present on persisted initial-state agents).
  decision?: string
}

export interface SimulationShape {
  id: number
  shape_uid: string
  name: string
  type: string
  layer: number
  visible: boolean
  properties?: Record<string, unknown>
  position: PointXY
  width?: number
  height?: number
  radius?: number
  points?: PointXY[]
  thickness?: number
  fill?: boolean
  stroke?: boolean
  stroke_width?: number
  // Visual override (image sprite / drawn figure). Absent = render geometry.
  display?: DisplayDTO
}

export interface SimulationEnvironment {
  width: number
  height: number
  torus: boolean
  // Overlap policy. Omitted/'' = none; 'block' = engine resolves overlaps.
  collision_mode?: string
  global_properties?: Record<string, unknown>
  shapes?: SimulationShape[]
}

// --- model snapshot (mirror ModelSnapshotDTO + Snapshot* sub-DTOs) ---

export interface ChoiceWeight {
  value: unknown
  weight: number
}

// Generation mode/distribution unions live canonically in model.ts
// (PropertyGenerationMode / PropertyGenerationDistribution); the snapshot reuses
// inline string-literal unions to avoid duplicate barrel exports.
export interface PropertyGeneration {
  mode?: 'default' | 'random' | 'llm'
  distribution?: 'uniform' | 'normal' | 'choice'
  mean?: number
  std_dev?: number
  min?: number
  max?: number
  choices?: ChoiceWeight[]
  prompt_context?: string
}

export interface SnapshotProperty {
  uid: string
  name: string
  type: string
  description?: string
  min_value?: number
  max_value?: number
  default_value?: unknown
  allowed_values?: unknown[]
  immutable: boolean
  llm_prompt_context?: string
  generation?: PropertyGeneration
}

export interface SnapshotPlacement {
  type?: string
  shape_uid?: string
  prompt_context?: string
  clusters?: number
  cluster_radius?: number
}

export interface SnapshotInitialization {
  population_count: number
  min_population?: number
  max_population?: number
  initial_state?: string
  count_mode?: string
  placement?: SnapshotPlacement
}

export interface SnapshotAgent {
  uid: string
  name: string
  type: string
  move_type: string
  properties?: SnapshotProperty[]
  states?: string[]
  initialization: SnapshotInitialization
  // Simulation body (collision/spatial shape). Absent = point.
  footprint?: AgentFootprintDTO
  // Visual override. Absent = auto (colored body).
  display?: DisplayDTO
  collidable?: boolean
}

// ScalarSpec mirrors ScalarSpecDTO: a fixed Value OR a [Min..Max] random range.
export interface ScalarSpec {
  value?: number
  min?: number
  max?: number
}

export interface SnapshotGeometry {
  position?: PointXY
  width?: ScalarSpec
  height?: ScalarSpec
  radius?: ScalarSpec
  points?: PointXY[]
  thickness?: number
  fill?: boolean
  stroke?: boolean
  stroke_width?: number
}

export interface SnapshotShape {
  uid: string
  name: string
  description?: string
  type: string
  layer: number
  visible: boolean
  properties?: SnapshotProperty[]
  initialization: SnapshotInitialization
  geometry?: SnapshotGeometry
  // Visual override (image sprite / drawn figure). Absent = render geometry.
  display?: DisplayDTO
}

export interface SnapshotEnvironment {
  uid?: string
  width: number
  height: number
  torus: boolean
  // Overlap policy. Omitted/'' = none; 'block' = engine resolves overlaps.
  collision_mode?: string
  global_properties?: SnapshotProperty[]
  shapes?: SnapshotShape[]
}

export interface ModelSnapshot {
  name: string
  description?: string
  is_spatial: boolean
  agents?: SnapshotAgent[]
  environment?: SnapshotEnvironment
}

// --- generation runs (mirror GenerationRunDTO + scope/warning) ---

export type GenerationScopeKind = 'full' | 'environment' | 'agents'
export type GenerationScopePart = 'population' | 'properties' | 'placement'

// Read form of a generation scope (GenerationScopeResponseDTO).
export interface GenerationScope {
  kind: GenerationScopeKind
  agent_uids?: string[]
  parts?: GenerationScopePart[]
}

export interface GenerationWarning {
  path: string
  reason: string
}

export interface GenerationRun {
  seq: number
  scope: GenerationScope
  status: string
  started_at: string
  finished_at?: string
  error?: string
  warnings?: GenerationWarning[]
  requests: number
}

// --- token usage (mirror SimulationTokenUsageDTO + sub-DTOs) ---

// TokenUsageDim is one token-usage dimension (total / per-model / per-agent /
// per-purpose / per-generation). Mirrors TokenUsageDTO. Named *Dim to avoid a
// barrel collision with the chat-domain TokenUsage.
export interface TokenUsageDim {
  // Routing identity (set only on the by-model breakdown entries).
  provider_uid?: string
  model_uid?: string
  effort?: string
  prompt_tokens: number
  completion_tokens: number
  // reasoning_tokens is the reasoning portion of completion_tokens (a subset,
  // already included in completion_tokens/total_tokens) — a breakdown detail for
  // reasoning models, never a summand. 0 for non-reasoning models.
  reasoning_tokens: number
  total_tokens: number
  calls: number
  latency_ms: number
}

// AgentTokenUsageDTO / PurposeTokenUsageDTO embed TokenUsageDTO (flattened on the
// wire), so the embedded fields sit alongside the discriminator field.
export interface AgentTokenUsage extends TokenUsageDim {
  agent_uid: string
}

export interface PurposeTokenUsage extends TokenUsageDim {
  purpose: string
}

export interface SimulationTokenUsage {
  total: TokenUsageDim
  by_model?: TokenUsageDim[]
  by_agent?: AgentTokenUsage[]
  by_purpose?: PurposeTokenUsage[]
  by_generation?: TokenUsageDim[]
  // ByRun is indexed by execution run seq-1; each entry's routing identity is
  // blanked when that run mixed multiple distinct targets.
  by_run?: TokenUsageDim[]
}

// Runtime LLM-usage purpose values (folded into by_purpose during a run; mirror
// data/simulations.LLMUsage* purpose strings). simulation_generation is the
// create/edit purpose, kept for completeness.
export type SimulationLlmUsagePurpose =
  | 'simulation_generation'
  | 'simulation_movement'
  | 'simulation_properties'
  | 'simulation_decision'
  | 'simulation_interaction'

// --- execution runs (mirror ExecutionRunDTO) ---

// RunStatus is the lifecycle of one engine run attempt (mirrors
// data/simulations.ExecutionRunStatus). Distinct from the simulation-wide
// SimulationStatus union.
export type RunStatus = 'running' | 'finished' | 'stopped' | 'failed'

// ExecutionRun is the audit record of one engine run attempt (mirrors
// ExecutionRunDTO). seq is 1-based and matches by_run[seq-1] token usage.
export interface ExecutionRun {
  seq: number
  started_at: string
  finished_at?: string
  steps_completed: number
  status: RunStatus
  error?: string
  // active_duration_ms is the run's active (pause-excluding) duration in
  // milliseconds; 0 until the run finalizes.
  active_duration_ms?: number
}

// --- statistics (mirror data/simulations.SimulationStatistics) ---

export interface AgentPopulationStat {
  agent_uid: string
  current: number
  peak: number
  min: number
  spawned: number
  removed: number
  by_state?: Record<string, number>
}

export interface RuleStatistic {
  rule_id: string
  executions: number
}

export interface EventStatistic {
  event_uid: string
  emitted: number
}

export interface InteractionStatistic {
  interaction_id: string
  count: number
}

export interface PropertyStatistic {
  agent_uid: string
  property_uid: string
  avg: number
  min: number
  max: number
}

// SimulationStatistics is the materialised run summary persisted on the
// simulation and also streamed/embedded per step (the StepDTO.statistics and
// StreamMessage.statistics payload share this shape, hence its use as `unknown`
// in those slots is narrowed here).
export interface SimulationStatistics {
  steps_completed: number
  total_agents: number
  population?: AgentPopulationStat[]
  agents_spawned: number
  agents_removed: number
  interactions: number
  events: number
  rule_executions: number
  movement_distance: number
  by_rule?: RuleStatistic[]
  by_event?: EventStatistic[]
  by_interaction?: InteractionStatistic[]
  properties?: PropertyStatistic[]
  custom?: Record<string, unknown>
}

// --- status history (mirror StatusChangeDTO) ---

export interface StatusChange {
  status: string
  at: string
  by: string
}

// --- validation (mirror ValidationResult / ValidationIssue) ---
// Named Simulation* to avoid a barrel collision with the model-domain
// ValidationResult / ValidationIssue (which have a different shape).

export interface SimulationValidationIssue {
  path: string
  reason: string
}

export interface SimulationValidationResult {
  errors: SimulationValidationIssue[]
  warnings: SimulationValidationIssue[]
  start_ready: boolean
}

// --- full simulation response (mirror SimulationResponse) ---

export interface Simulation {
  id: string
  name: string
  description?: string
  model_id: string

  seed: number
  steps: number

  // Per-simulation LLM target overrides (split from the old single llm_target):
  // generation seeds the world, execution drives the run engine. Null/absent =
  // inherit the model's configured target.
  llm_generation_target?: LLMTarget | null
  llm_execution_target?: LLMTarget | null
  agents?: SimulationAgent[]
  environment?: SimulationEnvironment | null
  widgets?: WidgetConfig[]

  status: SimulationStatus
  status_text?: string
  status_history?: StatusChange[]

  generation_runs?: GenerationRun[]
  token_usage?: SimulationTokenUsage | null

  // ExecutionRuns is the audit trail of every engine run, oldest first.
  execution_runs?: ExecutionRun[]
  // Statistics is the final computed statistics of the most recent run; null
  // until a run completes.
  statistics?: SimulationStatistics | null

  current_step: number

  created_at: string
  created_by: string
  updated_at: string
  updated_by: string
  started_at?: string
  finished_at?: string

  model_snapshot?: ModelSnapshot | null

  // Validation carries the canonical start_ready flag and any issues found.
  validation: SimulationValidationResult
}

// --- list / summary (mirror SimulationSummaryDTO / ListSimulationsResponse) ---

export interface SimulationSummary {
  id: string
  name: string
  description?: string
  model_id: string
  model_name?: string
  status: SimulationStatus
  steps: number
  agent_count: number
  total_tokens: number
  created_at: string
  updated_at: string
  created_by: string
}

export interface ListSimulationsResponse {
  items: SimulationSummary[]
  total: number
  has_more: boolean
}

// --- requests (mirror CreateSimulationRequest / UpdateSimulationRequest / GenerateRequest / DuplicateRequest) ---

export interface CreateSimulationRequest {
  model_id: string
  name?: string
  description?: string
  steps?: number
  seed?: number
  llm_generation_target?: LLMTarget | null
  llm_execution_target?: LLMTarget | null
}

// Full manual replace of all editable fields.
export interface UpdateSimulationRequest {
  name: string
  description: string
  steps: number
  seed: number
  llm_generation_target?: LLMTarget | null
  llm_execution_target?: LLMTarget | null
  agents: SimulationAgent[]
  environment: SimulationEnvironment | null
  widgets: WidgetConfig[]
}

export interface GenerateRequest {
  scope: GenerationScope
  llm_generation_target?: LLMTarget | null
}

export interface DuplicateRequest {
  name?: string
}

// --- SSE payloads (mirror modules/simulations/events.go payloads) ---

export interface GenerationProgressPayload {
  run: number
  stage: string
  detail: string
}

export interface GenerationCompletedPayload {
  run: number
  warnings: number
  requests: number
}

export interface GenerationFailedPayload {
  run: number
  error: string
}

export interface SimulationDeletedPayload {
  id: string
}

// Run lifecycle over the SSE hub (mirror engine.go RunProgressPayload /
// RunCompletionPayload; events "run.progress" / "run.completed"). The WS carries
// high-frequency agent frames; these keep the step counter + status authoritative
// even if the WS never connected (e.g. a very fast run).
export interface RunProgressPayload {
  run: number
  step: number
  total_steps: number
  status: SimulationStatus
}

export interface RunCompletionPayload {
  run: number
  status: SimulationStatus
  steps_completed: number
  error?: string
}

// ---------------------------------------------------------------------------
// RUN / ENGINE phase DTOs (mirror modules/simulations/{run_dtos,stream}.go).
// All keys snake_case to match the JSON wire format exactly.
// ---------------------------------------------------------------------------

// --- run request / control (mirror RunRequest / ControlRequest) ---

// RunRequest is the body of POST /:id/run. llm_execution_target, when present,
// overrides the simulation's persisted execution target for this run; absent =
// use the persisted/model-configured target.
export interface RunRequest {
  llm_execution_target?: LLMTarget | null
}

export type ControlAction = 'pause' | 'resume' | 'stop' | 'set_speed'

// ControlRequest is the body of POST /:id/control. speed_ms, when present, sets
// the inter-step delay in milliseconds (0 = fastest).
export interface ControlRequest {
  action: ControlAction
  speed_ms?: number
}

// --- stream ticket (mirror StreamTicketResponse) ---

// StreamTicketResponse is the body of POST /:id/stream-ticket: a single-use,
// short-lived opaque token plus its expiry, used to authenticate the WS upgrade.
export interface StreamTicketResponse {
  ticket: string
  expires_at: string
}

// --- steps (mirror AgentStepDTO / StepDTO / StepsPageResponse) ---

// AgentStepDTO is one agent's state within a persisted step frame (replay).
// position is omitted for non-spatial models.
export interface AgentStepDTO {
  id: number
  // agent_uid is the archetype's model-definition UID (matches the live WS
  // AgentFrame.agent_uid) so archetype-scoped widgets resolve in replay.
  // Absent on steps persisted before 2026-07-10.
  agent_uid?: string
  position?: PointXY
  properties?: Record<string, unknown>
  state?: string
  // decision is the compact LLM decision summary applied on this step (set
  // only on decision steps). Feeds the text_log llm_decisions kind in replay.
  decision?: string
}

// ShapeStepDTO is one shape's state within a persisted step frame (replay).
// Every recordable shape is present each step with its full property snapshot.
export interface ShapeStepDTO {
  id: number
  properties?: Record<string, unknown>
}

// StepDTO is the full state of one simulation step (replay/scrub history).
export interface StepDTO {
  step: number
  agents: AgentStepDTO[]
  shapes?: ShapeStepDTO[]
  global_properties?: Record<string, unknown>
  statistics?: SimulationStatistics
}

// StepsPageResponse is the paged response of GET /:id/steps.
export interface StepsPageResponse {
  items: StepDTO[]
  has_more: boolean
}

// --- WebSocket stream envelope (mirror stream.go FrameType / AgentFrame / StreamMessage) ---

// StreamFrameType is the discriminator of a live WS stream frame.
export type StreamFrameType =
  | 'step'
  | 'keyframe'
  | 'status'
  | 'progress'
  | 'error'
  | 'completion'

// AgentFrame is one entity's slice of a live stream frame. position/properties/
// state are omitted when absent so step (delta) frames stay compact. For a
// keyframe every live agent is present with its full state.
export interface AgentFrame {
  id: number
  // agent_uid is the archetype's model-definition UID so the dashboard can
  // resolve archetype-scoped widgets (population/state_distribution/
  // agent_table/agent_uids filters) from live frames. Absent from cores
  // older than 2026-07-10.
  agent_uid?: string
  position?: PointXY
  properties?: Record<string, unknown>
  state?: string
  // decision is the compact LLM decision summary applied to this agent THIS
  // step (only set on decision steps). Feeds text_log llm_decisions live.
  decision?: string
}

// ShapeFrame is one environment shape's slice of a live stream frame. Step and
// keyframe frames both carry the full property snapshot of every recordable
// shape, attached to the frame's first chunk only.
export interface ShapeFrame {
  id: number
  properties?: Record<string, unknown>
}

// StreamMessage is the typed JSON envelope sent over the run WebSocket. A single
// step may be split into several messages (chunk_index / total_chunks) when it
// carries more agents than the batch size; global_properties and shapes ride the
// first chunk only.
export interface StreamMessage {
  type: StreamFrameType
  step?: number
  chunk_index?: number
  total_chunks?: number
  agents?: AgentFrame[]
  shapes?: ShapeFrame[]
  global_properties?: Record<string, unknown>
  statistics?: SimulationStatistics
  // token_usage is the cumulative run-so-far breakdown (same shape as the
  // persisted SimulationTokenUsage); step_tokens is this step's delta. Both ride
  // a step/keyframe frame's first chunk. Mirrors stream.go StreamMessage.
  token_usage?: SimulationTokenUsage
  step_tokens?: StepTokenDelta
  status?: string
  error?: string
  timestamp: number
}

// StepTokenDelta is the token spend contributed by a SINGLE step (mirror
// stream.go StreamTokenDelta). reasoning_tokens is the reasoning portion of
// completion_tokens (a subset), not a summand.
export interface StepTokenDelta {
  prompt_tokens: number
  completion_tokens: number
  reasoning_tokens: number
  total_tokens: number
  calls: number
}
