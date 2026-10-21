package simulations

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
	mongomodels "intelli.sim.core/data/mongodb/models"
	"intelli.sim.core/data/simulations"
)

// TestSimulationDocumentFields verifies that a SimulationDocument survives a
// bson.Marshal + bson.Unmarshal round-trip without field loss, exercising bson
// tag naming and, crucially, the bson:",inline" embedding on
// AgentTokenUsageDocument and PurposeTokenUsageDocument.
func TestSimulationDocumentFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	in := SimulationDocument{
		ID:          bson.NewObjectID(),
		Name:        "predator-prey run",
		Description: "test run",
		ModelID:     bson.NewObjectID(),
		ModelSnapshot: &mongomodels.ModelDocument{
			Name: "predator-prey",
		},
		Seed:  42,
		Steps: 100,
		Agents: []SimulationAgentDocument{
			{
				ID:       1,
				AgentUID: "predator",
				Name:     "Predator #1",
				Position: &mongocommon.PointDocument{X: 1, Y: 2},
				Properties: map[string]interface{}{
					"energy": float64(50),
				},
				State: "hunting",
				Memory: []AgentMemoryDocument{
					{
						Step:      1,
						Type:      simulations.AgentMemoryDecision,
						Summary:   "hunt",
						Timestamp: now,
					},
				},
			},
		},
		Environment: &SimulationEnvironmentDocument{
			Width:  100,
			Height: 100,
			Torus:  true,
			GlobalProperties: map[string]interface{}{
				"phase": "day",
			},
			Shapes: []SimulationShapeDocument{
				{
					ID:       1,
					ShapeUID: "forest",
					Name:     "Forest",
					Type:     models.ShapeTypeRectangle,
					Visible:  true,
					Position: &mongocommon.PointDocument{X: 10, Y: 20},
				},
			},
		},
		Status:     simulations.SimulationStatusRunning,
		StatusText: "running step 5",
		StatusHistory: []SimulationStatusChangeDocument{
			{Status: simulations.SimulationStatusRunning, At: now, By: "u1"},
		},
		CurrentStep: 5,
		GenerationRuns: []GenerationRunDocument{
			{
				Seq:       1,
				Scope:     GenerationScopeDocument{Kind: simulations.ScopeFull},
				Status:    simulations.RunCompleted,
				StartedAt: now,
				Requests:  2,
			},
		},
		TokenUsage: &SimulationTokenUsageDocument{
			Total: mongocommon.TokenUsageDocument{TotalTokens: 30, Calls: 3},
			ByModel: []mongocommon.TokenUsageDocument{
				{
					LLMTargetDocument: mongocommon.LLMTargetDocument{
						ProviderUID: "openai",
						ModelUID:    "gpt-4o",
					},
					TotalTokens: 30,
					Calls:       3,
				},
			},
			// AgentTokenUsageDocument uses bson:",inline" for TokenUsageDocument —
			// the round-trip verifies the inline fields (total_tokens, calls, …)
			// are hoisted correctly alongside agent_uid.
			ByAgent: []AgentTokenUsageDocument{
				{
					AgentUID: "predator",
					TokenUsageDocument: mongocommon.TokenUsageDocument{
						TotalTokens: 20,
						Calls:       2,
					},
				},
			},
			// PurposeTokenUsageDocument similarly uses bson:",inline".
			ByPurpose: []PurposeTokenUsageDocument{
				{
					Purpose: simulations.LLMUsageSimulationDecision,
					TokenUsageDocument: mongocommon.TokenUsageDocument{
						TotalTokens: 20,
						Calls:       2,
					},
				},
			},
			ByGeneration: []mongocommon.TokenUsageDocument{
				{TotalTokens: 30, Calls: 3},
			},
		},
		CreatedAt: now,
		CreatedBy: "u1",
		UpdatedAt: now,
		UpdatedBy: "u1",
	}

	raw, err := bson.Marshal(in)
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var out SimulationDocument
	if err := bson.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}

	// Top-level scalar fields.
	if out.Status != in.Status {
		t.Errorf("Status: got %q want %q", out.Status, in.Status)
	}
	if out.StatusText != in.StatusText {
		t.Errorf("StatusText: got %q want %q", out.StatusText, in.StatusText)
	}
	if out.CurrentStep != in.CurrentStep {
		t.Errorf("CurrentStep: got %d want %d", out.CurrentStep, in.CurrentStep)
	}
	if out.Steps != in.Steps {
		t.Errorf("Steps: got %d want %d", out.Steps, in.Steps)
	}
	if out.Seed != in.Seed {
		t.Errorf("Seed: got %d want %d", out.Seed, in.Seed)
	}
	if out.ModelSnapshot == nil || out.ModelSnapshot.Name != in.ModelSnapshot.Name {
		t.Errorf("ModelSnapshot.Name: got %v", out.ModelSnapshot)
	}

	// Agent.
	if len(out.Agents) != 1 {
		t.Fatalf("Agents len: got %d want 1", len(out.Agents))
	}
	a := out.Agents[0]
	if a.ID != 1 || a.AgentUID != "predator" || a.Name != "Predator #1" {
		t.Errorf("agent fields: %+v", a)
	}
	if a.Position == nil || a.Position.X != 1 || a.Position.Y != 2 {
		t.Errorf("agent position: %+v", a.Position)
	}
	if a.State != "hunting" {
		t.Errorf("agent state: got %q", a.State)
	}
	if len(a.Memory) != 1 || a.Memory[0].Type != simulations.AgentMemoryDecision || a.Memory[0].Summary != "hunt" {
		t.Errorf("agent memory: %+v", a.Memory)
	}

	// Environment.
	if out.Environment == nil {
		t.Fatal("Environment is nil after round-trip")
	}
	env := out.Environment
	if env.Width != 100 || env.Height != 100 || !env.Torus {
		t.Errorf("environment scalars: %+v", env)
	}
	if len(env.Shapes) != 1 {
		t.Fatalf("Shapes len: got %d want 1", len(env.Shapes))
	}
	s := env.Shapes[0]
	if s.ID != 1 || s.ShapeUID != "forest" || s.Type != models.ShapeTypeRectangle || !s.Visible {
		t.Errorf("shape fields: %+v", s)
	}
	if s.Position == nil || s.Position.X != 10 || s.Position.Y != 20 {
		t.Errorf("shape position: %+v", s.Position)
	}

	// Token usage — inline embedding round-trip.
	if out.TokenUsage == nil {
		t.Fatal("TokenUsage is nil after round-trip")
	}
	tu := out.TokenUsage
	if tu.Total.TotalTokens != 30 || tu.Total.Calls != 3 {
		t.Errorf("Total: %+v", tu.Total)
	}
	if len(tu.ByAgent) != 1 {
		t.Fatalf("ByAgent len: got %d want 1", len(tu.ByAgent))
	}
	if tu.ByAgent[0].AgentUID != "predator" || tu.ByAgent[0].TotalTokens != 20 || tu.ByAgent[0].Calls != 2 {
		t.Errorf("ByAgent[0] inline fields: %+v", tu.ByAgent[0])
	}
	if len(tu.ByPurpose) != 1 {
		t.Fatalf("ByPurpose len: got %d want 1", len(tu.ByPurpose))
	}
	if tu.ByPurpose[0].Purpose != simulations.LLMUsageSimulationDecision || tu.ByPurpose[0].TotalTokens != 20 || tu.ByPurpose[0].Calls != 2 {
		t.Errorf("ByPurpose[0] inline fields: %+v", tu.ByPurpose[0])
	}

	// Deep-equal check for StatusHistory (exercises nested struct tags).
	if !reflect.DeepEqual(out.StatusHistory, in.StatusHistory) {
		t.Errorf("StatusHistory mismatch: got %+v want %+v", out.StatusHistory, in.StatusHistory)
	}
}

// TestSimulationStepMetadataRoundTrip verifies that a SimulationStepDocument
// survives a bson.Marshal + bson.Unmarshal round-trip without loss.
func TestSimulationStepMetadataRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	in := SimulationStepDocument{
		Metadata: SimulationStepDocumentMetadata{
			SimulationID: bson.NewObjectID(),
			Step:         3,
			ReferenceID:  1,
			Type:         simulations.MetadataTypeAgent,
			UID:          "predator",
		},
		Timestamp: now,
		Position:  &mongocommon.PointDocument{X: 4, Y: 7},
		Properties: map[string]interface{}{
			"energy": float64(50),
		},
	}

	raw, err := bson.Marshal(in)
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	var out SimulationStepDocument
	if err := bson.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}

	if out.Metadata.Step != in.Metadata.Step {
		t.Errorf("Step: got %d want %d", out.Metadata.Step, in.Metadata.Step)
	}
	if out.Metadata.ReferenceID != in.Metadata.ReferenceID {
		t.Errorf("ReferenceID: got %d want %d", out.Metadata.ReferenceID, in.Metadata.ReferenceID)
	}
	if out.Metadata.Type != in.Metadata.Type {
		t.Errorf("Type: got %q want %q", out.Metadata.Type, in.Metadata.Type)
	}
	if out.Metadata.UID != in.Metadata.UID {
		t.Errorf("UID: got %q want %q", out.Metadata.UID, in.Metadata.UID)
	}
	if out.Metadata.SimulationID != in.Metadata.SimulationID {
		t.Errorf("SimulationID: got %v want %v", out.Metadata.SimulationID, in.Metadata.SimulationID)
	}
	if out.Position == nil || out.Position.X != in.Position.X || out.Position.Y != in.Position.Y {
		t.Errorf("Position: got %+v want %+v", out.Position, in.Position)
	}
	if v, ok := out.Properties["energy"]; !ok || v != float64(50) {
		t.Errorf("Properties[energy]: got %v", out.Properties["energy"])
	}
}
