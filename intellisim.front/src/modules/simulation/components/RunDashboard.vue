<template>
  <div class="h-full flex flex-col min-h-0">
    <!-- Replay scrub / timeline control (finished runs only) -->
    <div
      v-if="replay && totalReplaySteps > 0"
      class="border-b border-zinc-200 bg-white px-4 py-2 flex items-center gap-3 shrink-0"
    >
      <button
        type="button"
        class="w-7 h-7 flex items-center justify-center rounded text-zinc-600 hover:bg-zinc-100 disabled:opacity-40"
        :title="playing ? 'Pause replay' : 'Play replay'"
        :disabled="loadingHistory"
        @click="togglePlay"
      >
        <i :class="playing ? 'pi pi-pause' : 'pi pi-play'" class="text-xs" />
      </button>
      <input
        type="range"
        class="flex-1 accent-violet-500"
        :min="minReplayStep"
        :max="maxReplayStep"
        :value="scrubStep"
        :disabled="loadingHistory"
        @input="onScrub(($event.target as HTMLInputElement).valueAsNumber)"
      />
      <span class="text-[11px] text-zinc-500 tabular-nums shrink-0 w-20 text-right">
        step {{ scrubStep }} / {{ maxReplayStep }}
      </span>
    </div>

    <div v-if="loadingHistory" class="px-4 py-1 text-[11px] text-zinc-400 shrink-0">Loading history…</div>

    <!-- Unified run screen: the world/environment occupies a fixed pane on the
         left (spatial) with the widgets in a free-placement grid on the right
         (or the full width when non-spatial), honoring each widget's saved
         position. Read-only (static) — arranging happens in the editor. -->
    <div class="flex-1 min-h-0">
      <DashboardLayoutGrid :widgets="widgets" :spatial="showWorld" :editable="false">
        <template #world>
          <div class="h-full min-h-0 flex flex-col">
            <RunShapesPanel :shapes="environment?.shapes ?? []" :live="store.shapeFrame" />
            <div class="flex-1 min-h-0">
              <RunWorldCanvas
                :simulationId="id"
                :agentFrame="agentFrame"
                :environment="environment ?? null"
                :snapshot="snapshot ?? null"
                :globalProperties="globalProperties"
                :stepDelayMs="stepDelayMs ?? 120"
                :simulationAgents="simulationAgents"
                :runStatus="runStatus ?? null"
                :replay="replay"
                :maxMemoryStep="replay ? scrubStep : undefined"
              />
            </div>
          </div>
        </template>
        <template #widget="{ widget }">
          <RunWidgetTile
            v-if="widget"
            :widget="widget"
            :ctx="liveCtx"
            :lineSeries="seriesForWidget(widget)"
            :logEntries="logsForWidget(widget)"
          />
        </template>
        <template #empty>This simulation has no widgets.</template>
      </DashboardLayoutGrid>
    </div>
  </div>
</template>

<script setup lang="ts">
// Run dashboard (Task 24/25): renders the simulation's typed widgets via the
// widgets factory (WidgetPreview), fed from the run store's live frame.
//
// • monitor/pie/histogram/bar/agent_table recompute instantly from a synthesised
//   live PreviewContext { agents, environment, snapshot }.
// • line_chart / text_log get the run store's ACCUMULATED series / log entries
//   (the net-new live-data path, useRunLiveData) — appended once per step.
//
// LIVE: a watcher on the store's currentStep folds each new step into the
// series/logs as frames stream in.
//
// REPLAY (finished runs): the dashboard fetches the persisted step history
// (listSteps, paged), exposes a scrub/timeline (+ play) control, and on any
// scrub re-drives the store's frame (setReplayFrame) + re-accumulates the
// series/logs up to the scrub step — so the finished run shows the IDENTICAL
// widgets it showed live ("same results after finish").
import { computed, onUnmounted, ref, watch } from 'vue'
import DashboardLayoutGrid from '@/shared/components/widgets/DashboardLayoutGrid.vue'
import RunWorldCanvas from '@/modules/simulation/components/RunWorldCanvas.vue'
import RunShapesPanel from '@/modules/simulation/components/RunShapesPanel.vue'
import RunWidgetTile from '@/modules/simulation/components/RunWidgetTile.vue'
import type { PreviewContext } from '@/shared/components/widgets/widgetData'
import type { LineSeriesPoint } from '@/shared/components/widgets/LineChartWidget.vue'
import type { TextLogEntry } from '@/shared/components/widgets/TextLogWidget.vue'
import { useRunStore, type AgentFrameState, type ShapeFrameState } from '@/modules/simulation/stores/runStore'
import { useRunLiveData } from '@/modules/simulation/composables/useRunLiveData'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'
import type {
  ModelSnapshot,
  SimulationAgent,
  SimulationEnvironment,
  SimulationStatistics,
  SimulationStatus,
  StepDTO,
  WidgetConfig,
} from '@/shared/types'

const props = defineProps<{
  id: string
  widgets: WidgetConfig[]
  // The live per-agent state, keyed by runtime instance id (runStore.agentFrame).
  agentFrame: Map<number, AgentFrameState>
  globalProperties?: Record<string, unknown>
  environment?: SimulationEnvironment | null
  snapshot?: ModelSnapshot | null
  statistics?: SimulationStatistics | null
  // The current step the store is displaying (live counter or scrub position).
  currentStep: number
  // Replay mode: the run has finished and we drive the frame from history.
  replay: boolean
  // Catch-up mode: the view (re)opened an ACTIVE (running/paused) run, so the
  // widgets' series/logs must be rebuilt from persisted history up to the current
  // step before continuing live accumulation. Unlike replay this adds NO scrub
  // bar and does NOT drive the canvas frame (the WS join keyframe does that).
  catchUp?: boolean
  // World-canvas inputs — the run dashboard now hosts the world tile (fixed left
  // pane) alongside the widgets, so it forwards these to RunWorldCanvas.
  simulationAgents?: SimulationAgent[]
  stepDelayMs?: number
  runStatus?: SimulationStatus | null
}>()

// Show the fixed world pane only for spatial models; non-spatial models let the
// widgets fill the full area (per the layout design). Mirrors RunWorldCanvas's own
// `isSpatial = snapshot?.is_spatial ?? false` so the pane and the canvas agree even
// when the snapshot is momentarily absent.
const showWorld = computed(() => props.snapshot?.is_spatial ?? false)

const store = useRunStore()
const api = useSimulationApi()
const live = useRunLiveData(store)

// ---------------------------------------------------------------------------
// live PreviewContext (instant-recompute widgets)
// ---------------------------------------------------------------------------
// Map the runtime frame (id→state) into the SimulationAgent[] shape the widgets
// compute consumes. The frame carries the archetype definition UID (agent_uid,
// 2026-07-10) so archetype-scoped widgets resolve live; the instance id remains
// the display name. Frames from an older core lack agent_uid — fall back to the
// instance id (the pre-fix behavior: archetype widgets degrade, nothing breaks).
const frameAgents = computed<SimulationAgent[]>(() =>
  Array.from(props.agentFrame.values()).map((a: AgentFrameState) => ({
    id: a.id,
    agent_uid: a.agent_uid ?? String(a.id),
    name: String(a.id),
    position: a.position,
    properties: a.properties,
    state: a.state,
    decision: a.decision,
  }))
)

const liveEnvironment = computed<SimulationEnvironment | null>(() => {
  const base = props.environment ?? null
  const globals = props.globalProperties ?? {}
  if (!base) {
    if (!Object.keys(globals).length) return null
    return { width: 0, height: 0, torus: false, global_properties: globals, shapes: [] }
  }
  return { ...base, global_properties: { ...(base.global_properties ?? {}), ...globals } }
})

const liveCtx = computed<PreviewContext>(() => ({
  agents: frameAgents.value,
  environment: liveEnvironment.value,
  snapshot: props.snapshot ?? null,
  statistics: props.statistics ?? null,
}))

const widgets = computed(() => props.widgets ?? [])

// ---------------------------------------------------------------------------
// accumulated line_chart series / text_log entries (per widget)
// ---------------------------------------------------------------------------
// Group the store's flat series map (`${widgetId}::${label}`) into a per-widget
// { label → points } record the LineChartWidget binds to. Recomputes when the
// store's series ref is reassigned (it is, on every pushSeriesPoint).
function seriesForWidget(w: WidgetConfig): Record<string, LineSeriesPoint[]> | undefined {
  if (w.type !== 'line_chart' || !w.series) return undefined
  const out: Record<string, LineSeriesPoint[]> = {}
  for (const s of w.series) out[s.label] = store.seriesFor(w.id, s.label)
  return out
}

function logsForWidget(w: WidgetConfig): TextLogEntry[] | undefined {
  if (w.type !== 'text_log') return undefined
  return store.logsFor(w.id)
}

// Catch-up in progress: gate live accumulation until the history rebuild has
// folded steps 0..startStep, so the store's monotonic per-step guard
// (beginAccumulateStep) then transparently dedups any live frame at a step we
// already accumulated from history.
const catchingUp = ref(false)

// ---------------------------------------------------------------------------
// LIVE accumulation — fold each new step into series/logs as frames arrive.
// ---------------------------------------------------------------------------
watch(
  () => props.currentStep,
  (step) => {
    if (props.replay) return // replay drives accumulation itself
    if (catchingUp.value) return // history rebuild owns accumulation meanwhile
    live.accumulate(widgets.value, liveCtx.value, props.statistics ?? null, step)
  },
  { immediate: true }
)

// ---------------------------------------------------------------------------
// REPLAY — fetch persisted history + scrub/timeline control.
// ---------------------------------------------------------------------------
const history = ref<StepDTO[]>([])
const loadingHistory = ref(false)
const scrubStep = ref(0)
const playing = ref(false)
let playTimer: ReturnType<typeof setInterval> | null = null

const totalReplaySteps = computed(() => history.value.length)
const minReplayStep = computed(() => history.value[0]?.step ?? 0)
const maxReplayStep = computed(() => history.value[history.value.length - 1]?.step ?? 0)

// Map a replay StepDTO's agents into the store frame shape (state lifted from the
// reserved "state" key when the DTO omits the typed field).
function dtoToFrame(s: StepDTO): AgentFrameState[] {
  return (s.agents ?? []).map((a) => ({
    id: a.id,
    agent_uid: a.agent_uid,
    position: a.position,
    properties: a.properties ?? {},
    state: a.state || (typeof a.properties?.state === 'string' ? (a.properties.state as string) : ''),
    decision: a.decision,
  }))
}

// Build the PreviewContext for a replay step (for re-accumulation), independent
// of the store frame (so we can fold series/logs without racing the watcher).
function ctxForStep(s: StepDTO): PreviewContext {
  const agents: SimulationAgent[] = dtoToFrame(s).map((a) => ({
    id: a.id,
    agent_uid: a.agent_uid ?? String(a.id),
    name: String(a.id),
    position: a.position,
    properties: a.properties,
    state: a.state,
    decision: a.decision,
  }))
  const base = props.environment ?? null
  const globals = s.global_properties ?? {}
  const environment: SimulationEnvironment | null = base
    ? { ...base, global_properties: { ...(base.global_properties ?? {}), ...globals } }
    : Object.keys(globals).length
      ? { width: 0, height: 0, torus: false, global_properties: globals, shapes: [] }
      : null
  return { agents, environment, snapshot: props.snapshot ?? null, statistics: s.statistics ?? null }
}

// Re-accumulate series/logs from the FIRST history step up to (and including)
// the index `upTo`, then drive the store frame to that step. Always rebuilds
// from scratch so scrubbing backward and forward both land on identical data
// (deterministic — proves "same results after finish"). Shape rows are FULL
// per-step snapshots of every recordable shape; folding them forward is a
// same-key overwrite, and the accumulated result replaces the store's shape map.
function rebuildTo(upTo: number): void {
  store.resetAccumulation()
  const shapeAccum = new Map<number, ShapeFrameState>()
  for (let i = 0; i <= upTo && i < history.value.length; i++) {
    const s = history.value[i]!
    for (const sh of s.shapes ?? []) {
      const prev = shapeAccum.get(sh.id)
      shapeAccum.set(sh.id, {
        id: sh.id,
        properties: { ...(prev?.properties ?? {}), ...(sh.properties ?? {}) },
      })
    }
    live.accumulate(widgets.value, ctxForStep(s), s.statistics ?? null, s.step)
  }
  const target = history.value[Math.min(upTo, history.value.length - 1)]
  if (target) {
    store.setReplayFrame(
      dtoToFrame(target),
      target.global_properties ?? {},
      target.step,
      target.statistics ?? null,
      Array.from(shapeAccum.values()),
    )
    scrubStep.value = target.step
  }
}

function onScrub(stepValue: number): void {
  stopPlay()
  // Map the step number to its index in history (history is dense + ordered).
  const idx = history.value.findIndex((s) => s.step >= stepValue)
  rebuildTo(idx < 0 ? history.value.length - 1 : idx)
}

function togglePlay(): void {
  if (playing.value) stopPlay()
  else startPlay()
}

function startPlay(): void {
  if (history.value.length === 0) return
  // If at the end, restart from the beginning.
  let idx = history.value.findIndex((s) => s.step === scrubStep.value)
  if (idx < 0 || idx >= history.value.length - 1) idx = -1
  playing.value = true
  playTimer = setInterval(() => {
    idx++
    if (idx >= history.value.length) {
      stopPlay()
      return
    }
    rebuildTo(idx)
  }, 150)
}

function stopPlay(): void {
  playing.value = false
  if (playTimer) {
    clearInterval(playTimer)
    playTimer = null
  }
}

// Fetch ALL persisted steps for replay (paged), oldest→newest.
async function loadHistory(): Promise<void> {
  loadingHistory.value = true
  stopPlay()
  history.value = []
  try {
    const pageSize = 500
    let page = 1
    const all: StepDTO[] = []
    for (;;) {
      // Explicit full step range: the backend treats an omitted `to` as 0 (window
      // [0,0] → only the initial step), so we pass an open upper bound. Steps are
      // capped at 100000 by validation; 1e9 covers any run.
      const resp = await api.listSteps(props.id, { from: 0, to: 1_000_000_000, page, page_size: pageSize })
      all.push(...(resp.items ?? []))
      if (!resp.has_more || (resp.items?.length ?? 0) === 0) break
      page++
      if (page > 200) break // hard safety cap
    }
    all.sort((a, b) => a.step - b.step)
    history.value = all
    if (all.length) rebuildTo(all.length - 1) // land on the final step
  } finally {
    loadingHistory.value = false
  }
}

// Enter replay mode whenever the run finishes (or the view opens on a finished
// run); leave it when a fresh run starts (replay → false).
watch(
  () => props.replay,
  (isReplay) => {
    if (isReplay) void loadHistory()
    else {
      stopPlay()
      history.value = []
    }
  },
  { immediate: true }
)

// ---------------------------------------------------------------------------
// CATCH-UP — rebuild the widgets' series/logs from history for an ACTIVE run.
// ---------------------------------------------------------------------------
// When the view (re)opens a running/paused run, the widgets have no accumulated
// series/logs yet (they only build as live frames stream in). This folds the
// persisted history 0..startStep into the store's series/logs so the dashboard
// shows the run so far, then hands off to live accumulation for later steps.
//
// DEDUP: the store's beginAccumulateStep(step) guard is monotonic — it folds each
// step exactly once and rejects any step <= the last folded one. We (1) capture
// startStep = current_step at load START, (2) resetAccumulation() and fold every
// history row with step <= startStep in order (leaving lastAccumulatedStep at the
// highest such step), then (3) release the live gate. Any live frame that arrives
// during the load (buffered by the store as usual) is transparently deduped: a
// frame at step <= startStep is a no-op via the guard, and the FIRST live frame at
// step > startStep folds cleanly on top. To cover the (rare) case where the run
// advanced past startStep DURING the load but no further frame arrives after it,
// we fold the store's current step once on release. The canvas is driven
// independently by the WS join keyframe, so we never touch the frame or scrub.
const catchUpStartStep = ref(0)

async function catchUpHistory(): Promise<void> {
  catchingUp.value = true
  loadingHistory.value = true
  try {
    const startStep = props.currentStep
    catchUpStartStep.value = startStep
    store.resetAccumulation()

    const pageSize = 500
    let page = 1
    const rows: StepDTO[] = []
    for (;;) {
      const resp = await api.listSteps(props.id, {
        from: 0,
        to: startStep,
        page,
        page_size: pageSize,
      })
      rows.push(...(resp.items ?? []))
      if (!resp.has_more || (resp.items?.length ?? 0) === 0) break
      page++
      if (page > 200) break // hard safety cap
    }
    rows.sort((a, b) => a.step - b.step)
    for (const s of rows) {
      if (s.step > startStep) continue
      live.accumulate(widgets.value, ctxForStep(s), s.statistics ?? null, s.step)
    }
  } finally {
    loadingHistory.value = false
    catchingUp.value = false
    // Fold the current live step once so a run that advanced during the load
    // doesn't leave a one-step gap before the next frame arrives (guard-deduped
    // if it's <= what history already covered).
    if (!props.replay) {
      live.accumulate(widgets.value, liveCtx.value, props.statistics ?? null, props.currentStep)
    }
  }
}

// Run the catch-up once when the view opens an active run (catchUp true and not
// in replay). Replay owns its own history path, so it takes precedence.
watch(
  () => props.catchUp,
  (isCatchUp) => {
    if (isCatchUp && !props.replay) void catchUpHistory()
  },
  { immediate: true }
)

onUnmounted(stopPlay)
</script>
