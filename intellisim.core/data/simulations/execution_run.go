package simulations

import "time"

// ExecutionRunStatus is the lifecycle of one engine run attempt.
type ExecutionRunStatus string

const (
	ExecutionRunRunning  ExecutionRunStatus = "running"
	ExecutionRunFinished ExecutionRunStatus = "finished"
	ExecutionRunStopped  ExecutionRunStatus = "stopped"
	ExecutionRunFailed   ExecutionRunStatus = "failed"
)

// ExecutionRun is the audit record of one engine run, symmetric to GenerationRun.
// Seq is 1-based and matches index Seq-1 in SimulationTokenUsage.ByRun.
type ExecutionRun struct {
	Seq            uint64
	StartedAt      time.Time
	FinishedAt     *time.Time
	StepsCompleted uint64
	Status         ExecutionRunStatus
	Error          string
	// ActiveDuration is the wall-clock time this run spent actually running,
	// EXCLUDING paused intervals (StartedAt→FinishedAt minus the paused gaps in
	// StatusHistory). Computed once at finalize (and crash-reconcile); zero for a
	// run that never finalized or that finished before this field existed.
	ActiveDuration time.Duration
	// TerminationReason records WHY a finished run ended (U2):
	// TerminationConditionMet when the simulation's Termination condition fired,
	// TerminationMaxSteps when the configured step count was reached. Empty for
	// stopped/failed runs and for runs that predate the field.
	TerminationReason string
}

// TerminationReason values recorded on finished ExecutionRuns (U2).
const (
	TerminationConditionMet = "condition_met"
	TerminationMaxSteps     = "max_steps"
)
