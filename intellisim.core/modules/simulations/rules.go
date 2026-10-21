package simulations

import (
	"math"
	"sort"
	"strings"
	"time"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// nowFn is the clock used to timestamp memory events produced while applying
// rules. It is a package var so tests/the step pipeline can inject a fixed clock.
var nowFn = time.Now

// memEvent is one memory event produced by applying an action; the caller
// appends it to the agent's memory log via AppendMemory.
type memEvent struct {
	Type    datasims.AgentMemoryType
	Summary string
	Details any
}

// purposeSet is the set of purposes owned by an agent's LLM strategy. Rules must
// skip actions for any purpose the LLM controls for that agent.
type purposeSet map[purpose]bool

// has reports whether the set contains p (nil-safe).
func (s purposeSet) has(p purpose) bool {
	if s == nil {
		return false
	}
	return s[p]
}

// actionPurpose maps an action type to the LLM purpose that owns it, or returns
// ("", false) for actions no purpose can own (always run regardless of strategy).
func actionPurpose(t models.ActionType) (purpose, bool) {
	switch t {
	case models.ActionTypeMove, models.ActionTypeMoveToward, models.ActionTypeMoveAway, models.ActionTypeTeleport:
		return purposeMovement, true
	case models.ActionTypeSetProperty, models.ActionTypeModifyProperty:
		return purposeProperties, true
	case models.ActionTypeTransition:
		return purposeDecision, true
	// NOTE: call_llm deliberately has NO purpose. Mapping it to purposeDecision
	// suppressed it for exactly the agents whose LLM strategy could honor it
	// (ApplyRules skips rule actions for LLM-owned purposes) — the inverse of the
	// intended semantics. With no purpose, rules always execute it (the handler
	// gates on a decision-capable strategy) and ApplyIntentActions skips it.
	case models.ActionTypeInteract:
		return purposeInteraction, true
	default:
		return "", false
	}
}

// EvalCondition evaluates one condition for an agent against the runtime state and
// the agent's current LLM intent (used by ConditionTypeDecision). Unknown or
// malformed conditions evaluate to false rather than aborting the run.
func EvalCondition(rs *RuntimeState, ra *RuntimeAgent, intent *AgentIntent, c models.Condition) bool {
	switch c.Type {
	case models.ConditionTypeGlobalProperty:
		return evalGlobalProperty(rs, c)
	case models.ConditionTypeProperty:
		return evalProperty(rs, ra, c)
	case models.ConditionTypePosition:
		return evalPosition(rs, ra, c)
	case models.ConditionTypeProximity:
		return evalProximity(rs, ra, c)
	case models.ConditionTypeStep:
		return compareValues(float64(rs.Step), c.Operator, c.Value, c.MinValue, c.MaxValue)
	case models.ConditionTypeComposite:
		return evalComposite(rs, ra, intent, c)
	case models.ConditionTypeCount:
		return evalCount(rs, ra, c)
	case models.ConditionTypeBarrier:
		return evalBarrier(rs, ra, c)
	case models.ConditionTypeState:
		return compareValues(ra.A.State, c.Operator, c.Value, nil, nil)
	case models.ConditionTypeDistance:
		return evalDistance(rs, ra, c)
	case models.ConditionTypeDecision:
		return evalDecision(intent, c)
	case models.ConditionTypeRandom:
		return evalRandom(rs, c)
	case models.ConditionTypeMemory:
		return evalMemory(ra, c)
	case models.ConditionTypeEvent:
		return evalEvent(rs, ra, c)
	case models.ConditionTypeCustom:
		return evalCustom(ra, c)
	default:
		return false
	}
}

func evalGlobalProperty(rs *RuntimeState, c models.Condition) bool {
	if rs.Env == nil || rs.Env.GlobalProperties == nil {
		return false
	}
	v, ok := rs.Env.GlobalProperties[c.PropertyUID]
	if !ok {
		return false
	}
	return compareValues(v, c.Operator, c.Value, c.MinValue, c.MaxValue)
}

func evalProperty(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	switch c.PropertyUID {
	case models.PseudoPropertyState:
		return compareValues(ra.A.State, c.Operator, c.Value, nil, nil)
	case models.PseudoPropertyPosition:
		return evalPosition(rs, ra, c)
	}
	if ra.A.Properties == nil {
		return false
	}
	v, ok := ra.A.Properties[c.PropertyUID]
	if !ok {
		return false
	}
	return compareValues(v, c.Operator, c.Value, c.MinValue, c.MaxValue)
}

func evalPosition(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	if ra.A.Position == nil {
		return false
	}
	if c.Operator == models.ConditionOpInRange && c.MinValue != nil && c.MaxValue != nil {
		x, y := float64(ra.A.Position.X), float64(ra.A.Position.Y)
		inX := x >= *c.MinValue && x <= *c.MaxValue
		inY := y >= *c.MinValue && y <= *c.MaxValue
		return inX || inY
	}
	// "in"/"not_in" a shape template (U8): true when the agent's body overlaps
	// any instance of a targeted shape (touching counts; solidity irrelevant —
	// passable door/sensor regions are the primary use case).
	if c.Operator == models.ConditionOpIn || c.Operator == models.ConditionOpNotIn {
		uids := targetUIDs(c)
		if len(uids) == 0 || rs == nil || rs.Env == nil {
			return false
		}
		w, h, torus := envDims(rs)
		inside := false
		for i := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[i]
			if !containsString(uids, sh.ShapeUID) {
				continue
			}
			if BodyOverlapsShape(ra, sh, w, h, torus) {
				inside = true
				break
			}
		}
		if c.Operator == models.ConditionOpNotIn {
			return !inside
		}
		return inside
	}
	return false
}

// containsString reports whether s is in the list.
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// targetUIDs returns the archetype UID set a condition/proximity targets:
// TargetUIDs takes precedence, falling back to a single TargetUID. Empty => any.
func targetUIDs(c models.Condition) []string {
	if len(c.TargetUIDs) > 0 {
		return c.TargetUIDs
	}
	if c.TargetUID != "" {
		return []string{c.TargetUID}
	}
	return nil
}

// matchesTarget reports whether other's archetype UID is in the (possibly empty)
// uid set; an empty set matches any agent.
func matchesTarget(other *RuntimeAgent, uids []string) bool {
	if len(uids) == 0 {
		return true
	}
	for _, u := range uids {
		if other.A.AgentUID == u {
			return true
		}
	}
	return false
}

// neighborsWithin returns the runtime agents (excluding self) whose footprint is within
// edge-to-edge distance r of ra's footprint, matching the target uid set, using the spatial
// index when available. For point agents this is identical to center-distance proximity.
func neighborsWithin(rs *RuntimeState, ra *RuntimeAgent, r float64, uids []string) []*RuntimeAgent {
	if ra.A.Position == nil {
		return nil
	}
	q := bodyForAgent(ra)
	var out []*RuntimeAgent
	if rs.Index != nil {
		resolve := func(id uint64) CollisionBody { return bodyForAgent(rs.ByID[id]) }
		for _, id := range rs.Index.WithinBody(q, r, resolve) {
			if id == ra.A.ID {
				continue
			}
			other := rs.ByID[id]
			if other == nil || other.removed || !matchesTarget(other, uids) {
				continue
			}
			out = append(out, other)
		}
		return out
	}
	w, h, torus := envDims(rs)
	for _, other := range rs.Agents {
		if other == nil || other.removed || other.A.ID == ra.A.ID || other.A.Position == nil {
			continue
		}
		if !matchesTarget(other, uids) {
			continue
		}
		if BodyDistance(q, bodyForAgent(other), w, h, torus) <= r {
			out = append(out, other)
		}
	}
	return out
}

func evalProximity(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	if ra.A.Position == nil || c.Radius == nil {
		return false
	}
	count := len(neighborsWithin(rs, ra, *c.Radius, targetUIDs(c)))
	// Also count matching shapes within radius when a target shape UID is given. Distance is
	// edge-to-edge between the agent footprint and the shape body (a point agent near a large
	// shape now measures to the shape's edge, not its anchor).
	if rs.Env != nil {
		q := bodyForAgent(ra)
		w, h, torus := envDims(rs)
		for i := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[i]
			if !shapeMatchesTarget(sh, targetUIDs(c)) {
				continue
			}
			if BodyDistance(q, bodyForShape(sh), w, h, torus) <= *c.Radius {
				count++
			}
		}
	}
	if c.Value != nil {
		if expected, ok := coerceFloat(c.Value); ok {
			return compareValues(float64(count), c.Operator, expected, c.MinValue, c.MaxValue)
		}
	}
	return count > 0
}

// shapeMatchesTarget reports whether a shape's UID is in the target uid set
// (empty set => no shape match, since shapes are only counted when explicitly named).
func shapeMatchesTarget(sh *datasims.SimulationShape, uids []string) bool {
	if len(uids) == 0 {
		return false
	}
	for _, u := range uids {
		if sh.ShapeUID == u {
			return true
		}
	}
	return false
}

func evalComposite(rs *RuntimeState, ra *RuntimeAgent, intent *AgentIntent, c models.Condition) bool {
	if len(c.SubConditions) == 0 {
		return false
	}
	if c.LogicOperator == models.LogicOpNOT {
		return !EvalCondition(rs, ra, intent, c.SubConditions[0])
	}
	isAnd := c.LogicOperator == models.LogicOpAND || c.LogicOperator == ""
	for _, sub := range c.SubConditions {
		res := EvalCondition(rs, ra, intent, sub)
		if isAnd && !res {
			return false
		}
		if !isAnd && res {
			return true
		}
	}
	return isAnd
}

func evalCount(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	if ra.A.Position == nil || c.Radius == nil {
		return false
	}
	count := len(neighborsWithin(rs, ra, *c.Radius, targetUIDs(c)))
	return compareValues(float64(count), c.Operator, c.Value, c.MinValue, c.MaxValue)
}

func evalBarrier(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	if ra.A.Position == nil || rs.Env == nil {
		return false
	}
	w, h, torus := envDims(rs)
	// A blocking shape overlapping the agent footprint counts as a barrier (a footprint
	// straddling a barrier edge now triggers it; a point agent matches the old contains test).
	for i := range rs.Env.Shapes {
		sh := &rs.Env.Shapes[i]
		if len(targetUIDs(c)) > 0 && !shapeMatchesTarget(sh, targetUIDs(c)) {
			continue
		}
		if BodyOverlapsShape(ra, sh, w, h, torus) {
			return true
		}
	}
	// World edge is a barrier only for bounded (non-torus) worlds: the agent's footprint
	// AABB touches/exceeds an edge. For a point body this is the legacy edge test.
	body := bodyForAgent(ra)
	cx, cy := body.Center.X, body.Center.Y
	atEdge := cx-body.AABBHalf.X <= 0 || cy-body.AABBHalf.Y <= 0 ||
		cx+body.AABBHalf.X >= float64(rs.Env.Width) || cy+body.AABBHalf.Y >= float64(rs.Env.Height)
	return atEdge && !rs.Env.Torus
}

func evalDistance(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	if ra.A.Position == nil {
		return false
	}
	target, ok := nearestTargetBody(rs, ra, targetUIDs(c))
	if !ok {
		return false
	}
	w, h, torus := envDims(rs)
	d := BodyDistance(bodyForAgent(ra), target, w, h, torus)
	return compareValues(d, c.Operator, c.Value, c.MinValue, c.MaxValue)
}

func evalDecision(intent *AgentIntent, c models.Condition) bool {
	if intent == nil || len(intent.Decisions) == 0 {
		return false
	}
	want := toString(c.Value)
	for _, d := range intent.Decisions {
		if compareValues(d.Label, c.Operator, c.Value, nil, nil) {
			return true
		}
		// Default (no operator / equals) label match.
		if (c.Operator == "" || c.Operator == models.ConditionOpEquals) && d.Label == want {
			return true
		}
	}
	return false
}

func evalRandom(rs *RuntimeState, c models.Condition) bool {
	p, ok := coerceFloat(c.Value)
	if !ok {
		return false
	}
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	if rs.RNG == nil {
		return false
	}
	return rs.RNG.Float64() < p
}

func evalMemory(ra *RuntimeAgent, c models.Condition) bool {
	mt := datasims.AgentMemoryType("")
	within := 0
	if c.Parameters != nil {
		if s, ok := c.Parameters["memory_type"].(string); ok {
			mt = datasims.AgentMemoryType(s)
		}
		if w, ok := coerceFloat(c.Parameters["within"]); ok {
			within = int(w)
		}
	}
	return MemoryMatch(ra, mt, toString(c.Value), within)
}

// eventConditionUID resolves the event UID an event condition targets:
// TargetUID takes precedence, falling back to PropertyUID; empty when neither
// is set (matches evalEvent's historical resolution order).
func eventConditionUID(c models.Condition) string {
	if c.TargetUID != "" {
		return c.TargetUID
	}
	return c.PropertyUID
}

// matchEvent reports whether ev (whose definition is def, possibly nil)
// satisfies condition c's TTL-liveness and optional payload compare at the
// current step. It deliberately does NOT gate on delivery scope
// (eventReaches) — callers representing a positioned consumer (agents) must
// additionally check eventReaches; the environment (a positionless global
// observer) intentionally bypasses it. Shared by evalEvent and its
// environment-scoped counterpart so the two never drift.
func matchEvent(rs *RuntimeState, c models.Condition, ev *runtimeEvent, def *models.EventDefinition) bool {
	if !eventActive(def, ev, rs.Step) {
		return false
	}
	// Optional payload match: Value compared against Data when both present.
	if c.Value != nil && !compareValues(ev.Data, c.Operator, c.Value, c.MinValue, c.MaxValue) {
		return false
	}
	return true
}

func evalEvent(rs *RuntimeState, ra *RuntimeAgent, c models.Condition) bool {
	uid := eventConditionUID(c)
	if uid == "" {
		return false
	}
	def := eventDefByUID(rs, uid)
	for i := range rs.events {
		ev := &rs.events[i]
		if ev.EventUID != uid {
			continue
		}
		if !eventReaches(rs, def, ev, ra) {
			continue
		}
		if matchEvent(rs, c, ev, def) {
			return true
		}
	}
	return false
}

func evalCustom(ra *RuntimeAgent, c models.Condition) bool {
	if c.Parameters == nil {
		return false
	}
	propUID, ok := c.Parameters["property"].(string)
	if !ok || ra.A.Properties == nil {
		return false
	}
	v, exists := ra.A.Properties[propUID]
	if !exists {
		return false
	}
	op := models.ConditionOpEquals
	if s, ok := c.Parameters["operator"].(string); ok {
		op = models.ConditionOperator(s)
	}
	return compareValues(v, op, c.Parameters["value"], nil, nil)
}

// ---- actions ------------------------------------------------------------------

// ApplyAction mutates agent/environment/global state for one action. Movement
// actions set ra.Velocity (integration happens in the spatial phase). Spawn/remove
// /transform are queued on rs and applied at tick end. emit_event pushes to the
// event queue. Immutable properties are skipped (no error). Returns the memory
// events the action produced (caller appends them).
func ApplyAction(rs *RuntimeState, ra *RuntimeAgent, a models.Action) []memEvent {
	switch a.Type {
	case models.ActionTypeSetProperty:
		return applySetProperty(rs, ra, a)
	case models.ActionTypeModifyProperty:
		return applyModifyProperty(rs, ra, a)
	case models.ActionTypeMove:
		return applyMove(rs, ra, a)
	case models.ActionTypeMoveToward:
		return applyMoveToward(rs, ra, a, false)
	case models.ActionTypeMoveAway:
		return applyMoveToward(rs, ra, a, true)
	case models.ActionTypeTransition:
		return applyTransition(ra, a)
	case models.ActionTypeEmitEvent:
		return applyEmitEvent(rs, ra, a, models.EventProducerAgent)
	case models.ActionTypeTeleport:
		return applyTeleport(rs, ra, a)
	case models.ActionTypeSpawnAgent:
		return applySpawn(rs, ra, a, false)
	case models.ActionTypeTransform:
		return applySpawn(rs, ra, a, true)
	case models.ActionTypeRemoveAgent:
		return applyRemove(rs, ra)
	case models.ActionTypeInteract:
		return applyForcedInteract(rs, ra, a)
	case models.ActionTypeCallLLM:
		return applyCallLLM(rs, ra)
	case models.ActionTypeCustom:
		return applyCustom(ra, a)
	default:
		return nil
	}
}

// propByUID returns the archetype property descriptor for uid, or nil.
func propByUID(ra *RuntimeAgent, uid string) *models.AgentProperty {
	if ra.Archetype == nil {
		return nil
	}
	for i := range ra.Archetype.Properties {
		if ra.Archetype.Properties[i].UID == uid {
			return &ra.Archetype.Properties[i]
		}
	}
	return nil
}

// writeProperty sets a property value, honoring Immutable (no-op) and clamping
// numeric values to the descriptor bounds. Returns false when the write was
// skipped (immutable). Numeric values are coerced to float64.
func writeProperty(ra *RuntimeAgent, uid string, value any) bool {
	desc := propByUID(ra, uid)
	if desc != nil && desc.Immutable {
		return false
	}
	if ra.A.Properties == nil {
		ra.A.Properties = make(map[string]any)
	}
	return setPropValue(ra.A.Properties, desc, uid, value)
}

// setPropValue writes value into props[uid], honoring the descriptor: immutable
// properties are skipped, numeric properties are coerced to float64 and clamped
// to the descriptor bounds. Shared by agent property writes and environment
// global-property writes. Returns false when the write was skipped.
func setPropValue(props map[string]any, desc *models.AgentProperty, uid string, value any) bool {
	if desc != nil && desc.Immutable {
		return false
	}
	if desc != nil && desc.Type == models.PropertyTypeNumber {
		if f, ok := coerceFloat(value); ok {
			props[uid] = clampToBounds(f, desc)
			return true
		}
	}
	props[uid] = value
	return true
}

// modifyPropValue applies a modify_property action's modifier to
// props[a.PropertyUID], honoring the descriptor (immutable no-op, numeric
// clamping). Toggle operates on booleans; every other modifier requires the
// current value (and, where applicable, a.Value) to coerce to float64. Shared by
// agent property writes and environment global-property writes. Returns false
// when the write was skipped.
func modifyPropValue(props map[string]any, desc *models.AgentProperty, a models.Action) bool {
	if desc != nil && desc.Immutable {
		return false
	}
	if props == nil {
		return false
	}
	cur, exists := props[a.PropertyUID]
	if !exists {
		return false
	}

	// Toggle operates on booleans.
	if a.Modifier == models.PropertyModifierToggle {
		b, _ := cur.(bool)
		props[a.PropertyUID] = !b
		return true
	}

	curF, ok := coerceFloat(cur)
	if !ok {
		return false
	}
	var next float64
	switch a.Modifier {
	case models.PropertyModifierIncrement:
		next = curF + 1
	case models.PropertyModifierDecrement:
		next = curF - 1
	case models.PropertyModifierSet:
		v, ok := coerceFloat(a.Value)
		if !ok {
			return false
		}
		next = v
	case models.PropertyModifierAdd:
		v, ok := coerceFloat(a.Value)
		if !ok {
			return false
		}
		next = curF + v
	case models.PropertyModifierSubtract:
		v, ok := coerceFloat(a.Value)
		if !ok {
			return false
		}
		next = curF - v
	case models.PropertyModifierMultiply:
		v, ok := coerceFloat(a.Value)
		if !ok {
			return false
		}
		next = curF * v
	case models.PropertyModifierDivide:
		v, ok := coerceFloat(a.Value)
		if !ok || v == 0 {
			return false
		}
		next = curF / v
	default:
		return false
	}
	props[a.PropertyUID] = clampToBounds(next, desc)
	return true
}

// clampToBounds clamps f to a numeric descriptor's [Min,Max] when set.
func clampToBounds(f float64, desc *models.AgentProperty) float64 {
	if desc == nil {
		return f
	}
	if desc.MinValue != nil && f < *desc.MinValue {
		f = *desc.MinValue
	}
	if desc.MaxValue != nil && f > *desc.MaxValue {
		f = *desc.MaxValue
	}
	return f
}

func applySetProperty(rs *RuntimeState, ra *RuntimeAgent, a models.Action) []memEvent {
	if a.PropertyUID == "" || a.Value == nil {
		return nil
	}
	if a.PropertyUID == models.PseudoPropertyState {
		return nil // state changes go through ActionTypeTransition
	}
	if a.TargetUID == models.EnvironmentUID {
		return applyGlobalPropertyAction(rs, a)
	}
	if !writeProperty(ra, a.PropertyUID, a.Value) {
		return nil
	}
	return []memEvent{{Type: datasims.AgentMemoryProperty, Summary: "set " + a.PropertyUID}}
}

func applyModifyProperty(rs *RuntimeState, ra *RuntimeAgent, a models.Action) []memEvent {
	if a.PropertyUID == "" {
		return nil
	}
	if a.TargetUID == models.EnvironmentUID {
		return applyGlobalPropertyAction(rs, a)
	}
	if !modifyPropValue(ra.A.Properties, propByUID(ra, a.PropertyUID), a) {
		return nil
	}
	verb := "modify"
	if a.Modifier == models.PropertyModifierToggle {
		verb = "toggle"
	}
	return []memEvent{{Type: datasims.AgentMemoryProperty, Summary: verb + " " + a.PropertyUID}}
}

// applyCallLLM marks the agent for an on-demand LLM decision, consumed by the
// NEXT step's LLM phase (rules run after the LLM phase in the pipeline, so a
// same-step call would require a second mid-step LLM pass — breaking the batched
// parallel-gather and frozen-read invariants). The pending flag bypasses the
// DecisionFrequency gate for the decision purpose only; it is consume-once even
// when the resulting call fails (fail-soft — LastDecision replay / rules cover).
//
// Requires the agent's resolved strategy to be decision-capable (non-none with
// UseForDecisions); otherwise warns once and no-ops. A resolvable LLM target is
// NOT required here — a missing target degrades fail-soft in the LLM phase
// exactly like a scheduled call.
func applyCallLLM(rs *RuntimeState, ra *RuntimeAgent) []memEvent {
	strat := resolveArchetypeStrategy(rs.Model, ra.Archetype)
	if strat.Type == models.LLMStrategyNone || strat.Type == "" || !strat.UseForDecisions {
		warnOnce(rs, "call_llm ignored for archetype '"+ra.A.AgentUID+"': no decision-capable LLM strategy")
		return nil
	}
	ra.llmCallPending = true
	return []memEvent{{Type: datasims.AgentMemoryDecision, Summary: "call_llm requested"}}
}

// interactionByID returns the model interaction for the given model-local ID, or
// nil when unknown.
func interactionByID(rs *RuntimeState, id string) *models.Interaction {
	if rs.interactionsByID != nil {
		return rs.interactionsByID[id]
	}
	// Linear fallback for states built without NewRuntimeState (test literals).
	if rs.Model == nil {
		return nil
	}
	for i := range rs.Model.Interactions {
		if rs.Model.Interactions[i].ID == id {
			return &rs.Model.Interactions[i]
		}
	}
	return nil
}

// applyForcedInteract queues a force-fire request for a named interaction
// (Action.TargetUID carries the models.Interaction.ID). The request is drained
// by ProcessInteractions at the end of the interactions phase this tick: the
// interaction's Conditions and UseLLM mediation are skipped (the firing rule /
// LLM already made that decision), while geometric eligibility (radius / shape
// overlap / InitiatorUIDs membership) still applies at drain time. Invalid
// requests warn once and no-op. The interaction memory event is recorded on the
// actual fire by applyInteractionActions, so no memEvent is returned here.
func applyForcedInteract(rs *RuntimeState, ra *RuntimeAgent, a models.Action) []memEvent {
	if a.TargetUID == "" {
		warnOnce(rs, "interact action: missing target_uid (expected an interaction id)")
		return nil
	}
	in := interactionByID(rs, a.TargetUID)
	if in == nil {
		warnOnce(rs, "interact action: unknown interaction '"+a.TargetUID+"'")
		return nil
	}
	if !in.Enabled {
		warnOnce(rs, "interact action: interaction '"+a.TargetUID+"' is disabled")
		return nil
	}
	if !matchesUIDSet(ra.A.AgentUID, in.InitiatorUIDs) {
		warnOnce(rs, "interact action: agent archetype '"+ra.A.AgentUID+"' is not an initiator of interaction '"+a.TargetUID+"'")
		return nil
	}
	rs.forced = append(rs.forced, forcedInteraction{interactionID: a.TargetUID, initiatorID: ra.A.ID})
	return nil
}

// envPropDef returns the model descriptor for an environment global property, or
// nil when the property is not declared.
func envPropDef(rs *RuntimeState, uid string) *models.AgentProperty {
	if rs.envPropDefs != nil {
		return rs.envPropDefs[uid]
	}
	// Linear fallback for states built without NewRuntimeState (test literals).
	if rs.Model == nil || rs.Model.Environment == nil {
		return nil
	}
	for i := range rs.Model.Environment.GlobalProperties {
		if rs.Model.Environment.GlobalProperties[i].UID == uid {
			return &rs.Model.Environment.GlobalProperties[i]
		}
	}
	return nil
}

// applyGlobalPropertyAction routes a set/modify action whose TargetUID is the
// reserved EnvironmentUID onto the simulation's global properties. Only DECLARED
// globals are writable (the environment is schema'd: conditions, widgets, and the
// env step row all key off the declared descriptors) — an undeclared UID warns
// once and no-ops. Immutable descriptors no-op silently, mirroring agent
// properties.
//
// When rs.deferGlobalWrites is set (synchronous mutation phases), the write is
// queued on rs.globalWrites and applied after the commit phase, preserving the
// frozen-read invariant; otherwise it applies immediately (sequential mode and
// the serial env-rules phase). The returned memEvent lands on the ACTING agent's
// memory via the normal ApplyAction flow.
func applyGlobalPropertyAction(rs *RuntimeState, a models.Action) []memEvent {
	if rs == nil || rs.Env == nil {
		return nil
	}
	desc := envPropDef(rs, a.PropertyUID)
	if desc == nil {
		warnOnce(rs, "global property action: '"+a.PropertyUID+"' is not a declared environment global property")
		return nil
	}
	if desc.Immutable {
		return nil
	}
	if a.Type == models.ActionTypeSetProperty && a.Value == nil {
		return nil
	}
	if rs.deferGlobalWrites {
		rs.globalWrites = append(rs.globalWrites, a)
	} else if !applyGlobalWrite(rs, a, desc) {
		return nil
	}
	verb := "set"
	if a.Type == models.ActionTypeModifyProperty {
		verb = "modify"
		if a.Modifier == models.PropertyModifierToggle {
			verb = "toggle"
		}
	}
	return []memEvent{{Type: datasims.AgentMemoryProperty, Summary: verb + " env:" + a.PropertyUID}}
}

// applyGlobalWrite performs one set/modify write on Env.GlobalProperties.
func applyGlobalWrite(rs *RuntimeState, a models.Action, desc *models.AgentProperty) bool {
	if rs.Env.GlobalProperties == nil {
		rs.Env.GlobalProperties = make(map[string]any)
	}
	if a.Type == models.ActionTypeModifyProperty {
		return modifyPropValue(rs.Env.GlobalProperties, desc, a)
	}
	return setPropValue(rs.Env.GlobalProperties, desc, a.PropertyUID, a.Value)
}

// applyGlobalWrites drains the deferred global-property write queue in insertion
// order (set = last-write-wins, modify = accumulate over the evolving value —
// the same outcome sequential mode produces). Called after the synchronous
// commit phase with deferGlobalWrites already cleared.
func applyGlobalWrites(rs *RuntimeState) {
	for i := range rs.globalWrites {
		a := rs.globalWrites[i]
		desc := envPropDef(rs, a.PropertyUID)
		if desc == nil {
			continue // validated at queue time; defensive
		}
		applyGlobalWrite(rs, a, desc)
	}
	rs.globalWrites = rs.globalWrites[:0]
}

// movable reports whether the agent's archetype allows movement.
func movable(ra *RuntimeAgent) bool {
	return ra.Archetype != nil && ra.Archetype.MoveType == models.AgentMoveTypeMovable
}

func applyMove(rs *RuntimeState, ra *RuntimeAgent, a models.Action) []memEvent {
	if ra.A.Position == nil || !movable(ra) {
		return nil
	}
	speed := 1.0
	if a.Speed != nil {
		speed = *a.Speed
	}
	dir := 0.0
	if a.Direction != nil {
		dir = *a.Direction
	} else if rs.RNG != nil {
		dir = rs.RNG.Float64() * 2 * math.Pi
	}
	ra.Velocity = Vector{DX: math.Cos(dir) * speed, DY: math.Sin(dir) * speed}
	return []memEvent{{Type: datasims.AgentMemoryMovement, Summary: "move"}}
}

func applyMoveToward(rs *RuntimeState, ra *RuntimeAgent, a models.Action, away bool) []memEvent {
	if ra.A.Position == nil || !movable(ra) {
		return nil
	}
	uids := []string{}
	if len(a.TargetUIDs) > 0 {
		uids = a.TargetUIDs
	} else if a.TargetUID != "" {
		uids = []string{a.TargetUID}
	}
	target := nearestTargetPos(rs, ra, uids)
	if target == nil {
		return nil
	}
	speed := 1.0
	if a.Speed != nil {
		speed = *a.Speed
	}
	w, h, torus := envDims(rs)
	v := VectorToward(*ra.A.Position, *target, w, h, torus, speed)
	if away {
		v = Vector{DX: -v.DX, DY: -v.DY}
	}
	ra.Velocity = v
	summary := "move_toward"
	if away {
		summary = "move_away"
	}
	return []memEvent{{Type: datasims.AgentMemoryMovement, Summary: summary}}
}

func applyTransition(ra *RuntimeAgent, a models.Action) []memEvent {
	if a.NewState == "" {
		return nil
	}
	if !validState(ra, a.NewState) {
		return nil
	}
	ra.A.State = a.NewState
	return []memEvent{{Type: datasims.AgentMemoryDecision, Summary: "transition -> " + a.NewState}}
}

// validState reports whether s is one of the archetype's declared states. When
// the archetype declares no states, any state is accepted.
func validState(ra *RuntimeAgent, s string) bool {
	if ra.Archetype == nil || len(ra.Archetype.States) == 0 {
		return true
	}
	for _, st := range ra.Archetype.States {
		if st == s {
			return true
		}
	}
	return false
}

// authorizeEmit reports whether kind (and optionally archetypeUID) is allowed to
// emit the event described by def.
//
// The logic follows the DM-M10/M11 model constraints:
//   - def == nil OR empty def.Producers ⇒ permissive (no restriction declared).
//   - else: kind must be in def.Producers, AND (ProducerUIDs is empty OR
//     archetypeUID is in ProducerUIDs).
//
// This is a standalone helper so Tasks 17 (LLM events) and 18 (environment events)
// can call it with EventProducerLLM / EventProducerEnvironment respectively.
func authorizeEmit(def *models.EventDefinition, kind models.EventProducer, archetypeUID string) bool {
	if def == nil || len(def.Producers) == 0 {
		return true
	}
	// Check kind membership.
	kindOK := false
	for _, p := range def.Producers {
		if p == kind {
			kindOK = true
			break
		}
	}
	if !kindOK {
		return false
	}
	// Check archetype UID membership (empty ProducerUIDs ⇒ any archetype allowed).
	if len(def.ProducerUIDs) == 0 {
		return true
	}
	for _, u := range def.ProducerUIDs {
		if u == archetypeUID {
			return true
		}
	}
	return false
}

// validateEventPayload performs advisory validation of event payload data against
// the declared payload schema (def.Payload). Warnings are appended to rs.warnings;
// the event is NEVER rejected — this is purely informational.
//
// Rules:
//   - def is nil or def.Payload is empty ⇒ no-op.
//   - data is map[string]interface{}: for each declared AgentProperty field, warn
//     on missing key, type mismatch (coerceFloat for number types), or AllowedValues
//     violation.
//   - data is non-map with a multi-field payload (len > 1): append ONE
//     "payload shape mismatch" warning.
func validateEventPayload(rs *RuntimeState, def *models.EventDefinition, data any) {
	if def == nil || len(def.Payload) == 0 {
		return
	}
	m, isMap := data.(map[string]interface{})
	if !isMap {
		if len(def.Payload) > 1 {
			rs.warnings = append(rs.warnings, "emit "+def.UID+": payload shape mismatch (expected map with fields: "+payloadFieldList(def.Payload)+")")
		}
		return
	}
	for _, field := range def.Payload {
		val, ok := m[field.UID]
		if !ok {
			rs.warnings = append(rs.warnings, "emit "+def.UID+": payload missing field '"+field.UID+"'")
			continue
		}
		// Type check.
		switch field.Type {
		case models.PropertyTypeNumber:
			if _, fok := coerceFloat(val); !fok {
				rs.warnings = append(rs.warnings, "emit "+def.UID+": payload field '"+field.UID+"' expected number, got "+toString(val))
				continue
			}
		case models.PropertyTypeBoolean:
			if _, bok := val.(bool); !bok {
				rs.warnings = append(rs.warnings, "emit "+def.UID+": payload field '"+field.UID+"' expected boolean, got "+toString(val))
				continue
			}
		}
		// AllowedValues check (applies to any typed field with an AllowedValues set).
		if len(field.AllowedValues) > 0 {
			found := false
			want := toString(val)
			for _, av := range field.AllowedValues {
				if toString(av) == want {
					found = true
					break
				}
			}
			if !found {
				rs.warnings = append(rs.warnings, "emit "+def.UID+": payload field '"+field.UID+"' value '"+want+"' not in AllowedValues")
			}
		}
	}
}

// payloadFieldList returns a comma-separated list of field UIDs for a warning message.
func payloadFieldList(props []models.AgentProperty) string {
	var b strings.Builder
	for i, p := range props {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.UID)
	}
	return b.String()
}

func applyEmitEvent(rs *RuntimeState, ra *RuntimeAgent, a models.Action, kind models.EventProducer) []memEvent {
	if a.EventUID == "" {
		return nil
	}
	// Resolve the event definition for authorization + payload validation.
	def := eventDefByUID(rs, a.EventUID)
	// Authorization gate: denied emits produce no event, no memory entry, no stat.
	if !authorizeEmit(def, kind, ra.A.AgentUID) {
		return nil
	}
	// Advisory payload validation: appends warnings but never rejects.
	validateEventPayload(rs, def, a.EventData)
	var pos *common.Point
	if ra.A.Position != nil {
		p := *ra.A.Position
		pos = &p
	}
	// Stats increment happens AFTER the authorization gate passes.
	if rs.stats != nil {
		rs.stats.Events++
		rs.stats.ByEvent[a.EventUID]++
	}
	rs.events = append(rs.events, runtimeEvent{
		EventUID: a.EventUID,
		Source:   ra.A.ID,
		Position: pos,
		Data:     a.EventData,
		EmitStep: rs.Step,
	})
	return []memEvent{{Type: datasims.AgentMemoryDecision, Summary: "emit " + a.EventUID}}
}

func applyTeleport(rs *RuntimeState, ra *RuntimeAgent, a models.Action) []memEvent {
	if !movable(ra) {
		return nil
	}
	// Explicit coordinates via Parameters.
	if a.Parameters != nil {
		xf, xok := coerceFloat(a.Parameters["x"])
		yf, yok := coerceFloat(a.Parameters["y"])
		if xok && yok {
			ra.A.Position = clampedPoint(rs, xf, yf)
			ra.rem = Vec2{} // U4: a teleport lands exactly on its cell — discard sub-cell remainder
			return []memEvent{{Type: datasims.AgentMemoryMovement, Summary: "teleport"}}
		}
	}
	// Teleport to nearest matching target.
	uids := []string{}
	if len(a.TargetUIDs) > 0 {
		uids = a.TargetUIDs
	} else if a.TargetUID != "" {
		uids = []string{a.TargetUID}
	}
	if target := nearestTargetPos(rs, ra, uids); target != nil {
		ra.A.Position = clampedPoint(rs, float64(target.X), float64(target.Y))
		ra.rem = Vec2{} // U4: a teleport lands exactly on its cell — discard sub-cell remainder
		return []memEvent{{Type: datasims.AgentMemoryMovement, Summary: "teleport"}}
	}
	return nil
}

func applySpawn(rs *RuntimeState, ra *RuntimeAgent, a models.Action, transform bool) []memEvent {
	uid := a.SpawnAgentUID
	if transform && uid == "" {
		uid = a.TargetUID
	}
	if uid == "" {
		return nil
	}
	count := 1
	if a.SpawnCount > 0 {
		count = a.SpawnCount
	}
	if transform {
		count = 1
	}
	rs.spawns = append(rs.spawns, spawnRequest{
		archetypeUID: uid,
		count:        count,
		properties:   a.SpawnProperties,
		inheritFrom:  a.SpawnInheritFrom,
		transform:    transform,
		position:     a.SpawnPosition,
		parentID:     ra.A.ID,
	})
	summary := "spawn " + uid
	if transform {
		summary = "transform -> " + uid
	}
	return []memEvent{{Type: datasims.AgentMemoryDecision, Summary: summary}}
}

func applyRemove(rs *RuntimeState, ra *RuntimeAgent) []memEvent {
	if ra.removed {
		return nil
	}
	ra.removed = true
	rs.removals = append(rs.removals, ra.A.ID)
	return []memEvent{{Type: datasims.AgentMemoryDecision, Summary: "remove"}}
}

func applyCustom(ra *RuntimeAgent, a models.Action) []memEvent {
	if a.Parameters == nil {
		return nil
	}
	props, ok := a.Parameters["set_properties"].(map[string]interface{})
	if !ok {
		return nil
	}
	changed := false
	for k, v := range props {
		if writeProperty(ra, k, v) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return []memEvent{{Type: datasims.AgentMemoryProperty, Summary: "custom set"}}
}

// ---- environment event emission -----------------------------------------------

// evalEnvCondition evaluates one condition on behalf of the environment (a singleton
// with no position, no agent-level properties, and no state). It handles ONLY the
// condition types that are meaningful for a positionless entity:
//
//   - global_property, step, random: delegated to the same helpers used by EvalCondition.
//   - composite (AND/OR/NOT): recursively evaluated over the same environment context.
//   - event: delegated to evalEnvEvent, which matches TTL-liveness + optional payload
//     compare via the shared matchEvent helper but deliberately BYPASSES eventReaches
//     (delivery-scope gating) — the environment is a global observer with no position,
//     so a local/targeted-scope event would otherwise always be (wrongly) dropped.
//   - all other types (property, position, proximity, state, distance, count, barrier,
//     decision, memory, custom): return false — the environment has no position or
//     per-agent state to evaluate against.
//
// Pipeline note: runEnvironmentRules runs at phase 5b, AFTER the agent mutation
// phases have committed (step.go) — the sequential path queues events onto rs
// directly; the synchronous path folds workView.events into rs.events at commit
// (step.go ~:407) before this env pass. So an event an agent emits THIS tick IS
// visible to env rules the SAME tick. The reverse is the delayed direction:
// events an env rule emits here land in rs.events for the NEXT tick's
// prune/perceive cycle, so agents only observe env-emitted events on the next
// tick (see step.go ~:108-115).
func evalEnvCondition(rs *RuntimeState, c models.Condition) bool {
	switch c.Type {
	case models.ConditionTypeGlobalProperty:
		return evalGlobalProperty(rs, c)
	case models.ConditionTypeStep:
		return compareValues(float64(rs.Step), c.Operator, c.Value, c.MinValue, c.MaxValue)
	case models.ConditionTypeRandom:
		return evalRandom(rs, c)
	case models.ConditionTypeComposite:
		return evalEnvComposite(rs, c)
	case models.ConditionTypeEvent:
		return evalEnvEvent(rs, c)
	default:
		// agent-positional and agent-state conditions are meaningless for the environment.
		return false
	}
}

// evalEnvEvent evaluates an event condition on behalf of the environment. The
// environment is a positionless global observer, so — unlike evalEvent — it
// deliberately skips eventReaches: TTL-liveness and the optional payload
// compare (matchEvent) are the only gates, meaning even local/targeted-scope
// events are visible to an environment-scoped rule.
func evalEnvEvent(rs *RuntimeState, c models.Condition) bool {
	uid := eventConditionUID(c)
	if uid == "" {
		return false
	}
	def := eventDefByUID(rs, uid)
	for i := range rs.events {
		ev := &rs.events[i]
		if ev.EventUID != uid {
			continue
		}
		if matchEvent(rs, c, ev, def) {
			return true
		}
	}
	return false
}

// evalEnvComposite evaluates a composite condition (AND/OR/NOT) in the environment
// context, delegating each sub-condition to evalEnvCondition.
func evalEnvComposite(rs *RuntimeState, c models.Condition) bool {
	if len(c.SubConditions) == 0 {
		return false
	}
	if c.LogicOperator == models.LogicOpNOT {
		return !evalEnvCondition(rs, c.SubConditions[0])
	}
	isAnd := c.LogicOperator == models.LogicOpAND || c.LogicOperator == ""
	for _, sub := range c.SubConditions {
		res := evalEnvCondition(rs, sub)
		if isAnd && !res {
			return false
		}
		if !isAnd && res {
			return true
		}
	}
	return isAnd
}

// runEnvironmentRules performs the once-per-step environment evaluation pass.
// It iterates the run's sorted rules (rs.rules, falling back to rs.Model.Rules),
// selects every enabled rule whose AppliesTo slice contains models.EnvironmentUID,
// evaluates all its conditions via evalEnvCondition, and — when ALL conditions
// pass — executes the supported actions below.
//
// A synthetic RuntimeAgent with A.ID=0, A.AgentUID=EnvironmentUID, and nil
// Position is used as the emitter so that applyEmitEvent records Source=0 and
// Position=nil on every emitted event.
//
// Supported actions:
//   - emit_event      → applyEmitEvent with kind EventProducerEnvironment.
//   - set_property /
//     modify_property → written to Env.GlobalProperties via
//     applyGlobalPropertyAction (declared descriptors only; immutable + clamping
//     honored; TargetUID is ignored — the environment writes only itself). The
//     phase is serial on the real rs, so writes apply immediately and land in
//     the SAME tick's env step row; agents evaluate them next tick.
//
// All other action types warn once and are skipped (previously a silent no-op).
// CooldownSteps and ExecuteOnce are honored via rs.envCooldowns/envOnceFired
// (the synthetic agent is rebuilt every step, so this state lives on the run).
// A rule that executes at least one action counts toward RuleExecutions/ByRule,
// matching agent rules.
//
// Determinism: rules are iterated in the fixed priority order inherited from
// rs.rules; the environment is a singleton evaluated once per step.
func runEnvironmentRules(rs *RuntimeState) {
	if rs == nil || rs.Model == nil {
		return
	}
	src := rs.rules
	if len(src) == 0 {
		src = rs.Model.Rules
	}

	// Synthetic environment agent: no position, ID=0, AgentUID=EnvironmentUID.
	envA := &datasims.SimulationAgent{
		ID:       0,
		AgentUID: models.EnvironmentUID,
		Position: nil,
	}
	envAgent := &RuntimeAgent{
		A:         envA,
		Archetype: nil,
	}

	for _, r := range src {
		if !r.Enabled {
			continue
		}
		if !ruleAppliesToEnvironment(r) {
			continue
		}
		if r.ExecuteOnce && rs.envOnceFired[r.ID] {
			continue
		}
		if r.CooldownSteps > 0 {
			if last, ok := rs.envCooldowns[r.ID]; ok && rs.Step < last+uint64(r.CooldownSteps) {
				continue
			}
		}
		// Evaluate all conditions; all must pass (AND semantics).
		allPass := true
		for _, cond := range r.Conditions {
			if !evalEnvCondition(rs, cond) {
				allPass = false
				break
			}
		}
		if !allPass {
			continue
		}
		fired := false
		for _, action := range r.Actions {
			switch action.Type {
			case models.ActionTypeEmitEvent:
				if evts := applyEmitEvent(rs, envAgent, action, models.EventProducerEnvironment); len(evts) > 0 {
					fired = true
				}
			case models.ActionTypeSetProperty, models.ActionTypeModifyProperty:
				// The env-rules phase runs serially on the real rs with
				// deferGlobalWrites off, so the write applies immediately. The
				// memEvents are dropped: the synthetic env agent keeps no memory.
				if evts := applyGlobalPropertyAction(rs, action); len(evts) > 0 {
					fired = true
				}
			default:
				warnOnce(rs, "environment rule '"+r.ID+"': action '"+string(action.Type)+"' is not supported for environment-scoped rules")
			}
		}
		if fired {
			if rs.envCooldowns == nil {
				rs.envCooldowns = make(map[string]uint64)
			}
			rs.envCooldowns[r.ID] = rs.Step
			if r.ExecuteOnce {
				if rs.envOnceFired == nil {
					rs.envOnceFired = make(map[string]bool)
				}
				rs.envOnceFired[r.ID] = true
			}
			if rs.stats != nil {
				rs.stats.RuleExecutions++
				rs.stats.ByRule[r.ID]++
			}
		}
	}
}

// ruleAppliesToEnvironment reports whether the rule's AppliesTo slice contains
// the reserved EnvironmentUID. An empty AppliesTo slice does NOT match the
// environment (contrast with ruleApplies for agents where empty = all).
func ruleAppliesToEnvironment(r models.Rule) bool {
	for _, uid := range r.AppliesTo {
		if uid == models.EnvironmentUID {
			return true
		}
	}
	return false
}

// ---- rule orchestration -------------------------------------------------------

// ApplyRules runs every enabled rule whose AppliesTo includes the agent's
// archetype UID, sorted by Priority desc (stable for equal priority by slice
// order), AND-combining each rule's conditions. It honors CooldownSteps and
// ExecuteOnce, and SKIPS individual actions whose purpose the agent's LLM
// strategy owns (ownedPurposes). Memory events produced by actions are appended.
func ApplyRules(rs *RuntimeState, ra *RuntimeAgent, intent *AgentIntent, ownedPurposes purposeSet) {
	if ra == nil || ra.A == nil || ra.removed {
		return
	}
	rules := applicableRules(rs, ra)
	for i := range rules {
		r := &rules[i]
		if !r.Enabled {
			continue
		}
		if ra.OnceFired != nil && ra.OnceFired[r.ID] {
			continue
		}
		if onCooldown(ra, r, rs.Step) {
			continue
		}
		if !allConditionsTrue(rs, ra, intent, r.Conditions) {
			continue
		}
		fired := false
		for _, action := range r.Actions {
			if p, owned := actionPurpose(action.Type); owned && ownedPurposes.has(p) {
				continue // LLM owns this purpose; rule action suppressed
			}
			evts := ApplyAction(rs, ra, action)
			fired = true
			for _, e := range evts {
				AppendMemory(ra, rs.Step, e.Type, e.Summary, e.Details, ra.memoryCap(), nowFn())
			}
			if ra.removed {
				break
			}
		}
		if fired {
			recordFire(ra, r, rs.Step)
			AppendMemory(ra, rs.Step, datasims.AgentMemoryRule,
				"rule fired: "+ruleLabel(r),
				map[string]any{"rule_id": r.ID, "conditions": len(r.Conditions)},
				ra.memoryCap(), nowFn())
			if rs.stats != nil {
				rs.stats.RuleExecutions++
				rs.stats.ByRule[r.ID]++
			}
		}
		if ra.removed {
			return
		}
	}
}

// applicableRules returns the rules targeting this agent's archetype, sorted by
// descending priority. The runtime's pre-sorted rs.rules is used when present;
// otherwise the model's rules are filtered and sorted here.
func applicableRules(rs *RuntimeState, ra *RuntimeAgent) []models.Rule {
	src := rs.rules
	if len(src) == 0 && rs.Model != nil {
		src = rs.Model.Rules
	}
	var out []models.Rule
	for _, r := range src {
		if ruleApplies(r, ra) {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	return out
}

func ruleApplies(r models.Rule, ra *RuntimeAgent) bool {
	if len(r.AppliesTo) == 0 {
		return true
	}
	for _, uid := range r.AppliesTo {
		if uid == ra.A.AgentUID {
			return true
		}
	}
	return false
}

func allConditionsTrue(rs *RuntimeState, ra *RuntimeAgent, intent *AgentIntent, conds []models.Condition) bool {
	for _, c := range conds {
		if !EvalCondition(rs, ra, intent, c) {
			return false
		}
	}
	return true
}

func onCooldown(ra *RuntimeAgent, r *models.Rule, step uint64) bool {
	if r.CooldownSteps <= 0 || ra.Cooldowns == nil {
		return false
	}
	last, ok := ra.Cooldowns[r.ID]
	if !ok {
		return false
	}
	return step < last+uint64(r.CooldownSteps)
}

// ruleLabel returns a human-readable label for a rule: its Name when set, else
// its ID. Used to produce compact memory summaries after rule fires.
func ruleLabel(r *models.Rule) string {
	if r.Name != "" {
		return r.Name
	}
	return r.ID
}

func recordFire(ra *RuntimeAgent, r *models.Rule, step uint64) {
	if r.CooldownSteps > 0 {
		if ra.Cooldowns == nil {
			ra.Cooldowns = make(map[string]uint64)
		}
		ra.Cooldowns[r.ID] = step
	}
	if r.ExecuteOnce {
		if ra.OnceFired == nil {
			ra.OnceFired = make(map[string]bool)
		}
		ra.OnceFired[r.ID] = true
	}
}

// memoryCap returns the effective memory capacity for this agent. 0 means
// "use the package defaultMemoryCap (20)" — AppendMemory applies that floor.
// The value is resolved once at agent creation via resolveMemoryCap and stored
// in ra.memCap; this method is the single read-point for the rules/spatial phases.
func (ra *RuntimeAgent) memoryCap() int {
	return ra.memCap
}

// ---- shared helpers -----------------------------------------------------------

// envDims returns the world dimensions + torus flag, defaulting to zero/false
// when no environment is present.
func envDims(rs *RuntimeState) (uint64, uint64, bool) {
	if rs.Env == nil {
		return 0, 0, false
	}
	return rs.Env.Width, rs.Env.Height, rs.Env.Torus
}

// clampedPoint builds a position from float coordinates, clamped/wrapped to the
// world bounds.
func clampedPoint(rs *RuntimeState, x, y float64) *common.Point {
	w, h, torus := envDims(rs)
	var px, py float64
	if torus {
		px, py = wrapAxis(x, float64(w)), wrapAxis(y, float64(h))
	} else {
		px, py = clampAxis(x, float64(w)), clampAxis(y, float64(h))
	}
	p := common.Point{X: uint64(math.Round(px)), Y: uint64(math.Round(py))}
	return &p
}

// nearestTargetPos returns the position of the nearest agent or named shape
// matching the uid set (empty => any agent). Returns nil when nothing matches.
func nearestTargetPos(rs *RuntimeState, ra *RuntimeAgent, uids []string) *common.Point {
	if ra.A.Position == nil {
		return nil
	}
	w, h, torus := envDims(rs)
	var best *common.Point
	bestDist := math.MaxFloat64
	for _, other := range rs.Agents {
		if other == nil || other.removed || other.A.ID == ra.A.ID || other.A.Position == nil {
			continue
		}
		if !matchesTarget(other, uids) {
			continue
		}
		d := Distance(*ra.A.Position, *other.A.Position, w, h, torus)
		if d < bestDist {
			bestDist = d
			p := *other.A.Position
			best = &p
		}
	}
	if rs.Env != nil {
		for i := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[i]
			if !shapeMatchesTarget(sh, uids) {
				continue
			}
			p := shapeAimPoint(sh)
			d := Distance(*ra.A.Position, p, w, h, torus)
			if d < bestDist {
				bestDist = d
				best = &p
			}
		}
	}
	return best
}

// shapeAimPoint is the cell movement aims at when targeting a shape: the
// shape's geometric MIDDLE cell (floor of the collision-body center), not its
// top-left anchor — "move toward the door" must head for the middle of the
// door gap, not its corner.
func shapeAimPoint(sh *datasims.SimulationShape) common.Point {
	b := bodyForShape(sh)
	x, y := math.Floor(b.Center.X), math.Floor(b.Center.Y)
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return common.Point{X: uint64(x), Y: uint64(y)}
}

// nearestTargetBody returns the collision body of the nearest matching target (agent or
// shape) by edge-to-edge distance, used by the distance condition. Center-based movement
// targeting still uses nearestTargetPos.
func nearestTargetBody(rs *RuntimeState, ra *RuntimeAgent, uids []string) (CollisionBody, bool) {
	if ra.A.Position == nil {
		return CollisionBody{}, false
	}
	q := bodyForAgent(ra)
	w, h, torus := envDims(rs)
	var best CollisionBody
	found := false
	bestDist := math.MaxFloat64
	for _, other := range rs.Agents {
		if other == nil || other.removed || other.A.ID == ra.A.ID || other.A.Position == nil {
			continue
		}
		if !matchesTarget(other, uids) {
			continue
		}
		ob := bodyForAgent(other)
		if d := BodyDistance(q, ob, w, h, torus); d < bestDist {
			bestDist, best, found = d, ob, true
		}
	}
	if rs.Env != nil {
		for i := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[i]
			if !shapeMatchesTarget(sh, uids) {
				continue
			}
			sb := bodyForShape(sh)
			if d := BodyDistance(q, sb, w, h, torus); d < bestDist {
				bestDist, best, found = d, sb, true
			}
		}
	}
	return best, found
}

// eventDefByUID returns the model event definition for uid, or nil.
func eventDefByUID(rs *RuntimeState, uid string) *models.EventDefinition {
	if rs.eventDefs != nil {
		return rs.eventDefs[uid]
	}
	// Linear fallback for states built without NewRuntimeState (test literals).
	if rs.Model == nil {
		return nil
	}
	for i := range rs.Model.Events {
		if rs.Model.Events[i].UID == uid {
			return &rs.Model.Events[i]
		}
	}
	return nil
}

// eventActive reports whether an emitted event is still live at the given step,
// per its TTL (def may be nil => active only on the emit step).
func eventActive(def *models.EventDefinition, ev *runtimeEvent, step uint64) bool {
	ttl := uint64(0)
	if def != nil {
		ttl = uint64(def.TTLSteps)
	}
	return step <= ev.EmitStep+ttl
}

// eventReaches reports whether the event is perceivable by ra, honoring scope as
// the authoritative gate:
//   - targeted: reaches iff ra.A.AgentUID ∈ ConsumerUIDs (empty ConsumerUIDs ⇒ no one).
//   - local: within Radius of the emit position AND (ConsumerUIDs empty ⇒ any
//     in-radius agent; else ra.A.AgentUID ∈ ConsumerUIDs).
//   - global (and default): ConsumerUIDs empty ⇒ all; else ra.A.AgentUID ∈ set.
//
// A nil def defaults to global with empty ConsumerUIDs (reaches all).
func eventReaches(rs *RuntimeState, def *models.EventDefinition, ev *runtimeEvent, ra *RuntimeAgent) bool {
	if def == nil {
		return true
	}
	inConsumers := func() bool {
		for _, u := range def.ConsumerUIDs {
			if u == ra.A.AgentUID {
				return true
			}
		}
		return false
	}
	switch def.Scope {
	case models.EventScopeTargeted:
		// empty ConsumerUIDs means no one is targeted.
		return len(def.ConsumerUIDs) > 0 && inConsumers()
	case models.EventScopeLocal:
		if ev.Position == nil || ra.A.Position == nil || def.Radius == nil {
			return false
		}
		w, h, torus := envDims(rs)
		if Distance(*ev.Position, *ra.A.Position, w, h, torus) > *def.Radius {
			return false
		}
		return len(def.ConsumerUIDs) == 0 || inConsumers()
	default: // EventScopeGlobal and any unrecognised scope
		return len(def.ConsumerUIDs) == 0 || inConsumers()
	}
}

// compareValues compares actual against expected using op, with optional
// numeric range bounds for in_range. String/bool equality is exact; numeric
// comparisons coerce both sides to float64; contains/in/not_in use string
// semantics. Unknown operators return false.
func compareValues(actual any, op models.ConditionOperator, expected any, minV, maxV *float64) bool {
	switch op {
	case models.ConditionOpEquals, "":
		return valuesEqual(actual, expected)
	case models.ConditionOpNotEquals:
		return !valuesEqual(actual, expected)
	case models.ConditionOpGreaterThan, models.ConditionOpLessThan,
		models.ConditionOpGreaterOrEq, models.ConditionOpLessOrEq:
		af, aok := coerceFloat(actual)
		ef, eok := coerceFloat(expected)
		if !aok || !eok {
			return false
		}
		switch op {
		case models.ConditionOpGreaterThan:
			return af > ef
		case models.ConditionOpLessThan:
			return af < ef
		case models.ConditionOpGreaterOrEq:
			return af >= ef
		case models.ConditionOpLessOrEq:
			return af <= ef
		}
	case models.ConditionOpInRange:
		af, aok := coerceFloat(actual)
		if !aok {
			return false
		}
		if minV != nil && maxV != nil {
			return af >= *minV && af <= *maxV
		}
		return false
	case models.ConditionOpContains:
		return strings.Contains(toString(actual), toString(expected))
	case models.ConditionOpIn:
		return inSet(actual, expected)
	case models.ConditionOpNotIn:
		return !inSet(actual, expected)
	}
	return false
}

// valuesEqual compares two dynamic values: numerics by float, otherwise by
// string form (covers string/bool/enum/color).
func valuesEqual(a, b any) bool {
	if af, aok := coerceFloat(a); aok {
		if bf, bok := coerceFloat(b); bok {
			return af == bf
		}
		return false
	}
	return toString(a) == toString(b)
}

// inSet reports whether actual is a member of expected when expected is a slice
// (compared by string form); a scalar expected is treated as a single-element set.
func inSet(actual, expected any) bool {
	want := toString(actual)
	switch set := expected.(type) {
	case []any:
		for _, v := range set {
			if toString(v) == want {
				return true
			}
		}
		return false
	case []string:
		for _, v := range set {
			if v == want {
				return true
			}
		}
		return false
	default:
		return toString(expected) == want
	}
}
