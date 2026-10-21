package common

// TokenUsage is the shared, serialisable accounting of LLM token consumption for a
// SINGLE (provider, model, effort) target. The embedded LLMTarget records what was
// actually called, so usage stays attributable even when the model, provider, or effort
// changes within one chat or simulation run (effort directly drives token consumption).
type TokenUsage struct {
	LLMTarget

	PromptTokens     uint64 `json:"prompt_tokens"`
	CompletionTokens uint64 `json:"completion_tokens"`
	// ReasoningTokens is the reasoning portion of CompletionTokens (a SUBSET,
	// already included in CompletionTokens/TotalTokens); reported for cost
	// insight into reasoning models only. Never add it into the totals.
	ReasoningTokens uint64  `json:"reasoning_tokens"`
	TotalTokens     uint64  `json:"total_tokens"`
	Calls           uint64  `json:"calls"`
	LatencyMs       float64 `json:"latency_ms"` // cumulative wall-clock ms across all Calls
}

// TokenUsageSummary aggregates usage that may span MULTIPLE targets (the model/provider/
// effort can change mid-chat or mid-run). Total is the grand total; ByModel preserves the
// per-target breakdown so no attribution is lost.
type TokenUsageSummary struct {
	Total   TokenUsage   `json:"total"`    // grand aggregate; LLMTarget always empty
	ByModel []TokenUsage `json:"by_model"` // one entry per (provider, model, effort)
}

// Accumulate adds src's numeric fields into dst; it does not touch LLMTarget.
func Accumulate(dst *TokenUsage, src TokenUsage) {
	dst.PromptTokens += src.PromptTokens
	dst.CompletionTokens += src.CompletionTokens
	dst.ReasoningTokens += src.ReasoningTokens
	dst.TotalTokens += src.TotalTokens
	dst.Calls += src.Calls
	dst.LatencyMs += src.LatencyMs
}

// Add folds one call's usage into the summary: the grand total accumulates
// with an empty LLMTarget, and the per-target breakdown gains/extends the
// entry matching u's (provider, model, effort).
func (s *TokenUsageSummary) Add(u TokenUsage) {
	Accumulate(&s.Total, u)
	s.Total.LLMTarget = LLMTarget{}
	for i := range s.ByModel {
		t := s.ByModel[i].LLMTarget
		// Match on routing identity only — Generation/TimeoutMs are per-call
		// tuning knobs, not part of what makes two calls "the same model", so
		// they are intentionally excluded (LLMTarget is also no longer
		// comparable with == now that Generation is a map).
		if t.ProviderUID == u.ProviderUID && t.ModelUID == u.ModelUID && t.Effort == u.Effort {
			Accumulate(&s.ByModel[i], u)
			return
		}
	}
	s.ByModel = append(s.ByModel, u)
}
