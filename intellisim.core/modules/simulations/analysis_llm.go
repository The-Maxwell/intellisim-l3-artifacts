package simulations

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/modules/llm"
)

// analysisInvoker is the narrow LLM surface the analysis pipeline needs.
// llm.Service satisfies it.
type analysisInvoker interface {
	Reasoning(ctx context.Context, req llm.ReasoningRequest) (*llm.ReasoningCompletion, error)
	Simple(ctx context.Context, req llm.SimpleRequest) (*llm.Completion, error)
	ModelDescriptors(providerUID, modelUID string) ([]llm.ConfigParamDescriptor, bool, bool)
}

const analysisJSONReminder = "\n\nReturn ONLY the JSON object, with no markdown fences or commentary."

type analysisCaller struct {
	inv    analysisInvoker
	cfg    AnalysisConfig
	logger *zap.Logger
}

func newAnalysisCaller(inv analysisInvoker, cfg AnalysisConfig, logger *zap.Logger) *analysisCaller {
	return &analysisCaller{inv: inv, cfg: cfg, logger: logger.Named("analysis-llm")}
}

// callJSON makes a results-analysis LLM call, decodes strict JSON into out, and
// accumulates token usage. Picks Reasoning() when the model advertises reasoning,
// else Simple(). Retries up to cfg.MaxRetries on transport error or malformed JSON.
// Returns false (fail-soft) when no valid JSON was obtained.
func (c *analysisCaller) callJSON(ctx context.Context, target common.LLMTarget, system, user string, usage *common.TokenUsageSummary, out any) bool {
	if target.ProviderUID == "" || target.ModelUID == "" {
		return false
	}
	reasoning := false
	if _, adv, ok := c.inv.ModelDescriptors(target.ProviderUID, target.ModelUID); ok {
		reasoning = adv
	}
	userMsg := user + analysisJSONReminder

	attempts := c.cfg.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		text, u, ok := c.invokeOnce(ctx, target, reasoning, system, userMsg)
		if ok {
			accumulate(usage, u)
			if err := json.Unmarshal([]byte(extractJSON(text)), out); err == nil {
				return true
			}
			c.logger.Warn("analysis llm response not valid JSON; will retry if budget remains")
		}
		if ctx.Err() != nil {
			return false
		}
	}
	return false
}

func (c *analysisCaller) invokeOnce(ctx context.Context, target common.LLMTarget, reasoning bool, system, user string) (string, common.TokenUsage, bool) {
	callCtx := ctx
	if target.TimeoutMs == nil && c.cfg.Timeout() > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, c.cfg.Timeout())
		defer cancel()
	}
	if reasoning {
		resp, err := c.inv.Reasoning(callCtx, llm.ReasoningRequest{
			Target: target, Purpose: common.ModelCapabilityResultsAnalysis,
			Prompt: user, SystemPrompt: system, ResponseFormat: llm.ResponseFormatJSON,
		})
		if err != nil {
			c.logger.Warn("analysis reasoning call failed", zap.Error(err))
			return "", common.TokenUsage{}, false
		}
		return resp.Analysis, resp.Usage, true
	}
	resp, err := c.inv.Simple(callCtx, llm.SimpleRequest{
		Target: target, Purpose: common.ModelCapabilityResultsAnalysis,
		Message: user, SystemPrompt: system, ResponseFormat: llm.ResponseFormatJSON,
	})
	if err != nil {
		c.logger.Warn("analysis simple call failed", zap.Error(err))
		return "", common.TokenUsage{}, false
	}
	return resp.Text, resp.Usage, true
}

// accumulate folds one call's usage into the summary (Total + per-model bucket).
func accumulate(sum *common.TokenUsageSummary, u common.TokenUsage) {
	common.Accumulate(&sum.Total, u)
	for i := range sum.ByModel {
		if sum.ByModel[i].ProviderUID == u.ProviderUID && sum.ByModel[i].ModelUID == u.ModelUID && sum.ByModel[i].Effort == u.Effort {
			common.Accumulate(&sum.ByModel[i], u)
			return
		}
	}
	entry := common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: u.ProviderUID, ModelUID: u.ModelUID, Effort: u.Effort}}
	common.Accumulate(&entry, u)
	sum.ByModel = append(sum.ByModel, entry)
}

// extractJSON trims code fences / leading prose and returns the outermost {...}.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
