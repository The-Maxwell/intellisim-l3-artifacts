package simulations

import (
	"context"
	"time"
)

// ListFilter narrows, sorts, and pages a simulation listing. Empty CreatedBy =
// no owner filter (admin listing); empty ModelIDs = any model (a single id
// matches exactly, several match via $in). Page is 1-based.
type ListFilter struct {
	CreatedBy string
	ModelIDs  []string
	// Statuses restricts the listing to simulations in any of the given
	// statuses. Empty = no status constraint. Used by the startup run
	// reconcile to fetch only running/paused simulations instead of paging
	// through the whole collection.
	Statuses []SimulationStatus
	Page     int
	PageSize int
	// SortBy is a user-facing column token (name|model_name|status|steps|
	// agent_count|total_tokens|updated_at|created_by). Empty or unrecognized
	// values fall back to updated_at. SortOrder is "asc"|"desc"; anything other
	// than "asc" is treated as descending.
	SortBy    string
	SortOrder string
}

// SimulationSummary is the lightweight projection returned by
// SimulationsRepository.List. It avoids loading the full agent slice and
// model snapshot for listing purposes.
type SimulationSummary struct {
	ID          string
	Name        string
	Description string
	ModelID     string
	// ModelName is taken from ModelSnapshot.Name at projection time.
	ModelName   string
	Status      SimulationStatus
	Steps       uint64
	AgentCount  int
	TotalTokens uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
}

// SimulationsRepository is the storage-agnostic persistence port for
// simulation documents. Implementations live under data/mongodb. Not-found
// conditions are reported as common.ErrNotFound by GetByID; malformed ids as
// common.ErrInvalidID. Update and Delete are idempotent: targeting an id that
// does not exist is not an error.
type SimulationsRepository interface {
	Create(ctx context.Context, sim Simulation) (string, error)
	GetByID(ctx context.Context, id string) (*Simulation, error)
	Update(ctx context.Context, id string, sim Simulation) error
	Delete(ctx context.Context, id string) error
	// List returns one page of simulations (sorted per filter, default updated_at
	// desc) and whether more pages exist after it.
	List(ctx context.Context, f ListFilter) ([]SimulationSummary, bool, error)
	// Count returns the total number of simulations matching the filter, ignoring
	// paging and sort.
	Count(ctx context.Context, f ListFilter) (int64, error)
}

// SimulationStepsRepository is the storage-agnostic port for the per-step
// timeseries ("simulation_steps") — persistence plus the Level-3 analysis
// queries. agentUIDs/agentUID parameters refer to model definition UIDs
// (StepMetadata.UID).
type SimulationStepsRepository interface {
	SaveSteps(ctx context.Context, steps []SimulationStep) error
	GetStepRange(ctx context.Context, simulationID string) (min, max int64, err error)
	// GetSteps returns persisted steps in [from, to] (inclusive), ordered by step
	// asc, paginated (page is 1-based); hasMore reports whether further pages exist.
	GetSteps(ctx context.Context, simID string, from, to, page, pageSize uint64) ([]SimulationStep, bool, error)
	// DeleteBySimulation removes every persisted step row for a simulation (cascade on delete).
	DeleteBySimulation(ctx context.Context, simulationID string) error

	AggregatePropertyByStep(ctx context.Context, simulationID, propertyUID string, agentUIDs []string, stepRange *StepRange, aggregation Aggregation) ([]TimeSeriesPoint, error)
	CountByAgentTypePerStep(ctx context.Context, simulationID string, stepRange *StepRange) ([]PopulationPoint, error)
	FindSpatialAtStep(ctx context.Context, simulationID string, step int64, propertyUIDs []string) ([]SpatialAgentData, error)
	FindDistinctProperties(ctx context.Context, simulationID string) ([]PropertyInfo, error)
	// AggregateShapePropertyByStep mirrors AggregatePropertyByStep for shape rows
	// (metadata.type == "shape"); shapeUIDs filter by shape template UID (empty = all).
	AggregateShapePropertyByStep(ctx context.Context, simulationID, propertyUID string, shapeUIDs []string, stepRange *StepRange, aggregation Aggregation) ([]TimeSeriesPoint, error)
	// FindDistinctShapeProperties mirrors FindDistinctProperties for shape rows.
	FindDistinctShapeProperties(ctx context.Context, simulationID string) ([]PropertyInfo, error)
}
