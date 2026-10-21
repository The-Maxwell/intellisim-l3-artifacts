<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <div v-if="entries && entries.length" ref="scrollRef" class="h-full overflow-auto px-2 py-1 font-mono text-[11px] leading-snug">
      <div
        v-for="(e, i) in entries"
        :key="i"
        class="flex items-baseline gap-1.5 py-0.5 border-b border-zinc-100 last:border-0"
      >
        <span class="text-zinc-300 tabular-nums shrink-0">{{ e.step }}</span>
        <span :class="kindClass(e.kind)" class="shrink-0 uppercase text-[9px] font-semibold tracking-wide">{{ kindAbbrev(e.kind) }}</span>
        <span class="text-zinc-600 break-all">{{ e.message }}</span>
      </div>
    </div>
    <div v-else class="h-full flex flex-col items-center justify-center text-center gap-1 px-3">
      <i class="pi pi-list text-2xl text-zinc-300" />
      <span class="text-xs text-zinc-400">Log — entries appear during the run</span>
      <span v-if="kinds" class="text-[11px] text-zinc-300 truncate max-w-full">{{ kinds }}</span>
    </div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed text-log. A text_log tails event / state-transition / LLM-decision lines
// that only exist during a run. Pre-run (or before the first entry) it shows an
// empty-state placeholder listing the selected log kinds. During/after a run it
// receives the run store's accumulated entries (newest at the bottom, bounded
// FIFO) via `entries` and renders them with a step column + a kind tag, auto-
// scrolling to the tail as lines append (Task 25 live-data path / replay).
import { computed, nextTick, ref, watch } from 'vue'
import WidgetShell from './WidgetShell.vue'
import type { WidgetConfig } from '@/shared/types/widget'

// One accumulated text_log line (mirrors runStore.LogEntry).
export interface TextLogEntry {
  step: number
  kind: string
  message: string
}

const props = defineProps<{ widget: WidgetConfig; readonly?: boolean; entries?: TextLogEntry[] }>()
defineEmits<{ edit: []; delete: [] }>()

const kinds = computed(() => (props.widget.log_kinds ?? []).join(', '))
const scrollRef = ref<HTMLElement | null>(null)

// Auto-scroll to the newest entry whenever the count grows.
watch(
  () => props.entries?.length ?? 0,
  () => {
    void nextTick(() => {
      const el = scrollRef.value
      if (el) el.scrollTop = el.scrollHeight
    })
  }
)

function kindAbbrev(kind: string): string {
  switch (kind) {
    case 'state_transitions':
      return 'state'
    case 'llm_decisions':
      return 'decis'
    case 'events':
      return 'event'
    default:
      return kind.slice(0, 5)
  }
}

function kindClass(kind: string): string {
  switch (kind) {
    case 'state_transitions':
      return 'text-blue-500'
    case 'llm_decisions':
      return 'text-violet-500'
    case 'events':
      return 'text-amber-500'
    default:
      return 'text-zinc-400'
  }
}
</script>
