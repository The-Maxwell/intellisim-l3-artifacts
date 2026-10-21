package simulations

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ResolveValue produces the initial value for one agent/shape property using the
// supplied deterministic RNG. It returns the value (string, float64, or bool
// depending on the property type) and, optionally, a non-fatal warning describing
// a degradation that forced a fallback.
//
// Decision logic (when forceRandom is false):
//   - Generation.Mode == default: return DefaultValue (coerced) if present,
//     otherwise random-by-type plus a warning.
//   - Generation == nil: return DefaultValue (coerced) if present, otherwise
//     random-by-type (§4.1 nil-generation behaviour).
//   - Generation.Mode == llm: defensive random-by-type (the generator routes llm
//     values elsewhere; this guards against an unrouted call).
//   - Generation.Mode == random: random-by-type, honouring the distribution.
//
// When forceRandom is true the result is always random-by-type, still honouring
// the generation distribution / choices / min-max overrides when present.
func ResolveValue(rng *rand.Rand, p datamodels.AgentProperty, forceRandom bool) (any, *datasims.GenerationWarning) {
	gen := p.Generation

	if !forceRandom {
		switch {
		case gen != nil && gen.Mode == datamodels.GenModeDefault:
			if p.DefaultValue != nil {
				return coerceToType(p.DefaultValue, p.Type), nil
			}
			v, w := randomByType(rng, p, gen)
			if w == nil {
				w = &datasims.GenerationWarning{Path: p.UID, Reason: "default mode but no default value; randomized"}
			}
			return v, w
		case gen == nil:
			if p.DefaultValue != nil {
				return coerceToType(p.DefaultValue, p.Type), nil
			}
			return randomByType(rng, p, nil)
		default:
			// GenModeRandom and (defensively) GenModeLLM.
			return randomByType(rng, p, gen)
		}
	}

	return randomByType(rng, p, gen)
}

// randomByType draws a random value appropriate to the property type, honouring
// the generation spec (distribution, choices, min/max overrides) when gen != nil.
func randomByType(rng *rand.Rand, p datamodels.AgentProperty, gen *datamodels.PropertyGeneration) (any, *datasims.GenerationWarning) {
	switch p.Type {
	case datamodels.PropertyTypeNumber:
		return randomNumber(rng, p, gen), nil

	case datamodels.PropertyTypeBoolean:
		if gen != nil && gen.Distribution == datamodels.DistChoice && len(gen.Choices) > 0 {
			return weightedChoiceBool(rng, gen.Choices), nil
		}
		return rng.Intn(2) == 0, nil

	case datamodels.PropertyTypeEnum:
		if gen != nil && gen.Distribution == datamodels.DistChoice && len(gen.Choices) > 0 {
			return weightedChoiceString(rng, gen.Choices), nil
		}
		if len(p.AllowedValues) > 0 {
			return toString(p.AllowedValues[rng.Intn(len(p.AllowedValues))]), nil
		}
		return "", &datasims.GenerationWarning{Path: p.UID, Reason: "enum property has no allowed values; generated empty"}

	case datamodels.PropertyTypeString:
		if gen != nil && gen.Distribution == datamodels.DistChoice && len(gen.Choices) > 0 {
			return weightedChoiceString(rng, gen.Choices), nil
		}
		if len(p.AllowedValues) > 0 {
			return toString(p.AllowedValues[rng.Intn(len(p.AllowedValues))]), nil
		}
		return "", &datasims.GenerationWarning{Path: p.UID, Reason: "string property has no default or allowed values; generated empty"}

	case datamodels.PropertyTypeColor:
		if gen != nil && gen.Distribution == datamodels.DistChoice && len(gen.Choices) > 0 {
			return weightedChoiceString(rng, gen.Choices), nil
		}
		if len(p.AllowedValues) > 0 {
			return toString(p.AllowedValues[rng.Intn(len(p.AllowedValues))]), nil
		}
		return fmt.Sprintf("#%06X", rng.Intn(0x1000000)), nil

	default:
		// Unknown type: best-effort default value passthrough, else empty string.
		if p.DefaultValue != nil {
			return p.DefaultValue, nil
		}
		return "", nil
	}
}

// randomNumber draws a float64 according to the generation distribution.
func randomNumber(rng *rand.Rand, p datamodels.AgentProperty, gen *datamodels.PropertyGeneration) float64 {
	if gen != nil {
		switch {
		case gen.Distribution == datamodels.DistChoice && len(gen.Choices) > 0:
			return weightedChoiceFloat(rng, gen.Choices)
		case gen.Distribution == datamodels.DistNormal && gen.Mean != nil && gen.StdDev != nil:
			lo, hi := numBounds(p, gen)
			v := rng.NormFloat64()*(*gen.StdDev) + *gen.Mean
			return clampFloat(v, lo, hi)
		}
	}
	lo, hi := numBounds(p, gen)
	return lo + rng.Float64()*(hi-lo)
}

// numBounds resolves the [lo,hi] sampling range for a number property. The
// generation override (gen.Min/gen.Max) takes precedence over the property's
// MinValue/MaxValue. Missing low bound defaults to 0; missing high bound defaults
// to lo+1. A degenerate hi<=lo is widened to lo+1.
func numBounds(p datamodels.AgentProperty, gen *datamodels.PropertyGeneration) (float64, float64) {
	lo := 0.0
	switch {
	case gen != nil && gen.Min != nil:
		lo = *gen.Min
	case p.MinValue != nil:
		lo = *p.MinValue
	}

	hi := lo + 1
	switch {
	case gen != nil && gen.Max != nil:
		hi = *gen.Max
	case p.MaxValue != nil:
		hi = *p.MaxValue
	}

	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- weighted choice -----------------------------------------------------------

// pickChoice selects an index into choices by cumulative weight. Choices with
// Weight<=0 are skipped. If the total weight is <=0 it falls back to a uniform
// index over all choices.
func pickChoice(rng *rand.Rand, choices []datamodels.ChoiceWeight) int {
	total := 0.0
	for _, c := range choices {
		if c.Weight > 0 {
			total += c.Weight
		}
	}
	if total <= 0 {
		return rng.Intn(len(choices))
	}
	r := rng.Float64() * total
	cum := 0.0
	for i, c := range choices {
		if c.Weight <= 0 {
			continue
		}
		cum += c.Weight
		if cum > r {
			return i
		}
	}
	// Floating-point fallthrough: return the last positively-weighted index.
	for i := len(choices) - 1; i >= 0; i-- {
		if choices[i].Weight > 0 {
			return i
		}
	}
	return len(choices) - 1
}

func weightedChoiceString(rng *rand.Rand, choices []datamodels.ChoiceWeight) string {
	return toString(choices[pickChoice(rng, choices)].Value)
}

func weightedChoiceFloat(rng *rand.Rand, choices []datamodels.ChoiceWeight) float64 {
	f, _ := coerceFloat(choices[pickChoice(rng, choices)].Value)
	return f
}

func weightedChoiceBool(rng *rand.Rand, choices []datamodels.ChoiceWeight) bool {
	return toBool(choices[pickChoice(rng, choices)].Value)
}

// --- coercion helpers ----------------------------------------------------------

// coerceToType best-effort coerces a default value into the canonical shape for
// the property type: number→float64, boolean→bool, string/enum/color→string. It
// never panics; on failure it returns the value unchanged.
func coerceToType(v any, t datamodels.AgentPropertyType) any {
	switch t {
	case datamodels.PropertyTypeNumber:
		if f, ok := coerceFloat(v); ok {
			return f
		}
		return v
	case datamodels.PropertyTypeBoolean:
		if b, ok := v.(bool); ok {
			return b
		}
		return v
	case datamodels.PropertyTypeString, datamodels.PropertyTypeEnum, datamodels.PropertyTypeColor:
		if s, ok := v.(string); ok {
			return s
		}
		return toString(v)
	default:
		return v
	}
}

// coerceFloat coerces common numeric dynamic types to float64. A string is
// parsed as a last resort (trimmed) — legacy documents persisted before the
// modules/models save-time normalizer (ISS-06) may still carry a numeric-looking
// string ("1") on a rule Condition/Action Value; without this, such values
// silently failed every numeric compare/write. A non-numeric string still
// correctly fails (ok=false), preserving decision/state label semantics.
func coerceFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// --- count / scalar ------------------------------------------------------------

// ResolveScalar collapses a ScalarSpec to a single uint64. A nil spec yields 0;
// a fixed Value is returned verbatim; a [Min,Max] range draws a uniform inclusive
// integer (guarding Max<Min by returning Min); anything else yields 0.
func ResolveScalar(rng *rand.Rand, s *datamodels.ScalarSpec) uint64 {
	if s == nil {
		return 0
	}
	if s.Value != nil {
		return *s.Value
	}
	if s.Min != nil && s.Max != nil {
		min, max := *s.Min, *s.Max
		if max < min {
			return min
		}
		return min + uint64(rng.Int63n(int64(max-min+1)))
	}
	return 0
}

// ResolveCount resolves the initial population count for an agent archetype.
// CountFixed (the empty default) uses PopulationCount; CountRandomRange draws a
// uniform inclusive integer in [MinPopulation,MaxPopulation] when both are set,
// else falls back to PopulationCount; CountLLM falls back to PopulationCount (the
// generator overrides this with the LLM-chosen value).
func ResolveCount(rng *rand.Rand, init datamodels.AgentInitialization) uint64 {
	switch init.CountMode {
	case datamodels.CountRandomRange:
		if init.MinPopulation != nil && init.MaxPopulation != nil {
			min, max := *init.MinPopulation, *init.MaxPopulation
			if max < min {
				return min
			}
			return min + uint64(rng.Int63n(int64(max-min+1)))
		}
		return init.PopulationCount
	case datamodels.CountLLM:
		return init.PopulationCount
	default: // CountFixed and "" (empty).
		return init.PopulationCount
	}
}
