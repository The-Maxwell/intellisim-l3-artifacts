package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// U6 (spec 2026-07-18-u6-llm-envelope-numeric-tolerance-design.md): every
// integer-typed field of the LLM-authored envelope decodes any JSON number
// tolerantly — fractional rounds to nearest, negative-into-unsigned still
// rejected. Trigger: gpt-4.1 emitting the "1.2 m door" as fractional geometry
// killed whole generation drafts (EXP-A attempt 1).

// Reproduces the live failure shape: object-form fractional geometry width.
func TestEnvelope_FractionalGeometryObjectForm(t *testing.T) {
	raw := `{"display_message":"d","model":{"environment":{"width":38,"height":38,
		"shapes":[{"uid":"door","type":"rectangle",
			"geometry":{"position":{"x":37,"y":18},"width":{"value":1.2},"height":{"value":0.5}},
			"initialization":{}}]}}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	g := env.Model.Environment.Shapes[0].Geometry
	require.NotNil(t, g.Width.Value)
	require.EqualValues(t, 1, *g.Width.Value)
	require.NotNil(t, g.Height.Value)
	require.EqualValues(t, 1, *g.Height.Value) // 0.5 rounds half away from zero
}

// Bare fractional scalar was previously rejected ("not a non-negative integer").
func TestLLMScalarSpec_BareFractionalRounds(t *testing.T) {
	var g LLMShapeGeometry
	require.NoError(t, json.Unmarshal([]byte(`{"radius":2.7,"width":0.1}`), &g))
	require.EqualValues(t, 3, *g.Radius.Value)
	require.EqualValues(t, 0, *g.Width.Value)
}

// Ranged object form with fractional bounds.
func TestLLMScalarSpec_FractionalRange(t *testing.T) {
	var s LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`{"min":0.4,"max":1.6}`), &s))
	require.Nil(t, s.Value)
	require.EqualValues(t, 0, *s.Min)
	require.EqualValues(t, 2, *s.Max)
}

// encoding/json rejects even whole-number floats (100.0) into integer fields;
// the tolerant types must accept them.
func TestEnvelope_WholeNumberFloatCounts(t *testing.T) {
	raw := `{"display_message":"d","model":{"agents":[{"uid":"visitor",
		"initialization":{"population_count":100.0}}]}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	require.EqualValues(t, 100, *env.Model.Agents[0].Initialization.PopulationCount)
}

// Fractional positions and environment dimensions round.
func TestEnvelope_FractionalPositionsAndEnvDims(t *testing.T) {
	raw := `{"display_message":"d","model":{"environment":{"width":15.9,"height":15.2,
		"shapes":[{"uid":"s","type":"circle",
			"geometry":{"position":{"x":7.5,"y":14.49},"radius":{"value":2}},
			"initialization":{}}]}}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	require.EqualValues(t, 16, *env.Model.Environment.Width)
	require.EqualValues(t, 15, *env.Model.Environment.Height)
	p := env.Model.Environment.Shapes[0].Geometry.Position
	require.EqualValues(t, 8, p.X)
	require.EqualValues(t, 14, p.Y)
}

// Fractional rule/widget/event integers round (llmInt / llmUint fields).
func TestEnvelope_FractionalRuleWidgetEventInts(t *testing.T) {
	raw := `{"display_message":"d","model":{
		"rules":[{"id":"r1","priority":1.5,"cooldown_steps":2.4}],
		"events":[{"uid":"e1","ttl_steps":3.3}],
		"widgets":[{"id":"w1","type":"histogram","buckets":10.0,"position":{"x":2.5,"y":0,"w":4,"h":3.2}}]}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	require.EqualValues(t, 2, *env.Model.Rules[0].Priority)
	require.EqualValues(t, 2, *env.Model.Rules[0].CooldownSteps)
	require.EqualValues(t, 3, *env.Model.Events[0].TTLSteps)
	require.EqualValues(t, 10, *env.Model.Widgets[0].Buckets)
	require.EqualValues(t, 3, env.Model.Widgets[0].Position.X)
	require.EqualValues(t, 3, env.Model.Widgets[0].Position.H)
}

// Negative numbers into unsigned fields clamp to 0 (U6b): an error from a
// custom unmarshaler ABORTS the rest of the decode (verified empirically), so
// rejecting would kill the whole draft over one bad token. Readiness
// validation flags nonsense dimensions downstream.
func TestEnvelope_NegativeUnsignedClampsToZero(t *testing.T) {
	var s LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`{"value":-3}`), &s))
	require.EqualValues(t, 0, *s.Value)

	var bare LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`-1`), &bare))
	require.EqualValues(t, 0, *bare.Value)

	var g LLMShapeGeometry
	require.NoError(t, json.Unmarshal([]byte(`{"position":{"x":-4,"y":2}}`), &g))
	require.EqualValues(t, 0, g.Position.X)
	require.EqualValues(t, 2, g.Position.Y)
}

// Quoted string numbers decode ("width": "1.2") — custom types must handle
// strings themselves because erroring aborts the decode (U6b).
func TestEnvelope_StringNumbersParse(t *testing.T) {
	var g LLMShapeGeometry
	require.NoError(t, json.Unmarshal([]byte(`{"width":{"value":"1.2"},"radius":"7","thickness":"2.6"}`), &g))
	require.EqualValues(t, 1, *g.Width.Value)
	require.EqualValues(t, 7, *g.Radius.Value)
	require.EqualValues(t, 3, *g.Thickness)

	// Unparseable garbage zero-degrades instead of aborting.
	var g2 LLMShapeGeometry
	require.NoError(t, json.Unmarshal([]byte(`{"width":{"value":"wide"}}`), &g2))
	require.EqualValues(t, 0, *g2.Width.Value)
}

// llmBool accepts bools, numbers and common string forms; anything else is
// false. Reproduces the live attempt-2 failure ("fill" as a string).
func TestEnvelope_TolerantBools(t *testing.T) {
	raw := `{"display_message":"d","model":{"is_spatial":"true","environment":{"width":38,"height":38,
		"shapes":[{"uid":"wall","type":"rectangle",
			"geometry":{"position":{"x":0,"y":0},"width":{"value":38},"height":{"value":1},
				"fill":"#cccccc","stroke":"yes"},
			"initialization":{}}]},
		"rules":[{"id":"r1","enabled":"on","execute_once":0}]}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	require.True(t, bool(*env.Model.IsSpatial))
	g := env.Model.Environment.Shapes[0].Geometry
	require.False(t, bool(*g.Fill)) // unrecognized string form -> false
	require.True(t, bool(*g.Stroke))
	require.True(t, bool(*env.Model.Rules[0].Enabled))
	require.False(t, bool(*env.Model.Rules[0].ExecuteOnce))
}

// Fail-soft net: a type mismatch the tolerant scalars don't cover (an object
// into a plain string field) skips that field with a warning instead of
// killing the draft.
func TestParseEnvelope_FailSoftOnTypeMismatch(t *testing.T) {
	raw := `{"display_message":"d","model":{"description":{"oops":1},
		"agents":[{"uid":"visitor","initialization":{"population_count":100}}]}}`
	env, warns, err := parseEnvelopeSoft(raw)
	require.NoError(t, err)
	require.NotEmpty(t, warns)
	require.Equal(t, "", env.Model.Description) // mismatched leaf skipped
	require.Len(t, env.Model.Agents, 1)         // rest of the envelope kept
	require.EqualValues(t, 100, *env.Model.Agents[0].Initialization.PopulationCount)
}

// A decode that yields NOTHING (empty envelope) still fails hard even when the
// error is a type mismatch — there is no draft to salvage.
func TestParseEnvelope_EmptyEnvelopeStillFails(t *testing.T) {
	_, _, err := parseEnvelopeSoft(`{"display_message":123}`)
	require.Error(t, err)
}

// Negative llmInt fields remain legal (priority may be negative by design).
func TestEnvelope_NegativeIntFieldsLegal(t *testing.T) {
	raw := `{"display_message":"d","model":{"rules":[{"id":"r1","priority":-2.6}]}}`
	env, err := parseEnvelope(raw)
	require.NoError(t, err)
	require.EqualValues(t, -3, *env.Model.Rules[0].Priority)
}
