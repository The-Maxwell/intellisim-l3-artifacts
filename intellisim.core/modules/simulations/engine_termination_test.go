package simulations

import (
	"context"
	"testing"
	"time"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// U2: a run with a termination condition stops the moment it evaluates true at a
// step boundary (here: step >= 2 with Steps=100), finishing early with
// TerminationReason condition_met and the early step count persisted everywhere.
func TestRunManager_EarlyTermination(t *testing.T) {
	withFixedClock(t)
	sim := runnableSim("sim-term", 100, 2)
	sim.Termination = &models.Condition{Type: models.ConditionTypeStep, Operator: models.ConditionOpGreaterOrEq, Value: 2.0}
	repo := newFakeSimRepo(sim)
	b := newFakeBroadcaster()
	m := newTestManager(repo, &fakeStepSink{}, b)

	if err := m.Start(context.Background(), sim); err != nil {
		t.Fatalf("Start: %v", err)
	}
	b.waitDone(t, 2*time.Second)

	got := repo.snapshot("sim-term")
	if got.Status != datasims.SimulationStatusFinished {
		t.Fatalf("status=%q want finished", got.Status)
	}
	if got.CurrentStep != 2 {
		t.Fatalf("CurrentStep=%d want 2 (early stop, not the configured 100)", got.CurrentStep)
	}
	run := got.ExecutionRuns[0]
	if run.Status != datasims.ExecutionRunFinished {
		t.Fatalf("run status=%q want finished", run.Status)
	}
	if run.StepsCompleted != 2 {
		t.Fatalf("run StepsCompleted=%d want 2", run.StepsCompleted)
	}
	if run.TerminationReason != datasims.TerminationConditionMet {
		t.Fatalf("run TerminationReason=%q want %q", run.TerminationReason, datasims.TerminationConditionMet)
	}
}

// U2: a run without a termination condition behaves exactly as before and records
// max_steps as its termination reason.
func TestRunManager_MaxStepsTerminationReason(t *testing.T) {
	withFixedClock(t)
	sim := runnableSim("sim-max", 3, 1)
	repo := newFakeSimRepo(sim)
	b := newFakeBroadcaster()
	m := newTestManager(repo, &fakeStepSink{}, b)

	if err := m.Start(context.Background(), sim); err != nil {
		t.Fatalf("Start: %v", err)
	}
	b.waitDone(t, 2*time.Second)

	got := repo.snapshot("sim-max")
	if got.CurrentStep != 3 {
		t.Fatalf("CurrentStep=%d want 3", got.CurrentStep)
	}
	if run := got.ExecutionRuns[0]; run.TerminationReason != datasims.TerminationMaxSteps {
		t.Fatalf("run TerminationReason=%q want %q", run.TerminationReason, datasims.TerminationMaxSteps)
	}
}
