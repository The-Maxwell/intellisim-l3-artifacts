package simulations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/modules/llm"
)

// fakeAnalysisInvoker lets us drive reasoning/simple responses + the reasoning flag.
type fakeAnalysisInvoker struct {
	reasoning bool
	analysis  string
	simple    string
	calls     int
}

func (f *fakeAnalysisInvoker) ModelDescriptors(_, _ string) ([]llm.ConfigParamDescriptor, bool, bool) {
	return nil, f.reasoning, true
}
func (f *fakeAnalysisInvoker) Reasoning(_ context.Context, _ llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
	f.calls++
	return &llm.ReasoningCompletion{Analysis: f.analysis, Usage: common.TokenUsage{TotalTokens: 10, Calls: 1}}, nil
}
func (f *fakeAnalysisInvoker) Simple(_ context.Context, _ llm.SimpleRequest) (*llm.Completion, error) {
	f.calls++
	return &llm.Completion{Text: f.simple, Usage: common.TokenUsage{TotalTokens: 5, Calls: 1}}, nil
}

func TestCallAnalysisJSON_ReasoningModel(t *testing.T) {
	inv := &fakeAnalysisInvoker{reasoning: true, analysis: `{"title":"T","narrative":"N","key_findings":["a"],"causal_links":[]}`}
	caller := newAnalysisCaller(inv, AnalysisConfig{MaxRetries: 1, TimeoutSeconds: 30}, zapNop())
	usage := &common.TokenUsageSummary{}
	var out llmSectionResponse
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}
	ok := caller.callJSON(context.Background(), target, "sys", "user", usage, &out)
	require.True(t, ok)
	require.Equal(t, "T", out.Title)
	require.Equal(t, 1, inv.calls)
	require.Equal(t, uint64(10), usage.Total.TotalTokens)
}

func TestCallAnalysisJSON_MalformedRetriesThenFails(t *testing.T) {
	inv := &fakeAnalysisInvoker{reasoning: true, analysis: "not json"}
	caller := newAnalysisCaller(inv, AnalysisConfig{MaxRetries: 1, TimeoutSeconds: 30}, zapNop())
	usage := &common.TokenUsageSummary{}
	var out llmSectionResponse
	ok := caller.callJSON(context.Background(), common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}, "s", "u", usage, &out)
	require.False(t, ok)
	require.Equal(t, 2, inv.calls) // initial + 1 retry
}

func zapNop() *zap.Logger { return zap.NewNop() }
