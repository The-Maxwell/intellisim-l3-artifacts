package simulations

import (
	"context"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// globalsModel builds a spatial model with one movable archetype and declared
// environment global properties: temperature (number, clamped [0,100]),
// locked_const (immutable number), label (string).
func globalsModel(updateMode models.ModelUpdateMode, agentRules []models.Rule, envRules []models.Rule) *models.Model {
	minV, maxV := 0.0, 100.0
	return &models.Model{
		SimulationConfig: models.SimulationConfig{UpdateMode: updateMode},
		IsSpatial:        true,
		SpatialCellSize:  10,
		Agents: []models.Agent{{
			UID: "mover", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable,
			States:         []string{"idle"},
			Initialization: models.AgentInitialization{InitialState: "idle"},
		}},
		Environment: &models.Environment{
			UID: models.EnvironmentUID, Width: 100, Height: 80,
			GlobalProperties: []models.AgentProperty{
				{UID: "temperature", Type: models.PropertyTypeNumber, MinValue: &minV, MaxValue: &maxV, DefaultValue: float64(20)},
				{UID: "locked_const", Type: models.PropertyTypeNumber, Immutable: true, DefaultValue: float64(5)},
				{UID: "label", Type: models.PropertyTypeString, DefaultValue: "calm"},
			},
		},
		Rules: append(append([]models.Rule{}, agentRules...), envRules...),
	}
}

func globalsState(t *testing.T, model *models.Model, agents int) *RuntimeState {
	t.Helper()
	sim := &datasims.Simulation{
		ID: "sim-globals", Seed: 11, Steps: 10,
		Environment: &datasims.SimulationEnvironment{
			Width: 100, Height: 80,
			GlobalProperties: map[string]interface{}{
				"temperature": float64(20), "locked_const": float64(5), "label": "calm",
			},
		},
		ModelSnapshot: model,
	}
	for i := 0; i < agents; i++ {
		sim.Agents = append(sim.Agents, datasims.SimulationAgent{
			ID: uint64(i + 1), AgentUID: "mover", State: "idle",
			Position:   &common.Point{X: uint64(10 + 5*i), Y: 10},
			Properties: map[string]interface{}{},
		})
	}
	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	return rs
}

// TestEnvRule_WritesGlobalProperties verifies env-scoped rules can set/modify
// declared globals (with clamping), no-op on immutable/undeclared (warn once),
// warn once on unsupported action types, and count toward rule stats.
func TestEnvRule_WritesGlobalProperties(t *testing.T) {
	withFixedClock(t)
	envRules := []models.Rule{{
		ID: "env-cool", Enabled: true, Priority: 10, AppliesTo: []string{models.EnvironmentUID},
		Actions: []models.Action{
			{Type: models.ActionTypeModifyProperty, PropertyUID: "temperature", Modifier: models.PropertyModifierSubtract, Value: 30.0},
			{Type: models.ActionTypeSetProperty, PropertyUID: "label", Value: "cold"},
			{Type: models.ActionTypeSetProperty, PropertyUID: "locked_const", Value: 99.0}, // immutable -> no-op
			{Type: models.ActionTypeSetProperty, PropertyUID: "ghost", Value: 1.0},        // undeclared -> warnOnce
			{Type: models.ActionTypeMove},                                                 // unsupported -> warnOnce
		},
	}}
	rs := globalsState(t, globalsModel(models.UpdateModeSequential, nil, envRules), 1)

	// Two steps: temperature 20-30 -> clamped 0; stays 0. label -> "cold".
	for i := 0; i < 2; i++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("RunStep %d: %v", i, err)
		}
	}
	if got := rs.Env.GlobalProperties["temperature"]; got != float64(0) {
		t.Errorf("temperature = %v, want 0 (clamped to MinValue)", got)
	}
	if got := rs.Env.GlobalProperties["label"]; got != "cold" {
		t.Errorf("label = %v, want cold", got)
	}
	if got := rs.Env.GlobalProperties["locked_const"]; got != float64(5) {
		t.Errorf("locked_const = %v, want 5 (immutable)", got)
	}
	if _, exists := rs.Env.GlobalProperties["ghost"]; exists {
		t.Error("undeclared global 'ghost' must not be materialized")
	}
	var undeclaredWarn, unsupportedWarn int
	for _, w := range rs.warnings {
		if strings.Contains(w, "'ghost' is not a declared") {
			undeclaredWarn++
		}
		if strings.Contains(w, "'move' is not supported for environment-scoped rules") {
			unsupportedWarn++
		}
	}
	if undeclaredWarn != 1 || unsupportedWarn != 1 {
		t.Errorf("warnOnce counts: undeclared=%d unsupported=%d, want 1/1; warnings=%v", undeclaredWarn, unsupportedWarn, rs.warnings)
	}
	if rs.stats.ByRule["env-cool"] != 2 {
		t.Errorf("ByRule[env-cool] = %d, want 2 (fired both steps)", rs.stats.ByRule["env-cool"])
	}
}

// TestEnvRule_CooldownAndExecuteOnce verifies env-scoped rules honor
// CooldownSteps and ExecuteOnce across steps.
func TestEnvRule_CooldownAndExecuteOnce(t *testing.T) {
	withFixedClock(t)
	envRules := []models.Rule{
		{
			ID: "env-once", Enabled: true, Priority: 10, ExecuteOnce: true, AppliesTo: []string{models.EnvironmentUID},
			Actions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "temperature", Modifier: models.PropertyModifierAdd, Value: 1.0}},
		},
		{
			ID: "env-cool", Enabled: true, Priority: 5, CooldownSteps: 2, AppliesTo: []string{models.EnvironmentUID},
			Actions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "temperature", Modifier: models.PropertyModifierAdd, Value: 10.0}},
		},
	}
	rs := globalsState(t, globalsModel(models.UpdateModeSequential, nil, envRules), 1)

	// 4 steps: env-once fires once (+1). env-cool fires at steps 0 and 2 (+20).
	for i := 0; i < 4; i++ {
		if _, err := RunStep(context.Background(), rs, nil); err != nil {
			t.Fatalf("RunStep %d: %v", i, err)
		}
	}
	if got := rs.Env.GlobalProperties["temperature"]; got != float64(41) {
		t.Errorf("temperature = %v, want 41 (20 +1 once, +10 at steps 0 and 2)", got)
	}
}

// agentGlobalAddRule fires when temperature is at its initial value and adds 1
// to it (env-targeted modify).
func agentGlobalAddRule() models.Rule {
	return models.Rule{
		ID: "agent-heat", Enabled: true, Priority: 10, AppliesTo: []string{"mover"},
		Conditions: []models.Condition{
			{Type: models.ConditionTypeGlobalProperty, PropertyUID: "temperature", Operator: models.ConditionOpEquals, Value: 20.0},
		},
		Actions: []models.Action{
			{Type: models.ActionTypeModifyProperty, PropertyUID: "temperature", Modifier: models.PropertyModifierAdd, Value: 1.0, TargetUID: models.EnvironmentUID},
		},
	}
}

// TestAgentGlobalWrite_SequentialVsSynchronous verifies the mode semantics:
// sequential applies writes immediately (the second agent sees 21 and does not
// fire), synchronous defers them past the frozen reads (both agents see 20 and
// both writes accumulate).
func TestAgentGlobalWrite_SequentialVsSynchronous(t *testing.T) {
	withFixedClock(t)

	seq := globalsState(t, globalsModel(models.UpdateModeSequential, []models.Rule{agentGlobalAddRule()}, nil), 2)
	if _, err := RunStep(context.Background(), seq, nil); err != nil {
		t.Fatalf("sequential RunStep: %v", err)
	}
	if got := seq.Env.GlobalProperties["temperature"]; got != float64(21) {
		t.Errorf("sequential temperature = %v, want 21 (second agent saw the write)", got)
	}

	sync := globalsState(t, globalsModel(models.UpdateModeSynchronous, []models.Rule{agentGlobalAddRule()}, nil), 2)
	if _, err := RunStep(context.Background(), sync, nil); err != nil {
		t.Fatalf("synchronous RunStep: %v", err)
	}
	if got := sync.Env.GlobalProperties["temperature"]; got != float64(22) {
		t.Errorf("synchronous temperature = %v, want 22 (frozen reads, both writes fold)", got)
	}
	if len(sync.globalWrites) != 0 {
		t.Errorf("globalWrites queue not drained: %v", sync.globalWrites)
	}
	if sync.deferGlobalWrites {
		t.Error("deferGlobalWrites must be cleared after the synchronous drain")
	}

	// The acting agents record the env write in memory.
	found := false
	for _, m := range sync.Agents[0].A.Memory {
		if m.Type == datasims.AgentMemoryProperty && strings.Contains(m.Summary, "env:temperature") {
			found = true
		}
	}
	if !found {
		t.Errorf("agent memory missing env write entry; memory=%+v", sync.Agents[0].A.Memory)
	}

	// Determinism: a same-seed synchronous rerun produces the same value.
	sync2 := globalsState(t, globalsModel(models.UpdateModeSynchronous, []models.Rule{agentGlobalAddRule()}, nil), 2)
	if _, err := RunStep(context.Background(), sync2, nil); err != nil {
		t.Fatalf("synchronous rerun RunStep: %v", err)
	}
	if sync.Env.GlobalProperties["temperature"] != sync2.Env.GlobalProperties["temperature"] {
		t.Error("synchronous global writes are not reproducible across same-seed runs")
	}
}

// TestValidateRawAction_EnvironmentTarget verifies the LLM full-control decode
// path accepts env-targeted set/modify only for declared, mutable globals.
func TestValidateRawAction_EnvironmentTarget(t *testing.T) {
	rs := globalsState(t, globalsModel(models.UpdateModeSequential, nil, nil), 1)
	ra := rs.Agents[0]

	ok := func(r rawAction) bool { _, o := validateRawAction(rs, ra, r); return o }

	if !ok(rawAction{Type: "set_property", PropertyUID: "temperature", Value: 50.0, TargetUID: models.EnvironmentUID}) {
		t.Error("set on declared mutable global must be accepted")
	}
	if !ok(rawAction{Type: "modify_property", PropertyUID: "temperature", Modifier: "add", Value: 1.0, TargetUID: models.EnvironmentUID}) {
		t.Error("modify on declared mutable global must be accepted")
	}
	if ok(rawAction{Type: "set_property", PropertyUID: "locked_const", Value: 1.0, TargetUID: models.EnvironmentUID}) {
		t.Error("set on immutable global must be rejected")
	}
	if ok(rawAction{Type: "set_property", PropertyUID: "ghost", Value: 1.0, TargetUID: models.EnvironmentUID}) {
		t.Error("set on undeclared global must be rejected")
	}

	// The accepted intent must carry the environment TargetUID through.
	ai, o := validateRawAction(rs, ra, rawAction{Type: "set_property", PropertyUID: "temperature", Value: 50.0, TargetUID: models.EnvironmentUID})
	if !o || ai.TargetUID != models.EnvironmentUID {
		t.Errorf("intent TargetUID = %q, want environment", ai.TargetUID)
	}
}
