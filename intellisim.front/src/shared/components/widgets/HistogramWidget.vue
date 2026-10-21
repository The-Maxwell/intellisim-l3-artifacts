<template>
  <WidgetShell :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <Bar v-if="chartData" :data="chartData" :options="chartOptions" />
    <div v-else class="flex items-center justify-center h-full text-zinc-400 text-xs">No data</div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed histogram preview: buckets the initial values of an agent_property across
// the referenced agents' instances. Computes from initial-state data.
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
import { histogramBins, type PreviewContext } from './widgetData'
import type { WidgetConfig } from '@/shared/types/widget'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

const props = defineProps<{ widget: WidgetConfig; ctx: PreviewContext; readonly?: boolean }>()
defineEmits<{ edit: []; delete: [] }>()

const bins = computed(() => histogramBins(props.widget.source, props.widget.buckets, props.ctx))

const chartData = computed(() => {
  const b = bins.value
  if (b.labels.length === 0) return null
  return {
    labels: b.labels,
    datasets: [
      {
        label: props.widget.source?.property_uid ?? 'value',
        data: b.counts,
        backgroundColor: '#8B5CF6CC',
        borderColor: '#8B5CF6',
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
    x: { display: true, grid: { display: false }, ticks: { font: { size: 9 } } },
    y: { display: true, beginAtZero: true, grid: { color: '#f4f4f5' }, ticks: { font: { size: 10 }, precision: 0 } },
  },
}
</script>
