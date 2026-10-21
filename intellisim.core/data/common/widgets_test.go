package common

import (
	"slices"
	"strings"
	"testing"
)

// hasIssue returns true when issues contains an entry with the given path.
func hasIssue(issues []WidgetIssue, path string) bool {
	return slices.ContainsFunc(issues, func(i WidgetIssue) bool { return i.Path == path })
}

// ptr returns a pointer to the provided uint value (helper for Buckets).
func uptr(v uint) *uint { return &v }

// ── happy paths ────────────────────────────────────────────────────────────

func TestValidateWidget_HappyPaths(t *testing.T) {
	tests := []struct {
		name   string
		widget WidgetConfig
	}{
		{
			name: "monitor_agent_property",
			widget: WidgetConfig{
				ID:    "w1",
				Type:  WidgetMonitor,
				Title: "Age",
				Source: &WidgetSource{
					Type:        WidgetSourceAgentProperty,
					PropertyUID: "prop-age",
					Aggregation: AggAvg,
					AgentUIDs:   []string{"agent-1"},
				},
				Position: WidgetPosition{X: 0, Y: 0, W: 2, H: 2},
			},
		},
		{
			name: "monitor_global_property",
			widget: WidgetConfig{
				ID:    "w2",
				Type:  WidgetMonitor,
				Title: "Temperature",
				Source: &WidgetSource{
					Type:        WidgetSourceGlobalProperty,
					PropertyUID: "prop-temp",
				},
				Position: WidgetPosition{X: 0, Y: 0, W: 2, H: 2},
			},
		},
		{
			name: "monitor_population",
			widget: WidgetConfig{
				ID:    "w3",
				Type:  WidgetMonitor,
				Title: "Count",
				Source: &WidgetSource{
					Type:      WidgetSourcePopulation,
					AgentUIDs: []string{"agent-1"},
				},
				Position: WidgetPosition{X: 0, Y: 0, W: 2, H: 2},
			},
		},
		{
			name: "line_chart",
			widget: WidgetConfig{
				ID:    "w4",
				Type:  WidgetLineChart,
				Title: "Over time",
				Series: []WidgetSeries{
					{Label: "Pop", Source: WidgetSource{Type: WidgetSourcePopulation}},
				},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "bar_chart_series",
			widget: WidgetConfig{
				ID:    "w5",
				Type:  WidgetBarChart,
				Title: "Bar",
				Series: []WidgetSeries{
					{Label: "Pop", Source: WidgetSource{Type: WidgetSourcePopulation}},
				},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "bar_chart_source_population",
			widget: WidgetConfig{
				ID:    "w6",
				Type:  WidgetBarChart,
				Title: "Bar2",
				Source: &WidgetSource{Type: WidgetSourcePopulation},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "bar_chart_source_state_distribution",
			widget: WidgetConfig{
				ID:    "w7",
				Type:  WidgetBarChart,
				Title: "States",
				Source: &WidgetSource{
					Type:      WidgetSourceStateDistribution,
					AgentUIDs: []string{"agent-1"},
				},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "pie_chart_population",
			widget: WidgetConfig{
				ID:    "w8",
				Type:  WidgetPieChart,
				Title: "Pie",
				Source: &WidgetSource{Type: WidgetSourcePopulation},
				Position: WidgetPosition{W: 3, H: 3},
			},
		},
		{
			name: "pie_chart_state_distribution",
			widget: WidgetConfig{
				ID:    "w9",
				Type:  WidgetPieChart,
				Title: "Pie2",
				Source: &WidgetSource{
					Type:      WidgetSourceStateDistribution,
					AgentUIDs: []string{"agent-1"},
				},
				Position: WidgetPosition{W: 3, H: 3},
			},
		},
		{
			name: "histogram_no_buckets",
			widget: WidgetConfig{
				ID:    "w10",
				Type:  WidgetHistogram,
				Title: "Hist",
				Source: &WidgetSource{
					Type:        WidgetSourceAgentProperty,
					PropertyUID: "prop-age",
				},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "histogram_with_valid_buckets",
			widget: WidgetConfig{
				ID:    "w11",
				Type:  WidgetHistogram,
				Title: "Hist2",
				Source: &WidgetSource{
					Type:        WidgetSourceAgentProperty,
					PropertyUID: "prop-age",
				},
				Buckets:  uptr(10),
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "agent_table",
			widget: WidgetConfig{
				ID:    "w12",
				Type:  WidgetAgentTable,
				Title: "Agents",
				Source: &WidgetSource{
					Type:      WidgetSourceAgentProperty,
					AgentUIDs: []string{"agent-1"},
				},
				Columns:  []string{"name", "age"},
				Position: WidgetPosition{W: 6, H: 4},
			},
		},
		{
			name: "text_log",
			widget: WidgetConfig{
				ID:       "w13",
				Type:     WidgetTextLog,
				Title:    "Log",
				LogKinds: []WidgetLogKind{LogEvents, LogStateTransitions},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
		{
			name: "monitor_sim_metric",
			widget: WidgetConfig{
				ID:    "w14",
				Type:  WidgetMonitor,
				Title: "Total agents",
				Source: &WidgetSource{
					Type:   WidgetSourceSimMetric,
					Metric: MetricTotalAgents,
				},
				Position: WidgetPosition{W: 2, H: 2},
			},
		},
		{
			name: "line_chart_sim_metric_series",
			widget: WidgetConfig{
				ID:    "w15",
				Type:  WidgetLineChart,
				Title: "Interactions over time",
				Series: []WidgetSeries{
					{Label: "Interactions", Source: WidgetSource{Type: WidgetSourceSimMetric, Metric: MetricInteractions}},
				},
				Position: WidgetPosition{W: 4, H: 3},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := ValidateWidget(0, tc.widget)
			if len(issues) != 0 {
				t.Errorf("expected no issues, got %d: %+v", len(issues), issues)
			}
		})
	}
}

// ── failure cases ──────────────────────────────────────────────────────────

func TestValidateWidget_MissingIDAndTitle(t *testing.T) {
	w := WidgetConfig{
		Type:   WidgetMonitor,
		Source: &WidgetSource{Type: WidgetSourcePopulation},
	}
	issues := ValidateWidget(3, w)
	if !hasIssue(issues, "widgets[3].id") {
		t.Error("expected widgets[3].id issue for missing ID")
	}
	if !hasIssue(issues, "widgets[3].title") {
		t.Error("expected widgets[3].title issue for missing title")
	}
}

func TestValidateWidget_UnknownType(t *testing.T) {
	w := WidgetConfig{
		ID:    "wx",
		Type:  WidgetType("sparkline"),
		Title: "Unknown",
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].type") {
		t.Errorf("expected widgets[0].type issue, got %+v", issues)
	}
	if len(issues) != 1 {
		t.Errorf("expected exactly 1 issue (early return after unknown type), got %d: %+v", len(issues), issues)
	}
}

func TestValidateWidget_Monitor_WithSeries(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "prop-age",
			Aggregation: AggAvg,
		},
		Series: []WidgetSeries{
			{Label: "X", Source: WidgetSource{Type: WidgetSourcePopulation}},
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].series") {
		t.Errorf("expected widgets[0].series issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_AgentPropertyWithoutAggregation(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "prop-age",
			// Aggregation deliberately omitted
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.aggregation") {
		t.Errorf("expected widgets[0].source.aggregation issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_AgentPropertyWithoutPropertyUID(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			Aggregation: AggAvg,
			// PropertyUID deliberately omitted
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.property_uid") {
		t.Errorf("expected widgets[0].source.property_uid issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_GlobalPropertyWithAggregation(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceGlobalProperty,
			PropertyUID: "prop-temp",
			Aggregation: AggSum,
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.aggregation") {
		t.Errorf("expected widgets[0].source.aggregation forbidden issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_NoSource(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetMonitor,
		Title:    "Monitor",
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source") {
		t.Errorf("expected widgets[0].source required issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_UnknownSourceType(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceType("live_feed"),
			PropertyUID: "p1",
			Aggregation: AggAvg,
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.type") {
		t.Errorf("expected widgets[0].source.type unknown-value issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_UnknownAggregation(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "p1",
			Aggregation: WidgetAggregation("mode"),
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.aggregation") {
		t.Errorf("expected widgets[0].source.aggregation unknown-value issue, got %+v", issues)
	}
}

func TestValidateWidget_LineChart_ZeroSeries(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetLineChart,
		Title:    "Chart",
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].series") {
		t.Errorf("expected widgets[0].series issue for empty series, got %+v", issues)
	}
}

func TestValidateWidget_LineChart_SeriesMissingLabel(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetLineChart,
		Title: "Chart",
		Series: []WidgetSeries{
			{Label: "", Source: WidgetSource{Type: WidgetSourcePopulation}},
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].series[0].label") {
		t.Errorf("expected widgets[0].series[0].label issue, got %+v", issues)
	}
}

func TestValidateWidget_LineChart_Series_GlobalProperty_AgentUIDsForbidden(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetLineChart,
		Title: "Chart",
		Series: []WidgetSeries{
			{
				Label: "Temp",
				Source: WidgetSource{
					Type:        WidgetSourceGlobalProperty,
					PropertyUID: "prop-temp",
					AgentUIDs:   []string{"agent-1"},
				},
			},
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].series[0].source.agent_uids") {
		t.Errorf("expected widgets[0].series[0].source.agent_uids forbidden issue for global_property in series, got %+v", issues)
	}
}

func TestValidateWidget_BarChart_BothSeriesAndSource(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetBarChart,
		Title: "Bar",
		Source: &WidgetSource{Type: WidgetSourcePopulation},
		Series: []WidgetSeries{
			{Label: "X", Source: WidgetSource{Type: WidgetSourcePopulation}},
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	// validateBarChart emits the conflict to "source" and returns early.
	if !hasIssue(issues, "widgets[0].source") {
		t.Errorf("expected widgets[0].source conflict issue, got %+v", issues)
	}
}

func TestValidateWidget_BarChart_NeitherSeriesNorSource(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetBarChart,
		Title:    "Bar",
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	// validateBarChart emits the missing requirement to "source" and returns early.
	if !hasIssue(issues, "widgets[0].source") {
		t.Errorf("expected widgets[0].source missing-source-or-series issue, got %+v", issues)
	}
}

func TestValidateWidget_BarChart_StateDistribution_TwoAgentUIDs(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetBarChart,
		Title: "Bar",
		Source: &WidgetSource{
			Type:      WidgetSourceStateDistribution,
			AgentUIDs: []string{"agent-1", "agent-2"},
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.agent_uids") {
		t.Errorf("expected widgets[0].source.agent_uids issue for 2 UIDs, got %+v", issues)
	}
}

func TestValidateWidget_PieChart_AgentPropertySource(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetPieChart,
		Title: "Pie",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "p1",
			Aggregation: AggAvg,
		},
		Position: WidgetPosition{W: 3, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.type") {
		t.Errorf("expected widgets[0].source.type issue for agent_property in pie, got %+v", issues)
	}
}

func TestValidateWidget_PieChart_WithSeries(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetPieChart,
		Title: "Pie",
		Source: &WidgetSource{Type: WidgetSourcePopulation},
		Series: []WidgetSeries{
			{Label: "X", Source: WidgetSource{Type: WidgetSourcePopulation}},
		},
		Position: WidgetPosition{W: 3, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].series") {
		t.Errorf("expected widgets[0].series forbidden issue, got %+v", issues)
	}
}

func TestValidateWidget_Histogram_WithAggregation(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetHistogram,
		Title: "Hist",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "prop-age",
			Aggregation: AggAvg,
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.aggregation") {
		t.Errorf("expected widgets[0].source.aggregation forbidden issue for histogram, got %+v", issues)
	}
}

func TestValidateWidget_Histogram_BucketsOne(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetHistogram,
		Title: "Hist",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "prop-age",
		},
		Buckets:  uptr(1),
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].buckets") {
		t.Errorf("expected widgets[0].buckets issue for Buckets=1, got %+v", issues)
	}
}

func TestValidateWidget_Histogram_BucketsZero(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetHistogram,
		Title: "Hist",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "prop-age",
		},
		Buckets:  uptr(0),
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].buckets") {
		t.Errorf("expected widgets[0].buckets issue for Buckets=0, got %+v", issues)
	}
}

func TestValidateWidget_AgentTable_WithoutColumns(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetAgentTable,
		Title: "Table",
		Source: &WidgetSource{
			Type:      WidgetSourceAgentProperty,
			AgentUIDs: []string{"agent-1"},
		},
		Columns:  []string{},
		Position: WidgetPosition{W: 6, H: 4},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].columns") {
		t.Errorf("expected widgets[0].columns issue, got %+v", issues)
	}
}

func TestValidateWidget_AgentTable_TwoAgentUIDs(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetAgentTable,
		Title: "Table",
		Source: &WidgetSource{
			Type:      WidgetSourceAgentProperty,
			AgentUIDs: []string{"agent-1", "agent-2"},
		},
		Columns:  []string{"name"},
		Position: WidgetPosition{W: 6, H: 4},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.agent_uids") {
		t.Errorf("expected widgets[0].source.agent_uids issue for 2 UIDs, got %+v", issues)
	}
}

func TestValidateWidget_AgentTable_WithPropertyUID(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetAgentTable,
		Title: "Table",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			AgentUIDs:   []string{"agent-1"},
			PropertyUID: "prop-age",
		},
		Columns:  []string{"name"},
		Position: WidgetPosition{W: 6, H: 4},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.property_uid") {
		t.Errorf("expected widgets[0].source.property_uid forbidden issue for agent_table, got %+v", issues)
	}
}

func TestValidateWidget_AgentTable_NoSource(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetAgentTable,
		Title:    "Table",
		Columns:  []string{"name"},
		Position: WidgetPosition{W: 6, H: 4},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source") {
		t.Errorf("expected widgets[0].source required issue, got %+v", issues)
	}
}

func TestValidateWidget_AgentTable_WrongSourceType(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetAgentTable,
		Title: "Table",
		Source: &WidgetSource{
			Type:      WidgetSourcePopulation,
			AgentUIDs: []string{"agent-1"},
		},
		Columns:  []string{"name"},
		Position: WidgetPosition{W: 6, H: 4},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.type") {
		t.Errorf("expected widgets[0].source.type issue for wrong type in agent_table, got %+v", issues)
	}
}

func TestValidateWidget_TextLog_WithoutKinds(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetTextLog,
		Title:    "Log",
		LogKinds: []WidgetLogKind{},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].log_kinds") {
		t.Errorf("expected widgets[0].log_kinds issue, got %+v", issues)
	}
}

func TestValidateWidget_TextLog_DuplicateKinds(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetTextLog,
		Title:    "Log",
		LogKinds: []WidgetLogKind{LogEvents, LogEvents},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].log_kinds") {
		t.Errorf("expected widgets[0].log_kinds duplicate issue, got %+v", issues)
	}
}

func TestValidateWidget_TextLog_UnknownKind(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetTextLog,
		Title:    "Log",
		LogKinds: []WidgetLogKind{WidgetLogKind("bogus")},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	found := false
	for _, iss := range issues {
		if iss.Path == "widgets[0].log_kinds" && strings.Contains(iss.Reason, "bogus") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected widgets[0].log_kinds issue mentioning %q, got %+v", "bogus", issues)
	}
}

func TestValidateWidget_TextLog_WithSource(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetTextLog,
		Title:    "Log",
		LogKinds: []WidgetLogKind{LogEvents},
		Source:   &WidgetSource{Type: WidgetSourcePopulation},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source") {
		t.Errorf("expected widgets[0].source forbidden issue for text_log, got %+v", issues)
	}
}

func TestValidateWidget_TextLog_WithSeries(t *testing.T) {
	w := WidgetConfig{
		ID:       "w1",
		Type:     WidgetTextLog,
		Title:    "Log",
		LogKinds: []WidgetLogKind{LogEvents},
		Series: []WidgetSeries{
			{Label: "X", Source: WidgetSource{Type: WidgetSourcePopulation}},
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].series") {
		t.Errorf("expected widgets[0].series forbidden issue for text_log, got %+v", issues)
	}
}

// ── sim_metric failure cases ────────────────────────────────────────────────

func TestValidateWidget_Monitor_SimMetric_MissingMetric(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type: WidgetSourceSimMetric,
			// Metric deliberately omitted
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.metric") {
		t.Errorf("expected widgets[0].source.metric required issue, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_SimMetric_UnknownMetric(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:   WidgetSourceSimMetric,
			Metric: WidgetMetric("births"),
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	found := false
	for _, iss := range issues {
		if iss.Path == "widgets[0].source.metric" && strings.Contains(iss.Reason, "births") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected widgets[0].source.metric issue mentioning %q, got %+v", "births", issues)
	}
}

func TestValidateWidget_Monitor_SimMetric_WithPropertyUID(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceSimMetric,
			Metric:      MetricTotalAgents,
			PropertyUID: "prop-age",
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.property_uid") {
		t.Errorf("expected widgets[0].source.property_uid forbidden issue for sim_metric, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_SimMetric_WithAgentUIDs(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:      WidgetSourceSimMetric,
			Metric:    MetricTotalAgents,
			AgentUIDs: []string{"agent-1"},
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.agent_uids") {
		t.Errorf("expected widgets[0].source.agent_uids forbidden issue for sim_metric, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_SimMetric_WithAggregation(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceSimMetric,
			Metric:      MetricTotalAgents,
			Aggregation: AggAvg,
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.aggregation") {
		t.Errorf("expected widgets[0].source.aggregation forbidden issue for sim_metric, got %+v", issues)
	}
}

func TestValidateWidget_PieChart_SimMetricSource(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetPieChart,
		Title: "Pie",
		Source: &WidgetSource{
			Type:   WidgetSourceSimMetric,
			Metric: MetricTotalAgents,
		},
		Position: WidgetPosition{W: 3, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.type") {
		t.Errorf("expected widgets[0].source.type issue for sim_metric in pie_chart, got %+v", issues)
	}
}

func TestValidateWidget_BarChart_SimMetricSource(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetBarChart,
		Title: "Bar",
		Source: &WidgetSource{
			Type:   WidgetSourceSimMetric,
			Metric: MetricTotalAgents,
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.type") {
		t.Errorf("expected widgets[0].source.type issue for sim_metric in bar_chart single-source, got %+v", issues)
	}
}

func TestValidateWidget_Histogram_SimMetricSource(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetHistogram,
		Title: "Hist",
		Source: &WidgetSource{
			Type:   WidgetSourceSimMetric,
			Metric: MetricTotalAgents,
		},
		Position: WidgetPosition{W: 4, H: 3},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.type") {
		t.Errorf("expected widgets[0].source.type issue for sim_metric in histogram, got %+v", issues)
	}
}

func TestValidateWidget_Monitor_AgentProperty_WithMetricForbidden(t *testing.T) {
	w := WidgetConfig{
		ID:    "w1",
		Type:  WidgetMonitor,
		Title: "Monitor",
		Source: &WidgetSource{
			Type:        WidgetSourceAgentProperty,
			PropertyUID: "prop-age",
			Aggregation: AggAvg,
			Metric:      MetricTotalAgents,
		},
		Position: WidgetPosition{W: 2, H: 2},
	}
	issues := ValidateWidget(0, w)
	if !hasIssue(issues, "widgets[0].source.metric") {
		t.Errorf("expected widgets[0].source.metric forbidden issue for agent_property, got %+v", issues)
	}
}

func TestValidateWidget_IndexUsedInPath(t *testing.T) {
	w := WidgetConfig{
		// ID and title deliberately missing to trigger path issues at non-zero index
		Type:   WidgetMonitor,
		Source: &WidgetSource{Type: WidgetSourcePopulation},
	}
	issues := ValidateWidget(5, w)
	if !hasIssue(issues, "widgets[5].id") {
		t.Errorf("expected widgets[5].id in path, got %+v", issues)
	}
	if !hasIssue(issues, "widgets[5].title") {
		t.Errorf("expected widgets[5].title in path, got %+v", issues)
	}
}
