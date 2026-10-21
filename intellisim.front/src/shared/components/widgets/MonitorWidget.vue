<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <div class="h-full flex flex-col items-center justify-center">
      <span class="text-3xl font-semibold text-zinc-800 tabular-nums">{{ display }}</span>
      <span class="text-[11px] text-zinc-400 mt-1 truncate max-w-full">{{ subtitle }}</span>
    </div>
  </WidgetShell>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import WidgetShell from './WidgetShell.vue'
import { monitorValue, type PreviewContext } from './widgetData'
import type { WidgetConfig } from '@/shared/types/widget'

const props = defineProps<{ widget: WidgetConfig; ctx: PreviewContext; readonly?: boolean }>()
defineEmits<{ edit: []; delete: [] }>()

const value = computed(() => monitorValue(props.widget.source, props.ctx))

const display = computed(() => {
  const v = value.value
  if (v == null) return '—'
  // Trim trailing decimals on whole numbers; round others to 2dp.
  return Number.isInteger(v) ? String(v) : String(Math.round(v * 100) / 100)
})

const subtitle = computed(() => {
  const s = props.widget.source
  if (!s) return ''
  if (s.type === 'population') return 'population'
  if (s.type === 'global_property') return s.property_uid ?? 'global'
  if (s.type === 'agent_property') return `${s.aggregation ?? 'avg'} · ${s.property_uid ?? ''}`
  if (s.type === 'sim_metric') return (s.metric ?? '').replace(/_/g, ' ')
  return ''
})
</script>
