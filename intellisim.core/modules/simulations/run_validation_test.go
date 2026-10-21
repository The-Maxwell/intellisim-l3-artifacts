package simulations

import (
	"errors"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// validationModel builds a minimal valid model snapshot with one archetype "wolf"
// (numeric property "energy", two states). strat is the model DefaultStrategy;
// the model carries a usable LLM target so resolveTarget succeeds. When the
// strategy is a decision strategy, a ConditionTypeDecision rule supplies the
// decision menu.
func validationModel(strat models.LLMStrategy) *models.Model {
	arch := models.Agent{
		UID:      "wolf",
		Type:     models.AgentTypePlain,
		MoveType: models.AgentMoveTypeMovable,
		Properties: []models.AgentProperty{
			{UID: "energy", Type: models.PropertyTypeNumber},
		},
		States: []string{"idle", "hunting"},
	}
	m := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig:       models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
				DefaultStrategy: strat,
			},
		},
		Agents: []models.Agent{arch},
	}
	if strat.Type == models.LLMStrategyDecision {
		// Supply a decision menu so decision-strategy validation passes.
		m.Rules = []models.Rule{
			{
				ID:        "r1",
				Enabled:   true,
				AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{
					{Type: models.ConditionTypeDecision, Value: "search_food"},
					{Type: models.ConditionTypeDecision, Value: "flee"},
				},
			},
		}
	}
	return m
}

// validationSim builds a ready simulation with one positioned "wolf" agent over
// the given model snapshot.
func validationSim(m *models.Model) *datasims.Simulation {
	return &datasims.Simulation{
		Status:        datasims.SimulationStatusReady,
		Steps:         10,
		ModelSnapshot: m,
		Agents: []datasims.SimulationAgent{
			{
				ID:         1,
				AgentUID:   "wolf",
				State:      "idle",
				Position:   &common.Point{X: 10, Y: 10},
				Properties: map[string]any{"energy": float64(50)},
			},
		},
	}
}

func TestValidateRunnable_OK(t *testing.T) {
	m := validationModel(models.LLMStrategy{
		Type:             models.LLMStrategyFull,
		UseForMovement:   true,
		UseForProperties: true,
	})
	sim := validationSim(m)
	inv := &fakeInvoker{} // ValidateTarget returns nil
	if err := ValidateRunnable(sim, inv); err != nil {
		t.Fatalf("ValidateRunnable=%v want nil", err)
	}
}

func TestValidateRunnable_OKRulesOnly(t *testing.T) {
	// A pure rule-based run needs no LLM target at all.
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim := validationSim(m)
	// Even an invoker that would reject every target must not be consulted for a
	// rules-only run.
	inv := &fakeInvoker{validateErr: errors.New("should not be called")}
	if err := ValidateRunnable(sim, inv); err != nil {
		t.Fatalf("rules-only ValidateRunnable=%v want nil", err)
	}
}

func TestValidateRunnable_NotReady(t *testing.T) {
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim := validationSim(m)
	sim.Status = datasims.SimulationStatusRunning
	err := ValidateRunnable(sim, &fakeInvoker{})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("ValidateRunnable=%v want ErrNotRunnable", err)
	}
}

func TestValidateRunnable_NoAgents(t *testing.T) {
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim := validationSim(m)
	sim.Agents = nil
	err := ValidateRunnable(sim, &fakeInvoker{})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("ValidateRunnable=%v want ErrNotRunnable", err)
	}
}

func TestValidateRunnable_ZeroSteps(t *testing.T) {
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim := validationSim(m)
	sim.Steps = 0
	err := ValidateRunnable(sim, &fakeInvoker{})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("ValidateRunnable=%v want ErrNotRunnable", err)
	}
}

func TestValidateRunnable_DecisionMenuEmpty(t *testing.T) {
	// Decision strategy with NO ConditionTypeDecision rule => empty menu => error.
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true})
	m.Rules = nil // strip the decision menu the helper would otherwise add
	sim := validationSim(m)
	err := ValidateRunnable(sim, &fakeInvoker{})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("ValidateRunnable=%v want ErrNotRunnable (empty decision menu)", err)
	}
}

func TestValidateRunnable_LLMEnabledTargetInvalid(t *testing.T) {
	// Archetype UseForDecisions but ValidateTarget rejects the target => error.
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true})
	sim := validationSim(m)
	inv := &fakeInvoker{validateErr: errors.New("model not capable of simulation_decisions")}
	err := ValidateRunnable(sim, inv)
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("ValidateRunnable=%v want ErrNotRunnable (invalid target)", err)
	}
}

func TestValidateRunnable_LLMEnabledNoTargetDegradesOK(t *testing.T) {
	// LLM-enabled archetype but NO resolvable target: the purpose degrades to
	// rules at runtime, so run-start validation must not block.
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyFull, UseForMovement: true})
	m.SimulationConfig.LLMSimulationExecutionConfig.Target = common.LLMTarget{} // no provider/model anywhere
	sim := validationSim(m)
	// ValidateTarget must not even be consulted when there is no target.
	inv := &fakeInvoker{validateErr: errors.New("should not be called")}
	if err := ValidateRunnable(sim, inv); err != nil {
		t.Fatalf("ValidateRunnable=%v want nil (no target degrades to rules)", err)
	}
}

func TestValidateRunnable_StateIntegrityReuseValidateInstance(t *testing.T) {
	// An agent with a state not in the archetype's States is a ValidateInstance
	// error and must block the run.
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim := validationSim(m)
	sim.Agents[0].State = "bogus"
	err := ValidateRunnable(sim, &fakeInvoker{})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("ValidateRunnable=%v want ErrNotRunnable (bad state)", err)
	}
}

func TestValidateControl(t *testing.T) {
	cases := []struct {
		action  string
		status  datasims.SimulationStatus
		wantErr error
	}{
		// pause: only when running
		{"pause", datasims.SimulationStatusRunning, nil},
		{"pause", datasims.SimulationStatusPaused, ErrNotRunning},
		{"pause", datasims.SimulationStatusReady, ErrNotRunning},
		// resume: only when paused
		{"resume", datasims.SimulationStatusPaused, nil},
		{"resume", datasims.SimulationStatusRunning, ErrNotRunning},
		{"resume", datasims.SimulationStatusReady, ErrNotRunning},
		// stop: when running or paused
		{"stop", datasims.SimulationStatusRunning, nil},
		{"stop", datasims.SimulationStatusPaused, nil},
		{"stop", datasims.SimulationStatusReady, ErrNotRunning},
		{"stop", datasims.SimulationStatusFinished, ErrNotRunning},
		// set_speed: while running or paused (ISS-09 — the dedicated speed-only
		// action, so the UI slider isn't dead while a run is live).
		{"set_speed", datasims.SimulationStatusRunning, nil},
		{"set_speed", datasims.SimulationStatusPaused, nil},
		{"set_speed", datasims.SimulationStatusReady, ErrNotRunning},
		{"set_speed", datasims.SimulationStatusFinished, ErrNotRunning},
		{"set_speed", datasims.SimulationStatusStopped, ErrNotRunning},
		// unknown action
		{"frobnicate", datasims.SimulationStatusRunning, ErrNotRunnable},
		{"", datasims.SimulationStatusRunning, ErrNotRunnable},
	}
	for _, tc := range cases {
		err := ValidateControl(tc.action, tc.status)
		if tc.wantErr == nil {
			if err != nil {
				t.Errorf("ValidateControl(%q,%q)=%v want nil", tc.action, tc.status, err)
			}
			continue
		}
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("ValidateControl(%q,%q)=%v want %v", tc.action, tc.status, err, tc.wantErr)
		}
	}
}

func TestValidateRunnable_ZeroDimensionWorld(t *testing.T) {
	// A spatial model with a nil environment or zero width/height must not run
	// (positions would silently collapse to (0,0)).
	m := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	m.IsSpatial = true

	sim := validationSim(m) // no Environment set
	err := ValidateRunnable(sim, &fakeInvoker{})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("nil environment: ValidateRunnable=%v want ErrNotRunnable", err)
	}
	if !runnable(sim) {
		// runnable must agree (it mirrors the guard).
	} else {
		t.Fatal("runnable() must reject a spatial sim without an environment")
	}

	sim.Environment = &datasims.SimulationEnvironment{Width: 0, Height: 80}
	if err := ValidateRunnable(sim, &fakeInvoker{}); !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("zero width: ValidateRunnable=%v want ErrNotRunnable", err)
	}

	sim.Environment = &datasims.SimulationEnvironment{Width: 100, Height: 80}
	if err := ValidateRunnable(sim, &fakeInvoker{}); err != nil {
		t.Fatalf("valid dims: ValidateRunnable=%v want nil", err)
	}
	if !runnable(sim) {
		t.Fatal("runnable() must accept valid dims")
	}

	// Non-spatial models need no environment at all.
	m2 := validationModel(models.LLMStrategy{Type: models.LLMStrategyNone})
	sim2 := validationSim(m2)
	if err := ValidateRunnable(sim2, &fakeInvoker{}); err != nil {
		t.Fatalf("non-spatial without env: ValidateRunnable=%v want nil", err)
	}
}
