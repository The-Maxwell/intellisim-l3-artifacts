package models

import (
	"testing"

	"intelli.sim.core/data/common"
)

func u64(v uint64) *uint64    { return &v }
func f64(v float64) *float64  { return &v }

func TestNormalizeSpatial(t *testing.T) {
	m := &Model{
		Agents: []Agent{
			{ // empty footprint kind -> point; stray dims cleared
				UID:       "a",
				Footprint: &AgentFootprint{Kind: "", Radius: f64(5), Width: u64(3)},
				Display:   &Display{Kind: ""},
			},
			{ // explicit point -> dims cleared
				UID:       "b",
				Footprint: &AgentFootprint{Kind: FootprintPoint, Radius: f64(2), Points: []common.Point{{X: 1, Y: 1}}},
			},
			{ // circle preserved
				UID:       "c",
				Footprint: &AgentFootprint{Kind: FootprintCircle, Radius: f64(4)},
				Display:   &Display{Kind: DisplayImage, Image: &DisplayImageData{Data: "x"}},
			},
			{UID: "d"}, // nil footprint/display stays nil
		},
		Environment: &Environment{
			Shapes: []Shape{
				{UID: "s", Display: &Display{Kind: ""}},
				{UID: "s2"}, // nil display stays nil
			},
		},
	}

	m.NormalizeSpatial()

	if got := m.Agents[0].Footprint.Kind; got != FootprintPoint {
		t.Errorf("agent a: empty kind should normalize to point, got %q", got)
	}
	if m.Agents[0].Footprint.Radius != nil || m.Agents[0].Footprint.Width != nil {
		t.Errorf("agent a: point footprint must clear dims, got radius=%v width=%v", m.Agents[0].Footprint.Radius, m.Agents[0].Footprint.Width)
	}
	if m.Agents[0].Display.Kind != DisplayAuto {
		t.Errorf("agent a: empty display kind should normalize to auto, got %q", m.Agents[0].Display.Kind)
	}
	if m.Agents[1].Footprint.Radius != nil || m.Agents[1].Footprint.Points != nil {
		t.Errorf("agent b: explicit point must clear dims/points")
	}
	if m.Agents[2].Footprint.Kind != FootprintCircle || m.Agents[2].Footprint.Radius == nil || *m.Agents[2].Footprint.Radius != 4 {
		t.Errorf("agent c: circle footprint must be preserved")
	}
	if m.Agents[2].Display.Kind != DisplayImage {
		t.Errorf("agent c: image display kind must be preserved, got %q", m.Agents[2].Display.Kind)
	}
	if m.Agents[3].Footprint != nil || m.Agents[3].Display != nil {
		t.Errorf("agent d: nil footprint/display must stay nil")
	}
	if m.Environment.Shapes[0].Display.Kind != DisplayAuto {
		t.Errorf("shape s: empty display kind should normalize to auto")
	}
	if m.Environment.Shapes[1].Display != nil {
		t.Errorf("shape s2: nil display must stay nil")
	}
}

// TestNormalizeSpatial_FixedPositionPopulationCount verifies the fixed-position =>
// single-instance invariant: a shape whose geometry pins an absolute Position and whose
// PopulationCount is the zero value is coerced to 1 (so it satisfies module-level
// validation), while a user-authored non-zero count and non-fixed shapes are untouched.
func TestNormalizeSpatial_FixedPositionPopulationCount(t *testing.T) {
	m := &Model{
		Environment: &Environment{
			Shapes: []Shape{
				{ // fixed position + zero count -> coerced to 1
					UID:      "lake",
					Geometry: &ShapeGeometry{Position: &common.Point{X: 20, Y: 15}},
				},
				{ // fixed position + user-authored count -> untouched
					UID:            "pond",
					Initialization: AgentInitialization{PopulationCount: 3},
					Geometry:       &ShapeGeometry{Position: &common.Point{X: 1, Y: 2}},
				},
				{ // no fixed position -> untouched (stays 0)
					UID:      "field",
					Geometry: &ShapeGeometry{Radius: &ScalarSpec{Value: u64(5)}},
				},
				{UID: "nogeom"}, // nil geometry -> untouched
			},
		},
	}

	m.NormalizeSpatial()

	if got := m.Environment.Shapes[0].Initialization.PopulationCount; got != 1 {
		t.Errorf("lake: fixed-position zero count must coerce to 1, got %d", got)
	}
	if got := m.Environment.Shapes[1].Initialization.PopulationCount; got != 3 {
		t.Errorf("pond: user-authored count must be preserved, got %d", got)
	}
	if got := m.Environment.Shapes[2].Initialization.PopulationCount; got != 0 {
		t.Errorf("field: non-fixed shape count must be untouched, got %d", got)
	}
	if got := m.Environment.Shapes[3].Initialization.PopulationCount; got != 0 {
		t.Errorf("nogeom: nil-geometry shape count must be untouched, got %d", got)
	}
}

func TestNormalizeSpatial_NilSafe(t *testing.T) {
	var m *Model
	m.NormalizeSpatial() // must not panic
	(&Model{}).NormalizeSpatial()
}
