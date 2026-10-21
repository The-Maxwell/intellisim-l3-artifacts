<template>
  <div class="space-y-1">
    <p v-if="!history.length" class="text-xs text-zinc-400">No status history yet.</p>

    <ol v-else class="relative border-l border-zinc-200 ml-2 space-y-4 py-1">
      <li
        v-for="(entry, i) in history"
        :key="i"
        class="ml-4"
      >
        <!-- Timeline dot -->
        <span
          class="absolute -left-[6px] mt-1 w-3 h-3 rounded-full border-2 border-white ring-1"
          :class="dotClass(entry.status)"
        />

        <!-- Status badge + timestamp row -->
        <div class="flex items-center gap-2 flex-wrap">
          <Tag :value="entry.status" :severity="severity(entry.status)" class="text-[11px]" />
          <span class="text-[11px] text-zinc-400 tabular-nums">{{ formatAt(entry.at) }}</span>
          <span v-if="entry.by" class="text-[11px] text-zinc-400">· {{ entry.by }}</span>
        </div>
      </li>
    </ol>
  </div>
</template>

<script setup lang="ts">
// Read-only vertical timeline of simulation.status_history (StatusChange[]).
// Shows a coloured dot + status badge + ISO timestamp + actor ("by") for each
// recorded transition. Only rendered when the parent passes a non-empty history.
import Tag from 'primevue/tag'
import type { StatusChange, SimulationStatus } from '@/shared/types/simulation'
import { statusSeverity } from '@/modules/simulation/views/simulationStatus'

defineProps<{ history: StatusChange[] }>()

// Derive a PrimeVue Tag severity from the status string (shares the same logic
// as the top-bar status tag so colours are consistent).
function severity(status: string) {
  return statusSeverity(status as SimulationStatus)
}

// Map a status to a Tailwind ring/bg colour for the timeline dot.
function dotClass(status: string): string {
  switch (status) {
    case 'ready':
    case 'finished':
      return 'bg-emerald-400 ring-emerald-300'
    case 'generating':
    case 'running':
    case 'queued':
      return 'bg-blue-400 ring-blue-300'
    case 'generation_failed':
    case 'failed':
      return 'bg-red-400 ring-red-300'
    case 'paused':
      return 'bg-amber-400 ring-amber-300'
    case 'stopped':
    default:
      return 'bg-zinc-400 ring-zinc-300'
  }
}

// Format an ISO timestamp to a concise local date+time string.
function formatAt(iso: string): string {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return iso
  }
}
</script>
