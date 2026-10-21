package simulations

import (
	"time"

	"intelli.sim.core/data/simulations"
)

// GenerationRunDocument is the audit record of one generation attempt.
type GenerationRunDocument struct {
	Seq        uint64                          `bson:"seq"`
	Seed       uint64                          `bson:"seed"`
	Scope      GenerationScopeDocument         `bson:"scope"`
	Status     simulations.GenerationRunStatus `bson:"status"`
	StartedAt  time.Time                       `bson:"started_at"`
	FinishedAt *time.Time                      `bson:"finished_at,omitempty"`
	Error      string                          `bson:"error,omitempty"`
	Warnings   []GenerationWarningDocument     `bson:"warnings,omitempty"`
	Requests   uint                            `bson:"requests"`
}

// GenerationScopeDocument selects what a generation run (re)creates.
type GenerationScopeDocument struct {
	Kind      simulations.GenerationScopeKind `bson:"kind"`
	AgentUIDs []string                        `bson:"agent_uids,omitempty"`
	Parts     []simulations.GenerationPart    `bson:"parts,omitempty"`
}

// GenerationWarningDocument records a non-fatal degradation during generation.
type GenerationWarningDocument struct {
	Path   string `bson:"path"`
	Reason string `bson:"reason"`
}
