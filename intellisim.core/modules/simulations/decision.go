package simulations

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
)

// rawAction mirrors the constrained action vocabulary a full-control LLM may
// emit. It is the wire shape decoded from a full-strategy response; MapFullActions
// validates and converts it into the engine's ActionIntent. Only the action types
// and fields the LLM is allowed to drive are present (movement / property deltas /
// state transition / interaction / emit_event) — spawn/remove/transform are
// engine/rule concerns and are not exposed to the LLM.
//
// Task 17 adds emit_event support: the LLM may include actions of type
// "emit_event" with event_uid (required) and event_data (optional payload).
// validateRawAction enforces authorization via authorizeEmit before accepting.
type rawAction struct {
	Type        string   `json:"type"`
	PropertyUID string   `json:"property_uid"`
	Value       any      `json:"value"`
	Modifier    string   `json:"modifier"`
	TargetUID   string   `json:"target_uid"`
	Speed       *float64 `json:"speed"`
	Direction   *float64 `json:"direction"`
	NewState    string   `json:"new_state"`
	// emit_event fields (Task 17)
	EventUID  string `json:"event_uid"`
	EventData any    `json:"event_data"`
}

// jitterFrac bounds the per-agent stochastic variation applied to archetype
// directives: speeds and numeric property deltas wobble within ±jitterFrac, and
// movement headings rotate within ±movementJitterRad. Kept small so the directive
// still dominates while guaranteeing agents of one archetype never act identically.
const (
	jitterFrac        = 0.15
	movementJitterRad = 0.30
)

// resolveArchetypeMode returns the effective archetype application mode for a
// resolved strategy. An empty (or any unrecognized) mode resolves to identical so
// models persisted before the field existed keep their legacy behaviour; only an
// explicit "distribution" enables per-agent sampling.
func resolveArchetypeMode(s models.LLMStrategy) models.ArchetypeMode {
	if s.ArchetypeMode == models.ArchetypeModeDistribution {
		return models.ArchetypeModeDistribution
	}
	return models.ArchetypeModeIdentical
}

// resolveVariationStrength returns the effective per-agent jitter fraction for a
// resolved strategy: a non-positive value falls back to the built-in jitterFrac
// (0.15) and values above 1 are clamped, so VariationStrength always lands in
// (0,1]. It scales BOTH numeric-property jitter and movement speed/heading jitter.
func resolveVariationStrength(s models.LLMStrategy) float64 {
	v := s.VariationStrength
	if v <= 0 {
		return jitterFrac
	}
	if v > 1 {
		return 1
	}
	return v
}

// DecisionMenu returns the allowed decision labels for an archetype: the set of
// Condition.Value strings of ConditionTypeDecision conditions on enabled rules
// whose AppliesTo includes the archetype UID (composite sub-conditions are walked
// recursively). The result is de-duplicated, preserving first-seen order. An empty
// menu means the LLM may return a free label.
func DecisionMenu(model *models.Model, archetypeUID string) []string {
	if model == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var menu []string
	add := func(label string) {
		if label == "" {
			return
		}
		if _, dup := seen[label]; dup {
			return
		}
		seen[label] = struct{}{}
		menu = append(menu, label)
	}
	for i := range model.Rules {
		r := &model.Rules[i]
		if !r.Enabled || !ruleAppliesToUID(r, archetypeUID) {
			continue
		}
		for _, c := range r.Conditions {
			collectDecisionLabels(c, add)
		}
	}
	return menu
}

// DecisionMenuWithHints returns the archetype's decision labels annotated with
// what the rules do with each label — "label (triggers: t1, t2)" — derived
// from the action types of enabled rules that consume the label via a decision
// condition. Labels no rule consumes are returned bare. Prompt-side only:
// validation/fallback callers keep using DecisionMenu.
func DecisionMenuWithHints(model *models.Model, archetypeUID string) []string {
	labels := DecisionMenu(model, archetypeUID)
	if len(labels) == 0 {
		return nil
	}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, label+decisionLabelHint(model, archetypeUID, label))
	}
	return out
}

// decisionLabelHint renders " (triggers: …)" from the ≤4 first-seen action
// types of enabled rules (applying to the archetype) that consume the label.
// When any of those rules' actions carries a non-empty LLMPromptContext, the
// FIRST one found (first-seen rule order, first action with a hint) is appended
// after the types — "hunt (triggers: move_toward, modify_property — chase the
// nearest prey)" — trimmed and capped at maxHintContextRunes runes.
func decisionLabelHint(model *models.Model, archetypeUID, label string) string {
	const maxTypes = 4
	const maxHintContextRunes = 100
	seen := map[string]bool{}
	var types []string
	hint := ""
	for i := range model.Rules {
		r := &model.Rules[i]
		if !r.Enabled || !ruleAppliesToUID(r, archetypeUID) || !ruleConsumesDecision(r, label) {
			continue
		}
		for _, a := range r.Actions {
			s := string(a.Type)
			if !seen[s] && len(types) < maxTypes {
				seen[s] = true
				types = append(types, s)
			}
			if hint == "" {
				if c := strings.TrimSpace(a.LLMPromptContext); c != "" {
					hint = c
				}
			}
		}
		if len(types) == maxTypes && hint != "" {
			break
		}
	}
	if len(types) == 0 {
		return ""
	}
	out := " (triggers: " + strings.Join(types, ", ")
	if hint != "" {
		out += " — " + capRunes(hint, maxHintContextRunes)
	}
	return out + ")"
}

// capRunes truncates s to at most n runes, appending a "…" suffix when it had to
// cut. Shared by decisionLabelHint and llmEmittableEventLabels so the two prompt
// context caps can't drift apart.
func capRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// ruleConsumesDecision reports whether any (possibly nested) condition of the
// rule is a decision condition on the given label.
func ruleConsumesDecision(r *models.Rule, label string) bool {
	found := false
	for _, c := range r.Conditions {
		collectDecisionLabels(c, func(l string) {
			if l == label {
				found = true
			}
		})
	}
	return found
}

// llmEmittableEventLabels lists the events the LLM is authorized to emit for an
// archetype (Producers include "llm"; ProducerUIDs empty or listing the
// archetype), rendered as prompt labels: the event UID, enriched with the
// EventDefinition's trimmed LLMPromptContext when non-empty — e.g. "howl (wolf
// howl to rally the pack)". Used to ADVERTISE the already-parsed optional
// emit_uid/emit_data response fields on the decision AND full-control specs;
// authorization is still enforced at demux time. Model event order preserved
// (deterministic).
func llmEmittableEventLabels(model *models.Model, archetypeUID string) []string {
	if model == nil {
		return nil
	}
	const maxEmitContextRunes = 100
	var out []string
	for i := range model.Events {
		def := &model.Events[i]
		if !authorizeEmit(def, models.EventProducerLLM, archetypeUID) {
			continue
		}
		label := def.UID
		if ctx := strings.TrimSpace(def.LLMPromptContext); ctx != "" {
			label += " (" + capRunes(ctx, maxEmitContextRunes) + ")"
		}
		out = append(out, label)
	}
	return out
}
// ruleAppliesToUID reports whether a rule targets the given archetype UID. An
// empty AppliesTo applies to every archetype (matching the rule engine).
func ruleAppliesToUID(r *models.Rule, uid string) bool {
	if len(r.AppliesTo) == 0 {
		return true
	}
	for _, u := range r.AppliesTo {
		if u == uid {
			return true
		}
	}
	return false
}

// collectDecisionLabels emits every decision label reachable from a condition,
// recursing through composite sub-conditions.
func collectDecisionLabels(c models.Condition, add func(string)) {
	if c.Type == models.ConditionTypeDecision {
		add(toString(c.Value))
	}
	for _, sub := range c.SubConditions {
		collectDecisionLabels(sub, add)
	}
}

// ApplyDirectiveWithVariation fans one archetype-level directive out to each
// member of the group, resolving it relative to that agent's own state and local
// context and adding bounded seeded jitter. The per-agent RNG is seeded from
// rs.Sim.Seed ^ rs.Step ^ agentID so a run is reproducible yet each agent diverges:
//
//   - Movement: the directive's TargetRef is resolved per agent (nearest matching
//     neighbour/shape, or a literal "x,y" point) and turned into a velocity via
//     VectorToward (or away), then perturbed by a small per-agent heading/speed
//     jitter. The resolved velocity is written to ra.Velocity (the spatial phase
//     integrates it) and echoed on the returned MovementIntent.
//   - Properties: numeric values are jittered within ±jitterFrac and clamped to the
//     property's [Min,Max]; enum/bool/string values are copied verbatim (no jitter).
//   - Decisions: under ArchetypeModeIdentical the same label + a copy of the params
//     are handed to every member (downstream per-agent rules and own state produce
//     the divergence). Under ArchetypeModeDistribution the directive carries a
//     WEIGHTED DISTRIBUTION (DecisionDist) and each member samples ONE label via its
//     per-agent RNG, so members genuinely diverge from a single LLM call.
//   - Actions (archetype_full): identical mode copies the one action template to every
//     member; distribution mode (ActionVariants) samples one whole variant per member.
//
// VariationStrength (resolved from group.Strategy) scales the numeric/movement jitter.
// The per-member RNG is drawn in a FIXED order — movement, properties, decision
// sample, action sample — and the two sampling draws happen ONLY in distribution mode
// (guarded by len(DecisionDist)/len(ActionVariants)), so identical-mode runs reproduce
// the legacy RNG stream byte-for-byte.
//
// It returns one *AgentIntent per member keyed by instance id.
func ApplyDirectiveWithVariation(rs *RuntimeState, group *AgentGroup, directive *AgentIntent) map[uint64]*AgentIntent {
	out := make(map[uint64]*AgentIntent)
	if rs == nil || group == nil || directive == nil {
		return out
	}
	mode := resolveArchetypeMode(group.Strategy)
	strength := resolveVariationStrength(group.Strategy)
	for _, ra := range group.Members {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		rng := agentJitterRNG(rs, ra.A.ID)
		intent := &AgentIntent{}

		if directive.Movement != nil {
			if mv := resolveMovementForAgent(rs, ra, *directive.Movement, rng, strength); mv != nil {
				intent.Movement = mv
			}
		}
		if len(directive.Properties) > 0 {
			intent.Properties = varyProperties(ra, directive.Properties, rng, strength)
		}

		// Decision: sample from the distribution when present, else copy the shared label.
		if mode == models.ArchetypeModeDistribution && len(directive.DecisionDist) > 0 {
			idx := pickChoice(rng, decisionWeights(directive.DecisionDist))
			intent.Decisions = copyDecisions([]DecisionIntent{directive.DecisionDist[idx].Decision})
		} else if len(directive.Decisions) > 0 {
			intent.Decisions = copyDecisions(directive.Decisions)
		}

		// Actions (archetype_full): sample one whole variant when present, else copy the
		// shared action template. A sampled variant also carries its own emits.
		if mode == models.ArchetypeModeDistribution && len(directive.ActionVariants) > 0 {
			idx := pickChoice(rng, actionVariantWeights(directive.ActionVariants))
			v := directive.ActionVariants[idx]
			intent.Actions = append([]ActionIntent(nil), v.Actions...)
			intent.Emits = append(intent.Emits, v.Emits...)
		} else if len(directive.Actions) > 0 {
			intent.Actions = append([]ActionIntent(nil), directive.Actions...)
		}

		// Task 17: propagate LLM-emit intents to each member so ApplyIntentActions
		// can drain them via applyEmitEvent(..., EventProducerLLM). Directive-level
		// emits carry no per-agent variation (identical event UID + payload for all
		// members). Appended so they compose with any emits a sampled variant added.
		if len(directive.Emits) > 0 {
			intent.Emits = append(intent.Emits, directive.Emits...)
		}

		// Derive a compact LLM-response note from THIS MEMBER's sampled intent (not the
		// directive) so distribution mode records the per-agent outcome. In identical
		// mode intent.* == directive.*, so the note is unchanged. Never thread raw text.
		if len(intent.Decisions) > 0 {
			intent.LLMNote = "chose: " + intent.Decisions[0].Label
		} else if len(intent.Actions) > 0 {
			types := make([]string, 0, len(intent.Actions))
			seen := map[string]bool{}
			for _, ai := range intent.Actions {
				s := string(ai.Type)
				if !seen[s] {
					seen[s] = true
					types = append(types, s)
				}
			}
			intent.LLMNote = "actions: " + strings.Join(types, ",")
		}

		out[ra.A.ID] = intent
	}
	return out
}

// decisionWeights projects a decision distribution onto the ChoiceWeight slice the
// shared sampler (pickChoice) consumes; the Value is the source index so the picked
// index maps straight back into the distribution.
func decisionWeights(dist []WeightedDecision) []models.ChoiceWeight {
	out := make([]models.ChoiceWeight, len(dist))
	for i, wd := range dist {
		out[i] = models.ChoiceWeight{Value: i, Weight: wd.Weight}
	}
	return out
}

// actionVariantWeights projects action-set variants onto ChoiceWeight for pickChoice.
func actionVariantWeights(variants []WeightedActionSet) []models.ChoiceWeight {
	out := make([]models.ChoiceWeight, len(variants))
	for i, wv := range variants {
		out[i] = models.ChoiceWeight{Value: i, Weight: wv.Weight}
	}
	return out
}

// agentJitterRNG derives a per-agent deterministic RNG seeded from the run seed,
// step and agent id (Seed ^ Step ^ agentID), so jitter is reproducible across runs
// of the same seed while differing per agent and per step.
func agentJitterRNG(rs *RuntimeState, agentID uint64) *rand.Rand {
	var seed uint64
	if rs.Sim != nil {
		seed = rs.Sim.Seed
	}
	mix := seed ^ rs.Step ^ agentID
	return rand.New(rand.NewSource(int64(mix)))
}

// resolveMovementForAgent turns an archetype movement directive into a concrete,
// per-agent movement intent + velocity. The target reference is resolved relative
// to the agent's own position; the heading/speed are jittered by `strength` (the
// resolved VariationStrength). The jittered velocity is both written to ra.Velocity
// AND pinned on the returned MovementIntent.Resolved, so applySpatial integrates the
// jittered vector verbatim (the heading jitter previously died because applySpatial
// re-resolved the intent through the non-jittered resolveMovement). Returns nil when
// the agent cannot move (no position / immovable / no resolvable target).
func resolveMovementForAgent(rs *RuntimeState, ra *RuntimeAgent, dir MovementIntent, rng *rand.Rand, strength float64) *MovementIntent {
	if ra.A.Position == nil || !movable(ra) {
		return nil
	}
	w, h, torus := envDims(rs)
	speed := jitterScalar(rng, clampSpeed(dir.Speed), strength)
	headingJitter := movementJitterRad * strength

	switch dir.Intent {
	case "hold", "":
		ra.Velocity = Vector{}
		zero := Vector{}
		return &MovementIntent{Intent: "hold", TargetRef: dir.TargetRef, Speed: 0, Resolved: &zero}
	case "wander":
		angle := rng.Float64() * 2 * math.Pi
		v := Vector{DX: math.Cos(angle) * speed, DY: math.Sin(angle) * speed}
		ra.Velocity = v
		return &MovementIntent{Intent: "wander", Speed: speed, Resolved: &v}
	case "toward", "away":
		target := resolveTargetPoint(rs, ra, dir.TargetRef)
		if target == nil {
			// No resolvable target: fall back to a wander so the agent still
			// participates (and still diverges from its archetype siblings).
			angle := rng.Float64() * 2 * math.Pi
			v := Vector{DX: math.Cos(angle) * speed, DY: math.Sin(angle) * speed}
			ra.Velocity = v
			return &MovementIntent{Intent: "wander", TargetRef: dir.TargetRef, Speed: speed, Resolved: &v}
		}
		v := VectorToward(*ra.A.Position, *target, w, h, torus, speed)
		if dir.Intent == "away" {
			v = Vector{DX: -v.DX, DY: -v.DY}
		}
		v = jitterHeading(v, rng, headingJitter)
		ra.Velocity = v
		return &MovementIntent{Intent: dir.Intent, TargetRef: dir.TargetRef, Speed: speed, Resolved: &v}
	default:
		return nil
	}
}

// resolveMovement turns a (already-resolved) MovementIntent into a concrete
// velocity for an agent WITHOUT jitter — the deterministic core shared by the
// step's spatial phase and the archetype variation helper, so individual and
// archetype movement integrate identically. The intent's TargetRef is resolved
// relative to the agent's own position/local context; toward/away point at (or
// away from) the resolved target; wander picks a deterministic per-agent direction
// from rs.RNG; hold (or an unresolvable target) yields a zero vector. Static
// (immovable) agents and agents without a position yield a zero vector.
func resolveMovement(rs *RuntimeState, ra *RuntimeAgent, mv MovementIntent) Vector {
	if ra == nil || ra.A == nil || ra.A.Position == nil || !movable(ra) {
		return Vector{}
	}
	w, h, torus := envDims(rs)
	speed := clampSpeed(mv.Speed)

	switch mv.Intent {
	case "hold", "":
		return Vector{}
	case "wander":
		var angle float64
		if rs.RNG != nil {
			angle = rs.RNG.Float64() * 2 * math.Pi
		}
		return Vector{DX: math.Cos(angle) * speed, DY: math.Sin(angle) * speed}
	case "toward", "away":
		target := resolveTargetPoint(rs, ra, mv.TargetRef)
		if target == nil {
			return Vector{}
		}
		v := VectorToward(*ra.A.Position, *target, w, h, torus, speed)
		if mv.Intent == "away" {
			v = Vector{DX: -v.DX, DY: -v.DY}
		}
		return v
	default:
		return Vector{}
	}
}

// resolveTargetPoint resolves a movement TargetRef relative to the agent. A
// "x,y" literal yields that point (clamped/wrapped to the world); otherwise the
// ref is treated as an archetype/shape UID and the nearest matching neighbour or
// shape is used. An empty ref resolves to the nearest agent of any archetype.
func resolveTargetPoint(rs *RuntimeState, ra *RuntimeAgent, ref string) *common.Point {
	if p, ok := parsePoint(ref); ok {
		return clampedPoint(rs, float64(p.X), float64(p.Y))
	}
	var uids []string
	if ref != "" {
		uids = []string{ref}
	}
	return nearestTargetPos(rs, ra, uids)
}

// parsePoint parses a "x,y" literal into a Point (integer or float coordinates,
// negatives clamped to 0 by the caller's clamp). Returns ok=false when ref is not
// a coordinate literal.
func parsePoint(ref string) (common.Point, bool) {
	if !strings.Contains(ref, ",") {
		return common.Point{}, false
	}
	parts := strings.SplitN(ref, ",", 2)
	xf, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	yf, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return common.Point{}, false
	}
	if xf < 0 {
		xf = 0
	}
	if yf < 0 {
		yf = 0
	}
	return common.Point{X: uint64(math.Round(xf)), Y: uint64(math.Round(yf))}, true
}

// clampSpeed bounds a directive speed to [0,1] (the movement contract uses a
// normalized 0..1 speed).
func clampSpeed(s float64) float64 {
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}

// jitterScalar perturbs v by a uniform ±frac fraction of its own magnitude. A
// zero v stays zero (nothing to scale).
func jitterScalar(rng *rand.Rand, v, frac float64) float64 {
	if v == 0 || frac <= 0 {
		return v
	}
	delta := (rng.Float64()*2 - 1) * frac * v
	return v + delta
}

// jitterHeading rotates a velocity vector by a small uniform ±maxRad angle,
// preserving its magnitude. A zero vector is returned unchanged.
func jitterHeading(v Vector, rng *rand.Rand, maxRad float64) Vector {
	mag := math.Hypot(v.DX, v.DY)
	if mag == 0 {
		return v
	}
	angle := math.Atan2(v.DY, v.DX) + (rng.Float64()*2-1)*maxRad
	return Vector{DX: math.Cos(angle) * mag, DY: math.Sin(angle) * mag}
}

// varyProperties applies an archetype property directive per agent: numeric values
// are jittered within ±strength (the resolved VariationStrength) and clamped to the
// property's bounds; non-numeric values are copied verbatim. The directive's numeric
// value is treated as the target value (not a delta) so clamping keeps every agent
// in-bounds.
func varyProperties(ra *RuntimeAgent, dir map[string]any, rng *rand.Rand, strength float64) map[string]any {
	out := make(map[string]any, len(dir))
	// Iterate keys in sorted order: this function consumes RNG per numeric
	// property, so ranging the map directly would let Go's randomized map
	// iteration order decide the draw→property assignment — nondeterministic
	// across process runs, violating the engine determinism doctrine.
	uids := make([]string, 0, len(dir))
	for uid := range dir {
		uids = append(uids, uid)
	}
	sort.Strings(uids)
	for _, uid := range uids {
		v := dir[uid]
		desc := propByUID(ra, uid)
		if desc != nil && desc.Type == models.PropertyTypeNumber {
			if f, ok := coerceFloat(v); ok {
				out[uid] = clampToBounds(jitterScalar(rng, f, strength), desc)
				continue
			}
		}
		out[uid] = v
	}
	return out
}

// copyDecisions returns a deep-ish copy of the directive decisions (same labels,
// copied params maps) so per-agent intents don't share the directive's mutable
// params.
func copyDecisions(src []DecisionIntent) []DecisionIntent {
	out := make([]DecisionIntent, 0, len(src))
	for _, d := range src {
		cp := DecisionIntent{Label: d.Label}
		if len(d.Params) > 0 {
			cp.Params = make(map[string]any, len(d.Params))
			for k, v := range d.Params {
				cp.Params[k] = v
			}
		}
		out = append(out, cp)
	}
	return out
}

// MapFullActions converts a full-control LLM action list into []ActionIntent for
// the rules/spatial phases to apply. Each raw action is validated against the
// agent's archetype: property actions must reference a declared, mutable property;
// transitions must name a declared state; the action type must be one of the
// allowed full-control verbs. Invalid entries are dropped (a warning is logged via
// the runtime logger when available); valid entries are returned in order.
//
// Task 17: emit_event actions are validated via authorizeEmit(def,
// EventProducerLLM, archetypeUID) and, when authorized, returned as
// ActionIntent{Type: ActionTypeEmitEvent, EventUID, EventData}. They are consumed
// in ApplyIntentActions by the Emits drain path rather than the Actions loop, but
// carrying them as ActionIntents is consistent with the existing mapper contract.
func MapFullActions(rs *RuntimeState, ra *RuntimeAgent, raw []rawAction) []ActionIntent {
	out := make([]ActionIntent, 0, len(raw))
	for _, r := range raw {
		ai, ok := validateRawAction(rs, ra, r)
		if !ok {
			continue
		}
		out = append(out, ai)
	}
	return out
}

// validateRawAction validates one rawAction against the agent's properties/states
// and converts it to an ActionIntent. Returns ok=false for unknown action types,
// unknown/immutable properties, unknown states, or unauthorized event emits.
//
// Task 17: the emit_event case resolves the EventDefinition via eventDefByUID and
// gates on authorizeEmit(def, EventProducerLLM, archetypeUID). Locked on
// EventDefinition is a model-generation guard only (freezes the definition against
// LLM-editor rewrites); it is NOT applied here — runtime LLM emit authorization
// is controlled exclusively by the Producers / ProducerUIDs fields on the
// EventDefinition.
func validateRawAction(rs *RuntimeState, ra *RuntimeAgent, r rawAction) (ActionIntent, bool) {
	switch models.ActionType(r.Type) {
	case models.ActionTypeSetProperty:
		if r.PropertyUID == "" || r.Value == nil {
			return ActionIntent{}, false
		}
		// target_uid == "environment" routes the write to a declared environment
		// global property (applyGlobalPropertyAction re-validates at apply time).
		if r.TargetUID == models.EnvironmentUID {
			desc := envPropDef(rs, r.PropertyUID)
			if desc == nil || desc.Immutable {
				return ActionIntent{}, false
			}
			return ActionIntent{Type: models.ActionTypeSetProperty, PropertyUID: r.PropertyUID, Value: r.Value, TargetUID: models.EnvironmentUID}, true
		}
		desc := propByUID(ra, r.PropertyUID)
		if desc == nil || desc.Immutable {
			return ActionIntent{}, false
		}
		return ActionIntent{Type: models.ActionTypeSetProperty, PropertyUID: r.PropertyUID, Value: r.Value}, true

	case models.ActionTypeModifyProperty:
		if r.PropertyUID == "" {
			return ActionIntent{}, false
		}
		mod := models.PropertyModifier(r.Modifier)
		if !validModifier(mod) {
			return ActionIntent{}, false
		}
		if r.TargetUID == models.EnvironmentUID {
			desc := envPropDef(rs, r.PropertyUID)
			if desc == nil || desc.Immutable {
				return ActionIntent{}, false
			}
			return ActionIntent{Type: models.ActionTypeModifyProperty, PropertyUID: r.PropertyUID, Modifier: mod, Value: r.Value, TargetUID: models.EnvironmentUID}, true
		}
		desc := propByUID(ra, r.PropertyUID)
		if desc == nil || desc.Immutable {
			return ActionIntent{}, false
		}
		return ActionIntent{Type: models.ActionTypeModifyProperty, PropertyUID: r.PropertyUID, Modifier: mod, Value: r.Value}, true

	case models.ActionTypeTransition:
		if r.NewState == "" || !validState(ra, r.NewState) {
			return ActionIntent{}, false
		}
		return ActionIntent{Type: models.ActionTypeTransition, NewState: r.NewState}, true

	case models.ActionTypeMove:
		if !movable(ra) {
			return ActionIntent{}, false
		}
		return ActionIntent{Type: models.ActionTypeMove, Speed: r.Speed, Direction: r.Direction}, true

	case models.ActionTypeMoveToward, models.ActionTypeMoveAway:
		if !movable(ra) {
			return ActionIntent{}, false
		}
		return ActionIntent{Type: models.ActionType(r.Type), TargetUID: r.TargetUID, Speed: r.Speed}, true

	case models.ActionTypeInteract:
		return ActionIntent{Type: models.ActionTypeInteract, TargetUID: r.TargetUID}, true

	case models.ActionTypeEmitEvent:
		// Task 17: LLM-emitted events. Require a non-empty event_uid, resolve the
		// EventDefinition, and authorize the emit for EventProducerLLM with the
		// emitting agent's archetype UID. Unauthorized (no "llm" in Producers, or
		// ProducerUIDs mismatch) emits are silently dropped.
		if r.EventUID == "" {
			return ActionIntent{}, false
		}
		archetypeUID := ""
		if ra != nil && ra.A != nil {
			archetypeUID = ra.A.AgentUID
		}
		def := eventDefByUID(rs, r.EventUID)
		if !authorizeEmit(def, models.EventProducerLLM, archetypeUID) {
			return ActionIntent{}, false
		}
		return ActionIntent{
			Type:      models.ActionTypeEmitEvent,
			EventUID:  r.EventUID,
			EventData: r.EventData,
		}, true

	default:
		return ActionIntent{}, false
	}
}

// validModifier reports whether m is one of the recognized property modifiers.
func validModifier(m models.PropertyModifier) bool {
	switch m {
	case models.PropertyModifierSet, models.PropertyModifierAdd, models.PropertyModifierSubtract,
		models.PropertyModifierMultiply, models.PropertyModifierDivide,
		models.PropertyModifierIncrement, models.PropertyModifierDecrement, models.PropertyModifierToggle:
		return true
	default:
		return false
	}
}

// --- per-step decision recording (text_log "llm_decisions") ---------------------

// stepDecisionMaxRunes caps the recorded per-step decision summary so a verbose
// LLMNote never bloats step rows / WS frames.
const stepDecisionMaxRunes = 120

// summarizeIntent derives the compact per-step decision summary recorded for the
// text_log widget's "llm_decisions" kind. Decision labels win — they cover the
// decision strategies, per-member sampled distribution outcomes, and replayed
// LastDecision fallbacks (a replayed fallback IS the decision applied this step,
// so it is deliberately recorded). Full-control intents without decisions fall
// back to LLMNote (already a compact action summary, e.g. "actions: move,set_property").
// An empty result means nothing decision-like happened — nothing is recorded.
func summarizeIntent(in *AgentIntent) string {
	if in == nil {
		return ""
	}
	s := ""
	if len(in.Decisions) > 0 {
		labels := make([]string, 0, len(in.Decisions))
		for _, d := range in.Decisions {
			if d.Label != "" {
				labels = append(labels, d.Label)
			}
		}
		s = strings.Join(labels, ", ")
	}
	if s == "" {
		s = in.LLMNote
	}
	if r := []rune(s); len(r) > stepDecisionMaxRunes {
		s = string(r[:stepDecisionMaxRunes])
	}
	return s
}

// recordStepDecisions stamps this step's LLM decision summaries onto the live
// agents (transient RuntimeAgent.stepDecision) from the merged intents map,
// clearing every agent first so a stale summary never leaks across steps. Runs
// right after runLLMPhase — BEFORE the mutation phases clone the agents — so the
// summary rides the synchronous clone/commit cycle untouched. buildStepRows
// consumes it (SimulationStep.Decision) and clearStepDecisions wipes it after
// the rows are built.
func recordStepDecisions(rs *RuntimeState, intents map[uint64]*AgentIntent) {
	if rs == nil {
		return
	}
	clearStepDecisions(rs)
	for id, in := range intents {
		ra := rs.ByID[id]
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		if s := summarizeIntent(in); s != "" {
			ra.stepDecision = s
		}
	}
}

// clearStepDecisions wipes the transient per-step decision summary on all agents.
func clearStepDecisions(rs *RuntimeState) {
	if rs == nil {
		return
	}
	for _, ra := range rs.Agents {
		if ra != nil {
			ra.stepDecision = ""
		}
	}
}
