package simulations

import (
	"testing"
	"time"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/pkg/httpapi"
)

// TestStepsToDTOs_TwoAgentsAndEnvironment folds per-entity timeseries rows of one
// step into a single StepDTO with both agents (sorted by id) and the environment's
// global properties, asserting field parity.
func TestStepsToDTOs_TwoAgentsAndEnvironment(t *testing.T) {
	pos1 := common.Point{X: 10, Y: 20}
	pos2 := common.Point{X: 30, Y: 40}
	rows := []datasims.SimulationStep{
		// Intentionally out of id order to prove the mapper sorts by ReferenceID.
		{
			Metadata:   datasims.StepMetadata{Step: 7, ReferenceID: 2, Type: datasims.MetadataTypeAgent, UID: "wolf"},
			Position:   &pos2,
			Properties: map[string]interface{}{"energy": 5.0},
			State:      "hunting",
			Decision:   "chase prey",
		},
		{
			Metadata:   datasims.StepMetadata{Step: 7, ReferenceID: 1, Type: datasims.MetadataTypeAgent, UID: "wolf"},
			Position:   &pos1,
			Properties: map[string]interface{}{"energy": 9.0},
			State:      "idle",
		},
		{
			Metadata:   datasims.StepMetadata{Step: 7, ReferenceID: 0, Type: datasims.MetadataTypeEnvironment, UID: "environment"},
			Properties: map[string]interface{}{"food": 100.0},
			Statistics: &datasims.SimulationStatistics{StepsCompleted: 7, TotalAgents: 2},
		},
	}

	steps := stepsToDTOs(rows)
	if len(steps) != 1 {
		t.Fatalf("steps=%d want 1 (single step number grouped)", len(steps))
	}
	dto := steps[0]
	if dto.Step != 7 {
		t.Fatalf("Step=%d want 7", dto.Step)
	}
	if len(dto.Agents) != 2 {
		t.Fatalf("agents=%d want 2 (environment row excluded)", len(dto.Agents))
	}

	// Sorted ascending by id: agent 1 first.
	a1 := dto.Agents[0]
	if a1.ID != 1 {
		t.Fatalf("first agent id=%d want 1 (sorted)", a1.ID)
	}
	if a1.AgentUID != "wolf" {
		t.Fatalf("agent 1 AgentUID=%q want wolf (archetype UID surfaced for replay)", a1.AgentUID)
	}
	if a1.Position == nil || a1.Position.X != 10 || a1.Position.Y != 20 {
		t.Fatalf("agent 1 position=%+v want {10,20}", a1.Position)
	}
	if a1.State != "idle" {
		t.Fatalf("agent 1 state=%q want idle (from typed State field)", a1.State)
	}
	if a1.Properties["energy"] != 9.0 {
		t.Fatalf("agent 1 energy=%v want 9", a1.Properties["energy"])
	}

	a2 := dto.Agents[1]
	if a2.ID != 2 || a2.State != "hunting" {
		t.Fatalf("agent 2 id/state=%d/%q want 2/hunting", a2.ID, a2.State)
	}
	if a2.Decision != "chase prey" {
		t.Fatalf("agent 2 Decision=%q want %q (per-step LLM decision surfaced for replay)", a2.Decision, "chase prey")
	}
	if a1.Decision != "" {
		t.Fatalf("agent 1 Decision=%q want empty (no decision this step)", a1.Decision)
	}
	if a2.Position == nil || a2.Position.X != 30 || a2.Position.Y != 40 {
		t.Fatalf("agent 2 position=%+v want {30,40}", a2.Position)
	}

	if dto.GlobalProperties == nil || dto.GlobalProperties["food"] != 100.0 {
		t.Fatalf("global properties=%+v want {food:100}", dto.GlobalProperties)
	}

	// The environment row's Statistics snapshot is surfaced on the StepDTO so
	// replay can rebuild statistics-driven widgets.
	st, ok := dto.Statistics.(*datasims.SimulationStatistics)
	if !ok || st == nil {
		t.Fatalf("statistics=%+v want *SimulationStatistics", dto.Statistics)
	}
	if st.StepsCompleted != 7 || st.TotalAgents != 2 {
		t.Fatalf("statistics=%+v want {StepsCompleted:7, TotalAgents:2}", st)
	}
}

// TestStepsToDTOs_MultipleStepsOrderedAscending proves rows spanning several step
// numbers fold into one StepDTO each, ordered ascending regardless of input order.
func TestStepsToDTOs_MultipleStepsOrderedAscending(t *testing.T) {
	rows := []datasims.SimulationStep{
		{Metadata: datasims.StepMetadata{Step: 3, ReferenceID: 1, Type: datasims.MetadataTypeAgent}},
		{Metadata: datasims.StepMetadata{Step: 1, ReferenceID: 1, Type: datasims.MetadataTypeAgent}},
		{Metadata: datasims.StepMetadata{Step: 2, ReferenceID: 1, Type: datasims.MetadataTypeAgent}},
	}
	steps := stepsToDTOs(rows)
	if len(steps) != 3 {
		t.Fatalf("steps=%d want 3", len(steps))
	}
	if steps[0].Step != 1 || steps[1].Step != 2 || steps[2].Step != 3 {
		t.Fatalf("step order=%d,%d,%d want 1,2,3", steps[0].Step, steps[1].Step, steps[2].Step)
	}
}

func TestStepsToDTOs_ShapesIncluded(t *testing.T) {
	rows := []datasims.SimulationStep{
		{Metadata: datasims.StepMetadata{Step: 2, ReferenceID: 1, Type: datasims.MetadataTypeAgent, UID: "wolf"}, Properties: map[string]interface{}{"energy": 5}},
		{Metadata: datasims.StepMetadata{Step: 2, ReferenceID: 9, Type: datasims.MetadataTypeShape, UID: "lake"}, Properties: map[string]interface{}{"water_level": 42.0}},
		{Metadata: datasims.StepMetadata{Step: 2, ReferenceID: 4, Type: datasims.MetadataTypeShape, UID: "lake"}, Properties: map[string]interface{}{"water_level": 40.0}},
	}
	out := stepsToDTOs(rows)
	if len(out) != 1 {
		t.Fatalf("steps=%d want 1", len(out))
	}
	if len(out[0].Shapes) != 2 || out[0].Shapes[0].ID != 4 || out[0].Shapes[1].ID != 9 {
		t.Fatalf("shapes wrong (want id-sorted 4,9): %+v", out[0].Shapes)
	}
	if out[0].Shapes[1].Properties["water_level"] != 42.0 {
		t.Fatalf("shape properties lost: %+v", out[0].Shapes[1])
	}
}

// TestStepsToDTOs_Empty returns nil for no rows (mirrors the other nil-safe mappers).
func TestStepsToDTOs_Empty(t *testing.T) {
	if got := stepsToDTOs(nil); got != nil {
		t.Fatalf("stepsToDTOs(nil)=%v want nil", got)
	}
	if got := stepsToDTOs([]datasims.SimulationStep{}); got != nil {
		t.Fatalf("stepsToDTOs(empty)=%v want nil", got)
	}
}

// TestExecutionRunToDTO_FieldParity maps an ExecutionRun → ExecutionRunDTO and
// asserts every field carries over (incl. the optional FinishedAt pointer).
func TestExecutionRunToDTO_FieldParity(t *testing.T) {
	started := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	finished := started.Add(90 * time.Second)
	run := datasims.ExecutionRun{
		Seq:            2,
		StartedAt:      started,
		FinishedAt:     &finished,
		StepsCompleted: 42,
		Status:         datasims.ExecutionRunFinished,
		Error:          "",
	}
	dto := executionRunToDTO(run)
	if dto.Seq != 2 {
		t.Fatalf("Seq=%d want 2", dto.Seq)
	}
	if !dto.StartedAt.Equal(started) {
		t.Fatalf("StartedAt=%v want %v", dto.StartedAt, started)
	}
	if dto.FinishedAt == nil || !dto.FinishedAt.Equal(finished) {
		t.Fatalf("FinishedAt=%v want %v", dto.FinishedAt, finished)
	}
	if dto.StepsCompleted != 42 {
		t.Fatalf("StepsCompleted=%d want 42", dto.StepsCompleted)
	}
	if dto.Status != string(datasims.ExecutionRunFinished) {
		t.Fatalf("Status=%q want %q", dto.Status, datasims.ExecutionRunFinished)
	}
}

// TestExecutionRunsToDTO_PreservesOrderAndNil checks the slice mapper preserves
// order and returns nil for nil input.
func TestExecutionRunsToDTO_PreservesOrderAndNil(t *testing.T) {
	if got := executionRunsToDTO(nil); got != nil {
		t.Fatalf("executionRunsToDTO(nil)=%v want nil", got)
	}
	runs := []datasims.ExecutionRun{
		{Seq: 1, Status: datasims.ExecutionRunStopped},
		{Seq: 2, Status: datasims.ExecutionRunRunning},
	}
	dtos := executionRunsToDTO(runs)
	if len(dtos) != 2 || dtos[0].Seq != 1 || dtos[1].Seq != 2 {
		t.Fatalf("execution runs DTO=%+v want seq 1 then 2", dtos)
	}
	if dtos[0].Status != "stopped" || dtos[1].Status != "running" {
		t.Fatalf("statuses=%q,%q want stopped,running", dtos[0].Status, dtos[1].Status)
	}
}

// TestControlRequest_OneofValidation exercises the validate tags via the shared
// GoPlaygroundValidator: valid actions pass; an out-of-set action and an
// out-of-range speed fail.
func TestControlRequest_OneofValidation(t *testing.T) {
	v := httpapi.NewGoPlaygroundValidator()

	for _, action := range []string{"pause", "resume", "stop"} {
		if err := v.Validate(ControlRequest{Action: action}); err != nil {
			t.Fatalf("action %q should be valid, got %v", action, err)
		}
	}

	if err := v.Validate(ControlRequest{Action: "explode"}); err == nil {
		t.Fatal("action 'explode' should fail oneof validation")
	}
	if err := v.Validate(ControlRequest{}); err == nil {
		t.Fatal("empty action should fail required validation")
	}

	speed := 250
	if err := v.Validate(ControlRequest{Action: "pause", SpeedMs: &speed}); err != nil {
		t.Fatalf("in-range speed should pass, got %v", err)
	}
	tooFast := -1
	if err := v.Validate(ControlRequest{Action: "pause", SpeedMs: &tooFast}); err == nil {
		t.Fatal("negative speed should fail min validation")
	}
	tooSlow := 10001
	if err := v.Validate(ControlRequest{Action: "pause", SpeedMs: &tooSlow}); err == nil {
		t.Fatal("speed > 10000 should fail max validation")
	}
}

// TestControlRequest_SetSpeedValidation covers the dedicated set_speed action
// (ISS-09): it is a legal oneof value, requires speed_ms (required_if), and
// speed_ms is still range-checked when present; other actions remain
// unaffected by the required_if (speed_ms stays optional for them).
func TestControlRequest_SetSpeedValidation(t *testing.T) {
	v := httpapi.NewGoPlaygroundValidator()

	if err := v.Validate(ControlRequest{Action: "set_speed"}); err == nil {
		t.Fatal("set_speed with no speed_ms should fail required_if validation")
	}

	speed := 250
	if err := v.Validate(ControlRequest{Action: "set_speed", SpeedMs: &speed}); err != nil {
		t.Fatalf("set_speed with an in-range speed_ms should pass, got %v", err)
	}

	tooFast := -1
	if err := v.Validate(ControlRequest{Action: "set_speed", SpeedMs: &tooFast}); err == nil {
		t.Fatal("set_speed with a negative speed_ms should still fail min validation")
	}
	tooSlow := 10001
	if err := v.Validate(ControlRequest{Action: "set_speed", SpeedMs: &tooSlow}); err == nil {
		t.Fatal("set_speed with speed_ms > 10000 should still fail max validation")
	}

	// pause/resume/stop remain unaffected by required_if: speed_ms stays optional.
	if err := v.Validate(ControlRequest{Action: "pause"}); err != nil {
		t.Fatalf("pause with no speed_ms should still pass, got %v", err)
	}
}

// TestStreamTicketResponseShape is a compile/shape guard for the ticket response.
func TestStreamTicketResponseShape(t *testing.T) {
	exp := time.Now().Add(30 * time.Second)
	r := StreamTicketResponse{Ticket: "abc123", ExpiresAt: exp}
	if r.Ticket != "abc123" || !r.ExpiresAt.Equal(exp) {
		t.Fatalf("StreamTicketResponse=%+v unexpected", r)
	}
	// RunRequest is intentionally empty; ensure it remains constructible.
	_ = RunRequest{}
}
