package common

import (
	"intelli.sim.core/data/common"
)

// WidgetSourceDocument is the bson form of common.WidgetSource.
type WidgetSourceDocument struct {
	Type        common.WidgetSourceType  `bson:"type"`
	PropertyUID string                   `bson:"property_uid,omitempty"`
	AgentUIDs   []string                 `bson:"agent_uids,omitempty"`
	Aggregation common.WidgetAggregation `bson:"aggregation,omitempty"`
	Metric      common.WidgetMetric      `bson:"metric,omitempty"`
}

// WidgetSeriesDocument is the bson form of common.WidgetSeries.
type WidgetSeriesDocument struct {
	Label  string               `bson:"label"`
	Color  string               `bson:"color,omitempty"`
	Source WidgetSourceDocument `bson:"source"`
}

// WidgetPositionDocument is the bson form of common.WidgetPosition.
type WidgetPositionDocument struct {
	X uint `bson:"x"`
	Y uint `bson:"y"`
	W uint `bson:"w"`
	H uint `bson:"h"`
}

// WidgetConfigDocument is the bson form of common.WidgetConfig.
type WidgetConfigDocument struct {
	ID       string                  `bson:"id"`
	Type     common.WidgetType       `bson:"type"`
	Title    string                  `bson:"title"`
	Source   *WidgetSourceDocument   `bson:"source,omitempty"`
	Series   []WidgetSeriesDocument  `bson:"series,omitempty"`
	Columns  []string                `bson:"columns,omitempty"`
	Buckets  *uint                   `bson:"buckets,omitempty"`
	LogKinds []common.WidgetLogKind  `bson:"log_kinds,omitempty"`
	Position WidgetPositionDocument  `bson:"position"`
}

// ---- mappers ----

func widgetSourceToDocument(s common.WidgetSource) WidgetSourceDocument {
	return WidgetSourceDocument{
		Type:        s.Type,
		PropertyUID: s.PropertyUID,
		AgentUIDs:   append([]string(nil), s.AgentUIDs...),
		Aggregation: s.Aggregation,
		Metric:      s.Metric,
	}
}

func widgetSourceFromDocument(d WidgetSourceDocument) common.WidgetSource {
	return common.WidgetSource{
		Type:        d.Type,
		PropertyUID: d.PropertyUID,
		AgentUIDs:   append([]string(nil), d.AgentUIDs...),
		Aggregation: d.Aggregation,
		Metric:      d.Metric,
	}
}

func widgetSourcePtrToDocument(s *common.WidgetSource) *WidgetSourceDocument {
	if s == nil {
		return nil
	}
	d := widgetSourceToDocument(*s)
	return &d
}

func widgetSourcePtrFromDocument(d *WidgetSourceDocument) *common.WidgetSource {
	if d == nil {
		return nil
	}
	s := widgetSourceFromDocument(*d)
	return &s
}

func widgetSeriesToDocument(s common.WidgetSeries) WidgetSeriesDocument {
	return WidgetSeriesDocument{
		Label:  s.Label,
		Color:  s.Color,
		Source: widgetSourceToDocument(s.Source),
	}
}

func widgetSeriesFromDocument(d WidgetSeriesDocument) common.WidgetSeries {
	return common.WidgetSeries{
		Label:  d.Label,
		Color:  d.Color,
		Source: widgetSourceFromDocument(d.Source),
	}
}

func widgetPositionToDocument(p common.WidgetPosition) WidgetPositionDocument {
	return WidgetPositionDocument{X: p.X, Y: p.Y, W: p.W, H: p.H}
}

func widgetPositionFromDocument(d WidgetPositionDocument) common.WidgetPosition {
	return common.WidgetPosition{X: d.X, Y: d.Y, W: d.W, H: d.H}
}

// WidgetConfigToDocument converts a common.WidgetConfig to its bson document form.
func WidgetConfigToDocument(w common.WidgetConfig) WidgetConfigDocument {
	return WidgetConfigDocument{
		ID:       w.ID,
		Type:     w.Type,
		Title:    w.Title,
		Source:   widgetSourcePtrToDocument(w.Source),
		Series:   MapSlice(w.Series, widgetSeriesToDocument),
		Columns:  append([]string(nil), w.Columns...),
		Buckets:  w.Buckets,
		LogKinds: append([]common.WidgetLogKind(nil), w.LogKinds...),
		Position: widgetPositionToDocument(w.Position),
	}
}

// WidgetConfigFromDocument converts a WidgetConfigDocument to its entity form.
func WidgetConfigFromDocument(d WidgetConfigDocument) common.WidgetConfig {
	return common.WidgetConfig{
		ID:       d.ID,
		Type:     d.Type,
		Title:    d.Title,
		Source:   widgetSourcePtrFromDocument(d.Source),
		Series:   MapSlice(d.Series, widgetSeriesFromDocument),
		Columns:  append([]string(nil), d.Columns...),
		Buckets:  d.Buckets,
		LogKinds: append([]common.WidgetLogKind(nil), d.LogKinds...),
		Position: widgetPositionFromDocument(d.Position),
	}
}

// WidgetConfigsToDocuments converts a slice of common.WidgetConfig to document form.
// nil in → nil out.
func WidgetConfigsToDocuments(in []common.WidgetConfig) []WidgetConfigDocument {
	return MapSlice(in, WidgetConfigToDocument)
}

// WidgetConfigsFromDocuments converts a slice of WidgetConfigDocument to entity form.
// nil in → nil out.
func WidgetConfigsFromDocuments(in []WidgetConfigDocument) []common.WidgetConfig {
	return MapSlice(in, WidgetConfigFromDocument)
}
