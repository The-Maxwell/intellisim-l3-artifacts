package llm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

func TestEffortRoundtrip(t *testing.T) {
	for _, e := range []common.LLMEffort{"", common.LLMEffortMinimal, common.LLMEffortLow,
		common.LLMEffortMedium, common.LLMEffortHigh, common.LLMEffortMax} {
		assert.Equal(t, e, effortFromProto(effortToProto(e)), "effort %q", e)
	}
}

func TestCapabilityFromProto(t *testing.T) {
	assert.Equal(t, common.ModelCapabilityVisionAnalysis,
		capabilityFromProto(llmbridgev1.ModelCapability_MODEL_CAPABILITY_VISION_ANALYSIS))
	assert.Equal(t, common.ModelCapability(""),
		capabilityFromProto(llmbridgev1.ModelCapability_MODEL_CAPABILITY_UNSPECIFIED))
}

func TestProviderInfoFromProto(t *testing.T) {
	now := time.Now()
	reg := &llmbridgev1.ConnectRequest{
		ProviderUid: "p1", Name: "P1", Description: "d", Version: "1.0.0",
		Models: []*llmbridgev1.Model{{
			Uid: "m1", Name: "M1",
			Capabilities: []llmbridgev1.ModelCapability{
				llmbridgev1.ModelCapability_MODEL_CAPABILITY_CHAT,
				llmbridgev1.ModelCapability_MODEL_CAPABILITY_RESULTS_ANALYSIS,
			},
			Efforts:       []llmbridgev1.Effort{llmbridgev1.Effort_EFFORT_LOW},
			ContextWindow: 1000, MaxOutputTokens: 100,
		}},
	}
	info := providerInfoFromProto(reg, now)
	assert.Equal(t, "p1", info.UID)
	assert.Equal(t, now, info.ConnectedAt)
	require.Len(t, info.Models, 1)
	assert.Equal(t, []common.ModelCapability{common.ModelCapabilityChat, common.ModelCapabilityResultsAnalysis},
		info.Models[0].Capabilities)
	assert.Equal(t, []common.LLMEffort{common.LLMEffortLow}, info.Models[0].Efforts)
}

func TestUsageFromProto(t *testing.T) {
	u := usageFromProto("p1", &llmbridgev1.TokenUsage{
		ModelUid: "m1", Effort: llmbridgev1.Effort_EFFORT_HIGH,
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, LatencyMs: 12.5,
	})
	assert.Equal(t, "p1", u.ProviderUID)
	assert.Equal(t, "m1", u.ModelUID)
	assert.Equal(t, common.LLMEffortHigh, u.Effort)
	assert.Equal(t, uint64(15), u.TotalTokens)
	assert.Equal(t, uint64(1), u.Calls)
	assert.Equal(t, 12.5, u.LatencyMs)

	// nil usage still attributes the provider with Calls=1
	empty := usageFromProto("p1", nil)
	assert.Equal(t, "p1", empty.ProviderUID)
	assert.Equal(t, uint64(1), empty.Calls)
}

func TestGenerationParamsToProto(t *testing.T) {
	temp := 0.3
	mt := uint64(256)
	pb := generationParamsToProto(common.GenerationParams{
		"temperature": {Double: &temp},
		"max_tokens":  {Uint: &mt},
		"stop":        {List: []string{"X"}},
	})
	require.NotNil(t, pb)
	require.Len(t, pb.Params, 3)
	assert.Equal(t, 0.3, pb.Params["temperature"].GetDoubleValue())
	assert.Equal(t, uint64(256), pb.Params["max_tokens"].GetUintValue())
	assert.Equal(t, []string{"X"}, pb.Params["stop"].GetListValue().GetValues())

	// empty params map maps to nil (no config sent)
	assert.Nil(t, generationParamsToProto(nil))
}

func TestResponseFormatToProto(t *testing.T) {
	assert.Equal(t, llmbridgev1.ResponseFormat_RESPONSE_FORMAT_JSON, responseFormatToProto(ResponseFormatJSON))
	assert.Equal(t, llmbridgev1.ResponseFormat_RESPONSE_FORMAT_TEXT, responseFormatToProto(ResponseFormatText))
	assert.Equal(t, llmbridgev1.ResponseFormat_RESPONSE_FORMAT_UNSPECIFIED, responseFormatToProto(ResponseFormatDefault))
}

func TestChatMessagesToProto(t *testing.T) {
	out := chatMessagesToProto([]chats.ChatMessage{
		{Role: chats.ChatMessageUserRole, TextContent: "hi"},
	})
	require.Len(t, out, 1)
	assert.Equal(t, "user", out[0].Role)
	assert.Equal(t, "hi", out[0].Content)
}

func TestProviderErrorFromProto(t *testing.T) {
	err := providerErrorFromProto(&llmbridgev1.Error{
		Code: llmbridgev1.ErrorCode_ERROR_CODE_TIMEOUT, Message: "too slow",
	})
	assert.Equal(t, ProviderErrTimeout, err.Code)
	assert.Contains(t, err.Error(), "too slow")

	assert.Equal(t, ProviderErrUnspecified, providerErrorFromProto(&llmbridgev1.Error{Code: llmbridgev1.ErrorCode_ERROR_CODE_UNSPECIFIED, Message: "x"}).Code)
}
