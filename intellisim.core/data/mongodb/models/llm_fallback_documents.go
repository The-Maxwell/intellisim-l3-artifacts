package models

import (
	"intelli.sim.core/data/models"
)

// FallbackStepDocument is the bson form of models.FallbackStep.
type FallbackStepDocument struct {
	Type         models.FallbackPolicyType `bson:"type"`
	MaxRetries   uint                      `bson:"max_retries,omitempty"`
	RetryDelayMs uint                      `bson:"retry_delay_ms,omitempty"`
}

// FallbackPolicyDocument is the bson form of models.FallbackPolicy.
type FallbackPolicyDocument struct {
	Steps []FallbackStepDocument `bson:"steps"`
}
