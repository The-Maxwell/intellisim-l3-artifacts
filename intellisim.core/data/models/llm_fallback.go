package models

// FallbackPolicyType defines what to do when LLM calls fail
type FallbackPolicyType string

const (
	FallbackRetry        FallbackPolicyType = "retry"
	FallbackRulesOnly    FallbackPolicyType = "rules_only"
	FallbackLastDecision FallbackPolicyType = "last_decision"
	FallbackSkip         FallbackPolicyType = "skip"
)

type FallbackStep struct {
	Type         FallbackPolicyType
	MaxRetries   uint
	RetryDelayMs uint
}

type FallbackPolicy struct {
	Steps []FallbackStep
}
