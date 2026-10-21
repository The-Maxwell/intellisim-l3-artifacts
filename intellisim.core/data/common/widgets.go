package common

import (
	"fmt"
	"slices"
)

// WidgetType identifies the visual kind of a dashboard widget. Mirrors
// NetLogo-inspired constructs adapted for LLM-driven ABM dashboards.
type WidgetType string

const (
	WidgetMonitor    WidgetType = "monitor"
	WidgetLineChart  WidgetType = "line_chart"
	WidgetBarChart   WidgetType = "bar_chart"
	WidgetPieChart   WidgetType = "pie_chart"
	WidgetHistogram  WidgetType = "histogram"
	WidgetAgentTable WidgetType = "agent_table"
	WidgetTextLog    WidgetType = "text_log"
)

// allWidgetTypes is the closed-list used for unknown-value checks.
var allWidgetTypes = []WidgetType{
	WidgetMonitor,
	WidgetLineChart,
	WidgetBarChart,
	WidgetPieChart,
	WidgetHistogram,
	WidgetAgentTable,
	WidgetTextLog,
}

func knownWidgetType(t WidgetType) bool {
	return slices.Contains(allWidgetTypes, t)
}

// WidgetSourceType describes what a data source reads from the simulation state.
type WidgetSourceType string

const (
	WidgetSourceAgentProperty     WidgetSourceType = "agent_property"
	WidgetSourceGlobalProperty    WidgetSourceType = "global_property"
	WidgetSourcePopulation        WidgetSourceType = "population"
	WidgetSourceStateDistribution WidgetSourceType = "state_distribution"
	WidgetSourceSimMetric         WidgetSourceType = "sim_metric"
)

var allWidgetSourceTypes = []WidgetSourceType{
	WidgetSourceAgentProperty,
	WidgetSourceGlobalProperty,
	WidgetSourcePopulation,
	WidgetSourceStateDistribution,
	WidgetSourceSimMetric,
}

func knownSourceType(t WidgetSourceType) bool {
	return slices.Contains(allWidgetSourceTypes, t)
}

// WidgetMetric names a built-in engine run counter (from SimulationStatistics)
// that a sim_metric widget source exposes. All values are cumulative run counters.
type WidgetMetric string

const (
	MetricTotalAgents      WidgetMetric = "total_agents"
	MetricAgentsSpawned    WidgetMetric = "agents_spawned"
	MetricAgentsRemoved    WidgetMetric = "agents_removed"
	MetricInteractions     WidgetMetric = "interactions"
	MetricEvents           WidgetMetric = "events"
	MetricRuleExecutions   WidgetMetric = "rule_executions"
	MetricMovementDistance WidgetMetric = "movement_distance"
)

var allWidgetMetrics = []WidgetMetric{
	MetricTotalAgents,
	MetricAgentsSpawned,
	MetricAgentsRemoved,
	MetricInteractions,
	MetricEvents,
	MetricRuleExecutions,
	MetricMovementDistance,
}

func knownWidgetMetric(m WidgetMetric) bool {
	return slices.Contains(allWidgetMetrics, m)
}

// WidgetAggregation is the statistical reduction applied when collapsing
// multiple agent property values into a single scalar.
type WidgetAggregation string

const (
	AggAvg    WidgetAggregation = "avg"
	AggSum    WidgetAggregation = "sum"
	AggMin    WidgetAggregation = "min"
	AggMax    WidgetAggregation = "max"
	AggCount  WidgetAggregation = "count"
	AggMedian WidgetAggregation = "median"
)

var allAggregations = []WidgetAggregation{
	AggAvg,
	AggSum,
	AggMin,
	AggMax,
	AggCount,
	AggMedian,
}

func knownAggregation(a WidgetAggregation) bool {
	return slices.Contains(allAggregations, a)
}

// WidgetLogKind selects which simulation event stream a text_log widget tails.
type WidgetLogKind string

const (
	LogEvents           WidgetLogKind = "events"
	LogStateTransitions WidgetLogKind = "state_transitions"
	LogLLMDecisions     WidgetLogKind = "llm_decisions"
)

var allLogKinds = []WidgetLogKind{
	LogEvents,
	LogStateTransitions,
	LogLLMDecisions,
}

func knownLogKind(k WidgetLogKind) bool {
	return slices.Contains(allLogKinds, k)
}

// WidgetSource describes where a widget reads its data from the simulation.
// Which fields are required or forbidden depends on Type — see ValidateWidget.
type WidgetSource struct {
	Type        WidgetSourceType  `json:"type"`
	PropertyUID string            `json:"property_uid,omitempty"`
	AgentUIDs   []string          `json:"agent_uids,omitempty"`
	Aggregation WidgetAggregation `json:"aggregation,omitempty"`
	Metric      WidgetMetric      `json:"metric,omitempty"` // only for sim_metric sources
}

// WidgetSeries is one data series inside a multi-series chart widget.
type WidgetSeries struct {
	Label  string       `json:"label"`
	Color  string       `json:"color,omitempty"`
	Source WidgetSource `json:"source"`
}

// WidgetPosition is the grid-layout cell of a widget on the dashboard canvas.
type WidgetPosition struct {
	X uint `json:"x"`
	Y uint `json:"y"`
	W uint `json:"w"`
	H uint `json:"h"`
}

// WidgetConfig is a single dashboard widget descriptor. It lives on both
// model documents (as a template layout) and simulation documents (as an
// instance layout). Referential integrity (do the UIDs point at real
// archetypes / properties?) is validated elsewhere; this type carries only
// structural shape.
type WidgetConfig struct {
	ID       string          `json:"id"`
	Type     WidgetType      `json:"type"`
	Title    string          `json:"title"`
	Source   *WidgetSource   `json:"source,omitempty"`
	Series   []WidgetSeries  `json:"series,omitempty"`
	Columns  []string        `json:"columns,omitempty"`
	Buckets  *uint           `json:"buckets,omitempty"`
	LogKinds []WidgetLogKind `json:"log_kinds,omitempty"`
	Position WidgetPosition  `json:"position"`
}

// WidgetIssue is one structural validation problem found by ValidateWidget.
// Path uses the form "widgets[N].field" (or deeper, e.g. "widgets[N].source.type").
type WidgetIssue struct{ Path, Reason string }

// ValidateWidget enforces the per-type source matrix for a single WidgetConfig.
// idx is the widget's position in the enclosing slice and is used to construct
// "widgets[idx]..." path prefixes in every returned WidgetIssue.
func ValidateWidget(idx int, w WidgetConfig) []WidgetIssue {
	p := func(field string) string {
		return fmt.Sprintf("widgets[%d].%s", idx, field)
	}

	var issues []WidgetIssue
	add := func(field, reason string) {
		issues = append(issues, WidgetIssue{Path: p(field), Reason: reason})
	}

	// ── universal checks ─────────────────────────────────────────────────

	if w.ID == "" {
		add("id", "id is required")
	}
	if w.Title == "" {
		add("title", "title is required")
	}
	if !knownWidgetType(w.Type) {
		add("type", fmt.Sprintf("unknown widget type %q", w.Type))
		// Can't proceed with per-type checks — source matrix would be nonsense.
		return issues
	}

	// ── per-type dispatch ─────────────────────────────────────────────────

	switch w.Type {
	case WidgetMonitor:
		validateMonitor(idx, w, add)
	case WidgetLineChart:
		validateLineChart(idx, w, add)
	case WidgetBarChart:
		validateBarChart(idx, w, add)
	case WidgetPieChart:
		validatePieChart(idx, w, add)
	case WidgetHistogram:
		validateHistogram(idx, w, add)
	case WidgetAgentTable:
		validateAgentTable(idx, w, add)
	case WidgetTextLog:
		validateTextLog(idx, w, add)
	}

	return issues
}

// ── per-type validators ────────────────────────────────────────────────────

// addIssue signature used in helpers so they can append without carrying idx.
type issueAdder func(field, reason string)

func validateMonitor(_ int, w WidgetConfig, add issueAdder) {
	if len(w.Series) > 0 {
		add("series", "monitor does not support series; use a chart widget")
	}
	if len(w.Columns) > 0 {
		add("columns", "monitor does not support columns")
	}
	if len(w.LogKinds) > 0 {
		add("log_kinds", "monitor does not support log_kinds")
	}
	if w.Source == nil {
		add("source", "monitor requires exactly one source")
		return
	}
	validateSourceFields(*w.Source, add,
		[]WidgetSourceType{WidgetSourceAgentProperty, WidgetSourceGlobalProperty, WidgetSourcePopulation, WidgetSourceSimMetric})
}

func validateLineChart(_ int, w WidgetConfig, add issueAdder) {
	if w.Source != nil {
		add("source", "line_chart uses series, not a single source")
	}
	if len(w.Series) == 0 {
		add("series", "line_chart requires at least one series")
		return
	}
	validateSeriesSlice(w.Series, add,
		[]WidgetSourceType{WidgetSourceAgentProperty, WidgetSourceGlobalProperty, WidgetSourcePopulation, WidgetSourceSimMetric})
}

func validateBarChart(_ int, w WidgetConfig, add issueAdder) {
	hasSeries := len(w.Series) > 0
	hasSource := w.Source != nil

	if hasSeries && hasSource {
		add("source", "bar_chart must use either series or a single source, not both")
		return
	}
	if !hasSeries && !hasSource {
		add("source", "bar_chart requires either series or a single source")
		return
	}

	if hasSeries {
		validateSeriesSlice(w.Series, add,
			[]WidgetSourceType{WidgetSourceAgentProperty, WidgetSourceGlobalProperty, WidgetSourcePopulation})
		return
	}

	// single-source path: population or state_distribution only
	validateSourceFields(*w.Source, add,
		[]WidgetSourceType{WidgetSourcePopulation, WidgetSourceStateDistribution})
}

func validatePieChart(_ int, w WidgetConfig, add issueAdder) {
	if len(w.Series) > 0 {
		add("series", "pie_chart does not support series")
	}
	if w.Source == nil {
		add("source", "pie_chart requires exactly one source")
		return
	}
	validateSourceFields(*w.Source, add,
		[]WidgetSourceType{WidgetSourcePopulation, WidgetSourceStateDistribution})
}

func validateHistogram(_ int, w WidgetConfig, add issueAdder) {
	if len(w.Series) > 0 {
		add("series", "histogram does not support series")
	}
	if w.Source == nil {
		add("source", "histogram requires exactly one source")
		return
	}
	src := *w.Source
	if !knownSourceType(src.Type) {
		add("source.type", fmt.Sprintf("unknown source type %q", src.Type))
		return
	}
	if src.Type != WidgetSourceAgentProperty {
		add("source.type", "histogram source must be agent_property")
		return
	}
	if src.PropertyUID == "" {
		add("source.property_uid", "histogram agent_property source requires property_uid")
	}
	if src.Aggregation != "" {
		add("source.aggregation", "histogram uses raw distribution — aggregation must be empty")
	}
	if w.Buckets != nil && *w.Buckets < 2 {
		add("buckets", "buckets must be ≥ 2 when set")
	}
}

func validateAgentTable(_ int, w WidgetConfig, add issueAdder) {
	if len(w.Series) > 0 {
		add("series", "agent_table does not support series")
	}
	if w.Source == nil {
		add("source", "agent_table requires a source")
		return
	}
	src := *w.Source
	if !knownSourceType(src.Type) {
		add("source.type", fmt.Sprintf("unknown source type %q", src.Type))
		return
	}
	if src.Type != WidgetSourceAgentProperty {
		add("source.type", "agent_table source must be agent_property")
		return
	}
	if src.PropertyUID != "" {
		add("source.property_uid", "agent_table source must not specify property_uid (columns drive field selection)")
	}
	if src.Aggregation != "" {
		add("source.aggregation", "agent_table source must not specify aggregation")
	}
	if len(src.AgentUIDs) != 1 {
		add("source.agent_uids", "agent_table source requires exactly 1 agent_uid")
	}
	if len(w.Columns) == 0 {
		add("columns", "agent_table requires at least one column")
	}
}

func validateTextLog(_ int, w WidgetConfig, add issueAdder) {
	if w.Source != nil {
		add("source", "text_log does not support source")
	}
	if len(w.Series) > 0 {
		add("series", "text_log does not support series")
	}
	if len(w.Columns) > 0 {
		add("columns", "text_log does not support columns")
	}

	if len(w.LogKinds) == 0 {
		add("log_kinds", "text_log requires at least one log_kind")
		return
	}

	seen := make(map[WidgetLogKind]bool, len(w.LogKinds))
	hasDup := false
	for _, k := range w.LogKinds {
		if !knownLogKind(k) {
			add("log_kinds", fmt.Sprintf("unknown log kind %q", k))
		}
		if seen[k] {
			hasDup = true
		}
		seen[k] = true
	}
	if hasDup {
		add("log_kinds", "log_kinds must not contain duplicates")
	}
}

// ── shared source validation helpers ──────────────────────────────────────

// validateSeriesSlice validates each series in a chart widget's Series slice.
func validateSeriesSlice(series []WidgetSeries, add issueAdder, allowed []WidgetSourceType) {
	for i, s := range series {
		seriesAdd := func(field, reason string) {
			add(fmt.Sprintf("series[%d].%s", i, field), reason)
		}
		if s.Label == "" {
			add(fmt.Sprintf("series[%d].label", i), "series label is required")
		}
		validateSourceFields(s.Source, seriesAdd, allowed)
	}
}

// validateSourceFields checks a single WidgetSource against the given allowed types list
// and enforces per-type field rules. The add callback already carries the correct path
// prefix (either "source.*" for top-level sources or "series[i].source.*" for series).
func validateSourceFields(src WidgetSource, add issueAdder, allowed []WidgetSourceType) {
	if !knownSourceType(src.Type) {
		add("source.type", fmt.Sprintf("unknown source type %q", src.Type))
		return
	}
	if !slices.Contains(allowed, src.Type) {
		add("source.type", fmt.Sprintf("source type %q is not allowed for this widget", src.Type))
		return
	}

	// metric is only valid for sim_metric sources; forbid it on every other type.
	if src.Type != WidgetSourceSimMetric && src.Metric != "" {
		add("source.metric", "source.metric is only valid for sim_metric sources")
	}

	switch src.Type {
	case WidgetSourceAgentProperty:
		if src.PropertyUID == "" {
			add("source.property_uid", "agent_property source requires property_uid")
		}
		if src.Aggregation == "" {
			add("source.aggregation", "agent_property source requires aggregation")
		} else if !knownAggregation(src.Aggregation) {
			add("source.aggregation", fmt.Sprintf("unknown aggregation %q", src.Aggregation))
		}

	case WidgetSourceGlobalProperty:
		if src.PropertyUID == "" {
			add("source.property_uid", "global_property source requires property_uid")
		}
		if src.Aggregation != "" {
			add("source.aggregation", "global_property source must not specify aggregation")
		}
		if len(src.AgentUIDs) > 0 {
			add("source.agent_uids", "global_property source must not specify agent_uids")
		}

	case WidgetSourcePopulation:
		if src.PropertyUID != "" {
			add("source.property_uid", "population source must not specify property_uid")
		}
		if src.Aggregation != "" {
			add("source.aggregation", "population source must not specify aggregation")
		}
		// AgentUIDs are an optional filter — allowed.

	case WidgetSourceStateDistribution:
		if src.PropertyUID != "" {
			add("source.property_uid", "state_distribution source must not specify property_uid")
		}
		if src.Aggregation != "" {
			add("source.aggregation", "state_distribution source must not specify aggregation")
		}
		if len(src.AgentUIDs) != 1 {
			add("source.agent_uids", "state_distribution source requires exactly 1 agent_uid")
		}

	case WidgetSourceSimMetric:
		if src.Metric == "" {
			add("source.metric", "sim_metric source requires metric")
		} else if !knownWidgetMetric(src.Metric) {
			add("source.metric", fmt.Sprintf("unknown metric %q", src.Metric))
		}
		if src.PropertyUID != "" {
			add("source.property_uid", "sim_metric source must not specify property_uid")
		}
		if len(src.AgentUIDs) > 0 {
			add("source.agent_uids", "sim_metric source must not specify agent_uids")
		}
		if src.Aggregation != "" {
			add("source.aggregation", "sim_metric source must not specify aggregation")
		}
	}
}
