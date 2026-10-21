package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// --- MapGlobalValues -----------------------------------------------------------

func TestMapGlobalValues_HappyPath(t *testing.T) {
	props := []datamodels.AgentProperty{
		numberProp("speed", f64ptr(0), f64ptr(100)),
		{UID: "kind", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"a", "b"}},
		{UID: "alive", Type: datamodels.PropertyTypeBoolean},
		{UID: "label", Type: datamodels.PropertyTypeString},
	}
	resp := LLMGlobalValuesResponse{Values: map[string]any{
		"speed": float64(42),
		"kind":  "b",
		"alive": true,
		"label": "hello",
	}}
	rng := RunRNG(1, 1)
	out, warns := MapGlobalValues(resp, props, rng)

	if len(warns) != 0 {
		t.Fatalf("expected no warnings, got %v", warns)
	}
	if out["speed"].(float64) != 42 {
		t.Errorf("speed: got %v", out["speed"])
	}
	if out["kind"].(string) != "b" {
		t.Errorf("kind: got %v", out["kind"])
	}
	if out["alive"].(bool) != true {
		t.Errorf("alive: got %v", out["alive"])
	}
	if out["label"].(string) != "hello" {
		t.Errorf("label: got %v", out["label"])
	}
}

func TestMapGlobalValues_InvalidFallsBack(t *testing.T) {
	props := []datamodels.AgentProperty{
		numberProp("speed", f64ptr(0), f64ptr(100)),
		{UID: "kind", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"a", "b"}},
	}
	resp := LLMGlobalValuesResponse{Values: map[string]any{
		"speed": "not-a-number",
		"kind":  "z", // not in allowed values
	}}
	rng := RunRNG(1, 1)
	out, warns := MapGlobalValues(resp, props, rng)

	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warns), warns)
	}
	// Every prop present in the output regardless.
	if _, ok := out["speed"]; !ok {
		t.Errorf("speed missing from output")
	}
	if _, ok := out["kind"]; !ok {
		t.Errorf("kind missing from output")
	}
	sp := out["speed"].(float64)
	if sp < 0 || sp > 100 {
		t.Errorf("fallback speed %v out of bounds", sp)
	}
	if k := out["kind"].(string); k != "a" && k != "b" {
		t.Errorf("fallback kind %q not in allowed values", k)
	}
}

func TestMapGlobalValues_MissingFallsBack(t *testing.T) {
	props := []datamodels.AgentProperty{numberProp("speed", f64ptr(0), f64ptr(10))}
	rng := RunRNG(1, 1)
	out, warns := MapGlobalValues(LLMGlobalValuesResponse{Values: nil}, props, rng)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning for missing value, got %d", len(warns))
	}
	if _, ok := out["speed"]; !ok {
		t.Errorf("speed missing from output")
	}
}

// --- MapAgentBatch -------------------------------------------------------------

func batchProps() []datamodels.AgentProperty {
	return []datamodels.AgentProperty{
		numberProp("energy", f64ptr(0), f64ptr(100)),
		{UID: "mood", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"calm", "angry"}},
	}
}

func TestMapAgentBatch_ExactLenAndHappy(t *testing.T) {
	resp := LLMAgentBatchResponse{Instances: []LLMInstanceValues{
		{Instance: 0, Values: map[string]any{"energy": float64(50), "mood": "calm"}},
		{Instance: 1, Values: map[string]any{"energy": float64(70), "mood": "angry"}},
		{Instance: 2, Values: map[string]any{"energy": float64(30), "mood": "calm"}},
	}}
	rng := RunRNG(1, 1)
	out, warns := MapAgentBatch(resp, batchProps(), 3, rng)
	if len(out) != 3 {
		t.Fatalf("expected 3 maps, got %d", len(out))
	}
	if len(warns) != 0 {
		t.Fatalf("expected no warnings, got %v", warns)
	}
	if out[1]["energy"].(float64) != 70 {
		t.Errorf("instance 1 energy: got %v", out[1]["energy"])
	}
	if out[2]["mood"].(string) != "calm" {
		t.Errorf("instance 2 mood: got %v", out[2]["mood"])
	}
}

func TestMapAgentBatch_MissingInstanceFullyFilled(t *testing.T) {
	resp := LLMAgentBatchResponse{Instances: []LLMInstanceValues{
		{Instance: 0, Values: map[string]any{"energy": float64(50), "mood": "calm"}},
		// instance 1 absent
		{Instance: 2, Values: map[string]any{"energy": float64(30), "mood": "angry"}},
	}}
	rng := RunRNG(1, 1)
	out, warns := MapAgentBatch(resp, batchProps(), 3, rng)
	if len(out) != 3 {
		t.Fatalf("expected 3 maps, got %d", len(out))
	}
	// instance 1 must be fully filled with both props.
	if _, ok := out[1]["energy"]; !ok {
		t.Errorf("missing instance 1 energy not filled")
	}
	if _, ok := out[1]["mood"]; !ok {
		t.Errorf("missing instance 1 mood not filled")
	}
	if len(warns) == 0 {
		t.Errorf("expected at least one warning for missing instance")
	}
}

func TestMapAgentBatch_OutOfRangeIgnored(t *testing.T) {
	resp := LLMAgentBatchResponse{Instances: []LLMInstanceValues{
		{Instance: 0, Values: map[string]any{"energy": float64(50), "mood": "calm"}},
		{Instance: 9, Values: map[string]any{"energy": float64(99), "mood": "angry"}}, // out of range for batchLen=2
	}}
	rng := RunRNG(1, 1)
	out, warns := MapAgentBatch(resp, batchProps(), 2, rng)
	if len(out) != 2 {
		t.Fatalf("expected 2 maps, got %d", len(out))
	}
	// Expect exactly 2 warnings:
	//   [0] instances[9] — out-of-range response index (LLM RESPONSE index)
	//   [1] instances[1] — missing from response; randomized (OUTPUT index)
	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warns), warns)
	}
	foundOOR := false
	for _, w := range warns {
		if w.Path == "instances[9]" {
			foundOOR = true
		}
	}
	if !foundOOR {
		t.Errorf("expected a warning with Path \"instances[9]\", got %v", warns)
	}
}

func TestMapAgentBatch_MalformedFieldFallback(t *testing.T) {
	resp := LLMAgentBatchResponse{Instances: []LLMInstanceValues{
		{Instance: 0, Values: map[string]any{"energy": "bad", "mood": "calm"}},
	}}
	rng := RunRNG(1, 1)
	out, warns := MapAgentBatch(resp, batchProps(), 1, rng)
	if len(out) != 1 {
		t.Fatalf("expected 1 map, got %d", len(out))
	}
	e := out[0]["energy"].(float64)
	if e < 0 || e > 100 {
		t.Errorf("fallback energy %v out of bounds", e)
	}
	if len(warns) == 0 {
		t.Errorf("expected warning for malformed energy")
	}
}

func TestMapAgentBatch_NilSafe(t *testing.T) {
	// nil response, nil props, nil instance values must not panic.
	rng := RunRNG(1, 1)
	out, _ := MapAgentBatch(LLMAgentBatchResponse{}, nil, 2, rng)
	if len(out) != 2 {
		t.Fatalf("expected 2 maps, got %d", len(out))
	}
	resp := LLMAgentBatchResponse{Instances: []LLMInstanceValues{{Instance: 0, Values: nil}}}
	out2, _ := MapAgentBatch(resp, batchProps(), 1, rng)
	if len(out2) != 1 {
		t.Fatalf("expected 1 map, got %d", len(out2))
	}
}

// --- MapCount ------------------------------------------------------------------

func TestMapCount_ZeroUsesPopulation(t *testing.T) {
	init := datamodels.AgentInitialization{PopulationCount: 42}
	c, w := MapCount(LLMCountResponse{Count: 0}, init)
	if c != 42 {
		t.Errorf("expected 42, got %d", c)
	}
	if w == nil {
		t.Errorf("expected a warning for zero count")
	}
}

func TestMapCount_ClampLow(t *testing.T) {
	init := datamodels.AgentInitialization{PopulationCount: 10, MinPopulation: u64ptr(5)}
	c, w := MapCount(LLMCountResponse{Count: 2}, init)
	if c != 5 {
		t.Errorf("expected clamp to 5, got %d", c)
	}
	if w == nil {
		t.Errorf("expected a warning for low clamp")
	}
}

func TestMapCount_ClampHigh(t *testing.T) {
	init := datamodels.AgentInitialization{PopulationCount: 10, MaxPopulation: u64ptr(50)}
	c, w := MapCount(LLMCountResponse{Count: 999}, init)
	if c != 50 {
		t.Errorf("expected clamp to 50, got %d", c)
	}
	if w == nil {
		t.Errorf("expected a warning for high clamp")
	}
}

func TestMapCount_InRangeUntouched(t *testing.T) {
	init := datamodels.AgentInitialization{PopulationCount: 10, MinPopulation: u64ptr(5), MaxPopulation: u64ptr(50)}
	c, w := MapCount(LLMCountResponse{Count: 20}, init)
	if c != 20 {
		t.Errorf("expected 20, got %d", c)
	}
	if w != nil {
		t.Errorf("expected no warning, got %v", w)
	}
}

// --- MapDirectives -------------------------------------------------------------

func directiveEnv() *datasims.SimulationEnvironment {
	return &datasims.SimulationEnvironment{
		Width:  100,
		Height: 80,
		Shapes: []datasims.SimulationShape{
			{ShapeUID: "lake", Name: "Lake", Type: datamodels.ShapeTypeCircle, Position: common.Point{X: 50, Y: 40}, Radius: f64ptr(10)},
		},
	}
}

func TestMapDirectives_EachKindHappy(t *testing.T) {
	r := float64(5)
	resp := LLMPlacementResponse{Directives: []LLMDirective{
		{Kind: "cluster", Fraction: 0.25, Center: &LLMPoint{X: 30, Y: 30}, Radius: &r},
		{Kind: "region", Fraction: 0.25, Polygon: []LLMPoint{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 5, Y: 10}}},
		{Kind: "at_shape", Fraction: 0.25, ShapeUID: "lake"},
		{Kind: "explicit", Points: []LLMPoint{{X: 1, Y: 1}, {X: 2, Y: 2}}},
	}}
	dirs, warns := MapDirectives(resp, directiveEnv(), 20)
	if len(dirs) != 4 {
		t.Fatalf("expected 4 directives kept, got %d (warns %v)", len(dirs), warns)
	}
	if len(warns) != 0 {
		t.Fatalf("expected no warnings, got %v", warns)
	}
	if dirs[0].Kind != "cluster" || dirs[0].Center == nil {
		t.Errorf("cluster directive malformed: %+v", dirs[0])
	}
	if dirs[1].Kind != "region" || len(dirs[1].Polygon) != 3 {
		t.Errorf("region directive malformed: %+v", dirs[1])
	}
	if dirs[2].Kind != "at_shape" || dirs[2].ShapeUID != "lake" {
		t.Errorf("at_shape directive malformed: %+v", dirs[2])
	}
	if dirs[3].Kind != "explicit" || len(dirs[3].Points) != 2 {
		t.Errorf("explicit directive malformed: %+v", dirs[3])
	}
}

func TestMapDirectives_UnknownKindSkipped(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{{Kind: "teleport"}}}
	dirs, warns := MapDirectives(resp, directiveEnv(), 10)
	if len(dirs) != 0 {
		t.Errorf("expected 0 directives, got %d", len(dirs))
	}
	if len(warns) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warns))
	}
}

func TestMapDirectives_ClusterNoCenterSkipped(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{{Kind: "cluster"}}}
	dirs, warns := MapDirectives(resp, directiveEnv(), 10)
	if len(dirs) != 0 {
		t.Errorf("expected 0 directives, got %d", len(dirs))
	}
	if len(warns) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warns))
	}
}

func TestMapDirectives_RegionTooFewPointsSkipped(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{{Kind: "region", Polygon: []LLMPoint{{X: 0, Y: 0}, {X: 1, Y: 1}}}}}
	dirs, warns := MapDirectives(resp, directiveEnv(), 10)
	if len(dirs) != 0 {
		t.Errorf("expected 0 directives, got %d", len(dirs))
	}
	if len(warns) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warns))
	}
}

func TestMapDirectives_AtShapeUnknownUIDSkipped(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{{Kind: "at_shape", ShapeUID: "ghost"}}}
	dirs, warns := MapDirectives(resp, directiveEnv(), 10)
	if len(dirs) != 0 {
		t.Errorf("expected 0 directives, got %d", len(dirs))
	}
	if len(warns) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warns))
	}
}

func TestMapDirectives_ExplicitOver50Skipped(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{{Kind: "explicit", Points: []LLMPoint{{X: 1, Y: 1}}}}}
	dirs, warns := MapDirectives(resp, directiveEnv(), 60)
	if len(dirs) != 0 {
		t.Errorf("expected 0 directives for >50 explicit, got %d", len(dirs))
	}
	if len(warns) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warns))
	}
}

func TestMapDirectives_CoordinatesClamped(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{
		{Kind: "cluster", Center: &LLMPoint{X: 9999, Y: -50}},
		{Kind: "explicit", Points: []LLMPoint{{X: 9999, Y: 9999}}},
	}}
	dirs, _ := MapDirectives(resp, directiveEnv(), 10)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 directives, got %d", len(dirs))
	}
	c := dirs[0].Center
	if c == nil || c.X >= 100 || c.Y >= 80 {
		t.Errorf("cluster center not clamped: %+v", c)
	}
	p := dirs[1].Points[0]
	if p.X >= 100 || p.Y >= 80 {
		t.Errorf("explicit point not clamped: %+v", p)
	}
}

func TestMapDirectives_NilEnvSafe(t *testing.T) {
	resp := LLMPlacementResponse{Directives: []LLMDirective{
		{Kind: "cluster", Center: &LLMPoint{X: 1, Y: 1}},
	}}
	dirs, warns := MapDirectives(resp, nil, 10)
	// coordinate-bearing directive skipped with a warning, no panic.
	if len(dirs) != 0 {
		t.Errorf("expected coordinate directive skipped on nil env, got %d", len(dirs))
	}
	if len(warns) == 0 {
		t.Errorf("expected a warning on nil env")
	}
}
