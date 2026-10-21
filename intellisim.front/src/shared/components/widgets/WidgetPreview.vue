<template>
  <MonitorWidget
    v-if="widget.type === 'monitor'"
    :widget="widget"
    :ctx="ctx"
    :readonly="readonly"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <BarChartWidget
    v-else-if="widget.type === 'bar_chart'"
    :widget="widget"
    :ctx="ctx"
    :readonly="readonly"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <PieChartWidget
    v-else-if="widget.type === 'pie_chart'"
    :widget="widget"
    :ctx="ctx"
    :readonly="readonly"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <HistogramWidget
    v-else-if="widget.type === 'histogram'"
    :widget="widget"
    :ctx="ctx"
    :readonly="readonly"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <AgentTableWidget
    v-else-if="widget.type === 'agent_table'"
    :widget="widget"
    :ctx="ctx"
    :readonly="readonly"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <LineChartWidget
    v-else-if="widget.type === 'line_chart'"
    :widget="widget"
    :readonly="readonly"
    :series="lineSeries"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <TextLogWidget
    v-else-if="widget.type === 'text_log'"
    :widget="widget"
    :readonly="readonly"
    :entries="logEntries"
    @edit="$emit('edit')"
    @delete="$emit('delete')"
  />
  <WidgetShell v-else :title="widget.title" :readonly="readonly" @edit="$emit('edit')" @delete="$emit('delete')">
    <div class="flex items-center justify-center h-full text-zinc-400 text-xs">
      Unknown widget type: {{ widget.type }}
    </div>
  </WidgetShell>
</template>

<script setup lang="ts">
// Typed widget factory: switch on widget.type → the matching widgets render
// component. Each component renders its own WidgetShell (title bar + edit/delete);
// this factory just forwards the edit/delete events upward to the grid.
import WidgetShell from './WidgetShell.vue'
import MonitorWidget from './MonitorWidget.vue'
import BarChartWidget from './BarChartWidget.vue'
import PieChartWidget from './PieChartWidget.vue'
import HistogramWidget from './HistogramWidget.vue'
import AgentTableWidget from './AgentTableWidget.vue'
import LineChartWidget from './LineChartWidget.vue'
import TextLogWidget from './TextLogWidget.vue'
import type { LineSeriesPoint } from './LineChartWidget.vue'
import type { TextLogEntry } from './TextLogWidget.vue'
import type { PreviewContext } from './widgetData'
import type { WidgetConfig } from '@/shared/types/widget'

// ctx feeds the instant-recompute widgets (monitor/pie/histogram/bar/table).
// lineSeries / logEntries feed the time-axis widgets (line_chart / text_log)
// with the run store's accumulated data; both undefined pre-run → placeholders.
defineProps<{
  widget: WidgetConfig
  ctx: PreviewContext
  readonly?: boolean
  lineSeries?: Record<string, LineSeriesPoint[]>
  logEntries?: TextLogEntry[]
}>()
defineEmits<{ edit: []; delete: [] }>()
</script>
