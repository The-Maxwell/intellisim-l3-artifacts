package simulations

import (
	"context"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// findRule returns the RuleStatistic for ruleID, or nil.
func findRule(rules []datasims.RuleStatistic, ruleID string) *datasims.RuleStatistic {
	for i := range rules {
		if rules[i].RuleID == ruleID {
			return &rules[i]
		}
	}
	return nil
}

// findEvent returns the EventStatistic for eventUID, or nil.
func findEvent(evs []datasims.EventStatistic, eventUID string) *datasims.EventStatistic {
	for i := range evs {
		if evs[i].EventUID == eventUID {
			return &evs[i]
		}
	}
	return nil
}

// findInteraction returns the InteractionStatistic for interactionID, or nil.
func findInteraction(ints []datasims.InteractionStatistic, id string) *datasims.InteractionStatistic {
	for i := range ints {
		if ints[i].InteractionID == id {
			return &ints[i]
		}
	}
	return nil
}

// TestComputeStatistics_MergesAccumulator verifies that ComputeStatistics merges
// rs.stats counters into the output:
//   - flat counters (AgentsSpawned, AgentsRemoved, Interactions, Events,
//     RuleExecutions, MovementDistance)
//   - Population Peak/Min/Spawned/Removed for live archetypes
//   - a zero-Current row appended for an extinct archetype (present in PopSpawned
//     but no longer in rs.Agents)
//   - ByRule/ByEvent/ByInteraction slices are key-sorted
func TestComputeStatistics_MergesAccumulator(t *testing.T) {
	wolf := wolfArchetype()
	// Two live wolf agents, one extinct "ghost" archetype.
	agents := []*RuntimeAgent{
		{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", State: "idle", Properties: map[string]any{"energy": 50.0}}, Archetype: wolf},
		{A: &datasims.SimulationAgent{ID: 2, AgentUID: "wolf", State: "hunting", Properties: map[string]any{"energy": 80.0}}, Archetype: wolf},
	}
	rs := ruleRuntime(agents, nil)
	rs.Step = 10

	// Wire up a stats accumulator with known values.
	rs.stats = &statsAccumulator{
		AgentsSpawned:    5,
		AgentsRemoved:    3,
		Interactions:     7,
		Events:           2,
		RuleExecutions:   11,
		MovementDistance: 42.5,
		ByRule:           map[string]uint64{"rule_b": 4, "rule_a": 6},
		ByEvent:          map[string]uint64{"evt_x": 2},
		ByInteraction:    map[string]uint64{"inter_1": 7},
		PopSpawned:       map[string]uint64{"wolf": 5, "ghost": 3},
		PopRemoved:       map[string]uint64{"wolf": 3, "ghost": 3},
		PopPeak:          map[string]uint64{"wolf": 10, "ghost": 3},
		PopMin:           map[string]uint64{"wolf": 1, "ghost": 0},
		popMinSeen:       map[string]bool{"wolf": true, "ghost": true},
	}

	st := ComputeStatistics(rs)
	if st == nil {
		t.Fatal("ComputeStatistics returned nil")
	}

	// Flat counters must be merged.
	if st.AgentsSpawned != 5 {
		t.Errorf("AgentsSpawned=%d want 5", st.AgentsSpawned)
	}
	if st.AgentsRemoved != 3 {
		t.Errorf("AgentsRemoved=%d want 3", st.AgentsRemoved)
	}
	if st.Interactions != 7 {
		t.Errorf("Interactions=%d want 7", st.Interactions)
	}
	if st.Events != 2 {
		t.Errorf("Events=%d want 2", st.Events)
	}
	if st.RuleExecutions != 11 {
		t.Errorf("RuleExecutions=%d want 11", st.RuleExecutions)
	}
	if st.MovementDistance != 42.5 {
		t.Errorf("MovementDistance=%v want 42.5", st.MovementDistance)
	}

	// Wolf population row must carry Peak/Min/Spawned/Removed from stats.
	wolfPop := findPopulation(st.Population, "wolf")
	if wolfPop == nil {
		t.Fatal("missing wolf population entry")
	}
	if wolfPop.Peak != 10 {
		t.Errorf("wolf Peak=%d want 10", wolfPop.Peak)
	}
	if wolfPop.Min != 1 {
		t.Errorf("wolf Min=%d want 1", wolfPop.Min)
	}
	if wolfPop.Spawned != 5 {
		t.Errorf("wolf Spawned=%d want 5", wolfPop.Spawned)
	}
	if wolfPop.Removed != 3 {
		t.Errorf("wolf Removed=%d want 3", wolfPop.Removed)
	}

	// Ghost archetype (extinct: in PopSpawned but no live agents) must appear with Current=0.
	ghostPop := findPopulation(st.Population, "ghost")
	if ghostPop == nil {
		t.Fatal("missing ghost population entry (extinct archetype must be appended)")
	}
	if ghostPop.Current != 0 {
		t.Errorf("ghost Current=%d want 0 (extinct)", ghostPop.Current)
	}
	if ghostPop.Peak != 3 {
		t.Errorf("ghost Peak=%d want 3", ghostPop.Peak)
	}
	if ghostPop.Spawned != 3 {
		t.Errorf("ghost Spawned=%d want 3", ghostPop.Spawned)
	}
	if ghostPop.Removed != 3 {
		t.Errorf("ghost Removed=%d want 3", ghostPop.Removed)
	}

	// ByRule must be key-sorted (rule_a before rule_b).
	if len(st.ByRule) != 2 {
		t.Fatalf("ByRule len=%d want 2", len(st.ByRule))
	}
	if st.ByRule[0].RuleID != "rule_a" || st.ByRule[1].RuleID != "rule_b" {
		t.Errorf("ByRule order=%v want [rule_a rule_b]", []string{st.ByRule[0].RuleID, st.ByRule[1].RuleID})
	}
	ruleA := findRule(st.ByRule, "rule_a")
	if ruleA == nil || ruleA.Executions != 6 {
		t.Errorf("rule_a Executions=%v want 6", ruleA)
	}
	ruleB := findRule(st.ByRule, "rule_b")
	if ruleB == nil || ruleB.Executions != 4 {
		t.Errorf("rule_b Executions=%v want 4", ruleB)
	}

	// ByEvent must be present and sorted.
	if len(st.ByEvent) != 1 {
		t.Fatalf("ByEvent len=%d want 1", len(st.ByEvent))
	}
	evX := findEvent(st.ByEvent, "evt_x")
	if evX == nil || evX.Emitted != 2 {
		t.Errorf("evt_x Emitted=%v want 2", evX)
	}

	// ByInteraction must be present and sorted.
	if len(st.ByInteraction) != 1 {
		t.Fatalf("ByInteraction len=%d want 1", len(st.ByInteraction))
	}
	inter1 := findInteraction(st.ByInteraction, "inter_1")
	if inter1 == nil || inter1.Count != 7 {
		t.Errorf("inter_1 Count=%v want 7", inter1)
	}
}

// TestComputeStatistics_NilAccumulator verifies backward-compatibility: when
// rs.stats is nil the accumulating counters all stay zero and no ByRule/ByEvent/
// ByInteraction slices are emitted.
func TestComputeStatistics_NilAccumulator(t *testing.T) {
	wolf := wolfArchetype()
	agents := []*RuntimeAgent{
		{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", State: "idle"}, Archetype: wolf},
	}
	rs := ruleRuntime(agents, nil)
	rs.Step = 5
	rs.stats = nil // explicit nil

	st := ComputeStatistics(rs)
	if st == nil {
		t.Fatal("ComputeStatistics returned nil")
	}
	if st.AgentsSpawned != 0 {
		t.Errorf("AgentsSpawned=%d want 0 with nil accumulator", st.AgentsSpawned)
	}
	if st.AgentsRemoved != 0 {
		t.Errorf("AgentsRemoved=%d want 0 with nil accumulator", st.AgentsRemoved)
	}
	if st.Interactions != 0 {
		t.Errorf("Interactions=%d want 0 with nil accumulator", st.Interactions)
	}
	if st.Events != 0 {
		t.Errorf("Events=%d want 0 with nil accumulator", st.Events)
	}
	if st.RuleExecutions != 0 {
		t.Errorf("RuleExecutions=%d want 0 with nil accumulator", st.RuleExecutions)
	}
	if st.MovementDistance != 0 {
		t.Errorf("MovementDistance=%v want 0 with nil accumulator", st.MovementDistance)
	}
	if len(st.ByRule) != 0 {
		t.Errorf("ByRule=%v want empty with nil accumulator", st.ByRule)
	}
	if len(st.ByEvent) != 0 {
		t.Errorf("ByEvent=%v want empty with nil accumulator", st.ByEvent)
	}
	if len(st.ByInteraction) != 0 {
		t.Errorf("ByInteraction=%v want empty with nil accumulator", st.ByInteraction)
	}
	// Population still reflects live agents (existing snapshot behavior unchanged).
	wolfPop := findPopulation(st.Population, "wolf")
	if wolfPop == nil || wolfPop.Current != 1 {
		t.Errorf("wolf Current=%v want 1 (snapshot still works without accumulator)", wolfPop)
	}
	// Peak/Min/Spawned/Removed remain zero (no accumulator to source them from).
	if wolfPop.Peak != 0 || wolfPop.Min != 0 || wolfPop.Spawned != 0 || wolfPop.Removed != 0 {
		t.Errorf("wolf history fields non-zero without accumulator: Peak=%d Min=%d Spawned=%d Removed=%d",
			wolfPop.Peak, wolfPop.Min, wolfPop.Spawned, wolfPop.Removed)
	}
}

// findPopulation returns the AgentPopulation entry for archetype uid, or nil.
func findPopulation(pops []datasims.AgentPopulation, uid string) *datasims.AgentPopulation {
	for i := range pops {
		if pops[i].AgentUID == uid {
			return &pops[i]
		}
	}
	return nil
}

// findPropertyStat returns the PropertyStatistic for (agentUID, propertyUID), or nil.
func findPropertyStat(props []datasims.PropertyStatistic, agentUID, propertyUID string) *datasims.PropertyStatistic {
	for i := range props {
		if props[i].AgentUID == agentUID && props[i].PropertyUID == propertyUID {
			return &props[i]
		}
	}
	return nil
}

func TestComputeStatistics_PopulationAndStates(t *testing.T) {
	wolf := wolfArchetype()
	sheep := sheepArchetype()
	agents := []*RuntimeAgent{
		{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", State: "hunting", Properties: map[string]any{"energy": 40.0}}, Archetype: wolf},
		{A: &datasims.SimulationAgent{ID: 2, AgentUID: "wolf", State: "hunting", Properties: map[string]any{"energy": 60.0}}, Archetype: wolf},
		{A: &datasims.SimulationAgent{ID: 3, AgentUID: "wolf", State: "idle", Properties: map[string]any{"energy": 20.0}}, Archetype: wolf},
		{A: &datasims.SimulationAgent{ID: 4, AgentUID: "sheep", State: "grazing", Properties: map[string]any{"energy": 50.0}}, Archetype: sheep},
		{A: &datasims.SimulationAgent{ID: 5, AgentUID: "sheep", State: "grazing", Properties: map[string]any{"energy": 50.0}}, Archetype: sheep},
		{A: &datasims.SimulationAgent{ID: 6, AgentUID: "sheep", State: "grazing", Properties: map[string]any{"energy": 50.0}}, Archetype: sheep},
		{A: &datasims.SimulationAgent{ID: 7, AgentUID: "sheep", State: "grazing", Properties: map[string]any{"energy": 50.0}}, Archetype: sheep},
		{A: &datasims.SimulationAgent{ID: 8, AgentUID: "sheep", State: "fleeing", Properties: map[string]any{"energy": 50.0}}, Archetype: sheep},
	}
	rs := ruleRuntime(agents, nil)
	rs.Step = 7

	st := ComputeStatistics(rs)
	if st == nil {
		t.Fatal("ComputeStatistics returned nil")
	}
	if st.StepsCompleted != 7 {
		t.Fatalf("StepsCompleted=%d want 7", st.StepsCompleted)
	}
	if st.TotalAgents != 8 {
		t.Fatalf("TotalAgents=%d want 8", st.TotalAgents)
	}

	wolfPop := findPopulation(st.Population, "wolf")
	if wolfPop == nil {
		t.Fatal("missing wolf population entry")
	}
	if wolfPop.Current != 3 {
		t.Fatalf("wolf Current=%d want 3", wolfPop.Current)
	}
	if wolfPop.ByState["hunting"] != 2 || wolfPop.ByState["idle"] != 1 {
		t.Fatalf("wolf ByState=%v want hunting:2 idle:1", wolfPop.ByState)
	}

	sheepPop := findPopulation(st.Population, "sheep")
	if sheepPop == nil {
		t.Fatal("missing sheep population entry")
	}
	if sheepPop.Current != 5 {
		t.Fatalf("sheep Current=%d want 5", sheepPop.Current)
	}
	if sheepPop.ByState["grazing"] != 4 || sheepPop.ByState["fleeing"] != 1 {
		t.Fatalf("sheep ByState=%v want grazing:4 fleeing:1", sheepPop.ByState)
	}
}

func TestNewRuntimeState_InitsAccumulator(t *testing.T) {
	sim := &datasims.Simulation{
		ID:   "test-acc",
		Seed: 1,
		ModelSnapshot: &models.Model{
			Agents: []models.Agent{},
			Rules:  []models.Rule{},
		},
	}
	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	if rs.stats == nil {
		t.Fatal("rs.stats must not be nil after NewRuntimeState")
	}
	if rs.stats.ByRule == nil {
		t.Fatal("stats.ByRule map must be non-nil")
	}
	if rs.stats.ByEvent == nil {
		t.Fatal("stats.ByEvent map must be non-nil")
	}
	if rs.stats.ByInteraction == nil {
		t.Fatal("stats.ByInteraction map must be non-nil")
	}
	if rs.stats.PopSpawned == nil {
		t.Fatal("stats.PopSpawned map must be non-nil")
	}
	if rs.stats.PopRemoved == nil {
		t.Fatal("stats.PopRemoved map must be non-nil")
	}
	if rs.stats.PopPeak == nil {
		t.Fatal("stats.PopPeak map must be non-nil")
	}
	if rs.stats.PopMin == nil {
		t.Fatal("stats.PopMin map must be non-nil")
	}
	if rs.stats.popMinSeen == nil {
		t.Fatal("stats.popMinSeen map must be non-nil")
	}
}

func TestComputeStatistics_NumericPropertyAggregates(t *testing.T) {
	wolf := wolfArchetype()
	agents := []*RuntimeAgent{
		{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", State: "idle", Properties: map[string]any{"energy": 10.0, "species": "canis"}}, Archetype: wolf},
		{A: &datasims.SimulationAgent{ID: 2, AgentUID: "wolf", State: "idle", Properties: map[string]any{"energy": 30.0, "species": "canis"}}, Archetype: wolf},
		{A: &datasims.SimulationAgent{ID: 3, AgentUID: "wolf", State: "idle", Properties: map[string]any{"energy": 50.0, "species": "canis"}}, Archetype: wolf},
	}
	rs := ruleRuntime(agents, nil)

	st := ComputeStatistics(rs)
	if st == nil {
		t.Fatal("ComputeStatistics returned nil")
	}

	energyStat := findPropertyStat(st.Properties, "wolf", "energy")
	if energyStat == nil {
		t.Fatal("missing wolf energy property aggregate")
	}
	if energyStat.Avg != 30.0 {
		t.Fatalf("energy Avg=%v want 30", energyStat.Avg)
	}
	if energyStat.Min != 10.0 {
		t.Fatalf("energy Min=%v want 10", energyStat.Min)
	}
	if energyStat.Max != 50.0 {
		t.Fatalf("energy Max=%v want 50", energyStat.Max)
	}

	// "species" is a string property; it must NOT produce a numeric aggregate.
	if findPropertyStat(st.Properties, "wolf", "species") != nil {
		t.Fatal("string property 'species' must not appear in numeric aggregates")
	}
}

// TestAccumulator_CountsAcrossTick verifies that a single RunStepRows call
// increments the stats accumulator for rule firings, spawns, and ByRule map.
//
// Model: one "spawner" archetype agent, one rule "fire_spawn" that is always
// enabled (no conditions) and has two actions:
//   - ActionTypeSpawnAgent (spawns one "child")
//   - ActionTypeSetProperty on "energy" (ensures rule counts > 0 even with a
//     non-movement action that doesn't need an archetype description)
//
// After one tick we assert:
//   - rs.stats.RuleExecutions >= 1
//   - rs.stats.AgentsSpawned >= 1
//   - rs.stats.ByRule["fire_spawn"] >= 1
func TestAccumulator_CountsAcrossTick(t *testing.T) {
	withFixedClock(t)

	spawner := &models.Agent{
		UID:      "spawner",
		Type:     models.AgentTypePlain,
		MoveType: models.AgentMoveTypeStatic,
		States:   []string{"idle"},
		Properties: []models.AgentProperty{
			{UID: "energy", Type: models.PropertyTypeNumber, DefaultValue: float64(100)},
		},
		Initialization: models.AgentInitialization{InitialState: "idle"},
	}
	child := &models.Agent{
		UID:            "child",
		Type:           models.AgentTypePlain,
		MoveType:       models.AgentMoveTypeStatic,
		States:         []string{"idle"},
		Initialization: models.AgentInitialization{InitialState: "idle"},
	}
	energy := float64(50)
	rule := models.Rule{
		ID:        "fire_spawn",
		Enabled:   true,
		Priority:  10,
		AppliesTo: []string{"spawner"},
		// No conditions → always fires.
		Actions: []models.Action{
			{Type: models.ActionTypeSetProperty, PropertyUID: "energy", Value: &energy},
			{Type: models.ActionTypeSpawnAgent, SpawnAgentUID: "child", SpawnCount: 1},
		},
	}
	sim := &datasims.Simulation{
		ID:   "test-acc-tick",
		Seed: 7,
		ModelSnapshot: &models.Model{
			SimulationConfig: models.SimulationConfig{UpdateMode: models.UpdateModeSequential},
			Agents:           []models.Agent{*spawner, *child},
			Rules:            []models.Rule{rule},
		},
		Agents: []datasims.SimulationAgent{
			{
				ID:         1,
				AgentUID:   "spawner",
				State:      "idle",
				Position:   &common.Point{X: 10, Y: 10},
				Properties: map[string]interface{}{"energy": float64(100)},
			},
		},
	}

	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	if rs.stats == nil {
		t.Fatal("rs.stats is nil before RunStepRows")
	}

	if _, err := RunStepRows(context.Background(), rs, nil); err != nil {
		t.Fatalf("RunStepRows: %v", err)
	}

	if rs.stats.RuleExecutions < 1 {
		t.Fatalf("RuleExecutions=%d want >= 1", rs.stats.RuleExecutions)
	}
	if rs.stats.AgentsSpawned < 1 {
		t.Fatalf("AgentsSpawned=%d want >= 1", rs.stats.AgentsSpawned)
	}
	if rs.stats.ByRule["fire_spawn"] < 1 {
		t.Fatalf("ByRule[fire_spawn]=%d want >= 1", rs.stats.ByRule["fire_spawn"])
	}
}
