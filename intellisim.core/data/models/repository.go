package models

import (
	"context"
)

// ListFilter narrows, sorts, and pages model listing. Empty CreatedBy = no owner
// filter (admin listing).
type ListFilter struct {
	CreatedBy string
	Page      int // 1-based; values < 1 are treated as 1
	PageSize  int // values < 1 default to 20
	// SortBy is a user-facing column token (name|updated_at|created_by). Empty or
	// unrecognized values fall back to updated_at. SortOrder is "asc"|"desc";
	// anything other than "asc" is treated as descending.
	SortBy    string
	SortOrder string
}

// ModelOption is the minimal id+name projection used to populate selector UIs
// (e.g. the simulations "filter by model" control) without paging or loading
// the full model body.
type ModelOption struct {
	ID   string
	Name string
}

// ModelsRepository is the storage-agnostic persistence port for ABM model
// definitions. Implementations live under data/mongodb. Not-found conditions are
// reported as common.ErrNotFound by GetByID; malformed ids as common.ErrInvalidID.
// Update and Delete are idempotent: targeting an id that does not exist is not
// an error.
type ModelsRepository interface {
	Create(ctx context.Context, model Model) (string, error)
	GetByID(ctx context.Context, id string) (*Model, error)
	Update(ctx context.Context, id string, model Model) error
	Delete(ctx context.Context, id string) error
	// List returns one page of models (sorted per filter, default updated_at desc)
	// and whether more pages exist after it.
	List(ctx context.Context, filter ListFilter) ([]Model, bool, error)
	// Count returns the total number of models matching the filter, ignoring
	// paging and sort.
	Count(ctx context.Context, filter ListFilter) (int64, error)
	// ListOptions returns the id+name of every model for the given owner (empty
	// = all, admin listing), sorted by name, unpaginated — for selector UIs.
	ListOptions(ctx context.Context, createdBy string) ([]ModelOption, error)
}
