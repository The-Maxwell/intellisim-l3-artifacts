// Shared widget-type metadata (label + PrimeIcons icon + description).
// Extracted from WidgetEditor.vue so the arrange-layout placeholders can reuse
// the same labels/icons the editor's type picker uses (single source of truth).
import type { WidgetType } from '@/shared/types/widget'

export interface WidgetTypeMeta {
  value: WidgetType
  label: string
  icon: string
  description: string
}

export const WIDGET_TYPE_OPTIONS: WidgetTypeMeta[] = [
  { value: 'monitor', label: 'Monitor', icon: 'pi pi-hashtag', description: 'Single aggregated value' },
  { value: 'line_chart', label: 'Line chart', icon: 'pi pi-chart-line', description: 'Series over time' },
  { value: 'bar_chart', label: 'Bar chart', icon: 'pi pi-chart-bar', description: 'Population / states / series' },
  { value: 'pie_chart', label: 'Pie chart', icon: 'pi pi-chart-pie', description: 'Population / state share' },
  { value: 'histogram', label: 'Histogram', icon: 'pi pi-align-left', description: 'Property distribution' },
  { value: 'agent_table', label: 'Agent table', icon: 'pi pi-table', description: 'Instances × columns' },
  { value: 'text_log', label: 'Text log', icon: 'pi pi-list', description: 'Event / decision log' },
]

const BY_VALUE: Record<WidgetType, WidgetTypeMeta> = Object.fromEntries(
  WIDGET_TYPE_OPTIONS.map((o) => [o.value, o]),
) as Record<WidgetType, WidgetTypeMeta>

// Metadata for a widget type; falls back to a generic label/icon for unknown types.
export function widgetTypeMeta(type: WidgetType): WidgetTypeMeta {
  return BY_VALUE[type] ?? { value: type, label: type, icon: 'pi pi-box', description: '' }
}
