package simulations

import (
	"intelli.sim.core/data/common"
)

// Aggregation selects the accumulator for AggregatePropertyByStep.
type Aggregation string

const (
	AggregationAvg   Aggregation = "avg"
	AggregationSum   Aggregation = "sum"
	AggregationMin   Aggregation = "min"
	AggregationMax   Aggregation = "max"
	AggregationCount Aggregation = "count"
)

// IsValidAggregation reports whether a is one of the five known accumulator
// values. An empty string is NOT valid here — callers should check for empty
// before calling this (empty means "use the default", not "any value is ok").
func IsValidAggregation(a Aggregation) bool {
	switch a {
	case AggregationAvg, AggregationSum, AggregationMin, AggregationMax, AggregationCount:
		return true
	}
	return false
}

// StepRange bounds a query to [Start..End] (inclusive); nil bound = open.
type StepRange struct {
	Start *int64 `json:"start,omitempty"`
	End   *int64 `json:"end,omitempty"`
}

// TimeSeriesPoint is one aggregated value per step.
type TimeSeriesPoint struct {
	Step  int64       `json:"step"`
	Value interface{} `json:"value"` // plain Go numeric (normalized)
}

// PopulationPoint is the per-definition agent count at one step.
// Count is keyed by the model definition UID (StepMetadata.UID).
type PopulationPoint struct {
	Step  int64          `json:"step"`
	Count map[string]int `json:"count"`
}

// SpatialAgentData is the spatial state of one agent instance at a step.
type SpatialAgentData struct {
	ReferenceID uint64                 `json:"reference_id"` // runtime instance id
	UID         string                 `json:"uid"`          // model definition UID
	Type        MetadataType           `json:"type"`
	Position    *common.Point          `json:"position,omitempty"`
	State       string                 `json:"state,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// PropertyInfo describes one property observed in the step stream.
type PropertyInfo struct {
	UID      string `json:"uid"` // property UID (key of SimulationStep.Properties)
	DataType string `json:"data_type"`
}
