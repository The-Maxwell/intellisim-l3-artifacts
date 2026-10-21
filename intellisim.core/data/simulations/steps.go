package simulations

import (
	"time"

	"intelli.sim.core/data/common"
)

type MetadataType string

const (
	MetadataTypeAgent       MetadataType = "agent"
	MetadataTypeShape       MetadataType = "shape" // per-step state of recordable shapes (mutable properties / LLM shape strategy)
	MetadataTypeEnvironment MetadataType = "environment"
)

// SimulationStep is one per-entity row of the simulation timeseries.
type SimulationStep struct {
	ID         string                 `json:"id"`
	Metadata   StepMetadata           `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
	Position   *common.Point          `json:"position,omitempty"`
	Properties map[string]interface{} `json:"properties"`
	State      string                 `json:"state,omitempty"`
	// Decision is the compact summary of the LLM decision applied to this agent
	// on this step (decision label(s), or the action summary for full-control
	// strategies). Present ONLY on agent rows and ONLY on steps where the agent
	// received an LLM decision; empty otherwise. Feeds the text_log widget's
	// "llm_decisions" kind live (WS AgentFrame.decision) and in replay.
	Decision string `json:"decision,omitempty"`
	// Statistics is the accumulated run-history statistics snapshot at this step.
	// Carried ONLY on the per-step environment row (Type == MetadataTypeEnvironment)
	// so replay can rebuild statistics-driven widgets (the text_log "events" kind)
	// with the same numbers the live WS frame streamed. Nil on agent/shape rows.
	Statistics *SimulationStatistics `json:"statistics,omitempty"`
}

// StepMetadata identifies a per-entity step row.
//
//   - UID is the MODEL DEFINITION UID of the entity: Agent.UID for agent rows, the shape
//     template UID for shape rows, and "environment" for environment rows. Per-archetype
//     aggregations filter on Type + UID.
//   - ReferenceID is the runtime instance id, unique within the agents/shapes of the
//     simulation.
type StepMetadata struct {
	SimulationID string       `json:"simulation_id"`
	Step         int64        `json:"step"`
	ReferenceID  uint64       `json:"reference_id"`
	Type         MetadataType `json:"type"`
	UID          string       `json:"uid"`
}
