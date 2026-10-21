<template>
  <div class="h-full flex flex-col min-h-0">
    <div v-if="loading && !store.simulation" class="p-6 text-sm text-zinc-500">Loading…</div>

    <div
      v-else-if="!store.simulation"
      class="h-full flex flex-col items-center justify-center text-zinc-500 px-6 text-center"
    >
      <div class="w-16 h-16 bg-zinc-100 rounded-2xl flex items-center justify-center mb-4">
        <i class="pi pi-play-circle text-3xl text-zinc-400" />
      </div>
      <p class="text-lg font-medium text-zinc-700 mb-1">Simulation not found</p>
      <p class="text-sm text-zinc-500 mb-4">This simulation may have been deleted.</p>
      <Button label="Back to simulations" icon="pi pi-arrow-left" size="small" @click="router.replace('/simulations')" />
    </div>

    <template v-else>
      <!-- Top bar: back + name + connection + controls -->
      <div class="border-b border-zinc-200 bg-white px-6 h-14 shrink-0 flex items-center justify-between gap-4">
        <div class="min-w-0 flex items-center gap-2">
          <router-link
            :to="`/simulations/${id}`"
            class="text-zinc-400 hover:text-zinc-700 shrink-0"
            title="Back to editor"
          >
            <i class="pi pi-arrow-left text-sm" />
          </router-link>
          <span class="font-semibold text-sm text-zinc-800 truncate">
            {{ store.simulation.name || 'Untitled simulation' }}
          </span>
          <span
            class="inline-flex items-center gap-1 text-[11px]"
            :class="socket.connected.value ? 'text-emerald-600' : 'text-zinc-400'"
            :title="`Stream: ${socket.connectionStatus.value}`"
          >
            <span
              class="w-1.5 h-1.5 rounded-full"
              :class="socket.connected.value ? 'bg-emerald-500' : 'bg-zinc-300'"
            />
            {{ socket.connected.value ? 'live' : socket.connectionStatus.value }}
          </span>
        </div>

        <div class="flex items-center gap-2">
          <!-- Controls live during running/paused -->
          <RunControls
            v-if="store.active"
            :runStatus="store.runStatus"
            :currentStep="store.currentStep"
            :totalSteps="store.simulation.steps"
            :speedMs="store.speedMs"
            :tokenUsage="store.effectiveTokenUsage"
            :stepTokens="store.stepTokens"
            :elapsedLabel="elapsedLabel"
            @control="onControl"
            @speed="onSpeed"
          />
          <span class="inline-flex" v-tooltip.bottom="analysisHint">
            <Button
              label="Analysis"
              icon="pi pi-chart-bar"
              size="small"
              severity="secondary"
              :disabled="!['finished', 'paused'].includes(store.runStatus ?? '')"
              @click="router.push(`/simulations/${id}/analysis`)"
            />
          </span>
        </div>
      </div>

      <!-- READY: an optional execution-model override + a Start button -->
      <div
        v-if="isReady"
        class="flex-1 min-h-0 flex flex-col items-center justify-center text-center px-6"
      >
        <div class="w-16 h-16 bg-violet-50 rounded-2xl flex items-center justify-center mb-4">
          <i class="pi pi-play text-3xl text-violet-400" />
        </div>
        <p class="text-lg font-medium text-zinc-700 mb-1">Ready to run</p>
        <p class="text-sm text-zinc-500 mb-4">
          {{ store.simulation.steps }} step(s) · {{ agentCount }} agent(s)
        </p>

        <div class="w-full max-w-md mb-4 text-left">
          <label class="text-xs font-medium text-zinc-600 mb-1 block">
            Execution model override (optional)
          </label>
          <LlmExecutionConfig v-model="executionTarget" capability="simulation_execution" />
        </div>

        <Button label="Start run" icon="pi pi-play" :loading="starting" @click="start" />
      </div>

      <!-- RUNNING / PAUSED (live) or FINISHED / STOPPED / FAILED (replay):
           the SAME canvas + dashboard. Live frames drive the store during a run;
           after it finishes the dashboard fetches persisted history and a scrub
           control re-drives the store frame — so the finished run shows the
           identical widgets (Task 25). -->
      <div v-else-if="store.active || store.finished" class="flex-1 min-h-0 flex flex-col">
        <!-- Terminal banner (replay mode only) -->
        <div
          v-if="store.finished"
          class="border-b border-zinc-200 bg-zinc-50 px-6 py-1.5 flex items-center gap-2 text-xs text-zinc-500"
        >
          <i :class="terminalIcon" class="text-sm" />
          <span class="font-medium text-zinc-700">{{ terminalLabel }}</span>
          <span>· {{ store.currentStep }} step(s) completed</span>
          <span v-if="hasElapsed" data-testid="run-elapsed-total">· ran {{ elapsedLabel }}</span>
          <span v-if="store.simulation.status_text" class="truncate text-zinc-400">· {{ store.simulation.status_text }}</span>
        </div>

        <!-- Collapsible LLM token-usage panel: live cumulative breakdown from the
             WS during a run; the persisted total once terminal. Sits outside the
             widget grid so it never disturbs the user-arranged dashboard. -->
        <div class="border-b border-zinc-200 bg-white shrink-0">
          <button
            type="button"
            class="w-full flex items-center gap-2 px-6 py-1.5 text-xs text-zinc-600 hover:bg-zinc-50"
            data-testid="run-token-panel-toggle"
            @click="tokensExpanded = !tokensExpanded"
          >
            <i :class="tokensExpanded ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" class="text-[10px]" />
            <i class="pi pi-bolt text-amber-500" />
            <span class="font-medium">Token usage</span>
            <span class="tabular-nums text-zinc-500" data-testid="run-token-panel-total">
              {{ (store.effectiveTokenUsage?.total?.total_tokens ?? 0).toLocaleString() }} tok
            </span>
            <span
              v-if="(store.effectiveTokenUsage?.total?.reasoning_tokens ?? 0) > 0"
              class="text-zinc-400"
            >
              · {{ (store.effectiveTokenUsage?.total?.reasoning_tokens ?? 0).toLocaleString() }} reasoning
            </span>
          </button>
          <div
            v-show="tokensExpanded"
            class="px-6 pb-3 max-h-[40vh] overflow-auto"
            data-testid="run-token-panel"
          >
            <TokenUsagePanel :usage="store.effectiveTokenUsage" />
          </div>
        </div>

        <!-- Unified run screen: RunDashboard hosts the fixed world pane (spatial)
             + the widgets in their saved positions. The former Splitpanes layout
             is replaced by the user-arranged DashboardLayoutGrid inside. -->
        <div class="flex-1 min-h-0">
          <RunDashboard
            :id="id"
            :widgets="store.simulation.widgets ?? []"
            :agentFrame="store.agentFrame"
            :globalProperties="store.globalProperties"
            :environment="store.simulation.environment ?? null"
            :snapshot="store.simulation.model_snapshot ?? null"
            :statistics="store.statistics"
            :currentStep="store.currentStep"
            :replay="store.finished"
            :catchUp="catchUp"
            :simulationAgents="store.simulation.agents"
            :stepDelayMs="store.finished ? 120 : store.speedMs"
            :runStatus="store.runStatus"
          />
        </div>
      </div>

      <!-- Fallback (unexpected status) -->
      <div v-else class="flex-1 min-h-0 flex flex-col items-center justify-center text-center px-6 text-zinc-500">
        <p class="text-sm">Nothing to display for status “{{ store.runStatus }}”.</p>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// Live run view shell (Task 24). On mount it loads the simulation (store.load)
// and opens the run WebSocket (useSimulationRunSocket.connect), routing frames
// into the run store. The body switches on the run status:
//   • ready                    → a Start button (store.startRun)
//   • queued/running/paused    → live canvas + dashboard + top-bar controls
//   • finished/stopped/failed  → replay: same canvas + dashboard, driven from
//                                persisted step history with a scrub control
//                                (the dashboard owns the history fetch + scrub).
// On unmount it disconnects the socket and clears run state.
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Tooltip from 'primevue/tooltip'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import RunControls from '@/modules/simulation/components/RunControls.vue'
import RunDashboard from '@/modules/simulation/components/RunDashboard.vue'
import TokenUsagePanel from '@/modules/simulation/components/TokenUsagePanel.vue'
import { useRunStore } from '@/modules/simulation/stores/runStore'
import { useRunElapsed } from '@/modules/simulation/composables/useRunElapsed'
import { useSimulationRunSocket } from '@/modules/simulation/composables/useSimulationRunSocket'
import { useSimulationEvents } from '@/modules/simulation/composables/useSimulationEvents'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import type { ControlAction, ExecutionLlmTarget } from '@/shared/types'

const props = defineProps<{ id: string }>()

const vTooltip = Tooltip

const router = useRouter()
const store = useRunStore()
const { handleError } = useErrorHandler()

// Active run time (live ticker during run, frozen on pause, authoritative total
// on finish). Fed to the RunControls chip and the terminal banner.
const { elapsedLabel, hasElapsed } = useRunElapsed(store)

const loading = ref(true)
const starting = ref(false)

// Whether the collapsible run-screen token-usage panel is expanded (collapsed by
// default so it doesn't crowd the dashboard).
const tokensExpanded = ref(false)

// Catch-up: set once when the view OPENS an already-active (running/paused) run
// we navigated into mid-flight — the dashboard then rebuilds the widgets'
// series/logs from persisted history 0..current_step (the canvas is repopulated
// by the WS join keyframe). NOT set for a run started here via start(), which
// streams cleanly from step 0. Latched (never cleared) so it triggers the
// dashboard's one-shot catch-up exactly once.
const catchUp = ref(false)

// Hint shown on hover over the disabled "Analysis" nav button (empty when ready).
const analysisHint = computed(() =>
  ['finished', 'paused'].includes(store.runStatus ?? '')
    ? ''
    : 'Analysis is available only when the simulation has finished or is paused.'
)

// Execution-model override for the next run (run-start picker). Pre-filled from
// the simulation's persisted llm_execution_target; sent as the run body.
const executionTarget = ref<ExecutionLlmTarget | null>(null)
watch(
  () => store.simulation?.llm_execution_target,
  (t) => {
    executionTarget.value = t ?? null
  },
  { immediate: true }
)

// The run socket feeds the store. On reconnect the store refetches the latest
// persisted state (so a missed terminal status / statistics is recovered).
const socket = useSimulationRunSocket({
  onFrame: (msg) => store.applyFrame(msg),
  onStatus: (status, completed) => {
    store.applyStatus(status)
    if (completed) {
      // Terminal status: the server closes the stream room, so stop the socket
      // (otherwise it loops re-minting tickets that now 409 — the run is no
      // longer streamable), then refresh for the final statistics + history.
      socket.disconnect()
      void store.refresh()
    }
  },
  onError: (message) => handleError(new Error(message), 'Run stream'),
  onReconnect: () => {
    void store.refresh()
  },
})

// SSE is the always-on, authoritative run-lifecycle channel (step counter +
// status + completion), independent of the WS. The WS adds smooth agent frames
// but may never connect on a very fast run; SSE guarantees the view still
// advances and flips to replay on completion (spec §10.5).
const events = useSimulationEvents()

const isReady = computed(() => store.runStatus === 'ready')
const agentCount = computed(() => store.simulation?.agents?.length ?? 0)

const terminalLabel = computed(() => {
  switch (store.runStatus) {
    case 'finished':
      return 'Run finished'
    case 'stopped':
      return 'Run stopped'
    case 'failed':
      return 'Run failed'
    default:
      return 'Run ended'
  }
})

const terminalIcon = computed(() => {
  switch (store.runStatus) {
    case 'finished':
      return 'pi pi-check-circle'
    case 'failed':
      return 'pi pi-times-circle'
    default:
      return 'pi pi-stop-circle'
  }
})

async function start() {
  if (starting.value) return
  starting.value = true
  try {
    await store.startRun(props.id, executionTarget.value)
    // The run is now active (Start returns status=running) — open the live stream.
    socket.connect(props.id)
  } catch (err) {
    handleError(err, 'Start run')
  } finally {
    starting.value = false
  }
}

async function onControl(action: ControlAction) {
  try {
    await store.control(action)
  } catch (err) {
    handleError(err, 'Run control')
  }
}

async function onSpeed(ms: number) {
  try {
    await store.setSpeed(ms)
  } catch (err) {
    handleError(err, 'Run speed')
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await store.load(props.id)
  } catch (err) {
    handleError(err, 'Open simulation')
  } finally {
    loading.value = false
  }
  // Only open the live stream when there's an active run room to join. A `ready`
  // sim has no room yet (the ticket endpoint 409s until running) — we connect on
  // Start instead; a finished/stopped/failed sim replays from persisted history
  // (no WS). An already-running sim (navigated into mid-run) connects here.
  if (store.active) {
    // Rebuild the widgets from history for this in-flight run (canvas arrives via
    // the WS join keyframe). Latched: the dashboard runs it once.
    catchUp.value = true
    socket.connect(props.id)
  }

  // Subscribe to the run-lifecycle SSE for authoritative step/status/completion.
  events.subscribe(props.id, {
    onRunProgress: (p) => {
      store.applyProgress(p.step, p.status)
      // If the run is active but the live WS isn't up (Start-time connect lost the
      // race on a fast run, or it dropped), (re)open it for smooth agent frames.
      if (store.active && socket.connectionStatus.value === 'disconnected') {
        socket.connect(props.id)
      }
    },
    onRunCompleted: (p) => {
      store.applyStatus(p.status)
      // Terminal: stop the WS (its room is gone) and refresh for final stats +
      // flip the view into replay mode.
      socket.disconnect()
      void store.refresh()
    },
  })
})

onUnmounted(() => {
  socket.disconnect()
  events.unsubscribe()
  store.close()
})
</script>
