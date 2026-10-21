package simulations

import (
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// interactionRuntime builds a RuntimeState around a set of agents plus a model
// carrying the given interactions (fixed 100x80 non-torus world).
func interactionRuntime(agents []*RuntimeAgent, interactions []models.Interaction, shapes []datasims.SimulationShape) *RuntimeState {
	rs := ruleRuntime(agents, nil)
	rs.Model.Interactions = interactions
	rs.Env.Shapes = shapes
	return rs
}

func TestInteractions_AgentAgentWithinRadiusFires(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 12, Y: 11}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	in := models.Interaction{
		ID:               "hunt",
		Type:             models.InteractionTypeAgentAgent,
		Enabled:          true,
		InitiatorUIDs:    []string{"wolf"},
		TargetUIDs:       []string{"sheep"},
		Radius:           fptr(5),
		InitiatorActions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 10.0}},
	}
	rs := interactionRuntime([]*RuntimeAgent{wolf, sheep}, []models.Interaction{in}, nil)
	ProcessInteractions(rs, nil, nil)
	if got := wolf.A.Properties["energy"].(float64); got != 60.0 {
		t.Fatalf("wolf energy=%v want 60 (initiator action applied)", got)
	}
}

func TestInteractions_BidirectionalAppliesTargetThenReverse(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 12, Y: 11}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	in := models.Interaction{
		ID:            "fight",
		Type:          models.InteractionTypeAgentAgent,
		Enabled:       true,
		Bidirectional: true,
		InitiatorUIDs: []string{"wolf"},
		TargetUIDs:    []string{"sheep"},
		Radius:        fptr(5),
		// Initiator gains energy; target loses energy.
		InitiatorActions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 5.0}},
		TargetActions:    []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierSubtract, Value: 5.0}},
	}
	rs := interactionRuntime([]*RuntimeAgent{wolf, sheep}, []models.Interaction{in}, nil)
	ProcessInteractions(rs, nil, nil)
	// Forward: wolf +5 (init), sheep -5 (target). Reverse: sheep +5 (init), wolf -5 (target).
	if got := wolf.A.Properties["energy"].(float64); got != 50.0 {
		t.Fatalf("wolf energy=%v want 50 (forward +5 init, reverse -5 target)", got)
	}
	if got := sheep.A.Properties["energy"].(float64); got != 40.0 {
		t.Fatalf("sheep energy=%v want 40 (forward -5 target, reverse +5 init)", got)
	}
	// Both should have recorded an interaction memory event.
	if !hasInteractionMemory(wolf) || !hasInteractionMemory(sheep) {
		t.Fatalf("both agents should record an interaction memory event")
	}
}

func TestInteractions_OutsideRadiusSkipped(t *testing.T) {
	wolf := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 50.0}}, Archetype: wolfArchetype()}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep", Position: &common.Point{X: 90, Y: 70}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	in := models.Interaction{
		ID:               "hunt",
		Type:             models.InteractionTypeAgentAgent,
		Enabled:          true,
		InitiatorUIDs:    []string{"wolf"},
		TargetUIDs:       []string{"sheep"},
		Radius:           fptr(5),
		InitiatorActions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 10.0}},
	}
	rs := interactionRuntime([]*RuntimeAgent{wolf, sheep}, []models.Interaction{in}, nil)
	ProcessInteractions(rs, nil, nil)
	if got := wolf.A.Properties["energy"].(float64); got != 50.0 {
		t.Fatalf("wolf energy=%v want 50 (target out of radius => no fire)", got)
	}
}

func TestInteractions_AgentShapeUsesShapeContains(t *testing.T) {
	// Agent stands inside a rectangular shape; an agent_shape interaction should fire.
	w, h := uint64(20), uint64(20)
	shape := datasims.SimulationShape{
		ID:       100,
		ShapeUID: "grass",
		Type:     models.ShapeTypeRectangle,
		Position: common.Point{X: 5, Y: 5},
		Width:    &w,
		Height:   &h,
	}
	sheep := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "sheep", Position: &common.Point{X: 10, Y: 10}, Properties: map[string]any{"energy": 40.0}}, Archetype: sheepArchetype()}
	in := models.Interaction{
		ID:               "graze",
		Type:             models.InteractionTypeAgentShape,
		Enabled:          true,
		InitiatorUIDs:    []string{"sheep"},
		TargetUIDs:       []string{"grass"},
		InitiatorActions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 8.0}},
	}
	rs := interactionRuntime([]*RuntimeAgent{sheep}, []models.Interaction{in}, []datasims.SimulationShape{shape})
	ProcessInteractions(rs, nil, nil)
	if got := sheep.A.Properties["energy"].(float64); got != 48.0 {
		t.Fatalf("sheep energy=%v want 48 (inside grass shape => graze fires)", got)
	}

	// An agent outside the shape must not fire the interaction.
	sheep.A.Position = &common.Point{X: 90, Y: 70}
	sheep.A.Properties["energy"] = 40.0
	ProcessInteractions(rs, nil, nil)
	if got := sheep.A.Properties["energy"].(float64); got != 40.0 {
		t.Fatalf("sheep energy=%v want 40 (outside grass shape => no graze)", got)
	}
}

// sheepArchetype is a simple movable archetype with a bounded numeric energy.
func sheepArchetype() *models.Agent {
	return &models.Agent{
		UID:      "sheep",
		Type:     models.AgentTypeArchetype,
		MoveType: models.AgentMoveTypeMovable,
		Properties: []models.AgentProperty{
			{UID: "energy", Type: models.PropertyTypeNumber, MinValue: fptr(0), MaxValue: fptr(100)},
		},
	}
}

// hasInteractionMemory reports whether the agent recorded any interaction-typed
// memory event.
func hasInteractionMemory(ra *RuntimeAgent) bool {
	for _, m := range ra.A.Memory {
		if m.Type == datasims.AgentMemoryInteraction {
			return true
		}
	}
	return false
}
