package simulations

import (
	"context"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// TestSynchronous_InteractionWarningsFoldBack verifies that advisory warnings
// emitted during the synchronous interactions phase (which runs on the working
// view) are folded back onto the real RuntimeState instead of being dropped
// with the workView's slice header.
func TestSynchronous_InteractionWarningsFoldBack(t *testing.T) {
	withFixedClock(t)
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{UpdateMode: models.UpdateModeSynchronous},
		IsSpatial:        true,
		SpatialCellSize:  10,
		Agents: []models.Agent{
			{UID: "wolf", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable, States: []string{"idle"}, Initialization: models.AgentInitialization{InitialState: "idle"}},
			{UID: "sheep", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable, States: []string{"idle"}, Initialization: models.AgentInitialization{InitialState: "idle"}},
		},
		Events: []models.EventDefinition{{
			UID:   "howl",
			Scope: models.EventScopeGlobal,
			Payload: []models.AgentProperty{
				{UID: "level", Type: models.PropertyTypeNumber},
			},
		}},
		Interactions: []models.Interaction{{
			ID:            "hunt",
			Type:          models.InteractionTypeAgentAgent,
			Enabled:       true,
			InitiatorUIDs: []string{"wolf"},
			TargetUIDs:    []string{"sheep"},
			Radius:        fptr(10),
			// Payload map is missing the declared "level" field -> advisory warning
			// appended during the interactions phase.
			InitiatorActions: []models.Action{{
				Type:     models.ActionTypeEmitEvent,
				EventUID: "howl",
				EventData: map[string]interface{}{
					"other": 1.0,
				},
			}},
		}},
	}
	sim := &datasims.Simulation{
		ID: "sim-warn", Seed: 7, Steps: 5,
		Environment:   &datasims.SimulationEnvironment{Width: 100, Height: 80},
		ModelSnapshot: model,
		Agents: []datasims.SimulationAgent{
			{ID: 1, AgentUID: "wolf", State: "idle", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]interface{}{}},
			{ID: 2, AgentUID: "sheep", State: "idle", Position: &common.Point{X: 12, Y: 11}, Properties: map[string]interface{}{}},
		},
	}
	rs, err := NewRuntimeState(sim)
	if err != nil {
		t.Fatalf("NewRuntimeState: %v", err)
	}
	if _, err := RunStep(context.Background(), rs, nil); err != nil {
		t.Fatalf("RunStep: %v", err)
	}
	found := false
	for _, w := range rs.warnings {
		if strings.Contains(w, "payload missing field 'level'") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("interactions-phase warning lost in synchronous mode; warnings=%v", rs.warnings)
	}
}
