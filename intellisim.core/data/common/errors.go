package common

import "errors"

// Storage-agnostic sentinel errors. Repository implementations translate their
// driver-specific errors into these; consumers test with errors.Is.
var (
	// ErrNotFound is returned when the requested entity does not exist.
	ErrNotFound = errors.New("entity not found")
	// ErrInvalidID is returned when an entity id is malformed (e.g. not a valid hex id).
	ErrInvalidID = errors.New("invalid entity id")
)
