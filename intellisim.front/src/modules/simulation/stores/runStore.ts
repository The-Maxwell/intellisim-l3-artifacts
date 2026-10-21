import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type {
  ControlAction,
  ControlRequest,
  LLMTarget,
  Simulation,
  SimulationStatistics,
  SimulationStatus,
  SimulationTokenUsage,
  StepTokenDelta,
  StreamMessage,
} from '@/shared/types'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'

// Run state for the live/replay run view. Kept in a dedicated store (not folded
// into simulationStore, which already owns the editor's open-simulation +
// generation-SSE lifecycle) so the high-frequency frame/series/log accumulation
// stays isolated from the editor's dirty/validation bookkeeping. The run socket
// composable (useSimulationRunSocket) feeds applyFrame; the run view drives
// startRun/control and supplies toast handlers (toast needs setup context).

// AgentFrameState is one agent's accumulated state across step deltas — the
// store folds each delta frame onto the previous value so a `step` frame
// (changed-only) and a `keyframe` (full) both leave a complete picture.
export interface AgentFrameState {
  id: number
  // agent_uid is the archetype definition UID; identity, preserved across
  // step-delta merges. Absent when streamed by a pre-2026-07-10 core.
  agent_uid?: string
  position?: { x: number; y: number }
  properties: Record<string, unknown>
  state: string
  // decision is a per-step EVENT, not state: REPLACED on every frame (absent →
  // cleared), never carried forward, or repeat log entries would be fabricated.
  decision?: string
}

// ShapeFrameState is one environment shape's accumulated LIVE properties across
// step deltas — mirrors AgentFrameState: a `step` frame merges changed
// properties onto the previous value, a `keyframe` replaces the whole set.
export interface ShapeFrameState {
  id: number
  properties: Record<string, unknown>
}

// SeriesPoint is one accumulated line_chart sample at a step.
export interface SeriesPoint {
  step: number
  value: number
}

// LogEntry is one accumulated text_log line.
export interface LogEntry {
  step: number
  kind: string
  message: string
}

// Bound retained history so a long run never grows series/logs unbounded.
const MAX_SERIES_POINTS = 2000
const MAX_LOG_ENTRIES = 500

// LogAccumKey identifies a derived-log baseline (per-agent state / decision) so
// the widgets live-data path can detect transitions across steps.
export interface LogBaseline {
  states: Map<number, string>
  decisions: Map<number, string>
  statistics: SimulationStatistics | null
}

export const useRunStore = defineStore('simulationRun', () => {
  const api = useSimulationApi()

  // The simulation being run (mirrors a subset of simulationStore.current; the
  // run view loads it via get() so the run store stays self-contained).
  const simulation = ref<Simulation | null>(null)
  // The simulation-wide lifecycle status (queued/running/paused/finished/...).
  const runStatus = ref<SimulationStatus | null>(null)
  const currentStep = ref(0)
  // Inter-step delay the engine honours (ms); mirrors the last control speed_ms.
  const speedMs = ref(0)
  // Per-agent latest state, keyed by runtime instance id.
  const agentFrame = ref<Map<number, AgentFrameState>>(new Map())
  // Per-shape latest live properties, keyed by shape instance id. Empty for
  // models whose shapes never stream property frames.
  const shapeFrame = ref<Map<number, ShapeFrameState>>(new Map())
  // Latest environment global properties.
  const globalProperties = ref<Record<string, unknown>>({})
  // Accumulated line_chart series points, keyed `${widgetId}::${seriesLabel}`.
  const series = ref<Map<string, SeriesPoint[]>>(new Map())
  // Accumulated text_log entries, keyed by widget id (bounded FIFO).
  const logs = ref<Map<string, LogEntry[]>>(new Map())
  // Live (during run) or final (post-run) statistics.
  const statistics = ref<SimulationStatistics | null>(null)
  // Live cumulative token-usage breakdown from the WS (running/paused). Null
  // until the first token frame; superseded by the persisted value once terminal.
  const liveTokenUsage = ref<SimulationTokenUsage | null>(null)
  // The most recent per-step token delta (tokens this step just consumed).
  const stepTokens = ref<StepTokenDelta | null>(null)
  const error = ref<string | null>(null)

  // Per-agent state/decision + statistics baseline from the LAST accumulated
  // step, so the widgets live-data path can derive state-transition / decision
  // / event-count DELTAS for text_log. Not reactive (mutated in place; only the
  // bounded logs/series refs above drive rendering).
  const logBaseline: LogBaseline = { states: new Map(), decisions: new Map(), statistics: null }
  // The last step number we accumulated series/logs for — guards against folding
  // the same step twice (e.g. a chunked step arriving as several frames).
  let lastAccumulatedStep = -1

  const running = computed(() => runStatus.value === 'running')
  const paused = computed(() => runStatus.value === 'paused')
  const active = computed(
    () => runStatus.value === 'running' || runStatus.value === 'paused' || runStatus.value === 'queued'
  )
  const finished = computed(
    () =>
      runStatus.value === 'finished' ||
      runStatus.value === 'stopped' ||
      runStatus.value === 'failed'
  )

  // The token usage to display: the live WS breakdown wins while the run is
  // active (so the chip/panel update per step without waiting on refresh); the
  // persisted simulation.token_usage wins once terminal (authoritative final).
  const effectiveTokenUsage = computed<SimulationTokenUsage | null>(() =>
    finished.value
      ? simulation.value?.token_usage ?? liveTokenUsage.value ?? null
      : liveTokenUsage.value ?? simulation.value?.token_usage ?? null
  )

  function asMessage(e: unknown, fallback: string): string {
    return e instanceof Error ? e.message : fallback
  }

  // Adopt a (re)loaded simulation: seed the run status + statistics from it.
  function adopt(sim: Simulation): void {
    simulation.value = sim
    runStatus.value = sim.status
    currentStep.value = sim.current_step ?? 0
    statistics.value = sim.statistics ?? null
  }

  // Clear all per-run accumulation (frame/series/logs/stats). Called before a
  // fresh run or a replay rebuild so stale state never bleeds across runs.
  function resetRun(): void {
    agentFrame.value = new Map()
    shapeFrame.value = new Map()
    globalProperties.value = {}
    series.value = new Map()
    logs.value = new Map()
    statistics.value = null
    liveTokenUsage.value = null
    stepTokens.value = null
    currentStep.value = 0
    error.value = null
    logBaseline.states = new Map()
    logBaseline.decisions = new Map()
    logBaseline.statistics = null
    lastAccumulatedStep = -1
  }

  // Clear only the accumulated series/logs + their baselines (NOT the live
  // agent frame). Used when entering replay mode before re-accumulating history
  // up to a scrub point, so the line/log widgets rebuild deterministically.
  function resetAccumulation(): void {
    series.value = new Map()
    logs.value = new Map()
    logBaseline.states = new Map()
    logBaseline.decisions = new Map()
    logBaseline.statistics = null
    lastAccumulatedStep = -1
  }

  // Fold one stream frame onto the accumulated state. A keyframe replaces the
  // whole agent set; a step delta merges changed agents onto their prior value.
  function applyFrame(msg: StreamMessage): void {
    if (msg.type === 'keyframe') {
      const next = new Map<number, AgentFrameState>()
      for (const a of msg.agents ?? []) {
        next.set(a.id, {
          id: a.id,
          agent_uid: a.agent_uid,
          position: a.position,
          properties: a.properties ?? {},
          state: a.state ?? '',
          decision: a.decision,
        })
      }
      agentFrame.value = next
    } else {
      // step / progress delta: merge changed agents onto their previous value.
      // agent_uid is identity (preserve across merges); decision is a per-step
      // event (replace, never carry forward — see AgentFrameState).
      const next = new Map(agentFrame.value)
      for (const a of msg.agents ?? []) {
        const prev = next.get(a.id)
        next.set(a.id, {
          id: a.id,
          agent_uid: a.agent_uid ?? prev?.agent_uid,
          position: a.position ?? prev?.position,
          properties: { ...(prev?.properties ?? {}), ...(a.properties ?? {}) },
          state: a.state || prev?.state || '',
          decision: a.decision,
        })
      }
      agentFrame.value = next
    }

    // Shapes ride the FIRST chunk only (like global_properties), so a chunked
    // keyframe must not wipe the shape map on chunks 2+. A keyframe first chunk
    // carries the full current shape set (replace); a step frame carries only
    // shapes with changed properties (merge per id).
    const firstChunk = (msg.chunk_index ?? 0) === 0
    if (msg.type === 'keyframe') {
      if (firstChunk) {
        const nextShapes = new Map<number, ShapeFrameState>()
        for (const s of msg.shapes ?? []) {
          nextShapes.set(s.id, { id: s.id, properties: s.properties ?? {} })
        }
        shapeFrame.value = nextShapes
      }
    } else if (msg.shapes?.length) {
      const nextShapes = new Map(shapeFrame.value)
      for (const s of msg.shapes) {
        const prev = nextShapes.get(s.id)
        nextShapes.set(s.id, {
          id: s.id,
          properties: { ...(prev?.properties ?? {}), ...(s.properties ?? {}) },
        })
      }
      shapeFrame.value = nextShapes
    }

    if (msg.global_properties) {
      globalProperties.value = { ...globalProperties.value, ...msg.global_properties }
    }
    if (msg.statistics) {
      statistics.value = msg.statistics
    }
    // Live token usage rides the first chunk of a step/keyframe frame. The
    // cumulative breakdown always replaces (it's authoritative each frame); the
    // delta records this step's spend. Keyframes carry a zero delta.
    if (firstChunk && msg.token_usage) {
      liveTokenUsage.value = msg.token_usage
    }
    if (firstChunk && msg.step_tokens) {
      stepTokens.value = msg.step_tokens
    }
    if (typeof msg.step === 'number') {
      currentStep.value = msg.step
    }
  }

  // Append one accumulated line_chart point (bounded FIFO per series key). The
  // widgets live-data path (Task 25) calls this once per step per series.
  function pushSeriesPoint(widgetId: string, seriesLabel: string, step: number, value: number): void {
    const key = `${widgetId}::${seriesLabel}`
    const points = series.value.get(key) ?? []
    points.push({ step, value })
    if (points.length > MAX_SERIES_POINTS) {
      points.splice(0, points.length - MAX_SERIES_POINTS)
    }
    series.value.set(key, points)
  }

  // Append one accumulated text_log entry (bounded FIFO per widget id).
  function pushLogEntry(widgetId: string, step: number, kind: string, message: string): void {
    const entries = logs.value.get(widgetId) ?? []
    entries.push({ step, kind, message })
    if (entries.length > MAX_LOG_ENTRIES) {
      entries.splice(0, entries.length - MAX_LOG_ENTRIES)
    }
    logs.value.set(widgetId, entries)
  }

  // Read the accumulated series points for a (widget, series-label) pair. Used
  // by the live-data path's LineChart binding (returns [] when none yet).
  function seriesFor(widgetId: string, seriesLabel: string): SeriesPoint[] {
    return series.value.get(`${widgetId}::${seriesLabel}`) ?? []
  }

  // Read the accumulated text_log entries for a widget id ([] when none yet).
  function logsFor(widgetId: string): LogEntry[] {
    return logs.value.get(widgetId) ?? []
  }

  // The derived-log baseline (per-agent state/decision + prior statistics) the
  // live-data path reads to compute this step's transition deltas, then updates.
  function getLogBaseline(): LogBaseline {
    return logBaseline
  }

  // Guard so a single step is folded into the series/logs exactly once even when
  // it arrives as several chunked frames. Returns true if this step is new.
  function beginAccumulateStep(step: number): boolean {
    if (step <= lastAccumulatedStep) return false
    lastAccumulatedStep = step
    return true
  }

  // Replace the live frame wholesale from a persisted replay step (StepDTO):
  // every agent's full state, the step's global properties and statistics. Unlike
  // applyFrame (which merges deltas), replay steps are full snapshots, so this
  // resets the frame each time the scrub position changes.
  function setReplayFrame(agents: AgentFrameState[], globals: Record<string, unknown>, step: number, stats: SimulationStatistics | null, shapes: ShapeFrameState[] = []): void {
    const next = new Map<number, AgentFrameState>()
    for (const a of agents) next.set(a.id, a)
    agentFrame.value = next
    // Shapes are pre-accumulated by the caller (persisted shape rows are deltas,
    // so RunDashboard folds them forward up to the scrub step) — replace the map.
    const nextShapes = new Map<number, ShapeFrameState>()
    for (const s of shapes) nextShapes.set(s.id, s)
    shapeFrame.value = nextShapes
    globalProperties.value = { ...globals }
    currentStep.value = step
    if (stats) statistics.value = stats
  }

  // Track a run lifecycle status change arriving over the WS (status/completion)
  // or SSE. Mirrored into runStatus so the controls/chips react.
  function applyStatus(status: string): void {
    if (isSimulationStatus(status)) {
      runStatus.value = status
    }
  }

  // Apply an SSE run.progress event: advance the step counter (monotonic — never
  // regress below a higher value already seen over the WS) and adopt the status.
  // This is the authoritative lifecycle channel; the WS only adds smooth agent
  // frames, so progress works even if the WS never connected (fast runs).
  function applyProgress(step: number, status: string): void {
    if (step > currentStep.value) currentStep.value = step
    applyStatus(status)
  }

  function isSimulationStatus(s: string): s is SimulationStatus {
    return (
      s === 'generating' ||
      s === 'ready' ||
      s === 'generation_failed' ||
      s === 'queued' ||
      s === 'running' ||
      s === 'paused' ||
      s === 'finished' ||
      s === 'failed' ||
      s === 'stopped'
    )
  }

  // Open/load the simulation for the run view.
  async function load(id: string): Promise<void> {
    error.value = null
    try {
      const sim = await api.get(id)
      adopt(sim)
    } catch (e) {
      error.value = asMessage(e, 'Failed to load simulation')
      throw e
    }
  }

  // Start an engine run: clears accumulation, fires POST /:id/run (202), and
  // adopts the post-start (queued/running) state. An optional execution-target
  // override is forwarded in the run body for this run only.
  async function startRun(id: string, llmExecutionTarget?: LLMTarget | null): Promise<void> {
    error.value = null
    resetRun()
    try {
      const sim = await api.runSimulation(
        id,
        llmExecutionTarget ? { llm_execution_target: llmExecutionTarget } : {}
      )
      adopt(sim)
    } catch (e) {
      error.value = asMessage(e, 'Failed to start run')
      throw e
    }
  }

  // Signal the live run's control channel (pause/resume/stop + optional speed).
  async function control(action: ControlAction, newSpeedMs?: number): Promise<void> {
    const id = simulation.value?.id
    if (!id) return
    const body: ControlRequest = { action }
    if (typeof newSpeedMs === 'number') body.speed_ms = newSpeedMs
    await api.controlSimulation(id, body)
    if (typeof newSpeedMs === 'number') speedMs.value = newSpeedMs
    // Optimistic local status hint; the authoritative status arrives via WS/SSE.
    if (action === 'pause') runStatus.value = 'paused'
    else if (action === 'resume') runStatus.value = 'running'
    else if (action === 'stop') runStatus.value = 'stopped'
  }

  // Set the run speed without changing pause/resume state (works while running
  // OR paused — unlike piggybacking on 'resume', which 409s if already running).
  async function setSpeed(newSpeedMs: number): Promise<void> {
    const id = simulation.value?.id
    if (!id) return
    await api.controlSimulation(id, { action: 'set_speed', speed_ms: newSpeedMs })
    speedMs.value = newSpeedMs
  }

  // Refetch the run simulation (e.g. after a terminal status) to adopt the final
  // persisted statistics + status.
  async function refresh(): Promise<void> {
    const id = simulation.value?.id
    if (!id) return
    try {
      const sim = await api.get(id)
      adopt(sim)
    } catch {
      /* transient — the next event/refresh recovers */
    }
  }

  // Tear down all run state (used on view unmount).
  function close(): void {
    simulation.value = null
    runStatus.value = null
    speedMs.value = 0
    resetRun()
  }

  return {
    simulation,
    runStatus,
    currentStep,
    speedMs,
    agentFrame,
    shapeFrame,
    globalProperties,
    series,
    logs,
    statistics,
    liveTokenUsage,
    stepTokens,
    error,
    running,
    paused,
    active,
    finished,
    effectiveTokenUsage,
    adopt,
    resetRun,
    resetAccumulation,
    applyFrame,
    setReplayFrame,
    applyStatus,
    applyProgress,
    pushSeriesPoint,
    pushLogEntry,
    seriesFor,
    logsFor,
    getLogBaseline,
    beginAccumulateStep,
    load,
    startRun,
    control,
    setSpeed,
    refresh,
    close,
  }
})
