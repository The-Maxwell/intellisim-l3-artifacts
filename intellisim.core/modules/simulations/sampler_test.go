package simulations

import (
	"math"
	"testing"

	datamodels "intelli.sim.core/data/models"
)

func f64ptr(v float64) *float64 { return &v }
func u64ptr(v uint64) *uint64   { return &v }

// numberProp builds a number property with the given bounds (nil for unset).
func numberProp(uid string, min, max *float64) datamodels.AgentProperty {
	return datamodels.AgentProperty{UID: uid, Type: datamodels.PropertyTypeNumber, MinValue: min, MaxValue: max}
}

func asFloat(t *testing.T, v any) float64 {
	t.Helper()
	switch n := v.(type) {
	case float64:
		return n
	default:
		t.Fatalf("expected float64, got %T (%v)", v, v)
		return 0
	}
}

func asString(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", v, v)
	}
	return s
}

func asBool(t *testing.T, v any) bool {
	t.Helper()
	b, ok := v.(bool)
	if !ok {
		t.Fatalf("expected bool, got %T (%v)", v, v)
	}
	return b
}

// --- determinism --------------------------------------------------------------

func TestResolveValue_Determinism(t *testing.T) {
	props := []datamodels.AgentProperty{
		numberProp("speed", f64ptr(0), f64ptr(100)),
		{UID: "kind", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"a", "b", "c"}},
		{UID: "alive", Type: datamodels.PropertyTypeBoolean},
		{UID: "color", Type: datamodels.PropertyTypeColor},
	}

	run := func() []any {
		rng := RunRNG(7, 1)
		out := make([]any, 0, len(props)*5)
		for i := 0; i < 5; i++ {
			for _, p := range props {
				v, _ := ResolveValue(rng, p, false)
				out = append(out, v)
			}
		}
		return out
	}

	a := run()
	b := run()
	if len(a) != len(b) {
		t.Fatalf("length mismatch %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("determinism violated at %d: %v vs %v", i, a[i], b[i])
		}
	}
}

// --- uniform number ------------------------------------------------------------

func TestResolveValue_UniformNumberWithinBounds(t *testing.T) {
	rng := RunRNG(1, 1)
	p := numberProp("speed", f64ptr(10), f64ptr(20))
	for i := 0; i < 1000; i++ {
		v, w := ResolveValue(rng, p, false)
		if w != nil {
			t.Fatalf("unexpected warning: %+v", w)
		}
		n := asFloat(t, v)
		if n < 10 || n > 20 {
			t.Fatalf("value %v out of [10,20]", n)
		}
	}
}

// --- normal distribution -------------------------------------------------------

func TestResolveValue_NormalCenteredAndClamped(t *testing.T) {
	rng := RunRNG(2, 1)
	p := numberProp("h", f64ptr(0), f64ptr(10))
	p.Generation = &datamodels.PropertyGeneration{
		Mode:         datamodels.GenModeRandom,
		Distribution: datamodels.DistNormal,
		Mean:         f64ptr(5),
		StdDev:       f64ptr(1),
	}
	const n = 5000
	var sum float64
	for i := 0; i < n; i++ {
		v, _ := ResolveValue(rng, p, false)
		f := asFloat(t, v)
		if f < 0 || f > 10 {
			t.Fatalf("normal value %v not clamped to [0,10]", f)
		}
		sum += f
	}
	mean := sum / n
	if math.Abs(mean-5) > 0.2 {
		t.Fatalf("normal mean %v far from 5", mean)
	}
}

func TestResolveValue_NormalClampedHard(t *testing.T) {
	// Mean far above max → every draw should clamp to hi.
	rng := RunRNG(3, 1)
	p := numberProp("h", f64ptr(0), f64ptr(10))
	p.Generation = &datamodels.PropertyGeneration{
		Mode:         datamodels.GenModeRandom,
		Distribution: datamodels.DistNormal,
		Mean:         f64ptr(1000),
		StdDev:       f64ptr(1),
	}
	for i := 0; i < 500; i++ {
		v, _ := ResolveValue(rng, p, false)
		if got := asFloat(t, v); got != 10 {
			t.Fatalf("expected clamp to 10, got %v", got)
		}
	}
}

// --- weighted choice -----------------------------------------------------------

func TestResolveValue_WeightedChoiceRespectsWeights(t *testing.T) {
	rng := RunRNG(4, 1)
	p := datamodels.AgentProperty{UID: "k", Type: datamodels.PropertyTypeEnum}
	p.Generation = &datamodels.PropertyGeneration{
		Mode:         datamodels.GenModeRandom,
		Distribution: datamodels.DistChoice,
		Choices: []datamodels.ChoiceWeight{
			{Value: "x", Weight: 0.7},
			{Value: "y", Weight: 0.3},
		},
	}
	const n = 10000
	counts := map[string]int{}
	for i := 0; i < n; i++ {
		v, _ := ResolveValue(rng, p, false)
		counts[asString(t, v)]++
	}
	px := float64(counts["x"]) / n
	if math.Abs(px-0.7) > 0.05 {
		t.Fatalf("weighted choice px=%v not ~0.7", px)
	}
}

func TestResolveValue_NumberChoice(t *testing.T) {
	rng := RunRNG(5, 1)
	p := numberProp("k", nil, nil)
	p.Generation = &datamodels.PropertyGeneration{
		Mode:         datamodels.GenModeRandom,
		Distribution: datamodels.DistChoice,
		Choices: []datamodels.ChoiceWeight{
			{Value: float64(1), Weight: 1},
			{Value: float64(2), Weight: 1},
		},
	}
	for i := 0; i < 100; i++ {
		v, _ := ResolveValue(rng, p, false)
		f := asFloat(t, v)
		if f != 1 && f != 2 {
			t.Fatalf("number choice produced %v", f)
		}
	}
}

// --- default mode --------------------------------------------------------------

func TestResolveValue_DefaultModeReturnsDefault(t *testing.T) {
	rng := RunRNG(6, 1)
	p := numberProp("speed", f64ptr(0), f64ptr(100))
	p.DefaultValue = float64(42)
	p.Generation = &datamodels.PropertyGeneration{Mode: datamodels.GenModeDefault}
	v, w := ResolveValue(rng, p, false)
	if w != nil {
		t.Fatalf("unexpected warning %+v", w)
	}
	if asFloat(t, v) != 42 {
		t.Fatalf("expected default 42, got %v", v)
	}
}

func TestResolveValue_DefaultModeNoDefaultWarnsAndRandomizes(t *testing.T) {
	rng := RunRNG(6, 2)
	p := numberProp("speed", f64ptr(0), f64ptr(100))
	p.Generation = &datamodels.PropertyGeneration{Mode: datamodels.GenModeDefault}
	v, w := ResolveValue(rng, p, false)
	if w == nil {
		t.Fatal("expected warning for default mode without default value")
	}
	n := asFloat(t, v)
	if n < 0 || n > 100 {
		t.Fatalf("randomized value %v out of bounds", n)
	}
}

// --- nil generation ------------------------------------------------------------

func TestResolveValue_NilGenWithDefault(t *testing.T) {
	rng := RunRNG(8, 1)
	p := numberProp("speed", f64ptr(0), f64ptr(100))
	p.DefaultValue = float64(7)
	v, w := ResolveValue(rng, p, false)
	if w != nil {
		t.Fatalf("unexpected warning %+v", w)
	}
	if asFloat(t, v) != 7 {
		t.Fatalf("expected default 7, got %v", v)
	}
}

func TestResolveValue_NilGenNumberNoDefaultWithinBounds(t *testing.T) {
	rng := RunRNG(9, 1)
	p := numberProp("speed", f64ptr(3), f64ptr(8))
	for i := 0; i < 500; i++ {
		v, _ := ResolveValue(rng, p, false)
		n := asFloat(t, v)
		if n < 3 || n > 8 {
			t.Fatalf("value %v out of [3,8]", n)
		}
	}
}

func TestResolveValue_EnumNilGenMember(t *testing.T) {
	rng := RunRNG(10, 1)
	allowed := []interface{}{"red", "green", "blue"}
	p := datamodels.AgentProperty{UID: "c", Type: datamodels.PropertyTypeEnum, AllowedValues: allowed}
	for i := 0; i < 200; i++ {
		v, w := ResolveValue(rng, p, false)
		if w != nil {
			t.Fatalf("unexpected warning %+v", w)
		}
		s := asString(t, v)
		if s != "red" && s != "green" && s != "blue" {
			t.Fatalf("enum produced non-member %q", s)
		}
	}
}

func TestResolveValue_EnumNoAllowedWarns(t *testing.T) {
	rng := RunRNG(11, 1)
	p := datamodels.AgentProperty{UID: "c", Type: datamodels.PropertyTypeEnum}
	v, w := ResolveValue(rng, p, false)
	if w == nil {
		t.Fatal("expected warning for enum with no allowed values")
	}
	if asString(t, v) != "" {
		t.Fatalf("expected empty string, got %q", v)
	}
}

func TestResolveValue_BoolRoughlyHalf(t *testing.T) {
	rng := RunRNG(12, 1)
	p := datamodels.AgentProperty{UID: "alive", Type: datamodels.PropertyTypeBoolean}
	const n = 10000
	trues := 0
	for i := 0; i < n; i++ {
		v, _ := ResolveValue(rng, p, false)
		if asBool(t, v) {
			trues++
		}
	}
	frac := float64(trues) / n
	if math.Abs(frac-0.5) > 0.05 {
		t.Fatalf("bool fraction %v not ~0.5", frac)
	}
}

func TestResolveValue_StringNoDefaultNoAllowedWarns(t *testing.T) {
	rng := RunRNG(13, 1)
	p := datamodels.AgentProperty{UID: "label", Type: datamodels.PropertyTypeString}
	v, w := ResolveValue(rng, p, false)
	if w == nil {
		t.Fatal("expected warning for string with no default/allowed")
	}
	if asString(t, v) != "" {
		t.Fatalf("expected empty string, got %q", v)
	}
}

func TestResolveValue_ColorGeneratesHex(t *testing.T) {
	rng := RunRNG(14, 1)
	p := datamodels.AgentProperty{UID: "color", Type: datamodels.PropertyTypeColor}
	v, w := ResolveValue(rng, p, false)
	if w != nil {
		t.Fatalf("unexpected warning %+v", w)
	}
	s := asString(t, v)
	if len(s) != 7 || s[0] != '#' {
		t.Fatalf("expected #RRGGBB color, got %q", s)
	}
}

func TestResolveValue_ForceRandomIgnoresDefault(t *testing.T) {
	// forceRandom must draw a random number even in default mode with a default.
	p := numberProp("speed", f64ptr(0), f64ptr(100))
	p.DefaultValue = float64(42)
	p.Generation = &datamodels.PropertyGeneration{Mode: datamodels.GenModeDefault}
	// Across many draws at least one must differ from the default.
	rng := RunRNG(15, 1)
	allDefault := true
	for i := 0; i < 50; i++ {
		v, _ := ResolveValue(rng, p, true)
		if asFloat(t, v) != 42 {
			allDefault = false
			break
		}
	}
	if allDefault {
		t.Fatal("forceRandom kept returning the default value")
	}
}

// --- ResolveScalar -------------------------------------------------------------

func TestResolveScalar(t *testing.T) {
	rng := RunRNG(16, 1)
	if got := ResolveScalar(rng, nil); got != 0 {
		t.Fatalf("nil scalar should be 0, got %d", got)
	}
	if got := ResolveScalar(rng, &datamodels.ScalarSpec{Value: u64ptr(9)}); got != 9 {
		t.Fatalf("fixed scalar should be 9, got %d", got)
	}
	s := &datamodels.ScalarSpec{Min: u64ptr(5), Max: u64ptr(10)}
	for i := 0; i < 1000; i++ {
		got := ResolveScalar(rng, s)
		if got < 5 || got > 10 {
			t.Fatalf("range scalar %d out of [5,10]", got)
		}
	}
	// min==max degenerate
	if got := ResolveScalar(rng, &datamodels.ScalarSpec{Min: u64ptr(4), Max: u64ptr(4)}); got != 4 {
		t.Fatalf("degenerate range should be 4, got %d", got)
	}
	// max<min guard → min
	if got := ResolveScalar(rng, &datamodels.ScalarSpec{Min: u64ptr(8), Max: u64ptr(3)}); got != 8 {
		t.Fatalf("inverted range should clamp to min 8, got %d", got)
	}
}

func TestResolveScalar_RangeDeterminism(t *testing.T) {
	s := &datamodels.ScalarSpec{Min: u64ptr(0), Max: u64ptr(1000)}
	seqA := make([]uint64, 50)
	seqB := make([]uint64, 50)
	rngA := RunRNG(17, 1)
	rngB := RunRNG(17, 1)
	for i := range seqA {
		seqA[i] = ResolveScalar(rngA, s)
		seqB[i] = ResolveScalar(rngB, s)
	}
	for i := range seqA {
		if seqA[i] != seqB[i] {
			t.Fatalf("scalar determinism violated at %d: %d vs %d", i, seqA[i], seqB[i])
		}
	}
}

// --- ResolveCount --------------------------------------------------------------

func TestResolveCount(t *testing.T) {
	rng := RunRNG(18, 1)

	// fixed (empty mode == fixed)
	init := datamodels.AgentInitialization{PopulationCount: 12}
	if got := ResolveCount(rng, init); got != 12 {
		t.Fatalf("fixed count should be 12, got %d", got)
	}
	init.CountMode = datamodels.CountFixed
	if got := ResolveCount(rng, init); got != 12 {
		t.Fatalf("explicit fixed count should be 12, got %d", got)
	}

	// random_range with min/max
	rr := datamodels.AgentInitialization{
		PopulationCount: 100,
		CountMode:       datamodels.CountRandomRange,
		MinPopulation:   u64ptr(5),
		MaxPopulation:   u64ptr(9),
	}
	for i := 0; i < 1000; i++ {
		got := ResolveCount(rng, rr)
		if got < 5 || got > 9 {
			t.Fatalf("random_range count %d out of [5,9]", got)
		}
	}

	// random_range without min/max → falls back to PopulationCount
	rrNo := datamodels.AgentInitialization{PopulationCount: 7, CountMode: datamodels.CountRandomRange}
	if got := ResolveCount(rng, rrNo); got != 7 {
		t.Fatalf("random_range without bounds should fall back to 7, got %d", got)
	}

	// llm → PopulationCount fallback
	llm := datamodels.AgentInitialization{PopulationCount: 33, CountMode: datamodels.CountLLM}
	if got := ResolveCount(rng, llm); got != 33 {
		t.Fatalf("llm count should fall back to 33, got %d", got)
	}
}

// --- ISS-06 engine defense: coerceFloat parses numeric-looking strings --------

// TestCoerceFloat_NumericString verifies the legacy-document defense: a
// numeric-looking string (as could still be persisted before the
// modules/models save-time normalizer existed) parses successfully, trimmed of
// surrounding whitespace.
func TestCoerceFloat_NumericString(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"3.5", 3.5},
		{"42", 42},
		{"  7 ", 7},
		{"-1.25", -1.25},
	}
	for _, c := range cases {
		got, ok := coerceFloat(c.in)
		if !ok {
			t.Fatalf("coerceFloat(%q) ok=false, want true", c.in)
		}
		if got != c.want {
			t.Fatalf("coerceFloat(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

// TestCoerceFloat_NonNumericString_StaysUnparsed verifies a genuine label
// string (e.g. a decision/state value) still correctly fails to coerce.
func TestCoerceFloat_NonNumericString_StaysUnparsed(t *testing.T) {
	if _, ok := coerceFloat("hungry"); ok {
		t.Fatal(`coerceFloat("hungry") ok=true, want false`)
	}
	if _, ok := coerceFloat(""); ok {
		t.Fatal(`coerceFloat("") ok=true, want false`)
	}
}
