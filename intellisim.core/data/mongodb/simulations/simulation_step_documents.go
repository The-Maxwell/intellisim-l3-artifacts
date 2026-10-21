package simulations

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/data/simulations"
)

type SimulationStepDocument struct {
	ID         bson.ObjectID                  `bson:"_id,omitempty"`
	Metadata   SimulationStepDocumentMetadata `bson:"metadata"`
	Timestamp  time.Time                      `bson:"timestamp"`
	Position   *mongocommon.PointDocument     `bson:"position,omitempty"`
	Properties map[string]interface{}         `bson:"properties"`
	State      string                         `bson:"state,omitempty"`
	// Decision is the per-step LLM decision summary, carried only on agent rows
	// of steps where the agent received an LLM decision (see
	// simulations.SimulationStep.Decision).
	Decision string `bson:"decision,omitempty"`
	// Statistics is the accumulated statistics snapshot at this step, carried only
	// on the environment row (see simulations.SimulationStep.Statistics).
	Statistics *SimulationStatisticsDocument `bson:"statistics,omitempty"`
}

// SimulationStepDocumentMetadata identifies a per-entity step row.
//
//   - UID is the MODEL DEFINITION UID of the entity: Agent.UID for agent rows, the shape
//     template UID for shape rows, and "environment" for environment rows. Per-archetype
//     aggregations filter on Type + UID.
//   - ReferenceID is the runtime instance id, unique within the agents/shapes of the
//     simulation.
type SimulationStepDocumentMetadata struct {
	SimulationID bson.ObjectID            `bson:"simulation_id"`
	Step         int64                    `bson:"step"`
	ReferenceID  uint64                   `bson:"reference_id"` // runtime instance id, unique within the agents/shapes in simulation
	Type         simulations.MetadataType `bson:"type"`
	UID          string                   `bson:"uid"`
}
