package provider

import sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"

func fptr(v float64) *float64 { return &v }

// visionMaxImages is how many images a vision-capable model accepts in ONE
// vision request. Advertised to core, which validates the attached image count
// against it before invoking the provider.
const visionMaxImages = 5

// standardConfigParams are the tunables for non-reasoning chat models.
func standardConfigParams(maxOut uint32) []sdk.ConfigParam {
	return []sdk.ConfigParam{
		{Key: "temperature", Type: sdk.ConfigParamDouble, Label: "Temperature", Min: fptr(0), Max: fptr(2), Default: &sdk.ConfigValue{Double: fptr(1)}},
		{Key: "top_p", Type: sdk.ConfigParamDouble, Label: "Top P", Min: fptr(0), Max: fptr(1), Default: &sdk.ConfigValue{Double: fptr(1)}},
		{Key: "frequency_penalty", Type: sdk.ConfigParamDouble, Label: "Frequency penalty", Min: fptr(-2), Max: fptr(2), Default: &sdk.ConfigValue{Double: fptr(0)}},
		{Key: "presence_penalty", Type: sdk.ConfigParamDouble, Label: "Presence penalty", Min: fptr(-2), Max: fptr(2), Default: &sdk.ConfigValue{Double: fptr(0)}},
		{Key: "max_tokens", Type: sdk.ConfigParamUint, Label: "Max output tokens", Min: fptr(1), Max: fptr(float64(maxOut))},
		{Key: "seed", Type: sdk.ConfigParamUint, Label: "Seed"},
		{Key: "stop", Type: sdk.ConfigParamStringList, Label: "Stop sequences"},
	}
}

// reasoningConfigParams are the tunables for reasoning models (no sampling params;
// max_tokens maps to max_completion_tokens; effort is the separate `efforts` mechanism).
func reasoningConfigParams(maxOut uint32) []sdk.ConfigParam {
	return []sdk.ConfigParam{
		{Key: "max_tokens", Type: sdk.ConfigParamUint, Label: "Max output tokens", Min: fptr(1), Max: fptr(float64(maxOut))},
		{Key: "seed", Type: sdk.ConfigParamUint, Label: "Seed"},
		{Key: "stop", Type: sdk.ConfigParamStringList, Label: "Stop sequences"},
	}
}

// Models is the registration table: per-model capabilities (closed list) and
// allowed effort levels. Tweak here to change what core may route to us.
var Models = []sdk.Model{
	{
		UID: "gpt-4o", Name: "GPT-4o", MaxImages: visionMaxImages,
		Capabilities: []sdk.Capability{
			sdk.CapabilityChat, sdk.CapabilityModelGeneration,
			sdk.CapabilitySimulationGeneration,
			sdk.CapabilitySimulationExecution, sdk.CapabilitySimulationDecisions,
			sdk.CapabilityVisionAnalysis,
		},
		ContextWindow: 128000, MaxOutputTokens: 16384,
		ConfigParams: standardConfigParams(16384), Reasoning: false,
	},
	{
		UID: "gpt-4o-mini", Name: "GPT-4o Mini",
		Capabilities:  []sdk.Capability{sdk.CapabilityChat, sdk.CapabilitySimulationDecisions},
		ContextWindow: 128000, MaxOutputTokens: 16384,
		ConfigParams: standardConfigParams(16384), Reasoning: false,
	},
	{
		UID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo",
		Capabilities:  []sdk.Capability{sdk.CapabilityChat},
		ContextWindow: 16385, MaxOutputTokens: 4096,
		ConfigParams: standardConfigParams(4096), Reasoning: false,
	},
	{
		UID: "gpt-4.1", Name: "GPT-4.1", MaxImages: visionMaxImages,
		Capabilities: []sdk.Capability{
			sdk.CapabilityChat, sdk.CapabilityModelGeneration,
			sdk.CapabilitySimulationGeneration,
			sdk.CapabilitySimulationExecution, sdk.CapabilityVisionAnalysis,
		},
		ContextWindow: 1048576, MaxOutputTokens: 32768,
		ConfigParams: standardConfigParams(32768), Reasoning: false,
	},
	{
		UID: "gpt-4.1-mini", Name: "GPT-4.1 Mini",
		Capabilities:  []sdk.Capability{sdk.CapabilityChat, sdk.CapabilitySimulationDecisions},
		ContextWindow: 1048576, MaxOutputTokens: 32768,
		ConfigParams: standardConfigParams(32768), Reasoning: false,
	},
	{
		UID: "gpt-4.1-nano", Name: "GPT-4.1 Nano",
		Capabilities:  []sdk.Capability{sdk.CapabilityChat, sdk.CapabilitySimulationDecisions},
		ContextWindow: 1048576, MaxOutputTokens: 32768,
		ConfigParams: standardConfigParams(32768), Reasoning: false,
	},
	{
		UID: "o3", Name: "O3",
		Capabilities: []sdk.Capability{
			sdk.CapabilityChat, sdk.CapabilityResultsAnalysis,
			sdk.CapabilitySimulationGeneration, sdk.CapabilitySimulationExecution,
		},
		Efforts:       []sdk.Effort{sdk.EffortLow, sdk.EffortMedium, sdk.EffortHigh},
		ContextWindow: 200000, MaxOutputTokens: 100000,
		ConfigParams: reasoningConfigParams(100000), Reasoning: true,
	},
	{
		UID: "o4-mini", Name: "O4 Mini",
		Capabilities: []sdk.Capability{
			sdk.CapabilityChat, sdk.CapabilityResultsAnalysis, sdk.CapabilitySimulationDecisions,
		},
		Efforts:       []sdk.Effort{sdk.EffortLow, sdk.EffortMedium, sdk.EffortHigh},
		ContextWindow: 200000, MaxOutputTokens: 100000,
		ConfigParams: reasoningConfigParams(100000), Reasoning: true,
	},
}

// modelIndex is a uid lookup built at init.
var modelIndex = func() map[string]sdk.Model {
	idx := make(map[string]sdk.Model, len(Models))
	for _, m := range Models {
		idx[m.UID] = m
	}
	return idx
}()

// IsModelSupported reports whether this provider serves the model UID.
func IsModelSupported(uid string) bool {
	_, ok := modelIndex[uid]
	return ok
}

// reasoningModels take reasoning_effort and reject sampling parameters
// (temperature/top_p/penalties); they use max_completion_tokens. Derived from
// the Models table so Model.Reasoning is the single source of truth.
var reasoningModels = func() map[string]bool {
	m := make(map[string]bool, len(Models))
	for _, model := range Models {
		if model.Reasoning {
			m[model.UID] = true
		}
	}
	return m
}()

func IsReasoningModel(uid string) bool { return reasoningModels[uid] }
