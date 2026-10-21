<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <Pie v-if="chartData" :data="chartData" :options="chartOptions" />
    <div v-else class="flex items-center justify-center h-full text-zinc-400 text-xs">No data</div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed pie-chart preview: population (per-agent counts) or state_distribution
// (per-state counts of one agent). Computes from initial-state data.
import { computed } from 'vue'
import { Pie } from 'vue-chartjs'
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js'
import WidgetShell from './WidgetShell.vue'
import { paletteColor } from './palette'
import { populationByAgent, stateDistribution, type NamedValue, type PreviewContext } from './widgetData'
import type { WidgetConfig } from '@/shared/types/widget'

ChartJS.register(ArcElement, Tooltip, Legend)

const props = defineProps<{ widget: WidgetConfig; ctx: PreviewContext; readonly?: boolean }>()
defineEmits<{ edit: []; delete: [] }>()

const points = computed<NamedValue[]>(() => {
  const w = props.widget
  if (w.source?.type === 'state_distribution') return stateDistribution(w.source, props.ctx)
  return populationByAgent(w.source, props.ctx)
})

const chartData = computed(() => {
  const pts = points.value
  if (pts.length === 0 || pts.every((p) => p.value === 0)) return null
  return {
    labels: pts.map((p) => p.label),
    datasets: [
      {
        data: pts.map((p) => p.value),
        backgroundColor: pts.map((_, i) => paletteColor(i) + 'CC'),
        borderColor: '#ffffff',
        borderWidth: 1,
      },
    ],
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: true, position: 'bottom' as const, labels: { font: { size: 10 }, boxWidth: 10 } } },
}
</script>
