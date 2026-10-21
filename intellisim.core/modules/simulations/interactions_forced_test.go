package simulations

import (
	"context"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// forcedHuntInteraction is an agent_agent interaction whose declared Conditions
// can never pass (step > 1e9), so it fires ONLY when force-fired.
func forcedHuntInteraction() models.Interaction {
	return models.Interaction{
		ID:            "hunt",
		Type:          models.InteractionTypeAgentAgent,
		Enabled:       true,
		InitiatorUIDs: []string{"wolf"},
		TargetUIDs:    []string{"sheep"},
		Radius:        fptr(10),
		Conditions: []models.Condition{
			{Type: models.ConditionTypeStep, Operator: models.ConditionOpGreaterThan, Value: 1e9},
		},
		InitiatorActions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 10.0}},
		TargetActions:    []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierSubtract, Value: 10.0}},
	}
}

// TestForcedInteract_FiresSkippingConditions verifies that an interact rule
// action force-fires the named interaction (mutating both parties + stats) even
// though the interaction's own Conditions are false, and that the regular pass
// alone does NOT fire it.
func TestForcedInteract_FiresSkippingConditions(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 12, Y: 11}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	rs := interactionRuntime([]*RuntimeAgent{wolf, sheep}, []models.Interaction{forcedHuntInteraction()}, nil)
	rs.stats = newStatsAccumulator()

	// Regular pass: conditions false -> nothing fires.
	ProcessInteractions(rs, nil, nil)
	if wolf.A.Properties["energy"].(float64) != 50.0 {
		t.Fatal("interaction must not fire through the regular pass (conditions false)")
	}

	// Rule action queues the force-fire; the drain fires it.
	evts := ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract, TargetUID: "hunt"})
	if len(evts) != 0 {
		t.Fatalf("interact action returns no immediate memEvents, got %v", evts)
	}
	if len(rs.forced) != 1 {
		t.Fatalf("forced queue = %d entries, want 1", len(rs.forced))
	}
	ProcessInteractions(rs, nil, nil)

	if got := wolf.A.Properties["energy"].(float64); got != 60.0 {
		t.Errorf("wolf energy = %v, want 60 (forced initiator action)", got)
	}
	if got := sheep.A.Properties["energy"].(float64); got != 30.0 {
		t.Errorf("sheep energy = %v, want 30 (forced target action)", got)
	}
	if rs.stats.Interactions != 1 || rs.stats.ByInteraction["hunt"] != 1 {
		t.Errorf("stats = %d/%d, want 1/1", rs.stats.Interactions, rs.stats.ByInteraction["hunt"])
	}
	if !hasInteractionMemory(wolf) || !hasInteractionMemory(sheep) {
		t.Error("both parties must record interaction memory on a forced fire")
	}
	if len(rs.forced) != 0 {
		t.Errorf("forced queue not reset after drain: %v", rs.forced)
	}
}

// TestForcedInteract_NearestTargetTieBreak verifies the deterministic target
// pick: nearest by body distance, ascending id on ties.
func TestForcedInteract_NearestTargetTieBreak(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	near := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 5, AgentUID: "sheep", Position: &common.Point{X: 12, Y: 10}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	far := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 17, Y: 10}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	// Equidistant tie pair: ids 9 and 3 both at distance 4 -> id 3 wins over 9,
	// but the truly nearest (id 5, distance 2) wins overall.
	tieHi := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 9, AgentUID: "sheep", Position: &common.Point{X: 6, Y: 10}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	in := forcedHuntInteraction()
	rs := interactionRuntime([]*RuntimeAgent{wolf, near, far, tieHi}, []models.Interaction{in}, nil)

	got := nearestForcedTarget(rs, wolf, in)
	if got == nil || got.A.ID != 5 {
		t.Fatalf("nearestForcedTarget = %+v, want id 5 (distance 2)", got)
	}

	// Remove the nearest; ids 9 (dist 4) and 2 (dist 7): 9 wins on distance.
	near.removed = true
	got = nearestForcedTarget(rs, wolf, in)
	if got == nil || got.A.ID != 9 {
		t.Fatalf("nearestForcedTarget after removal = %+v, want id 9", got)
	}
}

// TestForcedInteract_WarnPaths verifies queue-time and drain-time warn+no-op
// paths: missing/unknown/disabled interaction, non-initiator archetype, and no
// eligible target in range.
func TestForcedInteract_WarnPaths(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 90, Y: 70}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	disabled := forcedHuntInteraction()
	disabled.ID = "off"
	disabled.Enabled = false
	rs := interactionRuntime([]*RuntimeAgent{wolf, sheep}, []models.Interaction{forcedHuntInteraction(), disabled}, nil)

	ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract})                         // missing target_uid
	ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract, TargetUID: "ghost"})     // unknown
	ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract, TargetUID: "off"})       // disabled
	ApplyAction(rs, sheep, models.Action{Type: models.ActionTypeInteract, TargetUID: "hunt"})     // sheep is not an initiator
	if len(rs.forced) != 0 {
		t.Fatalf("invalid requests must not queue; forced=%v", rs.forced)
	}

	// Valid queue but the only target is out of radius -> drain-time warn.
	ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract, TargetUID: "hunt"})
	ProcessInteractions(rs, nil, nil)
	if wolf.A.Properties["energy"].(float64) != 50.0 {
		t.Error("forced fire must not happen without an eligible target in range")
	}

	wantWarns := []string{
		"missing target_uid",
		"unknown interaction 'ghost'",
		"'off' is disabled",
		"'sheep' is not an initiator",
		"no eligible target in range",
	}
	joined := strings.Join(rs.warnings, "\n")
	for _, w := range wantWarns {
		if !strings.Contains(joined, w) {
			t.Errorf("missing warning %q in %v", w, rs.warnings)
		}
	}
}

// TestForcedInteract_DedupAndNested verifies (a) duplicate requests for the same
// (initiator, interaction) pair fire once per step, and (b) an interaction whose
// own initiator action is interact chains within the same drain but cannot loop.
func TestForcedInteract_DedupAndNested(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 12, Y: 11}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}

	// hunt chains into itself via an interact initiator action: the dedup set
	// must terminate the cycle after the first fire.
	chained := forcedHuntInteraction()
	chained.InitiatorActions = append(chained.InitiatorActions,
		models.Action{Type: models.ActionTypeInteract, TargetUID: "hunt"})
	rs := interactionRuntime([]*RuntimeAgent{wolf, sheep}, []models.Interaction{chained}, nil)
	rs.stats = newStatsAccumulator()

	// Queue the same pair twice; the chain queues a third.
	ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract, TargetUID: "hunt"})
	ApplyAction(rs, wolf, models.Action{Type: models.ActionTypeInteract, TargetUID: "hunt"})
	ProcessInteractions(rs, nil, nil)

	if got := wolf.A.Properties["energy"].(float64); got != 60.0 {
		t.Errorf("wolf energy = %v, want 60 (exactly one forced fire)", got)
	}
	if rs.stats.ByInteraction["hunt"] != 1 {
		t.Errorf("ByInteraction[hunt] = %d, want 1 (dedup + cycle guard)", rs.stats.ByInteraction["hunt"])
	}
}

// TestForcedInteract_SequentialSynchronousParity verifies a full RunStep drives
// the rule -> queue -> drain path identically in both update modes.
func TestForcedInteract_SequentialSynchronousParity(t *testing.T) {
	withFixedClock(t)
	build := func(mode models.ModelUpdateMode) *RuntimeState {
		in := forcedHuntInteraction()
		model := &models.Model{
			SimulationConfig: models.SimulationConfig{UpdateMode: mode},
			IsSpatial:        true,
			SpatialCellSize:  10,
			Agents: []models.Agent{
				{UID: "wolf", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable, States: []string{"idle"},
					Properties:     []models.AgentProperty{{UID: "energy", Type: models.PropertyTypeNumber, DefaultValue: float64(50)}},
					Initialization: models.AgentInitialization{InitialState: "idle"}},
				{UID: "sheep", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable, States: []string{"idle"},
					Properties:     []models.AgentProperty{{UID: "energy", Type: models.PropertyTypeNumber, DefaultValue: float64(40)}},
					Initialization: models.AgentInitialization{InitialState: "idle"}},
			},
			Rules: []models.Rule{{
				ID: "attack", Enabled: true, Priority: 10, AppliesTo: []string{"wolf"},
				Actions: []models.Action{{Type: models.ActionTypeInteract, TargetUID: "hunt"}},
			}},
			Interactions: []models.Interaction{in},
		}
		sim := &datasims.Simulation{
			ID: "sim-forced", Seed: 3, Steps: 5,
			Environment:   &datasims.SimulationEnvironment{Width: 100, Height: 80},
			ModelSnapshot: model,
			Agents: []datasims.SimulationAgent{
				{ID: 1, AgentUID: "wolf", State: "idle", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]interface{}{"energy": float64(50)}},
				{ID: 2, AgentUID: "sheep", State: "idle", Position: &common.Point{X: 12, Y: 11}, Properties: map[string]interface{}{"energy": float64(40)}},
			},
		}
		rs, err := NewRuntimeState(sim)
		if err != nil {
			t.Fatalf("NewRuntimeState: %v", err)
		}
		return rs
	}

	seq := build(models.UpdateModeSequential)
	sync := build(models.UpdateModeSynchronous)
	if _, err := RunStep(context.Background(), seq, nil); err != nil {
		t.Fatalf("sequential RunStep: %v", err)
	}
	if _, err := RunStep(context.Background(), sync, nil); err != nil {
		t.Fatalf("synchronous RunStep: %v", err)
	}

	for _, tc := range []struct {
		name string
		rs   *RuntimeState
	}{{"sequential", seq}, {"synchronous", sync}} {
		if got := tc.rs.ByID[1].A.Properties["energy"].(float64); got != 60.0 {
			t.Errorf("%s wolf energy = %v, want 60", tc.name, got)
		}
		if got := tc.rs.ByID[2].A.Properties["energy"].(float64); got != 30.0 {
			t.Errorf("%s sheep energy = %v, want 30", tc.name, got)
		}
		if tc.rs.stats.ByInteraction["hunt"] != 1 {
			t.Errorf("%s ByInteraction[hunt] = %d, want 1", tc.name, tc.rs.stats.ByInteraction["hunt"])
		}
	}
}
