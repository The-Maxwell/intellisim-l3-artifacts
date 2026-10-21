package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
	"intelli.sim.core/data/common"
)

// RemoteProvider implements Provider by forwarding invocations through the
// ProviderBridge. It performs no validation — Service validates before calling.
type RemoteProvider struct {
	logger *zap.Logger
	bridge *ProviderBridge
	info   ProviderInfo
}

func NewRemoteProvider(logger *zap.Logger, bridge *ProviderBridge, reg *llmbridgev1.ConnectRequest, connectedAt time.Time) *RemoteProvider {
	return &RemoteProvider{
		logger: logger.Named("remote-provider").With(zap.String("provider_uid", reg.ProviderUid)),
		bridge: bridge,
		info:   providerInfoFromProto(reg, connectedAt),
	}
}

func (r *RemoteProvider) UID() string        { return r.info.UID }
func (r *RemoteProvider) Info() ProviderInfo { return r.info }

func (r *RemoteProvider) Simple(ctx context.Context, req SimpleRequest) (*Completion, error) {
	inv := r.newInvocation(req.Target)
	inv.ResponseFormat = responseFormatToProto(req.ResponseFormat)
	inv.Request = &llmbridgev1.Invocation_Simple{Simple: &llmbridgev1.SimpleRequest{
		Message:      req.Message,
		SystemPrompt: req.SystemPrompt,
		Config:       generationParamsToProto(req.Target.Generation),
	}}
	resp, err := r.bridge.Invoke(ctx, r.info.UID, inv)
	if err != nil {
		return nil, err
	}
	return r.completion(resp)
}

func (r *RemoteProvider) Chat(ctx context.Context, req ChatRequest) (*Completion, error) {
	inv := r.newInvocation(req.Target)
	inv.ResponseFormat = responseFormatToProto(req.ResponseFormat)
	inv.Request = &llmbridgev1.Invocation_Chat{Chat: &llmbridgev1.ChatRequest{
		Message:       req.Message,
		SystemMessage: req.SystemMessage,
		History:       chatMessagesToProto(req.Messages),
		Config:        generationParamsToProto(req.Target.Generation),
	}}
	resp, err := r.bridge.Invoke(ctx, r.info.UID, inv)
	if err != nil {
		return nil, err
	}
	return r.completion(resp)
}

func (r *RemoteProvider) Vision(ctx context.Context, req VisionRequest) (*Completion, error) {
	inv := r.newInvocation(req.Target)
	inv.ResponseFormat = responseFormatToProto(req.ResponseFormat)
	inv.Request = &llmbridgev1.Invocation_Vision{Vision: &llmbridgev1.VisionRequest{
		Message:      req.Message,
		Images:       visionImagesToProto(req.Images),
		Detail:       req.Detail,
		SystemPrompt: req.SystemPrompt,
		Config:       generationParamsToProto(req.Target.Generation),
	}}
	resp, err := r.bridge.Invoke(ctx, r.info.UID, inv)
	if err != nil {
		return nil, err
	}
	return r.completion(resp)
}

func (r *RemoteProvider) Reasoning(ctx context.Context, req ReasoningRequest) (*ReasoningCompletion, error) {
	inv := r.newInvocation(req.Target)
	inv.ResponseFormat = responseFormatToProto(req.ResponseFormat)
	inv.Request = &llmbridgev1.Invocation_Reasoning{Reasoning: &llmbridgev1.ReasoningRequest{
		Prompt:       req.Prompt,
		Context:      req.Context,
		SystemPrompt: req.SystemPrompt,
		Config:       generationParamsToProto(req.Target.Generation),
	}}
	resp, err := r.bridge.Invoke(ctx, r.info.UID, inv)
	if err != nil {
		return nil, err
	}
	if pbErr := resp.GetError(); pbErr != nil {
		return nil, providerErrorFromProto(pbErr)
	}
	reasoning := resp.GetReasoning()
	if reasoning == nil {
		return nil, fmt.Errorf("%w: provider %q (reasoning invocation)", ErrMalformedProviderResponse, r.info.UID)
	}
	return reasoningFromProto(r.info.UID, reasoning), nil
}

func (r *RemoteProvider) newInvocation(target common.LLMTarget) *llmbridgev1.Invocation {
	return &llmbridgev1.Invocation{
		InvocationId: uuid.NewString(),
		ModelUid:     target.ModelUID,
		Effort:       effortToProto(target.Effort),
	}
}

func (r *RemoteProvider) completion(resp *llmbridgev1.CompleteInvocationRequest) (*Completion, error) {
	if pbErr := resp.GetError(); pbErr != nil {
		return nil, providerErrorFromProto(pbErr)
	}
	comp := resp.GetCompletion()
	if comp == nil {
		return nil, fmt.Errorf("%w: provider %q", ErrMalformedProviderResponse, r.info.UID)
	}
	return completionFromProto(r.info.UID, comp), nil
}
