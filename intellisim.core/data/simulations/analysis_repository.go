package simulations

import (
	"context"
	"io"
)

// AnalysesRepository is the storage-agnostic port for analysis records
// (the "simulation_analyses" collection). Not-found is common.ErrNotFound;
// malformed ids are common.ErrInvalidID.
type AnalysesRepository interface {
	Create(ctx context.Context, a Analysis) (string, error)
	GetByID(ctx context.Context, id string) (*Analysis, error)
	Update(ctx context.Context, id string, a Analysis) error
	Delete(ctx context.Context, id string) error
	// ListBySimulation returns all analyses for a simulation, newest first.
	ListBySimulation(ctx context.Context, simulationID string) ([]Analysis, error)
	// DeleteBySimulationID removes all analysis records for a simulation (cascade).
	DeleteBySimulationID(ctx context.Context, simulationID string) error
	// FindUnfinished returns every analysis still in a non-terminal status
	// (pending/running) — used for crash-reconcile on startup.
	FindUnfinished(ctx context.Context) ([]Analysis, error)
}

// AnalysisFileInfo is the metadata returned when opening a stored PDF.
type AnalysisFileInfo struct {
	Length int64
}

// AnalysisFileStore is the storage-agnostic port for analysis PDF bytes (GridFS).
type AnalysisFileStore interface {
	// Upload stores data and returns the new file's hex id, the stored size, and an error.
	// metadata is attached so files can be found/cleaned up by simulation/analysis.
	Upload(ctx context.Context, name, mime, simulationID, analysisID string, data []byte) (gridfsID string, size int64, err error)
	// Open returns a reader over the stored file plus its info. Caller closes the reader.
	Open(ctx context.Context, gridfsID string) (io.ReadCloser, *AnalysisFileInfo, error)
	Delete(ctx context.Context, gridfsID string) error
	// DeleteBySimulation removes every stored file tagged with simulationID (cascade).
	DeleteBySimulation(ctx context.Context, simulationID string) error
}
