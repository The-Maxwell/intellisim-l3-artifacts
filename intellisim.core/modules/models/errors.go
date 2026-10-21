package models

import "errors"

var (
	ErrModelNotFound        = errors.New("model not found")
	ErrNoChanges            = errors.New("update request has no changes")
	ErrGenerationInProgress = errors.New("a generation is already in progress for this model")
	ErrModelUnavailable     = errors.New("the selected LLM is not available")
	ErrEffortNotAllowed     = errors.New("the requested effort is not allowed for this model")
	ErrTargetCapability     = errors.New("the selected LLM does not support the required capability")
	ErrInvalidCursor        = errors.New("cursor message does not belong to this model chat")
	ErrNotSpatial           = errors.New("environment operations require a spatial model")
	ErrNoChat               = errors.New("model has no companion chat")
	// ErrSimGenTargetRequired / ErrSimExecTargetRequired are raised on model save when the
	// model uses the LLM for a generation or execution aspect but the corresponding
	// SimulationConfig LLM target is unset.
	ErrSimGenTargetRequired  = errors.New("a simulation-generation LLM target is required because the model uses the LLM to seed the simulation")
	ErrSimExecTargetRequired = errors.New("a simulation-execution LLM target is required because the model uses the LLM during the run")
)
