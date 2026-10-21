package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

func TestEffortRoundtrip(t *testing.T) {
	for _, e := range []Effort{"", EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortMax} {
		assert.Equal(t, e, effortFromProto(effortToProto(e)), "effort %q", e)
	}
}

func TestCapabilityRoundtrip(t *testing.T) {
	for _, c := range []Capability{CapabilityChat, CapabilityModelGeneration,
		CapabilitySimulationGeneration, CapabilitySimulationExecution,
		CapabilitySimulationDecisions, CapabilityResultsAnalysis, CapabilityVisionAnalysis} {
		assert.Equal(t, c, capabilityFromProto(capabilityToProto(c)), "capability %q", c)
	}
}

func TestRegistrationToProto(t *testing.T) {
	reg := Registration{
		UID: "p1", Name: "P1", Description: "d", Version: "1.0.0",
		Models: []Model{{
			UID: "m1", Name: "M1",
			Capabilities:  []Capability{CapabilityChat, CapabilityVisionAnalysis},
			Efforts:       []Effort{EffortLow, EffortHigh},
			ContextWindow: 1000, MaxOutputTokens: 100,
		}},
	}
	pb := registrationToProto(reg)
	require.Len(t, pb.Models, 1)
	assert.Equal(t, "p1", pb.ProviderUid)
	assert.Equal(t, "m1", pb.Models[0].Uid)
	assert.Equal(t, []llmbridgev1.ModelCapability{
		llmbridgev1.ModelCapability_MODEL_CAPABILITY_CHAT,
		llmbridgev1.ModelCapability_MODEL_CAPABILITY_VISION_ANALYSIS,
	}, pb.Models[0].Capabilities)
	assert.Equal(t, []llmbridgev1.Effort{
		llmbridgev1.Effort_EFFORT_LOW, llmbridgev1.Effort_EFFORT_HIGH,
	}, pb.Models[0].Efforts)
	assert.Equal(t, uint32(1000), pb.Models[0].ContextWindow)
}

func TestGenerationConfigFromProto(t *testing.T) {
	temp := 0.5
	seed := uint64(42)
	pb := &llmbridgev1.GenerationConfig{
		Params: map[string]*llmbridgev1.ConfigValue{
			"temperature": {Value: &llmbridgev1.ConfigValue_DoubleValue{DoubleValue: temp}},
			"seed":        {Value: &llmbridgev1.ConfigValue_UintValue{UintValue: seed}},
			"stop":        {Value: &llmbridgev1.ConfigValue_ListValue{ListValue: &llmbridgev1.StringList{Values: []string{"END"}}}},
		},
	}
	cfg := generationConfigFromProto(pb)
	require.NotNil(t, cfg.Params["temperature"].Double)
	assert.Equal(t, 0.5, *cfg.Params["temperature"].Double)
	require.NotNil(t, cfg.Params["seed"].Uint)
	assert.Equal(t, uint64(42), *cfg.Params["seed"].Uint)
	assert.Equal(t, []string{"END"}, cfg.Params["stop"].List)
	assert.Nil(t, cfg.Params["temperature"].Uint)

	// nil proto -> empty config
	zero := generationConfigFromProto(nil)
	assert.Nil(t, zero.Params)

	// empty params -> empty config
	empty := generationConfigFromProto(&llmbridgev1.GenerationConfig{})
	assert.Nil(t, empty.Params)
}

func TestRequestsFromInvocation(t *testing.T) {
	inv := &llmbridgev1.Invocation{
		InvocationId: "id1", ModelUid: "m1", Effort: llmbridgev1.Effort_EFFORT_HIGH,
		Request: &llmbridgev1.Invocation_Chat{Chat: &llmbridgev1.ChatRequest{
			Message: "hi", SystemMessage: "sys",
			History: []*llmbridgev1.ChatMessage{{Role: "user", Content: "prev"}},
		}},
	}
	req := chatRequestFromProto(inv, inv.GetChat())
	assert.Equal(t, "m1", req.ModelUID)
	assert.Equal(t, EffortHigh, req.Effort)
	assert.Equal(t, "hi", req.Message)
	assert.Equal(t, []ChatMessage{{Role: "user", Content: "prev"}}, req.History)
}

func TestUsageToProto(t *testing.T) {
	u := Usage{ModelUID: "m1", Effort: EffortLow, PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3, LatencyMs: 4.5}
	pb := usageToProto(u)
	assert.Equal(t, "m1", pb.ModelUid)
	assert.Equal(t, llmbridgev1.Effort_EFFORT_LOW, pb.Effort)
	assert.Equal(t, uint64(3), pb.TotalTokens)
	assert.Equal(t, 4.5, pb.LatencyMs)
}

func TestConfigValueRoundTrip(t *testing.T) {
	f := 0.7
	i64 := int64(-3)
	u := uint64(256)
	b := true
	s := "json"
	cases := []ConfigValue{
		{Double: &f},
		{Int: &i64},
		{Uint: &u},
		{Bool: &b},
		{String: &s},
		{List: []string{"\n", "END"}},
	}
	for idx, in := range cases {
		out := configValueFromProto(configValueToProto(in))
		assert.Equal(t, in, out, "case %d round-trip", idx)
	}

	// Empty/unset value maps to a nil proto and back to a zero ConfigValue.
	assert.Nil(t, configValueToProto(ConfigValue{}))
	assert.Equal(t, ConfigValue{}, configValueFromProto(nil))
}
