package simulations

import (
	"math/rand"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// fptr returns a pointer to a float64 literal (test convenience).
func fptr(v float64) *float64 { return &v }

// ruleRuntime builds a minimal RuntimeState around a set of runtime agents for
// rule-engine tests. Spatial dimensions are fixed at 100x80, non-torus.
func ruleRuntime(agents []*RuntimeAgent, rules []models.Rule) *RuntimeState {
	idx := NewSpatialIndex(100, 80, 10, false)
	byID := make(map[uint64]*RuntimeAgent, len(agents))
	for _, ra := range agents {
		byID[ra.A.ID] = ra
		if ra.A.Position != nil {
			idx.Insert(ra.A.ID, *ra.A.Position)
		}
	}
	return &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  &models.Model{Rules: rules},
		Env:    &datasims.SimulationEnvironment{Width: 100, Height: 80},
		Agents: agents,
		ByID:   byID,
		Index:  idx,
		RNG:    rand.New(rand.NewSource(1)),
		rules:  rules,
	}
}

// wolfArchetype is a small archetype with a numeric "energy" property bounded
// [0,100] plus an immutable "species" property and two states.
func wolfArchetype() *models.Agent {
	return &models.Agent{
		UID:      "wolf",
		Type:     models.AgentTypeArchetype,
		MoveType: models.AgentMoveTypeMovable,
		States:   []string{"idle", "hunting"},
		Properties: []models.AgentProperty{
			{UID: "energy", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
			{UID: "species", Type: models.PropertyTypeString, Immutable: true},
		},
	}
}

func TestEvalCondition_PropertyGreaterThan(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Properties: map[string]any{"energy": 60.0}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	c := models.Condition{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpGreaterThan, Value: 50.0}
	if !EvalCondition(rs, ra, nil, c) {
		t.Fatal("energy 60 > 50 should be true")
	}
	c.Value = 70.0
	if EvalCondition(rs, ra, nil, c) {
		t.Fatal("energy 60 > 70 should be false")
	}
}

func TestEvalCondition_Composite_OR(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Properties: map[string]any{"energy": 10.0}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	c := models.Condition{
		Type:          models.ConditionTypeComposite,
		LogicOperator: models.LogicOpOR,
		SubConditions: []models.Condition{
			{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpGreaterThan, Value: 50.0}, // false
			{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 20.0},    // true
		},
	}
	if !EvalCondition(rs, ra, nil, c) {
		t.Fatal("OR with one true sub should be true")
	}
}

func TestEvalCondition_Composite_NOT(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Properties: map[string]any{"energy": 60.0}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	c := models.Condition{
		Type:          models.ConditionTypeComposite,
		LogicOperator: models.LogicOpNOT,
		SubConditions: []models.Condition{
			{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpGreaterThan, Value: 50.0}, // true => NOT => false
		},
	}
	if EvalCondition(rs, ra, nil, c) {
		t.Fatal("NOT of a true condition should be false")
	}
}

func TestEvalCondition_Proximity_UsesIndex(t *testing.T) {
	self := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}}, Archetype: wolfArchetype()}
	near := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 12, Y: 11}}, Archetype: &models.Agent{UID: "sheep"}}
	far := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 3, AgentUID: "sheep", Position: &common.Point{X: 90, Y: 70}}, Archetype: &models.Agent{UID: "sheep"}}
	rs := ruleRuntime([]*RuntimeAgent{self, near, far}, nil)
	c := models.Condition{Type: models.ConditionTypeProximity, Radius: fptr(5), TargetUIDs: []string{"sheep"}}
	if !EvalCondition(rs, self, nil, c) {
		t.Fatal("a sheep within radius 5 should satisfy proximity")
	}
	// Far away, large radius excluded: shrink radius so only the far one would count.
	c2 := models.Condition{Type: models.ConditionTypeProximity, Radius: fptr(1), TargetUIDs: []string{"sheep"}}
	if EvalCondition(rs, self, nil, c2) {
		t.Fatal("no sheep within radius 1 should fail proximity")
	}
}

func TestEvalCondition_Decision_MatchesIntent(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	intent := &AgentIntent{Decisions: []DecisionIntent{{Label: "flee"}}}
	c := models.Condition{Type: models.ConditionTypeDecision, Value: "flee"}
	if !EvalCondition(rs, ra, intent, c) {
		t.Fatal("decision label flee should match intent")
	}
	c.Value = "hunt"
	if EvalCondition(rs, ra, intent, c) {
		t.Fatal("decision label hunt should not match a flee intent")
	}
}

func TestEvalCondition_Memory(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Memory: []datasims.AgentMemory{
		{Step: 5, Type: datasims.AgentMemoryInteraction, Summary: "fought wolf"},
	}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	c := models.Condition{Type: models.ConditionTypeMemory, Value: "wolf", Parameters: map[string]any{"memory_type": "interaction"}}
	if !EvalCondition(rs, ra, nil, c) {
		t.Fatal("memory containing 'wolf' interaction should match")
	}
}

func TestEvalCondition_State(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, State: "hunting"}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	c := models.Condition{Type: models.ConditionTypeState, Operator: models.ConditionOpEquals, Value: "hunting"}
	if !EvalCondition(rs, ra, nil, c) {
		t.Fatal("state hunting should equal hunting")
	}
}

func TestEvalCondition_Step(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	rs.Step = 7
	c := models.Condition{Type: models.ConditionTypeStep, Operator: models.ConditionOpGreaterOrEq, Value: 5.0}
	if !EvalCondition(rs, ra, nil, c) {
		t.Fatal("step 7 >= 5 should be true")
	}
}

func TestEvalCondition_Random_UsesRNG(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	// Probability 1.0 must always pass; 0.0 must always fail.
	always := models.Condition{Type: models.ConditionTypeRandom, Value: 1.0}
	never := models.Condition{Type: models.ConditionTypeRandom, Value: 0.0}
	if !EvalCondition(rs, ra, nil, always) {
		t.Fatal("probability 1.0 should always pass")
	}
	if EvalCondition(rs, ra, nil, never) {
		t.Fatal("probability 0.0 should never pass")
	}
}

func TestApplyAction_ModifyProperty_AddClampsToMax(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Properties: map[string]any{"energy": 95.0}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	a := models.Action{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 20.0}
	ApplyAction(rs, ra, a)
	if got := ra.A.Properties["energy"].(float64); got != 100.0 {
		t.Fatalf("energy=%v want clamped to max 100", got)
	}
}

func TestApplyAction_SetProperty(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	a := models.Action{Type: models.ActionTypeSetProperty, PropertyUID: "energy", Value: 30.0}
	ApplyAction(rs, ra, a)
	if got := ra.A.Properties["energy"].(float64); got != 30.0 {
		t.Fatalf("energy=%v want set to 30", got)
	}
}

func TestApplyAction_Transition_SetsState(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, State: "idle"}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeTransition, NewState: "hunting"})
	if ra.A.State != "hunting" {
		t.Fatalf("state=%q want hunting", ra.A.State)
	}
	// Unknown state rejected (state unchanged).
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeTransition, NewState: "flying"})
	if ra.A.State != "hunting" {
		t.Fatalf("state=%q want unchanged (unknown state rejected)", ra.A.State)
	}
}

func TestApplyAction_MoveToward_SetsVelocity(t *testing.T) {
	self := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}}, Archetype: wolfArchetype()}
	prey := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 30, Y: 10}}, Archetype: &models.Agent{UID: "sheep"}}
	rs := ruleRuntime([]*RuntimeAgent{self, prey}, nil)
	a := models.Action{Type: models.ActionTypeMoveToward, TargetUID: "sheep", Speed: fptr(2)}
	ApplyAction(rs, self, a)
	if self.Velocity.DX <= 0 {
		t.Fatalf("velocity DX=%v want positive (toward prey to the right)", self.Velocity.DX)
	}
}

func TestApplyAction_EmitEvent_QueuesEvent(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Position: &common.Point{X: 5, Y: 5}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	rs.Model.Events = []models.EventDefinition{{UID: "howl", Name: "howl"}}
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl"})
	if len(rs.events) != 1 || rs.events[0].EventUID != "howl" {
		t.Fatalf("events=%+v want one howl event queued", rs.events)
	}
}

func TestApplyAction_RespectsImmutable(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Properties: map[string]any{"species": "canine"}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeSetProperty, PropertyUID: "species", Value: "feline"})
	if got := ra.A.Properties["species"].(string); got != "canine" {
		t.Fatalf("immutable species=%q want unchanged 'canine'", got)
	}
}

func TestApplyAction_SpawnAndRemoveQueued(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeSpawnAgent, SpawnAgentUID: "wolf", SpawnCount: 2})
	if len(rs.spawns) != 1 || rs.spawns[0].count != 2 {
		t.Fatalf("spawns=%+v want one spawn of count 2", rs.spawns)
	}
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeRemoveAgent})
	if !ra.removed || len(rs.removals) != 1 {
		t.Fatalf("expected agent 1 marked removed and queued; removed=%v removals=%v", ra.removed, rs.removals)
	}
}

func TestApplyRules_PriorityAndCooldown(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", State: "idle", Properties: map[string]any{"energy": 10.0}}, Archetype: wolfArchetype()}
	rules := []models.Rule{
		{ID: "low", Name: "low", Priority: 1, Enabled: true, AppliesTo: []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeTransition, NewState: "idle"}}},
		{ID: "high", Name: "high", Priority: 10, Enabled: true, AppliesTo: []string{"wolf"}, CooldownSteps: 3,
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeTransition, NewState: "hunting"}}},
	}
	rs := ruleRuntime([]*RuntimeAgent{ra}, rules)
	rs.Step = 1
	ApplyRules(rs, ra, nil, nil)
	if ra.A.State != "idle" {
		// high (priority 10) sets hunting, then low (priority 1) sets idle => idle wins as it runs last.
		t.Fatalf("after step 1 state=%q want idle (low priority runs last)", ra.A.State)
	}
	// The high-priority rule fired at step 1; cooldown 3 should block it from re-firing soon.
	if ra.Cooldowns["high"] != 1 {
		t.Fatalf("cooldown for high=%d want recorded at step 1", ra.Cooldowns["high"])
	}
	rs.Step = 2
	ra.A.State = "idle"
	ApplyRules(rs, ra, nil, nil)
	// At step 2, "high" is still in cooldown (fired step 1, cooldown 3 => next allowed at step 4).
	// So only "low" runs and state stays idle (it already was). Verify cooldown unchanged.
	if ra.Cooldowns["high"] != 1 {
		t.Fatalf("cooldown for high=%d want still 1 (blocked at step 2)", ra.Cooldowns["high"])
	}
}

func TestApplyRules_ExecuteOnce(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Properties: map[string]any{"energy": 10.0}}, Archetype: wolfArchetype()}
	rules := []models.Rule{
		{ID: "once", Name: "once", Priority: 1, Enabled: true, ExecuteOnce: true, AppliesTo: []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 5.0}}},
	}
	rs := ruleRuntime([]*RuntimeAgent{ra}, rules)
	rs.Step = 1
	ApplyRules(rs, ra, nil, nil)
	rs.Step = 2
	ApplyRules(rs, ra, nil, nil)
	if got := ra.A.Properties["energy"].(float64); got != 15.0 {
		t.Fatalf("energy=%v want 15 (rule fired exactly once: 10+5)", got)
	}
}

func TestApplyRules_SkipsLLMOwnedPurpose(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 10.0}}, Archetype: wolfArchetype()}
	prey := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 30, Y: 10}}, Archetype: &models.Agent{UID: "sheep"}}
	rules := []models.Rule{
		{ID: "move", Name: "move", Priority: 1, Enabled: true, AppliesTo: []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeMoveToward, TargetUID: "sheep", Speed: fptr(2)}}},
	}
	rs := ruleRuntime([]*RuntimeAgent{ra, prey}, rules)
	rs.Step = 1
	owned := purposeSet{purposeMovement: true}
	ApplyRules(rs, ra, nil, owned)
	if ra.Velocity.DX != 0 || ra.Velocity.DY != 0 {
		t.Fatalf("movement owned by LLM: rule move action must be skipped, velocity=%+v", ra.Velocity)
	}
}

// --- Task 7: rule memory write --------------------------------------------------

func TestApplyRules_WritesRuleMemoryOnFire(t *testing.T) {
	// A rule that fires (energy < 50) must produce an AgentMemoryRule entry in
	// the agent's memory log with the correct summary containing the rule label
	// and details containing the rule_id and conditions count.
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 1, AgentUID: "wolf",
			Properties: map[string]any{"energy": 10.0},
		},
		Archetype: wolfArchetype(),
	}
	rules := []models.Rule{
		{
			ID: "hunt_rule", Name: "hunt", Priority: 1, Enabled: true,
			AppliesTo:  []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeTransition, NewState: "hunting"}},
		},
	}
	rs := ruleRuntime([]*RuntimeAgent{ra}, rules)
	rs.Step = 3

	beforeLen := len(ra.A.Memory)
	ApplyRules(rs, ra, nil, nil)

	// At least one AgentMemoryRule entry must have been appended.
	var ruleEntries []datasims.AgentMemory
	for _, m := range ra.A.Memory[beforeLen:] {
		if m.Type == datasims.AgentMemoryRule {
			ruleEntries = append(ruleEntries, m)
		}
	}
	if len(ruleEntries) == 0 {
		t.Fatalf("no AgentMemoryRule entry found after rule fire; memory=%+v", ra.A.Memory)
	}
	entry := ruleEntries[0]
	if entry.Step != 3 {
		t.Fatalf("rule memory step=%d want 3", entry.Step)
	}
	// Summary must contain the rule's Name ("hunt").
	if !containsSub(entry.Summary, "hunt") {
		t.Fatalf("rule memory summary=%q must contain rule name 'hunt'", entry.Summary)
	}
	// Details must carry rule_id and conditions count.
	details, ok := entry.Details.(map[string]any)
	if !ok {
		t.Fatalf("rule memory details type=%T want map[string]any", entry.Details)
	}
	if details["rule_id"] != "hunt_rule" {
		t.Fatalf("rule_id=%v want hunt_rule", details["rule_id"])
	}
	if details["conditions"] != 1 {
		t.Fatalf("conditions=%v want 1", details["conditions"])
	}
}

func TestApplyRules_NoRuleMemoryWhenNoFire(t *testing.T) {
	// A rule whose conditions are false must produce NO AgentMemoryRule entry.
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 1, AgentUID: "wolf",
			Properties: map[string]any{"energy": 80.0}, // energy NOT < 50, condition false
		},
		Archetype: wolfArchetype(),
	}
	rules := []models.Rule{
		{
			ID: "hunt_rule", Name: "hunt", Priority: 1, Enabled: true,
			AppliesTo:  []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeTransition, NewState: "hunting"}},
		},
	}
	rs := ruleRuntime([]*RuntimeAgent{ra}, rules)
	rs.Step = 1
	ApplyRules(rs, ra, nil, nil)
	for _, m := range ra.A.Memory {
		if m.Type == datasims.AgentMemoryRule {
			t.Fatalf("unexpected AgentMemoryRule when no rule fired: %+v", m)
		}
	}
}

func TestApplyRules_RuleLabelFallsBackToID(t *testing.T) {
	// When a rule has no Name, the rule ID must be used as the label.
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 1, AgentUID: "wolf",
			Properties: map[string]any{"energy": 10.0},
		},
		Archetype: wolfArchetype(),
	}
	rules := []models.Rule{
		{
			// Name is empty — ruleLabel must fall back to ID "r-noname".
			ID: "r-noname", Priority: 1, Enabled: true,
			AppliesTo:  []string{"wolf"},
			Conditions: []models.Condition{{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpLessThan, Value: 50.0}},
			Actions:    []models.Action{{Type: models.ActionTypeTransition, NewState: "hunting"}},
		},
	}
	rs := ruleRuntime([]*RuntimeAgent{ra}, rules)
	rs.Step = 1
	ApplyRules(rs, ra, nil, nil)
	for _, m := range ra.A.Memory {
		if m.Type == datasims.AgentMemoryRule && containsSub(m.Summary, "r-noname") {
			return // found the expected entry
		}
	}
	t.Fatalf("expected rule memory with ID fallback label 'r-noname'; memory=%+v", ra.A.Memory)
}

// --- Task 15: eventReaches scope-authoritative fix ---------------------------------

// eventReachesRuntime builds a minimal RuntimeState with the given EventDefinition
// registered, a single agent ra, and Env set to 100x80 non-torus.
func eventReachesRuntime(def models.EventDefinition, ra *RuntimeAgent) (*RuntimeState, *runtimeEvent) {
	idx := NewSpatialIndex(100, 80, 10, false)
	if ra.A.Position != nil {
		idx.Insert(ra.A.ID, *ra.A.Position)
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{},
		Model: &models.Model{Events: []models.EventDefinition{def}},
		Env:   &datasims.SimulationEnvironment{Width: 100, Height: 80},
		Index: idx,
		RNG:   rand.New(rand.NewSource(1)),
	}
	ev := &runtimeEvent{EventUID: def.UID, EmitStep: 0}
	return rs, ev
}

// TestEventReaches_GlobalEmpty verifies that a global event with no ConsumerUIDs
// reaches any agent.
func TestEventReaches_GlobalEmpty(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}, Archetype: wolfArchetype()}
	def := models.EventDefinition{UID: "alarm", Scope: models.EventScopeGlobal, ConsumerUIDs: nil}
	rs, ev := eventReachesRuntime(def, ra)
	defPtr := &rs.Model.Events[0]
	if !eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("global event with empty ConsumerUIDs must reach all agents")
	}
}

// TestEventReaches_GlobalWithMatchingConsumer verifies that a global event with
// ConsumerUIDs set reaches an agent whose AgentUID is in the set.
func TestEventReaches_GlobalWithMatchingConsumer(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}, Archetype: wolfArchetype()}
	def := models.EventDefinition{UID: "alarm", Scope: models.EventScopeGlobal, ConsumerUIDs: []string{"wolf", "sheep"}}
	rs, ev := eventReachesRuntime(def, ra)
	defPtr := &rs.Model.Events[0]
	if !eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("global event with ConsumerUIDs containing agent's AgentUID must reach that agent")
	}
}

// TestEventReaches_GlobalWithNonMatchingConsumer verifies that a global event
// with ConsumerUIDs set does NOT reach an agent whose AgentUID is absent from the set.
func TestEventReaches_GlobalWithNonMatchingConsumer(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "deer"}, Archetype: wolfArchetype()}
	def := models.EventDefinition{UID: "alarm", Scope: models.EventScopeGlobal, ConsumerUIDs: []string{"wolf", "sheep"}}
	rs, ev := eventReachesRuntime(def, ra)
	defPtr := &rs.Model.Events[0]
	if eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("global event with ConsumerUIDs not containing agent's AgentUID must NOT reach that agent")
	}
}

// TestEventReaches_LocalWithinRadiusEmpty verifies that a local event with no
// ConsumerUIDs reaches any agent within Radius.
func TestEventReaches_LocalWithinRadiusEmpty(t *testing.T) {
	pos := &common.Point{X: 10, Y: 10}
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 12, Y: 10}}, Archetype: wolfArchetype()}
	r := 5.0
	def := models.EventDefinition{UID: "bang", Scope: models.EventScopeLocal, Radius: &r, ConsumerUIDs: nil}
	rs, ev := eventReachesRuntime(def, ra)
	ev.Position = pos
	defPtr := &rs.Model.Events[0]
	if !eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("local event within Radius with empty ConsumerUIDs must reach any agent")
	}
}

// TestEventReaches_LocalOutsideRadius verifies that a local event does NOT reach
// an agent outside its Radius, regardless of ConsumerUIDs.
func TestEventReaches_LocalOutsideRadius(t *testing.T) {
	pos := &common.Point{X: 10, Y: 10}
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 80, Y: 70}}, Archetype: wolfArchetype()}
	r := 5.0
	def := models.EventDefinition{UID: "bang", Scope: models.EventScopeLocal, Radius: &r, ConsumerUIDs: nil}
	rs, ev := eventReachesRuntime(def, ra)
	ev.Position = pos
	defPtr := &rs.Model.Events[0]
	if eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("local event: agent outside Radius must NOT receive the event")
	}
}

// TestEventReaches_LocalWithinRadiusMatchingConsumer verifies that a local event
// with ConsumerUIDs reaches an agent within radius whose AgentUID is in the set.
func TestEventReaches_LocalWithinRadiusMatchingConsumer(t *testing.T) {
	pos := &common.Point{X: 10, Y: 10}
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 12, Y: 10}}, Archetype: wolfArchetype()}
	r := 5.0
	def := models.EventDefinition{UID: "bang", Scope: models.EventScopeLocal, Radius: &r, ConsumerUIDs: []string{"wolf"}}
	rs, ev := eventReachesRuntime(def, ra)
	ev.Position = pos
	defPtr := &rs.Model.Events[0]
	if !eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("local event within radius with matching ConsumerUIDs must reach agent")
	}
}

// TestEventReaches_LocalWithinRadiusNonMatchingConsumer verifies that a local
// event with ConsumerUIDs does NOT reach an agent within radius but absent from the set.
func TestEventReaches_LocalWithinRadiusNonMatchingConsumer(t *testing.T) {
	pos := &common.Point{X: 10, Y: 10}
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "deer", Position: &common.Point{X: 12, Y: 10}}, Archetype: wolfArchetype()}
	r := 5.0
	def := models.EventDefinition{UID: "bang", Scope: models.EventScopeLocal, Radius: &r, ConsumerUIDs: []string{"wolf"}}
	rs, ev := eventReachesRuntime(def, ra)
	ev.Position = pos
	defPtr := &rs.Model.Events[0]
	if eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("local event within radius but agent AgentUID absent from ConsumerUIDs must NOT reach agent")
	}
}

// TestEventReaches_TargetedEmptyConsumers verifies that a targeted event with
// empty ConsumerUIDs reaches NO agent (empty => no one).
func TestEventReaches_TargetedEmptyConsumers(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}, Archetype: wolfArchetype()}
	def := models.EventDefinition{UID: "signal", Scope: models.EventScopeTargeted, ConsumerUIDs: nil}
	rs, ev := eventReachesRuntime(def, ra)
	defPtr := &rs.Model.Events[0]
	if eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("targeted event with empty ConsumerUIDs must reach NO agent (empty => no one)")
	}
}

// TestEventReaches_TargetedMatchingConsumer verifies that a targeted event
// reaches an agent whose AgentUID is in ConsumerUIDs.
func TestEventReaches_TargetedMatchingConsumer(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}, Archetype: wolfArchetype()}
	def := models.EventDefinition{UID: "signal", Scope: models.EventScopeTargeted, ConsumerUIDs: []string{"wolf", "sheep"}}
	rs, ev := eventReachesRuntime(def, ra)
	defPtr := &rs.Model.Events[0]
	if !eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("targeted event must reach agent whose AgentUID is in ConsumerUIDs")
	}
}

// TestEventReaches_TargetedNonMatchingConsumer verifies that a targeted event
// does NOT reach an agent whose AgentUID is absent from ConsumerUIDs.
func TestEventReaches_TargetedNonMatchingConsumer(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "deer"}, Archetype: wolfArchetype()}
	def := models.EventDefinition{UID: "signal", Scope: models.EventScopeTargeted, ConsumerUIDs: []string{"wolf", "sheep"}}
	rs, ev := eventReachesRuntime(def, ra)
	defPtr := &rs.Model.Events[0]
	if eventReaches(rs, defPtr, ev, ra) {
		t.Fatal("targeted event must NOT reach agent whose AgentUID is absent from ConsumerUIDs")
	}
}

// --- Task 16: producer authorization + advisory payload validation -----------------

// emitEventRuntime builds a minimal RuntimeState with the given EventDefinition,
// stats accumulator, and a single agent ra.
func emitEventRuntime(def *models.EventDefinition, ra *RuntimeAgent) *RuntimeState {
	var events []models.EventDefinition
	if def != nil {
		events = []models.EventDefinition{*def}
	}
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  &models.Model{Events: events},
		Env:    &datasims.SimulationEnvironment{Width: 100, Height: 80},
		Agents: []*RuntimeAgent{ra},
		ByID:   map[uint64]*RuntimeAgent{ra.A.ID: ra},
		RNG:    rand.New(rand.NewSource(1)),
		stats:  newStatsAccumulator(),
	}
	return rs
}

// TestAuthorizeEmit_NilDef verifies that a nil EventDefinition is permissive
// (any producer kind and any archetype UID are authorized).
func TestAuthorizeEmit_NilDef(t *testing.T) {
	if !authorizeEmit(nil, models.EventProducerAgent, "wolf") {
		t.Fatal("nil def must be permissive (authorizeEmit returns true)")
	}
	if !authorizeEmit(nil, models.EventProducerLLM, "") {
		t.Fatal("nil def must be permissive for any kind")
	}
}

// TestAuthorizeEmit_EmptyProducers verifies that an EventDefinition with an
// empty Producers slice is permissive (no restriction declared).
func TestAuthorizeEmit_EmptyProducers(t *testing.T) {
	def := &models.EventDefinition{UID: "howl", Producers: nil}
	if !authorizeEmit(def, models.EventProducerAgent, "wolf") {
		t.Fatal("empty Producers must be permissive")
	}
	if !authorizeEmit(def, models.EventProducerLLM, "anyUID") {
		t.Fatal("empty Producers must be permissive for LLM kind too")
	}
}

// TestAuthorizeEmit_KindInProducers_NoUIDs verifies that when Producers is
// non-empty and kind is in it, and ProducerUIDs is empty, any archetype is
// authorized.
func TestAuthorizeEmit_KindInProducers_NoUIDs(t *testing.T) {
	def := &models.EventDefinition{
		UID:          "howl",
		Producers:    []models.EventProducer{models.EventProducerAgent},
		ProducerUIDs: nil, // no restriction on specific UIDs
	}
	if !authorizeEmit(def, models.EventProducerAgent, "wolf") {
		t.Fatal("kind=agent in Producers, empty ProducerUIDs => any archetype authorized")
	}
	if !authorizeEmit(def, models.EventProducerAgent, "sheep") {
		t.Fatal("kind=agent in Producers, empty ProducerUIDs => sheep also authorized")
	}
}

// TestAuthorizeEmit_KindNotInProducers verifies that when Producers is
// non-empty but kind is absent, the emit is denied regardless of archetype.
func TestAuthorizeEmit_KindNotInProducers(t *testing.T) {
	def := &models.EventDefinition{
		UID:       "howl",
		Producers: []models.EventProducer{models.EventProducerLLM},
	}
	if authorizeEmit(def, models.EventProducerAgent, "wolf") {
		t.Fatal("kind=agent not in Producers(llm) => must be denied")
	}
	if authorizeEmit(def, models.EventProducerEnvironment, "wolf") {
		t.Fatal("kind=environment not in Producers(llm) => must be denied")
	}
}

// TestAuthorizeEmit_KindInProducers_WithUIDs_Match verifies that when both
// kind and archetype UID are in the respective sets, the emit is authorized.
func TestAuthorizeEmit_KindInProducers_WithUIDs_Match(t *testing.T) {
	def := &models.EventDefinition{
		UID:          "signal",
		Producers:    []models.EventProducer{models.EventProducerAgent},
		ProducerUIDs: []string{"wolf", "sheep"},
	}
	if !authorizeEmit(def, models.EventProducerAgent, "wolf") {
		t.Fatal("kind=agent+UID=wolf both authorized")
	}
}

// TestAuthorizeEmit_KindInProducers_WithUIDs_NoMatch verifies that when kind
// is authorized but the archetype UID is not in ProducerUIDs, the emit is denied.
func TestAuthorizeEmit_KindInProducers_WithUIDs_NoMatch(t *testing.T) {
	def := &models.EventDefinition{
		UID:          "signal",
		Producers:    []models.EventProducer{models.EventProducerAgent},
		ProducerUIDs: []string{"wolf"},
	}
	if authorizeEmit(def, models.EventProducerAgent, "deer") {
		t.Fatal("kind=agent authorized but UID=deer not in ProducerUIDs => must be denied")
	}
}

// TestApplyEmitEvent_AllowedProducer verifies that when the producer kind is
// authorized, the event is queued and stats are incremented.
func TestApplyEmitEvent_AllowedProducer(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 5, Y: 5}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID:       "howl",
		Producers: []models.EventProducer{models.EventProducerAgent},
	}
	rs := emitEventRuntime(def, ra)

	evts := applyEmitEvent(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl"}, models.EventProducerAgent)
	if len(rs.events) != 1 || rs.events[0].EventUID != "howl" {
		t.Fatalf("event not queued: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("expected memory events to be returned on authorized emit")
	}
	if rs.stats.Events != 1 {
		t.Fatalf("stats.Events=%d want 1", rs.stats.Events)
	}
}

// TestApplyEmitEvent_DeniedProducerKind verifies that when the producer kind
// is not in Producers, the event is NOT queued and stats are NOT incremented.
func TestApplyEmitEvent_DeniedProducerKind(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID:       "signal",
		Producers: []models.EventProducer{models.EventProducerLLM}, // only LLM may emit
	}
	rs := emitEventRuntime(def, ra)

	evts := applyEmitEvent(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "signal"}, models.EventProducerAgent)
	if len(rs.events) != 0 {
		t.Fatalf("event must NOT be queued when kind not authorized: events=%+v", rs.events)
	}
	if evts != nil {
		t.Fatal("memory events must be nil (no emit) on denied produce")
	}
	if rs.stats.Events != 0 {
		t.Fatalf("stats.Events=%d must be 0 on denied emit", rs.stats.Events)
	}
}

// TestApplyEmitEvent_DeniedArchetypeUID verifies that when the kind is
// authorized but the archetype UID is not in ProducerUIDs, the emit is denied.
func TestApplyEmitEvent_DeniedArchetypeUID(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "deer"},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID:          "signal",
		Producers:    []models.EventProducer{models.EventProducerAgent},
		ProducerUIDs: []string{"wolf"}, // deer is not allowed
	}
	rs := emitEventRuntime(def, ra)

	evts := applyEmitEvent(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "signal"}, models.EventProducerAgent)
	if len(rs.events) != 0 {
		t.Fatalf("event must NOT be queued when archetype UID not in ProducerUIDs: events=%+v", rs.events)
	}
	if evts != nil {
		t.Fatal("memory events must be nil on UID-denied emit")
	}
	if rs.stats.Events != 0 {
		t.Fatalf("stats.Events=%d must be 0 on UID-denied emit", rs.stats.Events)
	}
}

// TestApplyEmitEvent_EmptyProducers_Permissive verifies that when the event def
// has an empty Producers list, any kind may emit (permissive, backward-compat).
func TestApplyEmitEvent_EmptyProducers_Permissive(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 1, Y: 1}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID:       "alarm",
		Producers: nil, // empty = permissive
	}
	rs := emitEventRuntime(def, ra)

	evts := applyEmitEvent(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "alarm"}, models.EventProducerEnvironment)
	if len(rs.events) != 1 {
		t.Fatalf("empty Producers should be permissive: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("expected memory events on permissive emit")
	}
}

// TestApplyEmitEvent_NilDef_Permissive verifies that when no EventDefinition
// is found (nil def), the emit proceeds (permissive).
func TestApplyEmitEvent_NilDef_Permissive(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 2, Y: 2}},
		Archetype: wolfArchetype(),
	}
	// No event definitions registered in the model.
	rs := emitEventRuntime(nil, ra)

	evts := applyEmitEvent(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "unknown"}, models.EventProducerAgent)
	if len(rs.events) != 1 {
		t.Fatalf("nil def should be permissive: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("expected memory events on nil-def permissive emit")
	}
}

// TestValidateEventPayload_MissingKey verifies that a warning is appended when
// a declared payload field is absent from the data map, but the event is not rejected.
func TestValidateEventPayload_MissingKey(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 3, Y: 3}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID: "data_event",
		Payload: []models.AgentProperty{
			{UID: "intensity", Type: models.PropertyTypeNumber},
		},
		Producers: nil, // permissive
	}
	rs := emitEventRuntime(def, ra)

	// Emit with data map missing the "intensity" key.
	a := models.Action{
		Type:      models.ActionTypeEmitEvent,
		EventUID:  "data_event",
		EventData: map[string]any{"wrong_key": 42.0},
	}
	evts := applyEmitEvent(rs, ra, a, models.EventProducerAgent)

	// Event must still be emitted (advisory only).
	if len(rs.events) != 1 {
		t.Fatalf("payload mismatch must not prevent emit: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("memory events must be returned even when payload has warnings")
	}
	// At least one warning must be present.
	if len(rs.warnings) == 0 {
		t.Fatal("missing payload key 'intensity' must produce a warning in rs.warnings")
	}
}

// TestValidateEventPayload_TypeMismatch verifies that a warning is appended
// when a declared numeric payload field contains a non-numeric value.
func TestValidateEventPayload_TypeMismatch(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 4, Y: 4}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID: "typed_event",
		Payload: []models.AgentProperty{
			{UID: "count", Type: models.PropertyTypeNumber},
		},
		Producers: nil,
	}
	rs := emitEventRuntime(def, ra)

	a := models.Action{
		Type:      models.ActionTypeEmitEvent,
		EventUID:  "typed_event",
		EventData: map[string]any{"count": "not-a-number"},
	}
	evts := applyEmitEvent(rs, ra, a, models.EventProducerAgent)

	if len(rs.events) != 1 {
		t.Fatalf("type mismatch must not prevent emit: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("memory events must be returned even on type mismatch")
	}
	if len(rs.warnings) == 0 {
		t.Fatal("type mismatch for 'count' must produce a warning")
	}
}

// TestValidateEventPayload_AllowedValuesMismatch verifies that a warning is
// produced when a field's value is not in AllowedValues.
func TestValidateEventPayload_AllowedValuesMismatch(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 5, Y: 5}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID: "enum_event",
		Payload: []models.AgentProperty{
			{UID: "priority", Type: models.PropertyTypeEnum, AllowedValues: []interface{}{"low", "medium", "high"}},
		},
		Producers: nil,
	}
	rs := emitEventRuntime(def, ra)

	a := models.Action{
		Type:      models.ActionTypeEmitEvent,
		EventUID:  "enum_event",
		EventData: map[string]any{"priority": "critical"}, // not in AllowedValues
	}
	evts := applyEmitEvent(rs, ra, a, models.EventProducerAgent)

	if len(rs.events) != 1 {
		t.Fatalf("AllowedValues mismatch must not prevent emit: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("memory events must be returned on AllowedValues mismatch")
	}
	if len(rs.warnings) == 0 {
		t.Fatal("AllowedValues mismatch must produce a warning")
	}
}

// TestValidateEventPayload_NonMapData_MultiField verifies that when the event
// data is not a map and the payload declares multiple fields, ONE "payload shape
// mismatch" warning is appended (not one per field).
func TestValidateEventPayload_NonMapData_MultiField(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 6, Y: 6}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID: "multi_field",
		Payload: []models.AgentProperty{
			{UID: "x", Type: models.PropertyTypeNumber},
			{UID: "y", Type: models.PropertyTypeNumber},
		},
		Producers: nil,
	}
	rs := emitEventRuntime(def, ra)

	a := models.Action{
		Type:      models.ActionTypeEmitEvent,
		EventUID:  "multi_field",
		EventData: "scalar_value", // not a map
	}
	evts := applyEmitEvent(rs, ra, a, models.EventProducerAgent)

	if len(rs.events) != 1 {
		t.Fatalf("non-map data must not prevent emit: events=%+v", rs.events)
	}
	if len(evts) == 0 {
		t.Fatal("memory events must be returned on shape mismatch")
	}
	// Exactly ONE warning about shape mismatch (not two, one per field).
	if len(rs.warnings) == 0 {
		t.Fatal("non-map data with multi-field payload must produce a shape mismatch warning")
	}
	if len(rs.warnings) > 1 {
		t.Fatalf("non-map data must produce exactly ONE shape mismatch warning, got %d: %v", len(rs.warnings), rs.warnings)
	}
}

// TestValidateEventPayload_ValidData_NoWarnings verifies that correct payload
// data produces no warnings.
func TestValidateEventPayload_ValidData_NoWarnings(t *testing.T) {
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 7, Y: 7}},
		Archetype: wolfArchetype(),
	}
	def := &models.EventDefinition{
		UID: "valid_event",
		Payload: []models.AgentProperty{
			{UID: "power", Type: models.PropertyTypeNumber},
		},
		Producers: nil,
	}
	rs := emitEventRuntime(def, ra)

	a := models.Action{
		Type:      models.ActionTypeEmitEvent,
		EventUID:  "valid_event",
		EventData: map[string]any{"power": 42.0},
	}
	applyEmitEvent(rs, ra, a, models.EventProducerAgent)

	if len(rs.warnings) != 0 {
		t.Fatalf("valid payload must produce no warnings, got: %v", rs.warnings)
	}
}

// TestApplyAction_EmitEvent_PassesAgentKind verifies that ApplyAction delegates
// to applyEmitEvent with EventProducerAgent kind (the existing howl test updated
// for the new signature path through ApplyAction).
func TestApplyAction_EmitEvent_PassesAgentKind(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, Position: &common.Point{X: 5, Y: 5}}, Archetype: wolfArchetype()}
	rs := ruleRuntime([]*RuntimeAgent{ra}, nil)
	rs.Model.Events = []models.EventDefinition{{UID: "howl"}}
	rs.stats = newStatsAccumulator()
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl"})
	if len(rs.events) != 1 || rs.events[0].EventUID != "howl" {
		t.Fatalf("events=%+v want one howl event queued via ApplyAction", rs.events)
	}
	if rs.stats.Events != 1 {
		t.Fatalf("stats.Events=%d want 1 after ApplyAction emit", rs.stats.Events)
	}
}

// containsSub is a small inline helper for test assertions (avoids importing strings).
func containsSub(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Task 17: applyEmitEvent with EventProducerLLM + consumer-reaches tests --------

// llmEmitRuntime builds a RuntimeState wired for LLM-emit tests: a "howl" event
// that allows EventProducerLLM (no UID restriction), a global-scope receiver (sheep),
// and a producer wolf agent.
func llmEmitRuntime() (*RuntimeState, *RuntimeAgent, *RuntimeAgent) {
	wolfArch := &models.Agent{UID: "wolf", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable}
	sheepArch := &models.Agent{UID: "sheep", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeMovable}
	wolf := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}},
		Archetype: wolfArch,
	}
	sheep := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 50, Y: 50}},
		Archetype: sheepArch,
	}
	def := models.EventDefinition{
		UID:       "howl",
		Name:      "Howl",
		Scope:     models.EventScopeGlobal,
		Producers: []models.EventProducer{models.EventProducerLLM},
		TTLSteps:  0, // active only on emit step
	}
	idx := NewSpatialIndex(100, 80, 10, false)
	idx.Insert(wolf.A.ID, *wolf.A.Position)
	idx.Insert(sheep.A.ID, *sheep.A.Position)
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  &models.Model{Events: []models.EventDefinition{def}},
		Env:    &datasims.SimulationEnvironment{Width: 100, Height: 80},
		Agents: []*RuntimeAgent{wolf, sheep},
		ByID:   map[uint64]*RuntimeAgent{1: wolf, 2: sheep},
		Index:  idx,
		RNG:    rand.New(rand.NewSource(1)),
		stats:  newStatsAccumulator(),
	}
	return rs, wolf, sheep
}

// TestApplyEmitEvent_LLMProducer_QueuesAndStats verifies that applyEmitEvent with
// EventProducerLLM queues the event into rs.events and increments stats, identical
// to the agent-producer path — the only difference is the kind argument.
func TestApplyEmitEvent_LLMProducer_QueuesAndStats(t *testing.T) {
	rs, wolf, _ := llmEmitRuntime()

	evts := applyEmitEvent(rs, wolf,
		models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl", EventData: "pack_call"},
		models.EventProducerLLM,
	)
	if len(rs.events) != 1 {
		t.Fatalf("expected 1 event queued, got %d: %+v", len(rs.events), rs.events)
	}
	ev := rs.events[0]
	if ev.EventUID != "howl" {
		t.Fatalf("event uid=%q want howl", ev.EventUID)
	}
	if ev.Data != "pack_call" {
		t.Fatalf("event data=%v want 'pack_call'", ev.Data)
	}
	if ev.Source != wolf.A.ID {
		t.Fatalf("event source=%d want %d (emitting agent)", ev.Source, wolf.A.ID)
	}
	if len(evts) == 0 {
		t.Fatal("expected memory events returned on authorized LLM emit")
	}
	if rs.stats.Events != 1 {
		t.Fatalf("stats.Events=%d want 1 after LLM emit", rs.stats.Events)
	}
	if rs.stats.ByEvent["howl"] != 1 {
		t.Fatalf("stats.ByEvent[howl]=%d want 1", rs.stats.ByEvent["howl"])
	}
}

// TestApplyEmitEvent_LLMProducer_DeniedAgentOnlyEvent verifies that
// EventProducerLLM cannot emit an event whose Producers only includes
// EventProducerAgent (defense-in-depth check, complementary to decision_test.go).
func TestApplyEmitEvent_LLMProducer_DeniedAgentOnlyEvent(t *testing.T) {
	rs, wolf, _ := llmEmitRuntime()
	// Swap the def to agent-only.
	rs.Model.Events[0].Producers = []models.EventProducer{models.EventProducerAgent}

	evts := applyEmitEvent(rs, wolf,
		models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl"},
		models.EventProducerLLM,
	)
	if len(rs.events) != 0 {
		t.Fatalf("LLM must not emit agent-only event; got events=%+v", rs.events)
	}
	if evts != nil {
		t.Fatal("memory events must be nil on denied LLM emit")
	}
	if rs.stats.Events != 0 {
		t.Fatalf("stats.Events=%d want 0 on denied LLM emit", rs.stats.Events)
	}
}

// TestLLMEmit_ConsumerReaches_GlobalScope verifies the end-to-end path:
// an LLM-emitted event (EventProducerLLM) with global scope is perceivable by a
// consumer (evalEvent returns true for the consumer after the emit).
func TestLLMEmit_ConsumerReaches_GlobalScope(t *testing.T) {
	rs, wolf, sheep := llmEmitRuntime()
	rs.Step = 3

	// Emit via applyEmitEvent with EventProducerLLM.
	applyEmitEvent(rs, wolf,
		models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl"},
		models.EventProducerLLM,
	)
	if len(rs.events) != 1 {
		t.Fatalf("expected 1 event queued, got %d", len(rs.events))
	}

	// Consumer (sheep) should perceive the event this step (TTL=0, step==emitStep).
	c := models.Condition{Type: models.ConditionTypeEvent, TargetUID: "howl"}
	if !EvalCondition(rs, sheep, nil, c) {
		t.Fatal("sheep should perceive the LLM-emitted 'howl' event this step (global scope, TTL=0)")
	}
}

// --- Task 18: evalEnvCondition + runEnvironmentEvents tests -----------------------

// envEventRuntime builds a RuntimeState for environment-event tests: a global-scope
// event "alarm" that allows EventProducerEnvironment, a single consumer "wolf" agent,
// and optional global properties in the environment.
func envEventRuntime(globalProps map[string]any, rules []models.Rule) (*RuntimeState, *RuntimeAgent) {
	wolfArch := &models.Agent{UID: "wolf", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeStatic}
	wolf := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"},
		Archetype: wolfArch,
	}
	def := models.EventDefinition{
		UID:       "alarm",
		Scope:     models.EventScopeGlobal,
		Producers: []models.EventProducer{models.EventProducerEnvironment},
		TTLSteps:  1, // perceivable this step + next
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{},
		Model: &models.Model{Events: []models.EventDefinition{def}, Rules: rules},
		Env: &datasims.SimulationEnvironment{
			Width:            100,
			Height:           80,
			GlobalProperties: globalProps,
		},
		Agents: []*RuntimeAgent{wolf},
		ByID:   map[uint64]*RuntimeAgent{1: wolf},
		RNG:    rand.New(rand.NewSource(1)),
		stats:  newStatsAccumulator(),
		rules:  rules,
	}
	return rs, wolf
}

// TestEvalEnvCondition_GlobalProperty verifies that evalEnvCondition handles
// global_property conditions correctly.
func TestEvalEnvCondition_GlobalProperty(t *testing.T) {
	globalProps := map[string]any{"temperature": 42.0}
	rs, _ := envEventRuntime(globalProps, nil)

	cTrue := models.Condition{
		Type:        models.ConditionTypeGlobalProperty,
		PropertyUID: "temperature",
		Operator:    models.ConditionOpGreaterThan,
		Value:       30.0,
	}
	if !evalEnvCondition(rs, cTrue) {
		t.Fatal("global_property temperature 42 > 30 should be true for environment")
	}

	cFalse := models.Condition{
		Type:        models.ConditionTypeGlobalProperty,
		PropertyUID: "temperature",
		Operator:    models.ConditionOpGreaterThan,
		Value:       50.0,
	}
	if evalEnvCondition(rs, cFalse) {
		t.Fatal("global_property temperature 42 > 50 should be false for environment")
	}
}

// TestEvalEnvCondition_Step verifies that evalEnvCondition handles step conditions.
func TestEvalEnvCondition_Step(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)
	rs.Step = 5

	cTrue := models.Condition{
		Type:     models.ConditionTypeStep,
		Operator: models.ConditionOpGreaterOrEq,
		Value:    3.0,
	}
	if !evalEnvCondition(rs, cTrue) {
		t.Fatal("step 5 >= 3 should be true for environment")
	}

	cFalse := models.Condition{
		Type:     models.ConditionTypeStep,
		Operator: models.ConditionOpGreaterThan,
		Value:    10.0,
	}
	if evalEnvCondition(rs, cFalse) {
		t.Fatal("step 5 > 10 should be false for environment")
	}
}

// TestEvalEnvCondition_Random verifies that evalEnvCondition handles random conditions
// with probability 1.0 (always) and 0.0 (never).
func TestEvalEnvCondition_Random(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)

	always := models.Condition{Type: models.ConditionTypeRandom, Value: 1.0}
	never := models.Condition{Type: models.ConditionTypeRandom, Value: 0.0}

	if !evalEnvCondition(rs, always) {
		t.Fatal("random probability 1.0 must always be true for environment")
	}
	if evalEnvCondition(rs, never) {
		t.Fatal("random probability 0.0 must always be false for environment")
	}
}

// TestEvalEnvCondition_AgentPositional_ReturnsFalse verifies that agent-positional
// conditions (property, position, proximity, state, distance, count, barrier) return
// false when evaluated against the environment (which has no position/properties).
func TestEvalEnvCondition_AgentPositional_ReturnsFalse(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)

	agentConditions := []models.Condition{
		{Type: models.ConditionTypeProperty, PropertyUID: "energy", Operator: models.ConditionOpGreaterThan, Value: 0.0},
		{Type: models.ConditionTypePosition},
		{Type: models.ConditionTypeProximity, Radius: fptr(10)},
		{Type: models.ConditionTypeState, Operator: models.ConditionOpEquals, Value: "idle"},
		{Type: models.ConditionTypeDistance},
		{Type: models.ConditionTypeCount, Radius: fptr(10)},
		{Type: models.ConditionTypeBarrier},
	}

	for _, c := range agentConditions {
		if evalEnvCondition(rs, c) {
			t.Fatalf("agent-positional condition type %q must return false for environment, got true", c.Type)
		}
	}
}

// TestEvalEnvCondition_Composite_AND verifies composite AND over environment-valid conditions.
func TestEvalEnvCondition_Composite_AND(t *testing.T) {
	globalProps := map[string]any{"heat": 80.0}
	rs, _ := envEventRuntime(globalProps, nil)
	rs.Step = 5

	c := models.Condition{
		Type:          models.ConditionTypeComposite,
		LogicOperator: models.LogicOpAND,
		SubConditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "heat", Operator: models.ConditionOpGreaterThan, Value: 50.0}, // true
			{Type: models.ConditionTypeStep, Operator: models.ConditionOpGreaterOrEq, Value: 3.0},                                 // true
		},
	}
	if !evalEnvCondition(rs, c) {
		t.Fatal("AND of (heat>50 AND step>=3) should be true for environment")
	}

	// AND with an agent-positional condition (position) — must be false because
	// agent-positional returns false.
	cWithPositional := models.Condition{
		Type:          models.ConditionTypeComposite,
		LogicOperator: models.LogicOpAND,
		SubConditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "heat", Operator: models.ConditionOpGreaterThan, Value: 50.0},
			{Type: models.ConditionTypePosition}, // always false for environment
		},
	}
	if evalEnvCondition(rs, cWithPositional) {
		t.Fatal("AND with a positional sub-condition must be false for environment")
	}
}

// TestEvalEnvCondition_Composite_NOT verifies composite NOT over an environment condition.
func TestEvalEnvCondition_Composite_NOT(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)

	// NOT(random=1.0) => NOT(true) => false.
	c := models.Condition{
		Type:          models.ConditionTypeComposite,
		LogicOperator: models.LogicOpNOT,
		SubConditions: []models.Condition{
			{Type: models.ConditionTypeRandom, Value: 1.0},
		},
	}
	if evalEnvCondition(rs, c) {
		t.Fatal("NOT(random=1.0) must be false for environment")
	}
}

// --- ISS-11 engine half: environment-scoped rules can consume events ---------

// TestEvalEnvCondition_Event_FiresOnActiveEvent verifies that evalEnvCondition
// now supports event conditions: an env-scoped rule sees a live global event.
func TestEvalEnvCondition_Event_FiresOnActiveEvent(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)
	rs.Step = 1
	rs.events = []runtimeEvent{{EventUID: "alarm", EmitStep: 1}}

	c := models.Condition{Type: models.ConditionTypeEvent, TargetUID: "alarm"}
	if !evalEnvCondition(rs, c) {
		t.Fatal("event condition must fire when a matching active event exists")
	}
}

// TestEvalEnvCondition_Event_FallsBackToPropertyUID verifies the TargetUID ->
// PropertyUID fallback for resolving the event uid, matching evalEvent.
func TestEvalEnvCondition_Event_FallsBackToPropertyUID(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)
	rs.Step = 1
	rs.events = []runtimeEvent{{EventUID: "alarm", EmitStep: 1}}

	c := models.Condition{Type: models.ConditionTypeEvent, PropertyUID: "alarm"}
	if !evalEnvCondition(rs, c) {
		t.Fatal("event condition must resolve the event uid via property_uid when target_uid is empty")
	}
}

// TestEvalEnvCondition_Event_AbsentOrExpired verifies false when no matching
// event exists, and false again once the event's TTL has elapsed.
func TestEvalEnvCondition_Event_AbsentOrExpired(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)
	c := models.Condition{Type: models.ConditionTypeEvent, TargetUID: "alarm"}

	rs.Step = 1
	if evalEnvCondition(rs, c) {
		t.Fatal("event condition must be false when no matching event exists")
	}

	// def.TTLSteps == 1 (envEventRuntime): perceivable at EmitStep and EmitStep+1 only.
	rs.events = []runtimeEvent{{EventUID: "alarm", EmitStep: 1}}
	rs.Step = 3
	if evalEnvCondition(rs, c) {
		t.Fatal("event condition must be false once the event's TTL has elapsed")
	}
}

// TestEvalEnvCondition_Event_PayloadCompare verifies the optional Value-vs-Data
// payload compare (matchEvent), mirroring evalEvent's behaviour.
func TestEvalEnvCondition_Event_PayloadCompare(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)
	rs.Step = 1
	rs.events = []runtimeEvent{{EventUID: "alarm", EmitStep: 1, Data: 42.0}}

	match := models.Condition{Type: models.ConditionTypeEvent, TargetUID: "alarm", Operator: models.ConditionOpGreaterThan, Value: 30.0}
	if !evalEnvCondition(rs, match) {
		t.Fatal("event condition with a satisfied payload compare must fire")
	}

	noMatch := models.Condition{Type: models.ConditionTypeEvent, TargetUID: "alarm", Operator: models.ConditionOpGreaterThan, Value: 100.0}
	if evalEnvCondition(rs, noMatch) {
		t.Fatal("event condition with an unsatisfied payload compare must not fire")
	}
}

// TestEvalEnvCondition_Event_BypassesDeliveryScope verifies the environment
// (a positionless global observer) sees local- and targeted-scope events that
// would be dropped by eventReaches for a positioned agent consumer — env rules
// deliberately bypass that gate (only TTL-liveness + payload compare apply).
func TestEvalEnvCondition_Event_BypassesDeliveryScope(t *testing.T) {
	rs, _ := envEventRuntime(nil, nil)
	rs.Model.Events = append(rs.Model.Events,
		models.EventDefinition{UID: "whisper", Scope: models.EventScopeLocal, Radius: fptr(5)},
		models.EventDefinition{UID: "secret", Scope: models.EventScopeTargeted, ConsumerUIDs: []string{"someone-else"}},
	)
	rs.Step = 1
	rs.events = []runtimeEvent{
		{EventUID: "whisper", EmitStep: 1, Position: &common.Point{X: 999, Y: 999}}, // far away
		{EventUID: "secret", EmitStep: 1},                                          // targeted at someone else
	}

	if !evalEnvCondition(rs, models.Condition{Type: models.ConditionTypeEvent, TargetUID: "whisper"}) {
		t.Fatal("environment must see a local-scope event regardless of distance (it has no position)")
	}
	if !evalEnvCondition(rs, models.Condition{Type: models.ConditionTypeEvent, TargetUID: "secret"}) {
		t.Fatal("environment must see a targeted-scope event even when it is not in ConsumerUIDs")
	}
}

// TestEvalEnvCondition_Composite_Event verifies that a composite condition
// recurses into an event sub-condition (evalEnvComposite -> evalEnvCondition).
func TestEvalEnvCondition_Composite_Event(t *testing.T) {
	globalProps := map[string]any{"heat": 80.0}
	rs, _ := envEventRuntime(globalProps, nil)
	rs.Step = 1
	rs.events = []runtimeEvent{{EventUID: "alarm", EmitStep: 1}}

	c := models.Condition{
		Type:          models.ConditionTypeComposite,
		LogicOperator: models.LogicOpAND,
		SubConditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "heat", Operator: models.ConditionOpGreaterThan, Value: 50.0},
			{Type: models.ConditionTypeEvent, TargetUID: "alarm"},
		},
	}
	if !evalEnvCondition(rs, c) {
		t.Fatal("composite AND with a satisfied event sub-condition must fire")
	}

	// Remove the event: the composite must now be false.
	rs.events = nil
	if evalEnvCondition(rs, c) {
		t.Fatal("composite AND must be false once the event sub-condition no longer matches")
	}
}

// TestRunEnvironmentEvents_EmitsWhenConditionTrue verifies that runEnvironmentEvents
// emits an event when an env-scoped rule's condition passes.
func TestRunEnvironmentEvents_EmitsWhenConditionTrue(t *testing.T) {
	globalProps := map[string]any{"temperature": 42.0}
	rule := models.Rule{
		ID:        "env-rule",
		Name:      "env-rule",
		Enabled:   true,
		Priority:  10,
		AppliesTo: []string{models.EnvironmentUID},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpGreaterThan, Value: 30.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeEmitEvent, EventUID: "alarm"},
		},
	}
	rs, _ := envEventRuntime(globalProps, []models.Rule{rule})
	rs.Step = 1

	if len(rs.events) != 0 {
		t.Fatalf("precondition: events must be empty before runEnvironmentEvents, got %d", len(rs.events))
	}

	runEnvironmentRules(rs)

	if len(rs.events) != 1 {
		t.Fatalf("expected 1 event emitted, got %d: %+v", len(rs.events), rs.events)
	}
	if rs.events[0].EventUID != "alarm" {
		t.Fatalf("event uid=%q want alarm", rs.events[0].EventUID)
	}
	// Source must be 0 (synthetic environment agent).
	if rs.events[0].Source != 0 {
		t.Fatalf("event source=%d want 0 (environment)", rs.events[0].Source)
	}
	// Position must be nil (environment has no position).
	if rs.events[0].Position != nil {
		t.Fatalf("event position must be nil (environment), got %+v", rs.events[0].Position)
	}
	// Stats must be incremented.
	if rs.stats.Events != 1 {
		t.Fatalf("stats.Events=%d want 1 after env emit", rs.stats.Events)
	}
}

// TestRunEnvironmentEvents_NoEmitWhenConditionFalse verifies that runEnvironmentEvents
// does not emit when the rule's condition is false.
func TestRunEnvironmentEvents_NoEmitWhenConditionFalse(t *testing.T) {
	globalProps := map[string]any{"temperature": 10.0}
	rule := models.Rule{
		ID:        "env-rule",
		Enabled:   true,
		Priority:  10,
		AppliesTo: []string{models.EnvironmentUID},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpGreaterThan, Value: 30.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeEmitEvent, EventUID: "alarm"},
		},
	}
	rs, _ := envEventRuntime(globalProps, []models.Rule{rule})

	runEnvironmentRules(rs)

	if len(rs.events) != 0 {
		t.Fatalf("expected no events when condition is false, got %d", len(rs.events))
	}
}

// TestRunEnvironmentEvents_ProducerGated verifies that runEnvironmentEvents does NOT
// emit when the event definition does NOT include EventProducerEnvironment in Producers.
func TestRunEnvironmentEvents_ProducerGated(t *testing.T) {
	globalProps := map[string]any{"temperature": 42.0}
	rule := models.Rule{
		ID:        "env-rule",
		Enabled:   true,
		Priority:  10,
		AppliesTo: []string{models.EnvironmentUID},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpGreaterThan, Value: 30.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeEmitEvent, EventUID: "alarm"},
		},
	}
	rs, _ := envEventRuntime(globalProps, []models.Rule{rule})
	// Replace the event definition so it only allows agent producers (not environment).
	rs.Model.Events[0].Producers = []models.EventProducer{models.EventProducerAgent}

	runEnvironmentRules(rs)

	if len(rs.events) != 0 {
		t.Fatalf("environment must not emit when not in Producers; got %d events", len(rs.events))
	}
	if rs.stats.Events != 0 {
		t.Fatalf("stats.Events=%d must be 0 when producer gated", rs.stats.Events)
	}
}

// TestRunEnvironmentEvents_SkipsDisabledRule verifies that disabled rules are skipped.
func TestRunEnvironmentEvents_SkipsDisabledRule(t *testing.T) {
	globalProps := map[string]any{"temperature": 42.0}
	rule := models.Rule{
		ID:        "env-disabled",
		Enabled:   false, // disabled — must be skipped
		Priority:  10,
		AppliesTo: []string{models.EnvironmentUID},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpGreaterThan, Value: 30.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeEmitEvent, EventUID: "alarm"},
		},
	}
	rs, _ := envEventRuntime(globalProps, []models.Rule{rule})

	runEnvironmentRules(rs)

	if len(rs.events) != 0 {
		t.Fatalf("disabled env rule must not emit: got %d events", len(rs.events))
	}
}

// TestRunEnvironmentEvents_SkipsNonEmitActions verifies that non-emit_event actions
// in environment rules are skipped (e.g., set_property has no effect).
func TestRunEnvironmentEvents_SkipsNonEmitActions(t *testing.T) {
	globalProps := map[string]any{"temperature": 42.0}
	rule := models.Rule{
		ID:        "env-rule",
		Enabled:   true,
		Priority:  10,
		AppliesTo: []string{models.EnvironmentUID},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpGreaterThan, Value: 30.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeSetProperty, PropertyUID: "temperature", Value: 0.0}, // skipped
			{Type: models.ActionTypeEmitEvent, EventUID: "alarm"},                        // emitted
		},
	}
	rs, _ := envEventRuntime(globalProps, []models.Rule{rule})

	runEnvironmentRules(rs)

	// Only the emit_event action matters; the set_property is skipped but doesn't block emit.
	if len(rs.events) != 1 {
		t.Fatalf("expected 1 event from emit_event action, got %d", len(rs.events))
	}
	// The global property must NOT have been changed by the set_property action.
	if temp, ok := rs.Env.GlobalProperties["temperature"]; ok {
		if temp != 42.0 {
			t.Fatalf("set_property in env rule must be skipped; temperature changed to %v", temp)
		}
	}
}

// TestRunEnvironmentEvents_SkipsAgentScopedRules verifies that rules whose
// AppliesTo does NOT include EnvironmentUID are not evaluated by runEnvironmentEvents.
func TestRunEnvironmentEvents_SkipsAgentScopedRules(t *testing.T) {
	globalProps := map[string]any{"temperature": 42.0}
	// Rule targets "wolf" only, not "environment".
	rule := models.Rule{
		ID:        "wolf-rule",
		Enabled:   true,
		Priority:  10,
		AppliesTo: []string{"wolf"},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpGreaterThan, Value: 30.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeEmitEvent, EventUID: "alarm"},
		},
	}
	rs, _ := envEventRuntime(globalProps, []models.Rule{rule})

	runEnvironmentRules(rs)

	if len(rs.events) != 0 {
		t.Fatalf("agent-scoped rule must not be evaluated by runEnvironmentEvents; got %d events", len(rs.events))
	}
}

// TestLLMEmit_ConsumerReaches_LocalScope verifies that a local-scope LLM-emitted
// event is perceivable within Radius but not outside.
func TestLLMEmit_ConsumerReaches_LocalScope(t *testing.T) {
	wolfArch := &models.Agent{UID: "wolf", Type: models.AgentTypeArchetype}
	sheepArch := &models.Agent{UID: "sheep", Type: models.AgentTypeArchetype}
	radius := 20.0
	wolf := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}},
		Archetype: wolfArch,
	}
	// Near sheep: within radius 20 of wolf.
	sheepNear := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 20, Y: 10}},
		Archetype: sheepArch,
	}
	// Far sheep: outside radius 20.
	sheepFar := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 3, AgentUID: "sheep", Position: &common.Point{X: 80, Y: 70}},
		Archetype: sheepArch,
	}
	def := models.EventDefinition{
		UID:       "howl",
		Scope:     models.EventScopeLocal,
		Radius:    &radius,
		Producers: []models.EventProducer{models.EventProducerLLM},
	}
	idx := NewSpatialIndex(100, 80, 10, false)
	idx.Insert(wolf.A.ID, *wolf.A.Position)
	idx.Insert(sheepNear.A.ID, *sheepNear.A.Position)
	idx.Insert(sheepFar.A.ID, *sheepFar.A.Position)
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  &models.Model{Events: []models.EventDefinition{def}},
		Env:    &datasims.SimulationEnvironment{Width: 100, Height: 80},
		Agents: []*RuntimeAgent{wolf, sheepNear, sheepFar},
		ByID:   map[uint64]*RuntimeAgent{1: wolf, 2: sheepNear, 3: sheepFar},
		Index:  idx,
		RNG:    rand.New(rand.NewSource(1)),
		stats:  newStatsAccumulator(),
	}
	rs.Step = 1

	applyEmitEvent(rs, wolf,
		models.Action{Type: models.ActionTypeEmitEvent, EventUID: "howl"},
		models.EventProducerLLM,
	)

	c := models.Condition{Type: models.ConditionTypeEvent, TargetUID: "howl"}
	if !EvalCondition(rs, sheepNear, nil, c) {
		t.Fatal("near sheep (within radius 20) must perceive local-scope LLM howl")
	}
	if EvalCondition(rs, sheepFar, nil, c) {
		t.Fatal("far sheep (outside radius 20) must NOT perceive local-scope LLM howl")
	}
}
