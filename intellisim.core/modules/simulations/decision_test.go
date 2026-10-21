package simulations

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// containsStr (validation.go) and fptr (rules_test.go) are reused here.

// decisionRuleModel builds a model with ConditionTypeDecision rules whose Value
// strings form the decision menu for the "wolf" archetype.
func decisionRuleModel() *models.Model {
	return &models.Model{
		Agents: []models.Agent{
			{UID: "wolf", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable},
			{UID: "sheep", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable},
		},
		Rules: []models.Rule{
			{
				ID:        "r1",
				Enabled:   true,
				AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{
					{Type: models.ConditionTypeDecision, Value: "search_food"},
				},
			},
			{
				ID:        "r2",
				Enabled:   true,
				AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{
					{Type: models.ConditionTypeProperty, PropertyUID: "energy"},
					{
						Type:          models.ConditionTypeComposite,
						LogicOperator: models.LogicOpOR,
						SubConditions: []models.Condition{
							{Type: models.ConditionTypeDecision, Value: "flee"},
							{Type: models.ConditionTypeDecision, Value: "rest"},
						},
					},
				},
			},
			{
				// A decision condition that targets ONLY sheep — must not leak into
				// the wolf menu.
				ID:        "r3",
				Enabled:   true,
				AppliesTo: []string{"sheep"},
				Conditions: []models.Condition{
					{Type: models.ConditionTypeDecision, Value: "graze"},
				},
			},
		},
	}
}

// twoWolfState builds a runtime state with two movable wolves at different
// positions plus a single sheep target, sharing one archetype. It is the fixture
// for the variation tests.
func twoWolfState() (*RuntimeState, *AgentGroup) {
	model := decisionRuleModel()
	wolfArch := &model.Agents[0]

	w1 := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 1, AgentUID: "wolf",
			Position:   &common.Point{X: 10, Y: 10},
			Properties: map[string]any{"energy": 50.0},
			State:      "idle",
		},
		Archetype: wolfArch,
	}
	w2 := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 2, AgentUID: "wolf",
			Position:   &common.Point{X: 80, Y: 60},
			Properties: map[string]any{"energy": 50.0},
			State:      "idle",
		},
		Archetype: wolfArch,
	}
	sheep := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 3, AgentUID: "sheep",
			Position: &common.Point{X: 45, Y: 35},
		},
		Archetype: &model.Agents[1],
	}

	env := &datasims.SimulationEnvironment{Width: 100, Height: 100}
	idx := NewSpatialIndex(env.Width, env.Height, 10, env.Torus)

	rs := &RuntimeState{
		Sim:    &datasims.Simulation{Seed: 7},
		Model:  model,
		Env:    env,
		Agents: []*RuntimeAgent{w1, w2, sheep},
		ByID: map[uint64]*RuntimeAgent{
			1: w1, 2: w2, 3: sheep,
		},
		Index: idx,
		RNG:   rand.New(rand.NewSource(7)),
		Step:  5,
	}
	rs.Index.Rebuild(rs.Agents)

	group := &AgentGroup{
		ArchetypeUID: "wolf",
		Archetype:    wolfArch,
		Members:      []*RuntimeAgent{w1, w2},
	}
	return rs, group
}

// manyWolves builds a runtime state with n movable wolves sharing one archetype, all
// at the SAME position (so any per-agent divergence comes purely from jitter/sampling,
// NOT from geometry), plus a stationary "sheep" target so "toward" resolves. The
// group's Strategy carries the given archetype mode + variation strength. Seed/Step
// are fixed for determinism.
func manyWolves(n int, mode models.ArchetypeMode, strength float64) (*RuntimeState, *AgentGroup) {
	model := decisionRuleModel()
	wolfArch := &model.Agents[0]
	wolfArch.Properties = []models.AgentProperty{
		{UID: "energy", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
	}
	wolfArch.States = []string{"idle", "hunting"}

	env := &datasims.SimulationEnvironment{Width: 500, Height: 500}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{Seed: 7},
		Model: model,
		Env:   env,
		ByID:  map[uint64]*RuntimeAgent{},
		RNG:   rand.New(rand.NewSource(7)),
		Step:  5,
	}
	idx := NewSpatialIndex(env.Width, env.Height, 10, env.Torus)

	members := make([]*RuntimeAgent, 0, n)
	for i := 0; i < n; i++ {
		id := uint64(i + 1)
		ra := &RuntimeAgent{
			A: &datasims.SimulationAgent{
				ID: id, AgentUID: "wolf", State: "idle",
				Position:   &common.Point{X: 50, Y: 50},
				Properties: map[string]any{"energy": 50.0},
			},
			Archetype: wolfArch,
		}
		members = append(members, ra)
		rs.Agents = append(rs.Agents, ra)
		rs.ByID[id] = ra
	}
	preyID := uint64(n + 1)
	prey := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: preyID, AgentUID: "sheep", Position: &common.Point{X: 200, Y: 200}},
		Archetype: &model.Agents[1],
	}
	rs.Agents = append(rs.Agents, prey)
	rs.ByID[preyID] = prey

	rs.Index = idx
	idx.Rebuild(rs.Agents)

	group := &AgentGroup{
		ArchetypeUID: "wolf",
		Archetype:    wolfArch,
		Members:      members,
		Strategy: models.LLMStrategy{
			Type: models.LLMStrategyDecision, UseForDecisions: true,
			ArchetypeMode: mode, VariationStrength: strength,
		},
	}
	return rs, group
}

func TestApplyDirectiveWithVariation_DecisionDistribution_RespectsWeights(t *testing.T) {
	rs, group := manyWolves(200, models.ArchetypeModeDistribution, 0)
	dir := &AgentIntent{DecisionDist: []WeightedDecision{
		{Decision: DecisionIntent{Label: "flee"}, Weight: 9},
		{Decision: DecisionIntent{Label: "hold"}, Weight: 1},
	}}

	intents := ApplyDirectiveWithVariation(rs, group, dir)

	counts := map[string]int{}
	for _, ra := range group.Members {
		in := intents[ra.A.ID]
		if in == nil || len(in.Decisions) != 1 {
			t.Fatalf("agent %d must carry exactly one sampled decision: %+v", ra.A.ID, in)
		}
		counts[in.Decisions[0].Label]++
	}
	if counts["flee"] == 0 || counts["hold"] == 0 {
		t.Fatalf("distribution must produce BOTH labels across members: %v", counts)
	}
	frac := float64(counts["flee"]) / 200
	if frac < 0.8 || frac > 0.98 {
		t.Fatalf("flee fraction %v not ~0.9 (weights 9:1)", frac)
	}
}

func TestApplyDirectiveWithVariation_DecisionDistribution_Deterministic(t *testing.T) {
	run := func() []string {
		rs, group := manyWolves(20, models.ArchetypeModeDistribution, 0)
		dir := &AgentIntent{DecisionDist: []WeightedDecision{
			{Decision: DecisionIntent{Label: "flee"}, Weight: 1},
			{Decision: DecisionIntent{Label: "hold"}, Weight: 1},
		}}
		intents := ApplyDirectiveWithVariation(rs, group, dir)
		out := make([]string, len(group.Members))
		for i, ra := range group.Members {
			out[i] = intents[ra.A.ID].Decisions[0].Label
		}
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("sampling not deterministic at %d: %q vs %q", i, a[i], b[i])
		}
	}
}

func TestApplyDirectiveWithVariation_IdenticalModeIgnoresDist(t *testing.T) {
	rs, group := manyWolves(10, models.ArchetypeModeIdentical, 0)
	// Identical mode must use directive.Decisions and IGNORE DecisionDist even if present.
	dir := &AgentIntent{
		Decisions:    []DecisionIntent{{Label: "search_food"}},
		DecisionDist: []WeightedDecision{{Decision: DecisionIntent{Label: "flee"}, Weight: 1}},
	}
	intents := ApplyDirectiveWithVariation(rs, group, dir)
	for _, ra := range group.Members {
		if got := intents[ra.A.ID].Decisions[0].Label; got != "search_food" {
			t.Fatalf("identical mode must ignore DecisionDist; agent %d got %q", ra.A.ID, got)
		}
	}
}

func TestApplyDirectiveWithVariation_ActionVariantsSampled(t *testing.T) {
	rs, group := manyWolves(60, models.ArchetypeModeDistribution, 0)
	dir := &AgentIntent{ActionVariants: []WeightedActionSet{
		{Actions: []ActionIntent{{Type: models.ActionTypeTransition, NewState: "hunting"}}, Weight: 1},
		{Actions: []ActionIntent{{Type: models.ActionTypeSetProperty, PropertyUID: "energy", Value: 10.0}}, Weight: 1},
	}}

	intents := ApplyDirectiveWithVariation(rs, group, dir)

	hunting, setprop := 0, 0
	for _, ra := range group.Members {
		in := intents[ra.A.ID]
		if in == nil || len(in.Actions) != 1 {
			t.Fatalf("agent %d must carry exactly ONE variant's actions (no mixing): %+v", ra.A.ID, in)
		}
		switch in.Actions[0].Type {
		case models.ActionTypeTransition:
			hunting++
		case models.ActionTypeSetProperty:
			setprop++
		}
	}
	if hunting == 0 || setprop == 0 {
		t.Fatalf("both action variants must be sampled across members: transition=%d set_property=%d", hunting, setprop)
	}
	if hunting+setprop != 60 {
		t.Fatalf("every member must carry a variant: got %d of 60", hunting+setprop)
	}
}

func TestApplyDirectiveWithVariation_HeadingJitterEffective(t *testing.T) {
	// Two agents at the SAME position moving toward the same target: with the heading
	// jitter live, their resolved velocity headings MUST differ (the old code discarded
	// the heading jitter, so same-position agents got identical headings).
	rs, group := manyWolves(2, models.ArchetypeModeIdentical, 0) // strength 0 → default 0.15
	dir := &AgentIntent{Movement: &MovementIntent{Intent: "toward", TargetRef: "sheep", Speed: 1}}
	intents := ApplyDirectiveWithVariation(rs, group, dir)

	m1, m2 := intents[1].Movement, intents[2].Movement
	if m1 == nil || m2 == nil || m1.Resolved == nil || m2.Resolved == nil {
		t.Fatalf("archetype movement must pin a Resolved velocity: m1=%+v m2=%+v", m1, m2)
	}
	h1 := math.Atan2(m1.Resolved.DY, m1.Resolved.DX)
	h2 := math.Atan2(m2.Resolved.DY, m2.Resolved.DX)
	if h1 == h2 {
		t.Fatalf("same-position agents got identical headings — heading jitter is not reaching velocity (%v == %v)", h1, h2)
	}

	// The heading spread must grow with VariationStrength (deterministic, same seeds).
	spread := func(strength float64) float64 {
		rs, group := manyWolves(8, models.ArchetypeModeIdentical, strength)
		ints := ApplyDirectiveWithVariation(rs, group, &AgentIntent{Movement: &MovementIntent{Intent: "toward", TargetRef: "sheep", Speed: 1}})
		mn, mx := math.MaxFloat64, -math.MaxFloat64
		for _, ra := range group.Members {
			v := ints[ra.A.ID].Movement.Resolved
			h := math.Atan2(v.DY, v.DX)
			mn, mx = math.Min(mn, h), math.Max(mx, h)
		}
		return mx - mn
	}
	if lo, hi := spread(0.05), spread(0.6); hi <= lo {
		t.Fatalf("heading spread must grow with VariationStrength: spread(0.05)=%v spread(0.6)=%v", lo, hi)
	}
}

func TestApplySpatial_HonorsResolvedVelocity(t *testing.T) {
	// applySpatial must integrate a pre-resolved per-agent velocity verbatim.
	rs, group := manyWolves(1, models.ArchetypeModeIdentical, 0)
	sentinel := Vector{DX: 0.123, DY: -0.456}
	intents := map[uint64]*AgentIntent{
		1: {Movement: &MovementIntent{Intent: "toward", TargetRef: "sheep", Speed: 1, Resolved: &sentinel}},
	}
	applySpatial(rs, group.Members, intents)
	if got := rs.ByID[1].Velocity; got != sentinel {
		t.Fatalf("applySpatial must use Resolved verbatim: got %+v want %+v", got, sentinel)
	}
}

func TestApplySpatial_IndividualMovementUnchanged(t *testing.T) {
	// With Resolved nil (individual strategies), applySpatial must resolve the intent
	// through resolveMovement exactly as before.
	rs, group := manyWolves(1, models.ArchetypeModeIdentical, 0)
	mv := MovementIntent{Intent: "toward", TargetRef: "sheep", Speed: 0.5} // Resolved nil
	want := resolveMovement(rs, rs.ByID[1], mv)
	intents := map[uint64]*AgentIntent{1: {Movement: &mv}}
	applySpatial(rs, group.Members, intents)
	if got := rs.ByID[1].Velocity; got != want {
		t.Fatalf("individual movement (Resolved nil) must use resolveMovement: got %+v want %+v", got, want)
	}
}

func TestDecisionMenu_DerivedFromRules(t *testing.T) {
	model := decisionRuleModel()
	menu := DecisionMenu(model, "wolf")
	if !containsStr(menu, "search_food") {
		t.Fatalf("menu=%v missing search_food", menu)
	}
	if !containsStr(menu, "flee") {
		t.Fatalf("menu=%v missing flee", menu)
	}
	if !containsStr(menu, "rest") {
		t.Fatalf("menu=%v missing rest (nested composite decision)", menu)
	}
	// graze belongs to a sheep-only rule and must NOT appear in the wolf menu.
	if containsStr(menu, "graze") {
		t.Fatalf("menu=%v leaked sheep-only label graze", menu)
	}
}

func TestApplyDirectiveWithVariation_AgentsDiffer(t *testing.T) {
	rs, group := twoWolfState()
	dir := &AgentIntent{Movement: &MovementIntent{Intent: "toward", TargetRef: "sheep", Speed: 1}}

	intents := ApplyDirectiveWithVariation(rs, group, dir)

	a := intents[1]
	b := intents[2]
	if a == nil || b == nil || a.Movement == nil || b.Movement == nil {
		t.Fatalf("both agents must receive a movement intent: a=%+v b=%+v", a, b)
	}

	// Each agent resolves the directive relative to its OWN position: the wolves
	// approach the sheep from opposite sides, so the resolved vectors must differ.
	w1 := rs.ByID[1]
	w2 := rs.ByID[2]
	v1 := w1.Velocity
	v2 := w2.Velocity
	if v1 == (Vector{}) || v2 == (Vector{}) {
		t.Fatalf("expected non-zero resolved velocities, got v1=%+v v2=%+v", v1, v2)
	}
	if v1 == v2 {
		t.Fatalf("archetype agents resolved identical velocities (no divergence): v1=%+v v2=%+v", v1, v2)
	}
}

func TestApplyDirectiveWithVariation_DecisionParamsCopiedSameLabel(t *testing.T) {
	rs, group := twoWolfState()
	dir := &AgentIntent{Decisions: []DecisionIntent{{Label: "search_food", Params: map[string]any{"k": 1}}}}

	intents := ApplyDirectiveWithVariation(rs, group, dir)

	for _, id := range []uint64{1, 2} {
		got := intents[id]
		if got == nil || len(got.Decisions) != 1 {
			t.Fatalf("agent %d missing decision: %+v", id, got)
		}
		if got.Decisions[0].Label != "search_food" {
			t.Fatalf("agent %d label=%q want search_food", id, got.Decisions[0].Label)
		}
		if got.Decisions[0].Params["k"] != 1 {
			t.Fatalf("agent %d params not copied: %+v", id, got.Decisions[0].Params)
		}
	}
}

func TestApplyDirectiveWithVariation_PropertyJitterStaysInBounds(t *testing.T) {
	rs, group := twoWolfState()
	// Give the archetype a bounded numeric property the directive nudges.
	group.Archetype.Properties = []models.AgentProperty{
		{UID: "energy", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
	}
	// Mid-range target so the ±jitter band stays strictly inside [0,100] and the
	// per-agent draws diverge (a near-bound value would clamp both to the bound).
	dir := &AgentIntent{Properties: map[string]any{"energy": 50.0}}

	intents := ApplyDirectiveWithVariation(rs, group, dir)

	saw := map[float64]bool{}
	for _, id := range []uint64{1, 2} {
		got := intents[id]
		if got == nil || got.Properties == nil {
			t.Fatalf("agent %d missing property intent: %+v", id, got)
		}
		v, ok := coerceFloat(got.Properties["energy"])
		if !ok {
			t.Fatalf("agent %d energy not numeric: %v", id, got.Properties["energy"])
		}
		if v < 0 || v > 100 {
			t.Fatalf("agent %d jittered energy %v out of [0,100]", id, v)
		}
		saw[v] = true
	}
	if len(saw) < 2 {
		t.Fatalf("expected per-agent jitter to diverge the property value, saw=%v", saw)
	}
}

func TestMapFullActions_DropsInvalid(t *testing.T) {
	rs, group := twoWolfState()
	ra := group.Members[0]
	ra.Archetype.Properties = []models.AgentProperty{
		{UID: "energy", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
	}
	ra.Archetype.States = []string{"idle", "hunting"}

	raw := []rawAction{
		{Type: string(models.ActionTypeModifyProperty), PropertyUID: "energy", Modifier: string(models.PropertyModifierAdd), Value: 5.0},
		{Type: string(models.ActionTypeModifyProperty), PropertyUID: "ghost", Modifier: string(models.PropertyModifierAdd), Value: 5.0}, // unknown prop -> drop
		{Type: string(models.ActionTypeTransition), NewState: "hunting"},
		{Type: string(models.ActionTypeTransition), NewState: "flying"}, // unknown state -> drop
		{Type: "not_a_real_action"}, // unknown action -> drop
	}

	got := MapFullActions(rs, ra, raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 valid actions, got %d: %+v", len(got), got)
	}
	if got[0].Type != models.ActionTypeModifyProperty || got[0].PropertyUID != "energy" {
		t.Fatalf("first action wrong: %+v", got[0])
	}
	if got[1].Type != models.ActionTypeTransition || got[1].NewState != "hunting" {
		t.Fatalf("second action wrong: %+v", got[1])
	}
}

// ---- Task 17: emit_event in full-control strategy (validateRawAction / MapFullActions) ----

// emitEventModel builds a model whose "howl" event allows producers "llm" and
// "agent", restricted to the "wolf" archetype UID in ProducerUIDs.
func emitEventModel() *models.Model {
	return &models.Model{
		Agents: []models.Agent{
			{UID: "wolf", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable},
			{UID: "sheep", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable},
		},
		Events: []models.EventDefinition{
			{
				UID:       "howl",
				Name:      "Howl",
				Scope:     models.EventScopeGlobal,
				Producers: []models.EventProducer{models.EventProducerLLM, models.EventProducerAgent},
				// ProducerUIDs restricts to "wolf" only so "sheep" will be unauthorized.
				ProducerUIDs: []string{"wolf"},
			},
			{
				// "danger": agent-only (no "llm" producer) so LLM emits are unauthorized.
				UID:       "danger",
				Name:      "Danger",
				Scope:     models.EventScopeGlobal,
				Producers: []models.EventProducer{models.EventProducerAgent},
			},
			{
				// "alert": no Producers declared => permissive (any producer allowed).
				UID:   "alert",
				Name:  "Alert",
				Scope: models.EventScopeGlobal,
			},
		},
	}
}

// emitWolfState builds a RuntimeState with one wolf agent backed by emitEventModel.
func emitWolfState() (*RuntimeState, *RuntimeAgent) {
	m := emitEventModel()
	arch := &m.Agents[0]
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       1,
			AgentUID: "wolf",
			State:    "idle",
			Position: &common.Point{X: 10, Y: 10},
		},
		Archetype: arch,
	}
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{Seed: 1},
		Model:  m,
		Env:    &datasims.SimulationEnvironment{Width: 100, Height: 100},
		Agents: []*RuntimeAgent{ra},
		ByID:   map[uint64]*RuntimeAgent{1: ra},
		RNG:    rand.New(rand.NewSource(1)),
	}
	return rs, ra
}

// TestValidateRawAction_EmitEvent_Authorized verifies that validateRawAction
// accepts an emit_event action when the EventDefinition's Producers include "llm"
// and the agent's archetype UID is in ProducerUIDs.
func TestValidateRawAction_EmitEvent_Authorized(t *testing.T) {
	rs, ra := emitWolfState()
	r := rawAction{Type: "emit_event", EventUID: "howl", EventData: "pack_call"}
	ai, ok := validateRawAction(rs, ra, r)
	if !ok {
		t.Fatal("expected emit_event 'howl' to be authorized for wolf archetype")
	}
	if ai.Type != models.ActionTypeEmitEvent {
		t.Fatalf("action type=%q want emit_event", ai.Type)
	}
	if ai.EventUID != "howl" {
		t.Fatalf("event uid=%q want howl", ai.EventUID)
	}
	if ai.EventData != "pack_call" {
		t.Fatalf("event data=%v want 'pack_call'", ai.EventData)
	}
}

// TestValidateRawAction_EmitEvent_UnauthorizedNoLLMProducer verifies that an
// emit_event is dropped when the EventDefinition's Producers do NOT include "llm".
func TestValidateRawAction_EmitEvent_UnauthorizedNoLLMProducer(t *testing.T) {
	rs, ra := emitWolfState()
	// "danger" only has EventProducerAgent; LLM emit should be unauthorized.
	r := rawAction{Type: "emit_event", EventUID: "danger"}
	_, ok := validateRawAction(rs, ra, r)
	if ok {
		t.Fatal("expected emit_event 'danger' to be rejected (no 'llm' producer)")
	}
}

// TestValidateRawAction_EmitEvent_UnauthorizedWrongArchetype verifies that a wolf
// event is rejected when the emitting agent's archetype is "sheep" (ProducerUIDs
// only includes "wolf").
func TestValidateRawAction_EmitEvent_UnauthorizedWrongArchetype(t *testing.T) {
	rs, _ := emitWolfState()
	// Build a sheep agent.
	sheepArch := &rs.Model.Agents[1]
	sheep := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 2, AgentUID: "sheep"},
		Archetype: sheepArch,
	}
	r := rawAction{Type: "emit_event", EventUID: "howl"}
	_, ok := validateRawAction(rs, sheep, r)
	if ok {
		t.Fatal("expected emit_event 'howl' to be rejected for sheep (not in ProducerUIDs)")
	}
}

// TestValidateRawAction_EmitEvent_MissingUID verifies that an emit_event without
// an event_uid is rejected.
func TestValidateRawAction_EmitEvent_MissingUID(t *testing.T) {
	rs, ra := emitWolfState()
	r := rawAction{Type: "emit_event", EventUID: ""}
	_, ok := validateRawAction(rs, ra, r)
	if ok {
		t.Fatal("expected emit_event with empty uid to be rejected")
	}
}

// TestValidateRawAction_EmitEvent_PermissiveNoProducers verifies that an
// EventDefinition with no declared Producers permits any producer (permissive).
func TestValidateRawAction_EmitEvent_PermissiveNoProducers(t *testing.T) {
	rs, ra := emitWolfState()
	// "alert" has no Producers => permissive.
	r := rawAction{Type: "emit_event", EventUID: "alert"}
	ai, ok := validateRawAction(rs, ra, r)
	if !ok {
		t.Fatal("expected emit_event 'alert' (no producers declared) to be authorized")
	}
	if ai.EventUID != "alert" {
		t.Fatalf("event uid=%q want alert", ai.EventUID)
	}
}

// TestMapFullActions_EmitEvent_PartitionedToEmits verifies that demuxFullActions
// (via MapFullActions) routes emit_event ActionIntents to intent.Emits, not
// intent.Actions, when the whole pipeline is exercised.
func TestMapFullActions_EmitEvent_PartitionedToEmits(t *testing.T) {
	rs, ra := emitWolfState()
	// Provide a property so we have a non-emit action too.
	ra.Archetype.Properties = []models.AgentProperty{
		{UID: "energy", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
	}
	ra.A.Properties = map[string]any{"energy": 50.0}

	raw := []rawAction{
		{Type: "set_property", PropertyUID: "energy", Value: 30.0},
		{Type: "emit_event", EventUID: "howl", EventData: "call"},
		{Type: "emit_event", EventUID: "danger"}, // unauthorized — should be dropped
	}
	mapped := MapFullActions(rs, ra, raw)

	// set_property + howl emit => 2 ActionIntents; danger is unauthorized => dropped.
	if len(mapped) != 2 {
		t.Fatalf("expected 2 mapped (1 set_property + 1 emit), got %d: %v", len(mapped), mapped)
	}
	// Verify types.
	if mapped[0].Type != models.ActionTypeSetProperty {
		t.Fatalf("mapped[0].Type=%q want set_property", mapped[0].Type)
	}
	if mapped[1].Type != models.ActionTypeEmitEvent || mapped[1].EventUID != "howl" {
		t.Fatalf("mapped[1]=%+v want emit_event howl", mapped[1])
	}
}

// TestApplyIntentActions_EmitEvent_UsesLLMProducer verifies that ApplyIntentActions
// drains in.Emits via applyEmitEvent(..., EventProducerLLM): the event is queued
// into rs.events, stats are incremented, and a memory entry is written.
func TestApplyIntentActions_EmitEvent_UsesLLMProducer(t *testing.T) {
	withFixedClock(t)
	rs, ra := emitWolfState()
	rs.stats = &statsAccumulator{ByEvent: map[string]uint64{}}

	intent := &AgentIntent{
		Emits: []EmitIntent{
			{EventUID: "howl", Data: "from_intent"},
		},
	}
	ApplyIntentActions(rs, ra, intent)

	if len(rs.events) != 1 {
		t.Fatalf("expected 1 event queued, got %d", len(rs.events))
	}
	if rs.events[0].EventUID != "howl" {
		t.Fatalf("event uid=%q want howl", rs.events[0].EventUID)
	}
	if rs.stats.Events != 1 {
		t.Fatalf("stats.Events=%d want 1 (LLM emit must increment stats)", rs.stats.Events)
	}
	if rs.stats.ByEvent["howl"] != 1 {
		t.Fatalf("stats.ByEvent[howl]=%d want 1", rs.stats.ByEvent["howl"])
	}
	// Memory: ApplyIntentActions flushes a memory entry for each emit.
	found := false
	for _, m := range ra.A.Memory {
		if m.Summary == "emit howl" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected memory entry 'emit howl', got %+v", ra.A.Memory)
	}
}

func TestDecisionMenuWithHints(t *testing.T) {
	m := &models.Model{
		Rules: []models.Rule{
			{
				ID: "r1", Enabled: true, AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{{Type: models.ConditionTypeDecision, Value: "hunt"}},
				Actions: []models.Action{
					{Type: models.ActionTypeMoveToward, TargetUID: "sheep"},
					{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierSubtract, Value: 2.0},
				},
			},
			{
				ID: "r2", Enabled: true, AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{{Type: models.ConditionTypeDecision, Value: "rest"}},
				Actions:    []models.Action{{Type: models.ActionTypeSetProperty, PropertyUID: "energy", Value: 100.0}},
			},
		},
	}
	menu := DecisionMenuWithHints(m, "wolf")
	if len(menu) != 2 {
		t.Fatalf("want 2 hinted labels, got %v", menu)
	}
	if menu[0] != "hunt (triggers: move_toward, modify_property)" {
		t.Errorf("unexpected hint for hunt: %q", menu[0])
	}
	if menu[1] != "rest (triggers: set_property)" {
		t.Errorf("unexpected hint for rest: %q", menu[1])
	}
}

func TestDecisionLabelHint_AppendsActionPromptContext(t *testing.T) {
	// The FIRST non-empty Action.LLMPromptContext (first-seen rule order, first
	// action with a hint) is appended after the trigger types, trimmed.
	m := &models.Model{
		Rules: []models.Rule{
			{
				ID: "r1", Enabled: true, AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{{Type: models.ConditionTypeDecision, Value: "hunt"}},
				Actions: []models.Action{
					{Type: models.ActionTypeMoveToward, TargetUID: "sheep"},
					{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierSubtract, Value: 2.0, LLMPromptContext: " chase the nearest prey "},
				},
			},
			{
				ID: "r2", Enabled: true, AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{{Type: models.ConditionTypeDecision, Value: "hunt"}},
				Actions:    []models.Action{{Type: models.ActionTypeTransition, NewState: "hunting", LLMPromptContext: "later hint must lose"}},
			},
		},
	}
	menu := DecisionMenuWithHints(m, "wolf")
	want := "hunt (triggers: move_toward, modify_property, transition — chase the nearest prey)"
	if len(menu) != 1 || menu[0] != want {
		t.Errorf("hinted label = %v, want [%q]", menu, want)
	}
}

func TestDecisionLabelHint_ContextCapped(t *testing.T) {
	long := strings.Repeat("x", 150)
	m := &models.Model{
		Rules: []models.Rule{{
			ID: "r1", Enabled: true, AppliesTo: []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeDecision, Value: "hunt"}},
			Actions:    []models.Action{{Type: models.ActionTypeMoveToward, TargetUID: "sheep", LLMPromptContext: long}},
		}},
	}
	hint := decisionLabelHint(m, "wolf", "hunt")
	want := " (triggers: move_toward — " + strings.Repeat("x", 100) + "…)"
	if hint != want {
		t.Errorf("capped hint = %q, want %q", hint, want)
	}
}

func TestLLMEmittableEventLabels(t *testing.T) {
	m := &models.Model{
		Events: []models.EventDefinition{
			{UID: "howl", Producers: []models.EventProducer{models.EventProducerLLM}, LLMPromptContext: "  wolf howl to rally the pack  "},
			{UID: "rain", Producers: []models.EventProducer{models.EventProducerEnvironment}},
			{UID: "bark", Producers: []models.EventProducer{models.EventProducerLLM}, ProducerUIDs: []string{"dog"}},
			{UID: "growl", Producers: []models.EventProducer{models.EventProducerLLM}},
		},
	}
	got := llmEmittableEventLabels(m, "wolf")
	want := []string{"howl (wolf howl to rally the pack)", "growl"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestLLMEmittableEventLabels_ContextCapped(t *testing.T) {
	long := strings.Repeat("y", 150)
	m := &models.Model{
		Events: []models.EventDefinition{
			{UID: "howl", Producers: []models.EventProducer{models.EventProducerLLM}, LLMPromptContext: long},
		},
	}
	got := llmEmittableEventLabels(m, "wolf")
	want := "howl (" + strings.Repeat("y", 100) + "…)"
	if len(got) != 1 || got[0] != want {
		t.Errorf("emittable label = %v, want [%q]", got, want)
	}
}

// TestVaryProperties_DeterministicAcrossMapOrder guards the engine determinism
// doctrine. varyProperties consumes one RNG draw per numeric property; if it
// ranged over the directive map directly, Go's randomized map iteration order
// would decide which draw lands on which property, so repeated calls (with the
// same seed) could produce different assignments across process runs. Iterating
// the directive keys in SORTED order pins draw→property. We rebuild the map on
// every call precisely so its (randomized) iteration order varies between calls;
// the byte-stability assertion is only meaningful because of that — the pre-fix
// code cannot guarantee it. Run with -count=1 (no cached pass).
func TestVaryProperties_DeterministicAcrossMapOrder(t *testing.T) {
	arch := &models.Agent{
		UID: "wolf", Type: models.AgentTypeArchetype,
		Properties: []models.AgentProperty{
			{UID: "a", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
			{UID: "b", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
		},
	}
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}, Archetype: arch}

	run := func() map[string]any {
		dir := map[string]any{} // fresh map each call → varied iteration order
		dir["a"] = 50.0
		dir["b"] = 50.0
		return varyProperties(ra, dir, rand.New(rand.NewSource(7)), 0.2)
	}
	want := run()
	for i := 0; i < 50; i++ {
		got := run()
		if got["a"] != want["a"] || got["b"] != want["b"] {
			t.Fatalf("varyProperties not byte-stable across calls (map-iteration-order leak): want a=%v b=%v, got a=%v b=%v",
				want["a"], want["b"], got["a"], got["b"])
		}
	}
}

// TestApplyIntentActions_EmitEvent_UnauthorizedDropped verifies that an EmitIntent
// for an unauthorized event (no "llm" in Producers) is silently dropped even if
// it somehow reaches ApplyIntentActions (defense-in-depth: applyEmitEvent re-checks).
func TestApplyIntentActions_EmitEvent_UnauthorizedDropped(t *testing.T) {
	rs, ra := emitWolfState()
	rs.stats = &statsAccumulator{ByEvent: map[string]uint64{}}

	intent := &AgentIntent{
		Emits: []EmitIntent{
			{EventUID: "danger"}, // agent-only, not llm
		},
	}
	ApplyIntentActions(rs, ra, intent)

	if len(rs.events) != 0 {
		t.Fatalf("expected 0 events queued (unauthorized emit), got %d", len(rs.events))
	}
	if rs.stats.Events != 0 {
		t.Fatalf("stats.Events=%d want 0 (unauthorized emit must not increment stats)", rs.stats.Events)
	}
}
