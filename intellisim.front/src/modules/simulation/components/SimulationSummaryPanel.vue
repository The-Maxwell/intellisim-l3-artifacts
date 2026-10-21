<template>
  <div class="h-full flex flex-col min-h-0 bg-white">
    <!-- Header -->
    <div class="flex items-center gap-2 px-4 py-2.5 border-b border-zinc-200">
      <span class="w-2 h-2 rounded-full flex-none" :class="readyDotClass" />
      <span class="text-sm font-semibold text-zinc-800">Simulation summary</span>
      <span class="ml-auto text-[11px] text-zinc-400">live</span>
    </div>

    <div class="flex-1 min-h-0 overflow-y-auto p-3 space-y-3">
      <!-- Readiness verdict + its blockers, kept together at the top. -->
      <div class="space-y-1.5">
        <div class="flex items-center gap-3 rounded-lg border px-3 py-2" :class="bannerClass">
          <span
            class="w-7 h-7 rounded-lg grid place-items-center text-white flex-none"
            :class="ready ? 'bg-emerald-500' : 'bg-rose-500'"
          >
            <i :class="ready ? 'pi pi-check' : 'pi pi-exclamation-triangle'" />
          </span>
          <div class="min-w-0">
            <div class="text-sm font-semibold" :class="ready ? 'text-emerald-700' : 'text-rose-700'">
              {{ ready ? 'Ready to run' : 'Not ready to run' }}
            </div>
            <div class="text-[11px] text-zinc-500">{{ readySubtitle }}</div>
          </div>
        </div>
        <ul v-if="issues.length" class="space-y-1">
          <li v-for="(iss, i) in issues" :key="i">
            <button
              type="button"
              class="w-full flex items-start gap-2 rounded-md px-2.5 py-1.5 text-left text-xs transition"
              :class="iss.kind === 'error' ? 'bg-rose-50 hover:bg-rose-100' : 'bg-amber-50 hover:bg-amber-100'"
              @click="goIssue(iss.path)"
            >
              <span
                class="flex-none mt-px font-mono text-[9px] font-bold px-1 py-0.5 rounded text-white"
                :class="iss.kind === 'error' ? 'bg-rose-500' : 'bg-amber-500'"
              >{{ iss.kind === 'error' ? 'ERR' : 'WARN' }}</span>
              <span class="min-w-0 text-zinc-700">
                <code v-if="iss.path" class="text-[10.5px] text-zinc-500">{{ iss.path }}</code>{{ iss.path ? ' — ' : '' }}{{ iss.reason }}
                <span class="text-violet-600 whitespace-nowrap"> →</span>
              </span>
            </button>
          </li>
        </ul>
      </div>

      <!-- Count tiles -->
      <div class="grid grid-cols-2 gap-1.5">
        <button
          v-for="t in tiles"
          :key="t.key"
          type="button"
          class="text-left rounded-lg border border-zinc-200 bg-zinc-50 px-2.5 py-1.5 transition hover:border-violet-300 hover:bg-violet-50"
          @click="emit('navigate', t.tab)"
        >
          <div class="text-lg font-bold leading-none tabular-nums text-zinc-800">{{ t.n }}</div>
          <div class="mt-0.5 text-[9px] uppercase tracking-wide text-zinc-400">{{ t.label }}</div>
        </button>
      </div>

      <!-- Config -->
      <section>
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">Configuration</p>
        <div class="text-xs divide-y divide-zinc-100 rounded-lg border border-zinc-200">
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Status</span>
            <span class="font-mono text-[11px] text-zinc-700">{{ simulation.status }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Seed</span>
            <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ simulation.seed }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Generation model</span>
            <span class="font-mono text-[11px]" :class="genModel ? 'text-zinc-700' : 'text-zinc-400'">{{ genModel || 'inherit' }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Execution model</span>
            <span class="font-mono text-[11px]" :class="execModel ? 'text-zinc-700' : 'text-zinc-400'">{{ execModel || 'inherit' }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Tokens used</span>
            <span class="font-mono text-[11px] tabular-nums" :class="tokensTotal ? 'text-zinc-700' : 'text-zinc-400'">{{ tokensTotal.toLocaleString() }}</span>
          </div>
        </div>
      </section>

      <!-- Last run timing (active run time excludes paused intervals) -->
      <section v-if="lastRun">
        <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">Last run</p>
        <div class="text-xs divide-y divide-zinc-100 rounded-lg border border-zinc-200">
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Started</span>
            <span class="font-mono text-[11px] text-zinc-700">{{ startedLabel }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Finished</span>
            <span class="font-mono text-[11px] text-zinc-700">{{ finishedLabel }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Active duration</span>
            <span class="font-mono text-[11px] text-zinc-700 tabular-nums" data-testid="summary-run-duration">{{ durationLabel }}</span>
          </div>
        </div>
      </section>

      <!-- Agent census (per archetype) -->
      <section v-if="census.length">
        <button
          type="button"
          class="w-full text-left text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1 hover:text-violet-600"
          @click="emit('navigate', 'agents')"
        >
          Agent census
        </button>
        <div class="text-xs divide-y divide-zinc-100 rounded-lg border border-zinc-200">
          <div v-for="c in census" :key="c.uid" class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1 truncate">{{ c.name }}</span>
            <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ c.count }}</span>
          </div>
        </div>
      </section>

      <!-- Environment (spatial only) -->
      <section v-if="isSpatial">
        <button
          type="button"
          class="w-full text-left text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1 hover:text-violet-600"
          @click="emit('navigate', 'environment')"
        >
          Environment
        </button>
        <div class="text-xs divide-y divide-zinc-100 rounded-lg border border-zinc-200">
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Size</span>
            <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ env.width }} × {{ env.height }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Wrap (torus)</span>
            <span class="font-mono text-[11px] text-zinc-700">{{ env.torus ? 'on' : 'off' }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Collision</span>
            <span class="font-mono text-[11px] text-zinc-700">{{ env.collision }}</span>
          </div>
          <div class="flex items-center gap-2 px-2.5 py-1">
            <span class="text-zinc-500 flex-1">Shapes · global props</span>
            <span class="font-mono text-[11px] text-zinc-700 tabular-nums">{{ env.shapes }} · {{ env.globals }}</span>
          </div>
        </div>
      </section>

      <!-- Folded Token usage (collapsed by default; wide tables scroll inside) -->
      <details class="rounded-lg border border-zinc-200 group">
        <summary class="cursor-pointer select-none px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-zinc-500 flex items-center gap-2">
          <i class="pi pi-angle-right text-[10px] transition-transform group-open:rotate-90" />
          Token usage
        </summary>
        <div class="border-t border-zinc-100 p-2 overflow-x-auto">
          <TokenUsagePanel :usage="simulation.token_usage ?? null" />
        </div>
      </details>

      <!-- Folded Status history (collapsed by default) -->
      <details v-if="simulation.status_history?.length" class="rounded-lg border border-zinc-200 group">
        <summary class="cursor-pointer select-none px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-zinc-500 flex items-center gap-2">
          <i class="pi pi-angle-right text-[10px] transition-transform group-open:rotate-90" />
          Status history ({{ simulation.status_history.length }})
        </summary>
        <div class="border-t border-zinc-100 p-2 overflow-x-auto">
          <StatusHistoryPanel :history="simulation.status_history" />
        </div>
      </details>
    </div>
  </div>
</template>

<script setup lang="ts">
// Read-only simulation summary — replaces the JSON pane. Everything is DERIVED
// from the open Simulation + its SimulationValidationResult (no extra fetch).
// Surfaces run-readiness with its blockers, composition tiles, config, per-
// archetype census, environment facts, and the folded (collapsed) Token usage +
// Status history panels. Clicking a tile / issue emits navigate(tab) so the
// editor can jump to the relevant section. Mirrors the model editor's summary.
import { computed } from 'vue'
import TokenUsagePanel from './TokenUsagePanel.vue'
import StatusHistoryPanel from './StatusHistoryPanel.vue'
import { formatDuration, formatTimestamp } from '@/shared/utils/time'
import type { Simulation, SimulationValidationResult } from '@/shared/types'

const props = defineProps<{
  simulation: Simulation
  validation: SimulationValidationResult | null
}>()
const emit = defineEmits<{ navigate: [tab: string] }>()

const ready = computed(() => !!props.validation?.start_ready)
const errorCount = computed(() => props.validation?.errors?.length ?? 0)
const warningCount = computed(() => props.validation?.warnings?.length ?? 0)

const readyDotClass = computed(() => (ready.value ? 'bg-emerald-500' : 'bg-rose-500'))
const bannerClass = computed(() =>
  ready.value ? 'bg-emerald-50 border-emerald-200' : 'bg-rose-50 border-rose-200'
)
const readySubtitle = computed(() => {
  if (props.simulation.status === 'generating') return 'Generating…'
  const parts: string[] = []
  parts.push(ready.value ? 'No blockers' : `${errorCount.value} blocker${errorCount.value === 1 ? '' : 's'}`)
  if (warningCount.value) parts.push(`${warningCount.value} warning${warningCount.value === 1 ? '' : 's'}`)
  return parts.join(' · ')
})

// Errors first, then warnings (sim issues carry no severity field — errors are
// blocking, warnings advisory), tagged with a synthetic `kind`.
const issues = computed(() => [
  ...(props.validation?.errors ?? []).map((i) => ({ ...i, kind: 'error' as const })),
  ...(props.validation?.warnings ?? []).map((i) => ({ ...i, kind: 'warning' as const })),
])

const isSpatial = computed(() => props.simulation.model_snapshot?.is_spatial ?? false)
const archetypes = computed(() => props.simulation.model_snapshot?.agents ?? [])
const instances = computed(() => props.simulation.agents ?? [])

const tiles = computed(() => [
  { key: 'instances', label: 'Instances', n: instances.value.length, tab: 'agents' },
  { key: 'archetypes', label: 'Archetypes', n: archetypes.value.length, tab: 'agents' },
  { key: 'widgets', label: 'Widgets', n: (props.simulation.widgets ?? []).length, tab: 'widgets' },
  { key: 'steps', label: 'Steps', n: props.simulation.steps, tab: 'general' },
])

// Per-archetype instance counts (named from the snapshot).
const census = computed(() => {
  const counts: Record<string, number> = {}
  for (const a of instances.value) counts[a.agent_uid] = (counts[a.agent_uid] ?? 0) + 1
  return archetypes.value.map((arch) => ({
    uid: arch.uid,
    name: arch.name || arch.uid,
    count: counts[arch.uid] ?? 0,
  }))
})

const env = computed(() => {
  const e = props.simulation.environment
  return {
    width: e?.width ?? 0,
    height: e?.height ?? 0,
    torus: !!e?.torus,
    collision: e?.collision_mode === 'block' ? 'block' : 'none',
    shapes: e?.shapes?.length ?? 0,
    globals: e?.global_properties ? Object.keys(e.global_properties).length : 0,
  }
})

// Latest engine run (timing shown in the "Last run" block). Active duration is
// the pause-excluding run time computed by the backend at finalize.
const lastRun = computed(() => {
  const runs = props.simulation.execution_runs
  return runs && runs.length ? (runs[runs.length - 1] ?? null) : null
})
const startedLabel = computed(() => formatTimestamp(lastRun.value?.started_at))
const finishedLabel = computed(() =>
  lastRun.value?.finished_at ? formatTimestamp(lastRun.value.finished_at) : 'in progress'
)
const durationLabel = computed(() =>
  lastRun.value ? formatDuration(lastRun.value.active_duration_ms ?? 0) : '—'
)

const genModel = computed(() => props.simulation.llm_generation_target?.model_uid ?? '')
const execModel = computed(() => props.simulation.llm_execution_target?.model_uid ?? '')
const tokensTotal = computed(() => props.simulation.token_usage?.total?.total_tokens ?? 0)

// Map a validation-issue path (e.g. "agents[3].state", "environment.width",
// "steps") to a tab. Unknown heads → general; the editor clamps to existing tabs.
function goIssue(path: string) {
  const head = /^(\w+)/.exec(path || '')?.[1] ?? ''
  const map: Record<string, string> = {
    agents: 'agents',
    environment: 'environment',
    globals: 'environment',
    widgets: 'widgets',
  }
  emit('navigate', map[head] ?? 'general')
}
</script>
