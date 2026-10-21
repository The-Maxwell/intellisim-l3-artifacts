package models

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// LLMs occasionally emit geometry scalars as bare numbers ("radius": 5) despite
// the schema doc; the tolerant unmarshal must accept them as {"value":n} instead
// of rejecting the whole envelope.
func TestLLMScalarSpec_UnmarshalBareNumber(t *testing.T) {
	var g LLMShapeGeometry
	require.NoError(t, json.Unmarshal([]byte(`{"position":{"x":20,"y":15},"radius":5}`), &g))
	require.NotNil(t, g.Radius)
	require.NotNil(t, g.Radius.Value)
	require.EqualValues(t, 5, *g.Radius.Value)
	require.Nil(t, g.Radius.Min)
	require.Nil(t, g.Radius.Max)

	// Direct scalar target, whole-number float form.
	var s LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`7`), &s))
	require.NotNil(t, s.Value)
	require.EqualValues(t, 7, *s.Value)
}

// Values at/above 2^64 clamp to MaxUint64 (U6b): float64 rounds math.MaxUint64
// up to 2^64, so the guard must catch >= 2^64 and clamp — never overflow the
// uint64 conversion, and never error (a custom-unmarshaler error aborts the
// whole envelope decode).
func TestLLMScalarSpec_ClampsUint64Overflow(t *testing.T) {
	var s LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`18446744073709551616`), &s)) // 2^64
	require.EqualValues(t, uint64(math.MaxUint64), *s.Value)

	var s2 LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`18446744073709551615`), &s2)) // 2^64-1, rounds to 2^64 as float64
	require.EqualValues(t, uint64(math.MaxUint64), *s2.Value)
}

// The object forms must decode exactly as before the tolerant unmarshal.
func TestLLMScalarSpec_UnmarshalObjectForms(t *testing.T) {
	var fixed LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`{"value":5}`), &fixed))
	require.NotNil(t, fixed.Value)
	require.EqualValues(t, 5, *fixed.Value)
	require.Nil(t, fixed.Min)
	require.Nil(t, fixed.Max)

	var ranged LLMScalarSpec
	require.NoError(t, json.Unmarshal([]byte(`{"min":2,"max":8}`), &ranged))
	require.Nil(t, ranged.Value)
	require.NotNil(t, ranged.Min)
	require.NotNil(t, ranged.Max)
	require.EqualValues(t, 2, *ranged.Min)
	require.EqualValues(t, 8, *ranged.Max)

	// null keeps the pointer field nil (previous behavior).
	var g LLMShapeGeometry
	require.NoError(t, json.Unmarshal([]byte(`{"radius":null}`), &g))
	require.Nil(t, g.Radius)
}
