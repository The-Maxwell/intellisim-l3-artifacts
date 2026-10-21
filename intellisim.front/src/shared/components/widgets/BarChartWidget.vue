<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <Bar v-if="chartData" :data="chartData" :options="chartOptions" />
    <div v-else class="flex items-center justify-center h-full text-zinc-400 text-xs">No data</div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed bar-chart preview: three modes resolved from the widget config —
//   • series        → one bar per series (agent_property agg / global / population)
//   • population     → one bar per agent (instance counts)
//   • state_distribution → one bar per state of one agent
// Computes from the CURRENT initial-state data; never from live-run data.
import { computed } from 'vue'
import { Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import WidgetShell from './WidgetShell.vue'
import { paletteColor } from './palette'
import {
  populationByAgent,
  stateDistribution,
  seriesValues,
  type NamedValue,
  type PreviewContext,
} from './widgetData'
import type { WidgetConfig } from '@/shared/types/widget'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

const props = defineProps<{ widget: WidgetConfig; ctx: PreviewContext; readonly?: boolean }>()
defineEmits<{ edit: []; delete: [] }>()

const points = computed<NamedValue[]>(() => {
  const w = props.widget
  if (w.series && w.series.length) return seriesValues(w.series, props.ctx)
  if (w.source?.type === 'state_distribution') return stateDistribution(w.source, props.ctx)
  // default single-source path: population
  return populationByAgent(w.source, props.ctx)
})

// In series mode each bar is one series (index-aligned with widget.series), so
// an explicitly configured series color wins over the palette. Single-source
// modes (population / state_distribution) have no per-bar config — palette only.
function barColor(i: number): string {
  return props.widget.series?.[i]?.color || paletteColor(i)
}

// Append the CC alpha suffix only to a full #RRGGBB hex; any other user-provided
// form (#RGB / #RRGGBBAA) is used verbatim rather than corrupted.
function withAlpha(hex: string): string {
  return /^#[0-9a-fA-F]{6}$/.test(hex) ? hex + 'CC' : hex
}

const chartData = computed(() => {
  const pts = points.value
  if (pts.length === 0) return null
  return {
    labels: pts.map((p) => p.label),
    datasets: [
      {
        label: props.widget.title,
        data: pts.map((p) => p.value),
        backgroundColor: pts.map((_, i) => withAlpha(barColor(i))),
        borderColor: pts.map((_, i) => barColor(i)),
        borderWidth: 1,
      },
    ],
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
  scales: {
    x: { display: true, grid: { display: false }, ticks: { font: { size: 10 } } },
    y: { display: true, beginAtZero: true, grid: { color: '#f4f4f5' }, ticks: { font: { size: 10 } } },
  },
}
</script>
