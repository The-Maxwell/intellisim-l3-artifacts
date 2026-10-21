<template>
  <!-- Agent inspector panel — shown when an agent sprite is clicked in RunWorldCanvas.
       Displays id, archetype (agent_uid), current state, live properties (from the
       live agentFrame — updated per step), and a memory list.

       Memory + state are refreshed from the backend agent-detail endpoint
       (GET /simulations/:id/agents/:agentId): while a run is active the endpoint
       returns the LIVE runtime snapshot (fresh memory), else the persisted record.
       A manual refresh button re-fetches on demand and a light ~3s poll runs while
       the run is `running` (stopped on unselect/unmount/pause). Position/properties
       continue to come from the live stream frame (props.agent) as before — the
       fetch primarily keeps MEMORY (and state) current. -->
  <div
    v-if="agent"
    class="absolute bottom-4 left-4 w-72 max-h-[70%] bg-white/95 backdrop-blur rounded-xl shadow-lg border border-zinc-200 flex flex-col overflow-hidden text-xs z-20"
  >
    <!-- Header -->
    <div class="flex items-center justify-between px-3 py-2 border-b border-zinc-100 bg-zinc-50/80">
      <div class="flex items-center gap-2 min-w-0">
        <span
          class="w-2.5 h-2.5 rounded-full flex-shrink-0"
          :style="{ backgroundColor: archetypeColor }"
        />
        <span class="font-semibold text-zinc-800 truncate">{{ displayUid || 'agent' }} #{{ agent.id }}</span>
      </div>
      <div class="flex items-center gap-1 ml-2">
        <button
          type="button"
          class="w-5 h-5 flex items-center justify-center text-zinc-400 hover:text-zinc-700 rounded disabled:opacity-40"
          title="Refresh memory"
          :disabled="loading"
          @click="refresh"
        >
          <i class="pi pi-refresh text-[10px]" :class="{ 'pi-spin': loading }" />
        </button>
        <button
          type="button"
          class="w-5 h-5 flex items-center justify-center text-zinc-400 hover:text-zinc-700 rounded"
          title="Close inspector"
          @click="$emit('close')"
        >
          <i class="pi pi-times text-[10px]" />
        </button>
      </div>
    </div>

    <div class="overflow-y-auto flex-1 min-h-0">
      <!-- Identity -->
      <section class="px-3 py-2 border-b border-zinc-100">
        <div class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5">
          <span class="text-zinc-400">id</span>
          <span class="text-zinc-700 font-mono">{{ agent.id }}</span>
          <span class="text-zinc-400">archetype</span>
          <span class="text-zinc-700">{{ displayUid || '—' }}</span>
          <span class="text-zinc-400">state</span>
          <span class="font-medium" :class="stateClass">{{ displayState || '—' }}</span>
          <template v-if="agent.position">
            <span class="text-zinc-400">position</span>
            <span class="text-zinc-700 font-mono">
              ({{ agent.position.x }}, {{ agent.position.y }})
            </span>
          </template>
        </div>
      </section>

      <!-- Live properties (from agentFrame — updated each step) -->
      <section v-if="hasProperties" class="px-3 py-2 border-b border-zinc-100">
        <div class="text-zinc-400 uppercase tracking-wide text-[10px] mb-1">Properties</div>
        <div class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5">
          <template v-for="(val, key) in agent.properties" :key="key">
            <span class="text-zinc-500 truncate max-w-[100px]">{{ key }}</span>
            <span class="text-zinc-700 font-mono truncate">{{ formatValue(val) }}</span>
          </template>
        </div>
      </section>
      <section v-else class="px-3 py-2 border-b border-zinc-100">
        <div class="text-zinc-400 uppercase tracking-wide text-[10px] mb-1">Properties</div>
        <div class="text-zinc-400 italic">none</div>
      </section>

      <!-- Memory (fetched fresh from the agent-detail endpoint; newest-first) -->
      <section class="px-3 py-2">
        <div class="text-zinc-400 uppercase tracking-wide text-[10px] mb-1 flex items-center gap-1">
          Memory
          <span v-if="memory.length" class="normal-case">({{ memory.length }})</span>
          <i v-if="loading" class="pi pi-spin pi-spinner text-[9px] text-zinc-300" />
        </div>
        <div v-if="error" class="text-rose-500 italic">{{ error }}</div>
        <div v-else-if="memory.length" class="flex flex-col gap-1">
          <div
            v-for="(entry, i) in memoryNewestFirst"
            :key="i"
            class="flex items-start gap-1.5"
          >
            <span
              class="shrink-0 mt-0.5 rounded px-1 py-px text-[9px] font-medium uppercase tracking-wide"
              :class="memoryTypeClass(entry.type)"
            >{{ entry.type }}</span>
            <span class="text-zinc-500">step {{ entry.step }}:</span>
            <span class="text-zinc-700 break-words min-w-0">{{ entry.summary }}</span>
          </div>
        </div>
        <div v-else class="text-zinc-400 italic">
          No memory recorded yet.
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
// AgentInspector — shown in RunWorldCanvas when the user clicks an agent sprite.
// Receives the live AgentFrameState for id/state/position/properties (updated each
// step from the stream frame). Memory + state are refreshed from the backend
// agent-detail endpoint: while a run is active this returns the LIVE runtime
// snapshot (fresh memory), else the persisted record. A manual refresh button and
// a light ~3s poll (while running) keep the memory current; the poll clears on
// unselect / unmount / pause.
import { computed, onUnmounted, ref, watch } from 'vue'
import type { AgentFrameState } from '@/modules/simulation/stores/runStore'
import type { MemoryEntry, SimulationAgent, SimulationStatus } from '@/shared/types'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'

const props = defineProps<{
  // The simulation id (for the agent-detail fetch).
  simulationId: string
  // Live frame state (id, state, position, properties — updated per step).
  agent: AgentFrameState | null
  // Fallback memory list from the persisted simulation snapshot, shown until the
  // fresh fetch resolves (best-effort — may be stale mid-run).
  memory: MemoryEntry[]
  // The archetype uid (agent_uid) from the persisted snapshot, for display.
  agentUid?: string
  // The archetype's display color (hex string or undefined).
  archetypeColor?: string
  // The run lifecycle status — drives whether the memory poll runs.
  runStatus?: SimulationStatus | null
  // Replay mode: the run has finished and the canvas shows a scrubbed step. When
  // true, memory is filtered to the scrubbed step and the live poll is disabled.
  replay?: boolean
  // In replay, the currently-displayed (scrubbed) step; memory entries with a
  // higher step are hidden so the list stays consistent with the shown frame.
  maxStep?: number
}>()

defineEmits<{ close: [] }>()

const api = useSimulationApi()

// The freshly-fetched agent detail (memory + state). Falls back to the snapshot
// memory prop until the first fetch resolves.
const detail = ref<SimulationAgent | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const POLL_MS = 3000
let pollTimer: ReturnType<typeof setInterval> | null = null
// Guards against a stale in-flight fetch (agent switched / unmounted) clobbering
// the current selection's detail.
let fetchToken = 0

// Fresh memory when the fetch resolved for the current agent, else the snapshot
// fallback prop. In replay we filter to entries at or before the scrubbed step so
// the memory list stays consistent with the world frame being shown (memory is a
// step-tagged append-only log; note that entries evicted by the memory cap before
// the run finalized cannot be recovered — see the replay design spec).
const memory = computed<MemoryEntry[]>(() => {
  const base = detail.value?.memory ?? props.memory
  if (props.replay && props.maxStep != null) {
    return base.filter((e) => e.step <= (props.maxStep as number))
  }
  return base
})

// Prefer the live stream frame's state (updated per step); fall back to the
// fetched detail's state when the frame has none.
const displayState = computed<string>(() => props.agent?.state || detail.value?.state || '')

// Prefer the snapshot uid prop; fall back to the fetched detail's agent_uid.
const displayUid = computed<string | undefined>(() => props.agentUid || detail.value?.agent_uid)

const hasProperties = computed(
  () => props.agent?.properties && Object.keys(props.agent.properties).length > 0
)

// Show newest entries first so the most recent context is at the top.
const memoryNewestFirst = computed(() => [...memory.value].reverse())

const stateClass = computed(() => {
  if (!displayState.value) return 'text-zinc-400'
  return 'text-violet-700'
})

async function fetchDetail(): Promise<void> {
  const agentId = props.agent?.id
  if (agentId == null || !props.simulationId) return
  const myToken = ++fetchToken
  loading.value = true
  error.value = null
  try {
    const d = await api.getAgent(props.simulationId, agentId)
    // Discard if the selection changed / unmounted while awaiting.
    if (myToken !== fetchToken) return
    detail.value = d
  } catch (e) {
    if (myToken !== fetchToken) return
    const status = (e as { status?: number } | null)?.status
    error.value = status === 404 ? 'Agent not found.' : 'Failed to load memory.'
  } finally {
    if (myToken === fetchToken) loading.value = false
  }
}

function refresh(): void {
  void fetchDetail()
}

function stopPoll(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function startPoll(): void {
  stopPoll()
  if (props.replay) return // replay is static — no live memory to poll for
  if (props.runStatus !== 'running') return
  pollTimer = setInterval(() => {
    void fetchDetail()
  }, POLL_MS)
}

// (Re)fetch and (re)arm the poll whenever the selected agent changes. Switching
// agents drops the previous detail immediately (fetchToken invalidates the old
// in-flight request) so stale memory never lingers on the new selection.
watch(
  () => props.agent?.id,
  (id) => {
    stopPoll()
    detail.value = null
    error.value = null
    if (id == null) return
    void fetchDetail()
    startPoll()
  },
  { immediate: true }
)

// Re-arm / tear down the poll as the run status changes (pause stops it; a
// resume back to running restarts it) without disturbing the current detail.
watch(
  () => props.runStatus,
  () => {
    if (props.agent?.id == null) return
    startPoll()
  }
)

onUnmounted(() => {
  fetchToken++ // invalidate any in-flight fetch
  stopPoll()
})

function formatValue(val: unknown): string {
  if (val === null || val === undefined) return '—'
  if (typeof val === 'number') {
    return Number.isInteger(val) ? String(val) : val.toFixed(3)
  }
  if (typeof val === 'boolean') return val ? 'true' : 'false'
  if (typeof val === 'string') return val || '""'
  return JSON.stringify(val)
}

// Returns a badge class for each memory type so they are visually distinct.
function memoryTypeClass(type: string): string {
  switch (type) {
    case 'decision': return 'bg-violet-100 text-violet-700'
    case 'rule': return 'bg-amber-100 text-amber-700'
    case 'llm_response': return 'bg-blue-100 text-blue-700'
    case 'interaction': return 'bg-emerald-100 text-emerald-700'
    case 'movement': return 'bg-zinc-100 text-zinc-600'
    case 'property': return 'bg-rose-100 text-rose-700'
    default: return 'bg-zinc-100 text-zinc-500'
  }
}
</script>
