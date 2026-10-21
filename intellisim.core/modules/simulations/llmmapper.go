package simulations

import (
	"fmt"
	"math/rand"
	"strings"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// This file converts raw LLM responses (llmcontract.go) into the generator's
// internal value/directive shapes. Every converter is FAIL-SOFT: no path returns
// an error and none can panic on nil maps, nil pointers, empty slices, or a nil
// environment. Whenever a value is missing, mistyped, out of its allowed set, or
// otherwise unusable, the converter substitutes a deterministic-random fallback
// (via ResolveValue) or skips the offending item, recording a GenerationWarning
// so the generation run's audit trail explains every degradation.

// MapGlobalValues maps an LLM global-values response onto the environment global
// properties. The result map contains EVERY prop UID: any value that is missing,
// the wrong type, or (for enums) not in AllowedValues is replaced by a
// random-by-type fallback and flagged with a warning.
func MapGlobalValues(resp LLMGlobalValuesResponse, props []datamodels.AgentProperty, rng *rand.Rand) (map[string]any, []datasims.GenerationWarning) {
	out := make(map[string]any, len(props))
	var warns []datasims.GenerationWarning

	for _, p := range props {
		raw, present := resp.Values[p.UID]
		v, ok := coerceProperty(raw, present, p)
		if !ok {
			fb, _ := ResolveValue(rng, p, true)
			out[p.UID] = fb
			warns = append(warns, datasims.GenerationWarning{Path: p.UID, Reason: coerceFailReason(present)})
			continue
		}
		out[p.UID] = v
	}
	return out, warns
}

// MapAgentBatch maps an LLM agent-batch response onto exactly batchLen instance
// value maps. Instances are indexed by their (0-based) Instance field; entries
// whose index is out of [0,batchLen) or duplicates an already-seen index are
// ignored with a warning. Any instance index absent from the response is filled
// entirely via ResolveValue. Within a present instance, each property is coerced
// like a global value, falling back per-field on failure.
//
// Warning path namespaces: out-of-range and duplicate-index warnings use the
// LLM RESPONSE index (instances[<resp index>]); missing-instance and
// invalid-field warnings use the OUTPUT index (instances[<output index>] or
// instances[<output index>].<uid>).
func MapAgentBatch(resp LLMAgentBatchResponse, llmProps []datamodels.AgentProperty, batchLen int, rng *rand.Rand) ([]map[string]any, []datasims.GenerationWarning) {
	if batchLen < 0 {
		batchLen = 0
	}
	out := make([]map[string]any, batchLen)
	var warns []datasims.GenerationWarning

	// Index the response by instance, honouring the first occurrence of each
	// valid index.
	byIndex := make(map[int]map[string]any, len(resp.Instances))
	for _, inst := range resp.Instances {
		if inst.Instance < 0 || inst.Instance >= batchLen {
			warns = append(warns, datasims.GenerationWarning{
				Path:   fmt.Sprintf("instances[%d]", inst.Instance),
				Reason: "instance index out of range; ignored",
			})
			continue
		}
		if _, dup := byIndex[inst.Instance]; dup {
			warns = append(warns, datasims.GenerationWarning{
				Path:   fmt.Sprintf("instances[%d]", inst.Instance),
				Reason: "duplicate instance index; ignored",
			})
			continue
		}
		byIndex[inst.Instance] = inst.Values
	}

	for i := 0; i < batchLen; i++ {
		values, present := byIndex[i]
		m := make(map[string]any, len(llmProps))

		if !present {
			// Whole instance missing: fill every prop from the fallback.
			for _, p := range llmProps {
				fb, _ := ResolveValue(rng, p, true)
				m[p.UID] = fb
			}
			warns = append(warns, datasims.GenerationWarning{
				Path:   fmt.Sprintf("instances[%d]", i),
				Reason: "instance missing from response; randomized",
			})
			out[i] = m
			continue
		}

		for _, p := range llmProps {
			raw, has := values[p.UID]
			v, ok := coerceProperty(raw, has, p)
			if !ok {
				fb, _ := ResolveValue(rng, p, true)
				m[p.UID] = fb
				warns = append(warns, datasims.GenerationWarning{
					Path:   fmt.Sprintf("instances[%d].%s", i, p.UID),
					Reason: coerceFailReason(has),
				})
				continue
			}
			m[p.UID] = v
		}
		out[i] = m
	}
	return out, warns
}

// MapCount resolves the LLM-chosen population count against the archetype's
// initialization bounds. A zero count falls back to PopulationCount; the count is
// then clamped into [MinPopulation,MaxPopulation] when those are set. At most one
// warning is returned (nil when the count is used unchanged); when several
// adjustments would apply, the clamp warning is preferred over the zero warning
// as the more actionable signal.
func MapCount(resp LLMCountResponse, init datamodels.AgentInitialization) (uint64, *datasims.GenerationWarning) {
	c := resp.Count
	var warn *datasims.GenerationWarning

	if c == 0 {
		c = init.PopulationCount
		warn = &datasims.GenerationWarning{Path: "count", Reason: "llm returned zero count; used population count"}
	}

	if init.MinPopulation != nil && c < *init.MinPopulation {
		c = *init.MinPopulation
		warn = &datasims.GenerationWarning{Path: "count", Reason: fmt.Sprintf("llm count below minimum; clamped to %d", *init.MinPopulation)}
	}
	if init.MaxPopulation != nil && c > *init.MaxPopulation {
		c = *init.MaxPopulation
		warn = &datasims.GenerationWarning{Path: "count", Reason: fmt.Sprintf("llm count above maximum; clamped to %d", *init.MaxPopulation)}
	}

	return c, warn
}

// MapDirectives converts an LLM placement response into validated
// PlacementDirective values, clamping every coordinate into the environment grid.
// Unusable directives (unknown kind, missing geometry, unknown shape, explicit
// over the 50-agent cap, or any coordinate-bearing directive when env is nil) are
// skipped with a warning. The kept directives feed SampleDirectives; an empty
// result tells the generator to fall back to random placement.
func MapDirectives(resp LLMPlacementResponse, env *datasims.SimulationEnvironment, totalCount uint64) ([]PlacementDirective, []datasims.GenerationWarning) {
	var out []PlacementDirective
	var warns []datasims.GenerationWarning

	for i, d := range resp.Directives {
		kind := strings.ToLower(strings.TrimSpace(d.Kind))
		path := fmt.Sprintf("directives[%d].kind", i)

		switch kind {
		case "cluster":
			if d.Center == nil {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "cluster directive missing center; skipped"})
				continue
			}
			if env == nil {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "cluster directive needs an environment; skipped"})
				continue
			}
			center := clampLLMPoint(*d.Center, env)
			radius := 0.0
			if d.Radius != nil {
				radius = *d.Radius
			}
			out = append(out, PlacementDirective{
				Kind:     "cluster",
				Fraction: d.Fraction,
				Center:   &center,
				Radius:   radius,
			})

		case "region":
			if len(d.Polygon) < 3 {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "region directive needs at least 3 polygon points; skipped"})
				continue
			}
			if env == nil {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "region directive needs an environment; skipped"})
				continue
			}
			out = append(out, PlacementDirective{
				Kind:     "region",
				Fraction: d.Fraction,
				Polygon:  clampLLMPoints(d.Polygon, env),
			})

		case "at_shape":
			if env == nil {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "at_shape needs an environment; skipped"})
				continue
			}
			if !shapeExists(env, d.ShapeUID) {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: fmt.Sprintf("at_shape target shape %q not found; skipped", d.ShapeUID)})
				continue
			}
			out = append(out, PlacementDirective{
				Kind:     "at_shape",
				Fraction: d.Fraction,
				ShapeUID: d.ShapeUID,
			})

		case "explicit":
			if totalCount > 50 {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "explicit points not allowed for >50 agents; skipped"})
				continue
			}
			if len(d.Points) == 0 {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "explicit directive has no points; skipped"})
				continue
			}
			if env == nil {
				warns = append(warns, datasims.GenerationWarning{Path: path, Reason: "explicit directive needs an environment; skipped"})
				continue
			}
			out = append(out, PlacementDirective{
				Kind:     "explicit",
				Fraction: d.Fraction,
				Points:   clampLLMPoints(d.Points, env),
			})

		default:
			warns = append(warns, datasims.GenerationWarning{Path: path, Reason: fmt.Sprintf("unknown directive kind %q; skipped", d.Kind)})
		}
	}

	return out, warns
}

// --- coercion helpers ----------------------------------------------------------

// coerceProperty validates and coerces a raw LLM value against a property's type.
// It returns (value, true) on success and (nil, false) when the value is absent,
// the wrong type, or (for enums) outside AllowedValues. number→float64,
// boolean→bool, string/color/enum→string.
func coerceProperty(raw any, present bool, p datamodels.AgentProperty) (any, bool) {
	if !present || raw == nil {
		return nil, false
	}
	switch p.Type {
	case datamodels.PropertyTypeNumber:
		if f, ok := coerceFloat(raw); ok {
			return f, true
		}
		return nil, false
	case datamodels.PropertyTypeBoolean:
		if b, ok := raw.(bool); ok {
			return b, true
		}
		return nil, false
	case datamodels.PropertyTypeEnum:
		s, ok := raw.(string)
		if !ok {
			return nil, false
		}
		if !enumAllowed(s, p.AllowedValues) {
			return nil, false
		}
		return s, true
	case datamodels.PropertyTypeString, datamodels.PropertyTypeColor:
		if s, ok := raw.(string); ok {
			return s, true
		}
		return nil, false
	default:
		// Unknown type: accept the raw value verbatim.
		return raw, true
	}
}

// enumAllowed reports whether s matches one of the allowed values (stringified).
// An empty allowed set rejects everything (the enum is unusable, so the caller
// falls back). Matching is loose: every allowed value is converted to string via
// fmt.Sprintf so the LLM can return numeric-typed JSON values and still match.
// See enumValueAllowed in validation.go, which uses strict string comparison
// (with a numeric fallback only for BSON round-trips) because stored values are
// already coerced to the canonical type.
func enumAllowed(s string, allowed []interface{}) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if fmt.Sprintf("%v", a) == s {
			return true
		}
	}
	return false
}

func coerceFailReason(present bool) string {
	if !present {
		return "llm value missing; randomized"
	}
	return "llm value invalid or out of range; randomized"
}

// --- coordinate helpers --------------------------------------------------------

// clampLLMPoint rounds and clamps an LLMPoint into the environment grid, reusing
// placement.go's clampPoint so the LLM path and the algorithmic path share one
// bounds definition.
func clampLLMPoint(p LLMPoint, env *datasims.SimulationEnvironment) common.Point {
	return clampPoint(p.X, p.Y, env)
}

func clampLLMPoints(pts []LLMPoint, env *datasims.SimulationEnvironment) []common.Point {
	out := make([]common.Point, 0, len(pts))
	for _, p := range pts {
		out = append(out, clampPoint(p.X, p.Y, env))
	}
	return out
}

func shapeExists(env *datasims.SimulationEnvironment, uid string) bool {
	for _, s := range env.Shapes {
		if s.ShapeUID == uid {
			return true
		}
	}
	return false
}
