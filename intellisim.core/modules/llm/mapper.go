package llm

import (
	"time"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

func effortToProto(e common.LLMEffort) llmbridgev1.Effort {
	switch e {
	case common.LLMEffortMinimal:
		return llmbridgev1.Effort_EFFORT_MINIMAL
	case common.LLMEffortLow:
		return llmbridgev1.Effort_EFFORT_LOW
	case common.LLMEffortMedium:
		return llmbridgev1.Effort_EFFORT_MEDIUM
	case common.LLMEffortHigh:
		return llmbridgev1.Effort_EFFORT_HIGH
	case common.LLMEffortMax:
		return llmbridgev1.Effort_EFFORT_MAX
	default:
		return llmbridgev1.Effort_EFFORT_UNSPECIFIED
	}
}

func effortFromProto(e llmbridgev1.Effort) common.LLMEffort {
	switch e {
	case llmbridgev1.Effort_EFFORT_MINIMAL:
		return common.LLMEffortMinimal
	case llmbridgev1.Effort_EFFORT_LOW:
		return common.LLMEffortLow
	case llmbridgev1.Effort_EFFORT_MEDIUM:
		return common.LLMEffortMedium
	case llmbridgev1.Effort_EFFORT_HIGH:
		return common.LLMEffortHigh
	case llmbridgev1.Effort_EFFORT_MAX:
		return common.LLMEffortMax
	default:
		return ""
	}
}

func capabilityFromProto(c llmbridgev1.ModelCapability) common.ModelCapability {
	switch c {
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_CHAT:
		return common.ModelCapabilityChat
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_MODEL_GENERATION:
		return common.ModelCapabilityModelGeneration
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_GENERATION:
		return common.ModelCapabilitySimulationGeneration
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_EXECUTION:
		return common.ModelCapabilitySimulationExecution
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_DECISIONS:
		return common.ModelCapabilitySimulationDecisions
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_RESULTS_ANALYSIS:
		return common.ModelCapabilityResultsAnalysis
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_VISION_ANALYSIS:
		return common.ModelCapabilityVisionAnalysis
	default:
		return ""
	}
}

func configValueToProto(v common.ConfigValue) *llmbridgev1.ConfigValue {
	switch {
	case v.Double != nil:
		return &llmbridgev1.ConfigValue{Value: &llmbridgev1.ConfigValue_DoubleValue{DoubleValue: *v.Double}}
	case v.Int != nil:
		return &llmbridgev1.ConfigValue{Value: &llmbridgev1.ConfigValue_IntValue{IntValue: *v.Int}}
	case v.Uint != nil:
		return &llmbridgev1.ConfigValue{Value: &llmbridgev1.ConfigValue_UintValue{UintValue: *v.Uint}}
	case v.Bool != nil:
		return &llmbridgev1.ConfigValue{Value: &llmbridgev1.ConfigValue_BoolValue{BoolValue: *v.Bool}}
	case v.String != nil:
		return &llmbridgev1.ConfigValue{Value: &llmbridgev1.ConfigValue_StringValue{StringValue: *v.String}}
	case v.List != nil:
		return &llmbridgev1.ConfigValue{Value: &llmbridgev1.ConfigValue_ListValue{ListValue: &llmbridgev1.StringList{Values: v.List}}}
	default:
		return nil
	}
}

func configValueFromProto(pb *llmbridgev1.ConfigValue) common.ConfigValue {
	if pb == nil {
		return common.ConfigValue{}
	}
	switch x := pb.Value.(type) {
	case *llmbridgev1.ConfigValue_DoubleValue:
		v := x.DoubleValue
		return common.ConfigValue{Double: &v}
	case *llmbridgev1.ConfigValue_IntValue:
		v := x.IntValue
		return common.ConfigValue{Int: &v}
	case *llmbridgev1.ConfigValue_UintValue:
		v := x.UintValue
		return common.ConfigValue{Uint: &v}
	case *llmbridgev1.ConfigValue_BoolValue:
		v := x.BoolValue
		return common.ConfigValue{Bool: &v}
	case *llmbridgev1.ConfigValue_StringValue:
		v := x.StringValue
		return common.ConfigValue{String: &v}
	case *llmbridgev1.ConfigValue_ListValue:
		return common.ConfigValue{List: x.ListValue.GetValues()}
	default:
		return common.ConfigValue{}
	}
}

func generationParamsToProto(g common.GenerationParams) *llmbridgev1.GenerationConfig {
	if len(g) == 0 {
		return nil
	}
	params := make(map[string]*llmbridgev1.ConfigValue, len(g))
	for k, v := range g {
		params[k] = configValueToProto(v)
	}
	return &llmbridgev1.GenerationConfig{Params: params}
}

func responseFormatToProto(rf ResponseFormat) llmbridgev1.ResponseFormat {
	switch rf {
	case ResponseFormatText:
		return llmbridgev1.ResponseFormat_RESPONSE_FORMAT_TEXT
	case ResponseFormatJSON:
		return llmbridgev1.ResponseFormat_RESPONSE_FORMAT_JSON
	default:
		return llmbridgev1.ResponseFormat_RESPONSE_FORMAT_UNSPECIFIED
	}
}

func configParamTypeFromProto(t llmbridgev1.ConfigParamType) ConfigParamType {
	switch t {
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_DOUBLE:
		return ConfigParamDouble
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_INT:
		return ConfigParamInt
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_UINT:
		return ConfigParamUint
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_BOOL:
		return ConfigParamBool
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_STRING:
		return ConfigParamString
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_ENUM:
		return ConfigParamEnum
	case llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_STRING_LIST:
		return ConfigParamStringList
	default:
		return ""
	}
}

func configParamFromProto(pb *llmbridgev1.ConfigParam) ConfigParamDescriptor {
	d := ConfigParamDescriptor{
		Key: pb.Key, Type: configParamTypeFromProto(pb.Type), Required: pb.Required,
		Label: pb.Label, Description: pb.Description, EnumValues: pb.EnumValues,
		Min: pb.Min, Max: pb.Max,
	}
	if pb.DefaultValue != nil {
		v := configValueFromProto(pb.DefaultValue)
		d.Default = &v
	}
	return d
}

func chatMessagesToProto(messages []chats.ChatMessage) []*llmbridgev1.ChatMessage {
	out := make([]*llmbridgev1.ChatMessage, 0, len(messages))
	for _, m := range messages {
		out = append(out, &llmbridgev1.ChatMessage{Role: string(m.Role), Content: m.TextContent})
	}
	return out
}

func visionImagesToProto(images []VisionImage) []*llmbridgev1.VisionImage {
	if len(images) == 0 {
		return nil
	}
	out := make([]*llmbridgev1.VisionImage, 0, len(images))
	for _, img := range images {
		out = append(out, &llmbridgev1.VisionImage{
			ImageData:   img.Data,
			ImageFormat: img.Format,
			Description: img.Description,
		})
	}
	return out
}

func providerInfoFromProto(reg *llmbridgev1.ConnectRequest, connectedAt time.Time) ProviderInfo {
	models := make([]ModelInfo, 0, len(reg.Models))
	for _, m := range reg.Models {
		// Values are pre-validated by validateRegistration; dropping unknowns here is belt-and-suspenders.
		caps := make([]common.ModelCapability, 0, len(m.Capabilities))
		for _, c := range m.Capabilities {
			if mapped := capabilityFromProto(c); mapped != "" {
				caps = append(caps, mapped)
			}
		}
		efforts := make([]common.LLMEffort, 0, len(m.Efforts))
		for _, e := range m.Efforts {
			if mapped := effortFromProto(e); mapped != "" {
				efforts = append(efforts, mapped)
			}
		}
		cfgParams := make([]ConfigParamDescriptor, 0, len(m.ConfigParams))
		for _, cp := range m.ConfigParams {
			if cp != nil && cp.Key != "" {
				cfgParams = append(cfgParams, configParamFromProto(cp))
			}
		}
		models = append(models, ModelInfo{
			UID: m.Uid, Name: m.Name,
			Capabilities: caps, Efforts: efforts,
			ContextWindow: m.ContextWindow, MaxOutputTokens: m.MaxOutputTokens,
			ConfigParams: cfgParams, Reasoning: m.Reasoning,
			MaxImages: m.MaxImages,
		})
	}
	return ProviderInfo{
		UID: reg.ProviderUid, Name: reg.Name, Description: reg.Description,
		Version: reg.Version, ConnectedAt: connectedAt, Models: models,
	}
}

// usageFromProto adapts wire usage to common.TokenUsage. The provider reports
// the model/effort ACTUALLY used; core attributes the provider and Calls=1.
func usageFromProto(providerUID string, pb *llmbridgev1.TokenUsage) common.TokenUsage {
	if pb == nil {
		return common.TokenUsage{
			LLMTarget: common.LLMTarget{ProviderUID: providerUID},
			Calls:     1,
		}
	}
	return common.TokenUsage{
		LLMTarget: common.LLMTarget{
			ProviderUID: providerUID,
			ModelUID:    pb.ModelUid,
			Effort:      effortFromProto(pb.Effort),
		},
		PromptTokens:     pb.PromptTokens,
		CompletionTokens: pb.CompletionTokens,
		ReasoningTokens:  pb.ReasoningTokens,
		TotalTokens:      pb.TotalTokens,
		Calls:            1,
		LatencyMs:        pb.LatencyMs,
	}
}

func completionFromProto(providerUID string, pb *llmbridgev1.Completion) *Completion {
	return &Completion{
		Text:         pb.Text,
		FinishReason: pb.FinishReason,
		Usage:        usageFromProto(providerUID, pb.Usage),
	}
}

func reasoningFromProto(providerUID string, pb *llmbridgev1.ReasoningCompletion) *ReasoningCompletion {
	return &ReasoningCompletion{
		Analysis:     pb.Analysis,
		Reasoning:    pb.Reasoning,
		FinishReason: pb.FinishReason,
		Usage:        usageFromProto(providerUID, pb.Usage),
	}
}

func providerErrorFromProto(pb *llmbridgev1.Error) *ProviderError {
	code := ProviderErrUnspecified
	switch pb.Code {
	case llmbridgev1.ErrorCode_ERROR_CODE_INVALID_REQUEST:
		code = ProviderErrInvalidRequest
	case llmbridgev1.ErrorCode_ERROR_CODE_MODEL_NOT_SUPPORTED:
		code = ProviderErrModelNotSupported
	case llmbridgev1.ErrorCode_ERROR_CODE_EFFORT_NOT_SUPPORTED:
		code = ProviderErrEffortNotSupported
	case llmbridgev1.ErrorCode_ERROR_CODE_CAPABILITY_NOT_SUPPORTED:
		code = ProviderErrCapabilityNotSupported
	case llmbridgev1.ErrorCode_ERROR_CODE_UPSTREAM_ERROR:
		code = ProviderErrUpstreamError
	case llmbridgev1.ErrorCode_ERROR_CODE_TIMEOUT:
		code = ProviderErrTimeout
	case llmbridgev1.ErrorCode_ERROR_CODE_INTERNAL:
		code = ProviderErrInternal
	}
	return &ProviderError{Code: code, Message: pb.Message}
}
