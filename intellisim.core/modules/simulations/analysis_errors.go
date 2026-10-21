package simulations

import "errors"

var (
	// ErrAnalysisNotFound is returned when an analysis id is missing or belongs to another simulation.
	ErrAnalysisNotFound = errors.New("analysis not found")
	// ErrNotAnalyzable is returned when the simulation is not finished/paused.
	ErrNotAnalyzable = errors.New("simulation is not in an analyzable state")
	// ErrNoResults is returned when the simulation has no recorded steps to analyze.
	ErrNoResults = errors.New("simulation has no recorded results to analyze")
	// ErrAnalysisNotReady is returned when the PDF is requested before completion.
	ErrAnalysisNotReady = errors.New("analysis is not completed yet")
	// ErrAllLLMCallsFailed is returned when no section or synthesis call yielded usable output.
	ErrAllLLMCallsFailed = errors.New("analysis failed: no usable LLM output")
)
