// TypeScript mirror of the backend widget contract
// (../intellisim.core/data/common/widgets.go + modules/simulations/dtos.go
// WidgetConfigDTO / WidgetSourceDTO / WidgetSeriesDTO / WidgetPositionDTO).
// Keys are snake_case to match the JSON wire format exactly.

// --- enums (string-literal unions, mirror data/common/widgets.go) ---

export type WidgetType =
  | 'monitor'
  | 'line_chart'
  | 'bar_chart'
  | 'pie_chart'
  | 'histogram'
  | 'agent_table'
  | 'text_log'

export type WidgetSourceType =
  | 'agent_property'
  | 'global_property'
  | 'population'
  | 'state_distribution'
  | 'sim_metric'

export type WidgetAggregation = 'avg' | 'sum' | 'min' | 'max' | 'count' | 'median'

// WidgetMetric mirrors the sim_metric enum in data/common/widgets.go — cumulative
// run counters read from the SimulationStatistics snapshot.
export type WidgetMetric =
  | 'total_agents'
  | 'agents_spawned'
  | 'agents_removed'
  | 'interactions'
  | 'events'
  | 'rule_executions'
  | 'movement_distance'

export type WidgetLogKind = 'events' | 'state_transitions' | 'llm_decisions'

// --- widget config structure (mirrors WidgetConfigDTO and sub-DTOs) ---

export interface WidgetSource {
  type: WidgetSourceType
  property_uid?: string
  agent_uids?: string[]
  aggregation?: WidgetAggregation
  metric?: WidgetMetric
}

export interface WidgetSeries {
  label: string
  color?: string
  source: WidgetSource
}

export interface WidgetPosition {
  x: number
  y: number
  w: number
  h: number
}

export interface WidgetConfig {
  id: string
  type: WidgetType
  title: string
  source?: WidgetSource
  series?: WidgetSeries[]
  columns?: string[]
  buckets?: number
  log_kinds?: WidgetLogKind[]
  position: WidgetPosition
}

// WIDGET_SOURCE_MATRIX is the per-widget-type list of source types the widget
// editor (Task 17) may offer, derived from the backend ValidateWidget matrix in
// data/common/widgets.go. text_log uses log_kinds (no source) — empty list.
//   monitor          → agent_property | global_property | population | sim_metric
//   line_chart       → (series) agent_property | global_property | population | sim_metric
//   bar_chart        → population | state_distribution (single) OR
//                      (series) agent_property | global_property | population
//   pie_chart        → population | state_distribution
//   histogram        → agent_property
//   agent_table      → agent_property
//   text_log         → (none; uses log_kinds)
export const WIDGET_SOURCE_MATRIX: Record<WidgetType, WidgetSourceType[]> = {
  monitor: ['agent_property', 'global_property', 'population', 'sim_metric'],
  line_chart: ['agent_property', 'global_property', 'population', 'sim_metric'],
  // bar_chart has TWO mutually exclusive modes (Task 17 editor must enforce per-mode):
  //   • single-source path  → source type must be 'population' | 'state_distribution'
  //   • series path         → each series source type may be 'agent_property' | 'global_property' | 'population'
  // The union here covers both paths so the editor can offer all four types in its
  // picker, but validation must restrict the set based on which mode is active.
  bar_chart: ['agent_property', 'global_property', 'population', 'state_distribution'],
  pie_chart: ['population', 'state_distribution'],
  histogram: ['agent_property'],
  agent_table: ['agent_property'],
  text_log: [],
}
