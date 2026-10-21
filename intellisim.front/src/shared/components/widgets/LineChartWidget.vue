<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <Line v-if="chartData" :data="chartData" :options="chartOptions" />
    <div v-else class="h-full flex flex-col items-center justify-center text-center gap-1 px-3">
      <i class="pi pi-chart-line text-2xl text-zinc-300" />
      <span class="text-xs text-zinc-400">Time series — data appears during the run</span>
      <span v-if="seriesLabels" class="text-[11px] text-zinc-300 truncate max-w-full">{{ seriesLabels }}</span>
    </div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed line-chart. A line_chart plots its series OVER TIME, which only exists
// during/after a run. Pre-run (or before the first accumulated point) it shows an
// empty-state placeholder listing the configured series labels. During/after a
// run it receives the run store's accumulated points per series (keyed by step)
// via `series` and renders a Chart.js line — one dataset per configured series,
// aligned on the union of step numbers (Task 25 live-data path / replay).
import { computed } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import WidgetShell from './WidgetShell.vue'
import { paletteColor } from './palette'
import type { WidgetConfig } from '@/shared/types/widget'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend)

// One accumulated point of a series at a step.
export interface LineSeriesPoint {
  step: number
  value: number
}

const props = defineProps<{
  widget: WidgetConfig
  readonly?: boolean
  // Accumulated points per series, keyed by the series LABEL (run store feeds
  // this during/after a run). Undefined pre-run → the placeholder.
  series?: Record<string, LineSeriesPoint[]>
}>()
defineEmits<{ edit: []; delete: [] }>()

const configuredSeries = computed(() => props.widget.series ?? [])
const seriesLabels = computed(() => configuredSeries.value.map((s) => s.label).filter(Boolean).join(', '))

// The union of all step numbers across the series, sorted ascending — the shared
// x-axis. Each dataset is aligned to this axis (a series missing a step is null).
const steps = computed<number[]>(() => {
  const acc = props.series
  if (!acc) return []
  const set = new Set<number>()
  for (const label of Object.keys(acc)) {
    for (const p of acc[label] ?? []) set.add(p.step)
  }
  return [...set].sort((a, b) => a - b)
})

const chartData = computed(() => {
  const acc = props.series
  if (!acc || steps.value.length === 0) return null
  const axis = steps.value
  const datasets = configuredSeries.value.map((s, i) => {
    const byStep = new Map<number, number>()
    for (const p of acc[s.label] ?? []) byStep.set(p.step, p.value)
    if (byStep.size === 0) return null
    const color = s.color || paletteColor(i)
    return {
      label: s.label,
      data: axis.map((st) => (byStep.has(st) ? byStep.get(st)! : null)),
      borderColor: color,
      backgroundColor: color + '33',
      borderWidth: 1.5,
      pointRadius: 0,
      tension: 0.25,
      spanGaps: true,
    }
  }).filter((d): d is NonNullable<typeof d> => d !== null)
  if (datasets.length === 0) return null
  return { labels: axis.map((s) => String(s)), datasets }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  animation: false as const,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: {
      display: true,
      position: 'bottom' as const,
      labels: { font: { size: 10 }, boxWidth: 10 },
    },
  },
  scales: {
    x: { display: true, grid: { display: false }, ticks: { font: { size: 9 }, maxTicksLimit: 8 } },
    y: { display: true, grid: { color: '#f4f4f5' }, ticks: { font: { size: 10 } } },
  },
}
</script>
