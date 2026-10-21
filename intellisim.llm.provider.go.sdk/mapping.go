package sdk

import (
	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

func effortToProto(e Effort) llmbridgev1.Effort {
	switch e {
	case EffortMinimal:
		return llmbridgev1.Effort_EFFORT_MINIMAL
	case EffortLow:
		return llmbridgev1.Effort_EFFORT_LOW
	case EffortMedium:
		return llmbridgev1.Effort_EFFORT_MEDIUM
	case EffortHigh:
		return llmbridgev1.Effort_EFFORT_HIGH
	case EffortMax:
		return llmbridgev1.Effort_EFFORT_MAX
	default:
		return llmbridgev1.Effort_EFFORT_UNSPECIFIED
	}
}

func effortFromProto(e llmbridgev1.Effort) Effort {
	switch e {
	case llmbridgev1.Effort_EFFORT_MINIMAL:
		return EffortMinimal
	case llmbridgev1.Effort_EFFORT_LOW:
		return EffortLow
	case llmbridgev1.Effort_EFFORT_MEDIUM:
		return EffortMedium
	case llmbridgev1.Effort_EFFORT_HIGH:
		return EffortHigh
	case llmbridgev1.Effort_EFFORT_MAX:
		return EffortMax
	default:
		return ""
	}
}

func capabilityToProto(c Capability) llmbridgev1.ModelCapability {
	switch c {
	case CapabilityChat:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_CHAT
	case CapabilityModelGeneration:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_MODEL_GENERATION
	case CapabilitySimulationGeneration:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_GENERATION
	case CapabilitySimulationExecution:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_EXECUTION
	case CapabilitySimulationDecisions:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_DECISIONS
	case CapabilityResultsAnalysis:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_RESULTS_ANALYSIS
	case CapabilityVisionAnalysis:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_VISION_ANALYSIS
	default:
		return llmbridgev1.ModelCapability_MODEL_CAPABILITY_UNSPECIFIED
	}
}

func capabilityFromProto(c llmbridgev1.ModelCapability) Capability {
	switch c {
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_CHAT:
		return CapabilityChat
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_MODEL_GENERATION:
		return CapabilityModelGeneration
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_GENERATION:
		return CapabilitySimulationGeneration
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_EXECUTION:
		return CapabilitySimulationExecution
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_SIMULATION_DECISIONS:
		return CapabilitySimulationDecisions
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_RESULTS_ANALYSIS:
		return CapabilityResultsAnalysis
	case llmbridgev1.ModelCapability_MODEL_CAPABILITY_VISION_ANALYSIS:
		return CapabilityVisionAnalysis
	default:
		return ""
	}
}

func registrationToProto(r Registration) *llmbridgev1.ConnectRequest {
	models := make([]*llmbridgev1.Model, 0, len(r.Models))
	for _, m := range r.Models {
		caps := make([]llmbridgev1.ModelCapability, 0, len(m.Capabilities))
		for _, c := range m.Capabilities {
			caps = append(caps, capabilityToProto(c))
		}
		efforts := make([]llmbridgev1.Effort, 0, len(m.Efforts))
		for _, e := range m.Efforts {
			efforts = append(efforts, effortToProto(e))
		}
		cfgParams := make([]*llmbridgev1.ConfigParam, 0, len(m.ConfigParams))
		for _, cp := range m.ConfigParams {
			cfgParams = append(cfgParams, configParamToProto(cp))
		}
		models = append(models, &llmbridgev1.Model{
			Uid: m.UID, Name: m.Name,
			Capabilities: caps, Efforts: efforts,
			ContextWindow: m.ContextWindow, MaxOutputTokens: m.MaxOutputTokens,
			ConfigParams: cfgParams, Reasoning: m.Reasoning, MaxImages: m.MaxImages,
		})
	}
	return &llmbridgev1.ConnectRequest{
		ProviderUid: r.UID, Name: r.Name, Description: r.Description,
		Version: r.Version, Models: models,
	}
}

func generationConfigFromProto(pb *llmbridgev1.GenerationConfig) GenerationConfig {
	if pb == nil || len(pb.Params) == 0 {
		return GenerationConfig{}
	}
	params := make(map[string]ConfigValue, len(pb.Params))
	for k, v := range pb.Params {
		params[k] = configValueFromProto(v)
	}
	return GenerationConfig{Params: params}
}

func chatMessagesFromProto(pb []*llmbridgev1.ChatMessage) []ChatMessage {
	out := make([]ChatMessage, 0, len(pb))
	for _, m := range pb {
		out = append(out, ChatMessage{Role: m.Role, Content: m.Content})
	}
	return out
}

func simpleRequestFromProto(inv *llmbridgev1.Invocation, r *llmbridgev1.SimpleRequest) *SimpleRequest {
	return &SimpleRequest{
		ModelUID: inv.ModelUid, Effort: effortFromProto(inv.Effort),
		Message: r.Message, SystemPrompt: r.SystemPrompt,
		Config:         generationConfigFromProto(r.Config),
		ResponseFormat: responseFormatFromProto(inv.ResponseFormat),
	}
}

func chatRequestFromProto(inv *llmbridgev1.Invocation, r *llmbridgev1.ChatRequest) *ChatRequest {
	return &ChatRequest{
		ModelUID: inv.ModelUid, Effort: effortFromProto(inv.Effort),
		Message: r.Message, SystemMessage: r.SystemMessage,
		History:        chatMessagesFromProto(r.History),
		Config:         generationConfigFromProto(r.Config),
		ResponseFormat: responseFormatFromProto(inv.ResponseFormat),
	}
}

func visionImagesFromProto(pb []*llmbridgev1.VisionImage) []VisionImage {
	if len(pb) == 0 {
		return nil
	}
	out := make([]VisionImage, 0, len(pb))
	for _, img := range pb {
		if img == nil {
			continue
		}
		out = append(out, VisionImage{
			ImageData:   img.ImageData,
			ImageFormat: img.ImageFormat,
			Description: img.Description,
		})
	}
	return out
}

func visionRequestFromProto(inv *llmbridgev1.Invocation, r *llmbridgev1.VisionRequest) *VisionRequest {
	return &VisionRequest{
		ModelUID: inv.ModelUid, Effort: effortFromProto(inv.Effort),
		Message: r.Message, Images: visionImagesFromProto(r.Images),
		Detail: r.Detail, SystemPrompt: r.SystemPrompt,
		Config:         generationConfigFromProto(r.Config),
		ResponseFormat: responseFormatFromProto(inv.ResponseFormat),
	}
}

func reasoningRequestFromProto(inv *llmbridgev1.Invocation, r *llmbridgev1.ReasoningRequest) *ReasoningRequest {
	return &ReasoningRequest{
		ModelUID: inv.ModelUid, Effort: effortFromProto(inv.Effort),
		Prompt: r.Prompt, Context: r.Context, SystemPrompt: r.SystemPrompt,
		Config:         generationConfigFromProto(r.Config),
		ResponseFormat: responseFormatFromProto(inv.ResponseFormat),
	}
}

func usageToProto(u Usage) *llmbridgev1.TokenUsage {
	return &llmbridgev1.TokenUsage{
		ModelUid: u.ModelUID, Effort: effortToProto(u.Effort),
		PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens,
		ReasoningTokens: u.ReasoningTokens,
		TotalTokens:     u.TotalTokens, LatencyMs: u.LatencyMs,
	}
}

func completionToProto(c *Completion) *llmbridgev1.Completion {
	return &llmbridgev1.Completion{
		Text: c.Text, FinishReason: c.FinishReason, Usage: usageToProto(c.Usage),
	}
}

func reasoningCompletionToProto(c *ReasoningCompletion) *llmbridgev1.ReasoningCompletion {
	return &llmbridgev1.ReasoningCompletion{
		Analysis: c.Analysis, Reasoning: c.Reasoning,
		FinishReason: c.FinishReason, Usage: usageToProto(c.Usage),
	}
}

func configValueToProto(v ConfigValue) *llmbridgev1.ConfigValue {
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

func configValueFromProto(pb *llmbridgev1.ConfigValue) ConfigValue {
	if pb == nil {
		return ConfigValue{}
	}
	switch x := pb.Value.(type) {
	case *llmbridgev1.ConfigValue_DoubleValue:
		v := x.DoubleValue
		return ConfigValue{Double: &v}
	case *llmbridgev1.ConfigValue_IntValue:
		v := x.IntValue
		return ConfigValue{Int: &v}
	case *llmbridgev1.ConfigValue_UintValue:
		v := x.UintValue
		return ConfigValue{Uint: &v}
	case *llmbridgev1.ConfigValue_BoolValue:
		v := x.BoolValue
		return ConfigValue{Bool: &v}
	case *llmbridgev1.ConfigValue_StringValue:
		v := x.StringValue
		return ConfigValue{String: &v}
	case *llmbridgev1.ConfigValue_ListValue:
		return ConfigValue{List: x.ListValue.GetValues()}
	default:
		return ConfigValue{}
	}
}

func configParamTypeToProto(t ConfigParamType) llmbridgev1.ConfigParamType {
	switch t {
	case ConfigParamDouble:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_DOUBLE
	case ConfigParamInt:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_INT
	case ConfigParamUint:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_UINT
	case ConfigParamBool:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_BOOL
	case ConfigParamString:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_STRING
	case ConfigParamEnum:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_ENUM
	case ConfigParamStringList:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_STRING_LIST
	default:
		return llmbridgev1.ConfigParamType_CONFIG_PARAM_TYPE_UNSPECIFIED
	}
}

func configParamToProto(p ConfigParam) *llmbridgev1.ConfigParam {
	out := &llmbridgev1.ConfigParam{
		Key: p.Key, Type: configParamTypeToProto(p.Type), Required: p.Required,
		Label: p.Label, Description: p.Description, EnumValues: p.EnumValues,
	}
	out.Min = p.Min
	out.Max = p.Max
	if p.Default != nil {
		out.DefaultValue = configValueToProto(*p.Default)
	}
	return out
}

func responseFormatFromProto(rf llmbridgev1.ResponseFormat) ResponseFormat {
	switch rf {
	case llmbridgev1.ResponseFormat_RESPONSE_FORMAT_TEXT:
		return ResponseFormatText
	case llmbridgev1.ResponseFormat_RESPONSE_FORMAT_JSON:
		return ResponseFormatJSON
	default:
		return ResponseFormatDefault
	}
}

func errorCodeToProto(c ErrorCode) llmbridgev1.ErrorCode {
	switch c {
	case ErrCodeInvalidRequest:
		return llmbridgev1.ErrorCode_ERROR_CODE_INVALID_REQUEST
	case ErrCodeModelNotSupported:
		return llmbridgev1.ErrorCode_ERROR_CODE_MODEL_NOT_SUPPORTED
	case ErrCodeEffortNotSupported:
		return llmbridgev1.ErrorCode_ERROR_CODE_EFFORT_NOT_SUPPORTED
	case ErrCodeCapabilityNotSupported:
		return llmbridgev1.ErrorCode_ERROR_CODE_CAPABILITY_NOT_SUPPORTED
	case ErrCodeTimeout:
		return llmbridgev1.ErrorCode_ERROR_CODE_TIMEOUT
	case ErrCodeInternal:
		return llmbridgev1.ErrorCode_ERROR_CODE_INTERNAL
	case ErrCodeUpstreamError:
		return llmbridgev1.ErrorCode_ERROR_CODE_UPSTREAM_ERROR
	default:
		return llmbridgev1.ErrorCode_ERROR_CODE_UNSPECIFIED
	}
}
