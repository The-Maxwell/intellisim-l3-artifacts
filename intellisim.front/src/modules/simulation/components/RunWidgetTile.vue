<template>
  <WidgetPreview
    :widget="widget"
    :ctx="ctx"
    :readonly="true"
    :lineSeries="lineSeries"
    :logEntries="logEntries"
  />
</template>

<script setup lang="ts">
// A single widget cell rendered inside the run-screen layout grid. Extracted
// verbatim from RunDashboard's former per-widget loop so the run dashboard can
// fill DashboardLayoutGrid's #widget slot. Read-only; data comes from the run
// store via the props RunDashboard threads in (instant-recompute ctx + accumulated
// line series / log entries).
import WidgetPreview from '@/shared/components/widgets/WidgetPreview.vue'
import type { PreviewContext } from '@/shared/components/widgets/widgetData'
import type { LineSeriesPoint } from '@/shared/components/widgets/LineChartWidget.vue'
import type { TextLogEntry } from '@/shared/components/widgets/TextLogWidget.vue'
import type { WidgetConfig } from '@/shared/types/widget'

defineProps<{
  widget: WidgetConfig
  ctx: PreviewContext
  lineSeries?: Record<string, LineSeriesPoint[]>
  logEntries?: TextLogEntry[]
}>()
</script>
