package simulations

// This file defines the JSON response shapes the simulation generator expects
// back from the LLM for the four generation purposes (count, global values,
// per-instance agent values, placement directives). The corresponding requests
// are plain prompt text built in prompt.go; only the responses are structured.
//
// All shapes are decoded with encoding/json and then run through the fail-soft
// mappers in llmmapper.go, which validate/coerce every field and substitute a
// deterministic-random fallback for anything missing or malformed. Nothing here
// is trusted verbatim.

// LLMGlobalValuesResponse is the LLM reply for environment global-property
// initialisation: a flat map of propertyUID -> value.
type LLMGlobalValuesResponse struct {
	Values map[string]any `json:"values"`
}

// LLMAgentBatchResponse is the LLM reply for one batch of agent instances.
type LLMAgentBatchResponse struct {
	Instances []LLMInstanceValues `json:"instances"`
}

// LLMInstanceValues carries the property values for a single agent instance
// within a batch.
type LLMInstanceValues struct {
	// Instance is the 0-based index of this instance within the batch.
	Instance int `json:"instance"`
	// Values maps propertyUID -> value.
	Values map[string]any `json:"values"`
}

// LLMCountResponse is the LLM reply for archetype population-count selection.
type LLMCountResponse struct {
	Count     uint64 `json:"count"`
	Rationale string `json:"rationale,omitempty"`
}

// LLMPlacementResponse is the LLM reply for spatial placement: a list of
// directives the generator translates into PlacementDirective values.
type LLMPlacementResponse struct {
	Directives []LLMDirective `json:"directives"`
}

// LLMDirective is one placement instruction. Kind selects which optional fields
// are meaningful: cluster (center+radius), region (polygon), at_shape
// (shape_uid), explicit (points).
type LLMDirective struct {
	Kind     string     `json:"kind"`
	Fraction float64    `json:"fraction,omitempty"`
	Center   *LLMPoint  `json:"center,omitempty"`
	Radius   *float64   `json:"radius,omitempty"`
	Polygon  []LLMPoint `json:"polygon,omitempty"`
	ShapeUID string     `json:"shape_uid,omitempty"`
	Points   []LLMPoint `json:"points,omitempty"`
}

// LLMPoint is a floating-point coordinate as returned by the LLM; the mappers
// round and clamp it into the integer environment grid.
type LLMPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
