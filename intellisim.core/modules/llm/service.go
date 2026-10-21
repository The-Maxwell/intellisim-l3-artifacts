package llm

import (
	"context"
	"fmt"
	"slices"
	"time"

	"go.uber.org/zap"
	"intelli.sim.core/data/common"
)

// Service is the consumer-facing LLM API: it routes a request to the provider
// named by its LLMTarget, strictly validating target/effort/purpose against
// the provider's registration before anything reaches the wire.
type Service interface {
	Simple(ctx context.Context, req SimpleRequest) (*Completion, error)
	Chat(ctx context.Context, req ChatRequest) (*Completion, error)
	Vision(ctx context.Context, req VisionRequest) (*Completion, error)
	Reasoning(ctx context.Context, req ReasoningRequest) (*ReasoningCompletion, error)

	Providers() []ProviderInfo
	ProvidersByCapability(capability common.ModelCapability) []ProviderInfo
	ProvidersByModel(modelUID string) []ProviderInfo

	// ModelDescriptors returns a connected model's advertised config descriptors
	// + reasoning flag, or ok=false if provider/model is not currently registered.
	ModelDescriptors(providerUID, modelUID string) (descriptors []ConfigParamDescriptor, reasoning bool, ok bool)

	// MaxImages returns a connected model's advertised max images per vision
	// request, or ok=false if provider/model is not currently registered. A
	// value of 0 means the model did not advertise a limit (callers default to 1).
	MaxImages(providerUID, modelUID string) (maxImages uint32, ok bool)

	// ValidateTarget fully validates a target a user is about to assign to a
	// model/simulation/agent (provider/model/effort/capability + generation params + timeout).
	ValidateTarget(target common.LLMTarget, purpose common.ModelCapability) error
}

type service struct {
	logger   *zap.Logger
	registry Registry
	fitter   *ContextFitter
}

func NewService(logger *zap.Logger, registry Registry, fitter *ContextFitter) Service {
	return &service{logger: logger.Named("llm-service"), registry: registry, fitter: fitter}
}

func (s *service) Simple(ctx context.Context, req SimpleRequest) (*Completion, error) {
	provider, model, err := s.resolve(req.Target, req.Purpose, "")
	if err != nil {
		return nil, err
	}
	if err := s.fitter.ValidatePrompt(int(model.ContextWindow), s.maxOutput(req.Target.Generation, model),
		req.Message, req.SystemPrompt); err != nil {
		return nil, fmt.Errorf("simple request too large: %w", err)
	}
	ctx, cancel := withTargetTimeout(ctx, req.Target)
	defer cancel()
	return provider.Simple(ctx, req)
}

func (s *service) Chat(ctx context.Context, req ChatRequest) (*Completion, error) {
	provider, model, err := s.resolve(req.Target, req.Purpose, "")
	if err != nil {
		return nil, err
	}
	fitted, err := s.fitter.FitChat(ctx, req, int(model.ContextWindow), s.maxOutput(req.Target.Generation, model), s.Simple)
	if err != nil {
		return nil, err
	}
	req.Messages = fitted
	ctx, cancel := withTargetTimeout(ctx, req.Target)
	defer cancel()
	return provider.Chat(ctx, req)
}

func (s *service) Vision(ctx context.Context, req VisionRequest) (*Completion, error) {
	provider, model, err := s.resolve(req.Target, req.Purpose, common.ModelCapabilityVisionAnalysis)
	if err != nil {
		return nil, err
	}
	if err := s.fitter.ValidatePrompt(int(model.ContextWindow), s.maxOutput(req.Target.Generation, model),
		req.Message, req.SystemPrompt); err != nil {
		return nil, fmt.Errorf("vision request too large: %w", err)
	}
	ctx, cancel := withTargetTimeout(ctx, req.Target)
	defer cancel()
	return provider.Vision(ctx, req)
}

func (s *service) Reasoning(ctx context.Context, req ReasoningRequest) (*ReasoningCompletion, error) {
	provider, model, err := s.resolve(req.Target, req.Purpose, common.ModelCapabilityResultsAnalysis)
	if err != nil {
		return nil, err
	}
	if err := s.fitter.ValidatePrompt(int(model.ContextWindow), s.maxOutput(req.Target.Generation, model),
		req.Prompt, req.Context, req.SystemPrompt); err != nil {
		return nil, fmt.Errorf("reasoning request too large: %w", err)
	}
	ctx, cancel := withTargetTimeout(ctx, req.Target)
	defer cancel()
	return provider.Reasoning(ctx, req)
}

func (s *service) Providers() []ProviderInfo { return s.registry.List() }

func (s *service) ProvidersByCapability(capability common.ModelCapability) []ProviderInfo {
	return s.registry.ListByCapability(capability)
}

func (s *service) ProvidersByModel(modelUID string) []ProviderInfo {
	return s.registry.ListByModel(modelUID)
}

// resolve validates the target chain: provider connected → model registered →
// effort allowed → purpose (caller intent) supported → invoke-type capability
// (required) supported. Most specific failure wins; sentinels wrap with %w.
func (s *service) resolve(target common.LLMTarget, purpose, required common.ModelCapability) (Provider, ModelInfo, error) {
	if target.ProviderUID == "" {
		return nil, ModelInfo{}, fmt.Errorf("%w: target has empty provider uid", ErrProviderNotFound)
	}
	provider, ok := s.registry.Get(target.ProviderUID)
	if !ok {
		return nil, ModelInfo{}, fmt.Errorf("%w: %q", ErrProviderNotFound, target.ProviderUID)
	}
	if target.ModelUID == "" {
		return nil, ModelInfo{}, fmt.Errorf("%w: target has empty model uid (provider %q)", ErrModelNotFound, target.ProviderUID)
	}
	info := provider.Info()
	idx := slices.IndexFunc(info.Models, func(m ModelInfo) bool { return m.UID == target.ModelUID })
	if idx < 0 {
		return nil, ModelInfo{}, fmt.Errorf("%w: model %q on provider %q", ErrModelNotFound, target.ModelUID, target.ProviderUID)
	}
	model := info.Models[idx]
	if target.Effort != "" && !slices.Contains(model.Efforts, target.Effort) {
		return nil, ModelInfo{}, fmt.Errorf("%w: effort %q on model %q (allowed: %v)",
			ErrEffortNotAllowed, target.Effort, model.UID, model.Efforts)
	}
	if purpose != "" && !slices.Contains(model.Capabilities, purpose) {
		return nil, ModelInfo{}, fmt.Errorf("%w: purpose %q on model %q",
			ErrCapabilityNotSupported, purpose, model.UID)
	}
	if required != "" && !slices.Contains(model.Capabilities, required) {
		return nil, ModelInfo{}, fmt.Errorf("%w: model %q lacks required capability %q",
			ErrCapabilityNotSupported, model.UID, required)
	}
	if err := ValidateGenerationParams(target.Generation, model.ConfigParams); err != nil {
		return nil, ModelInfo{}, err
	}
	return provider, model, nil
}

func (s *service) maxOutput(params common.GenerationParams, model ModelInfo) int {
	// A provider may advertise "max_tokens" as uint/int/double — honor whichever
	// numeric variant carries the value so the context fitter budgets correctly.
	if v, ok := params["max_tokens"]; ok {
		switch {
		case v.Uint != nil && *v.Uint > 0:
			return int(*v.Uint)
		case v.Int != nil && *v.Int > 0:
			return int(*v.Int)
		case v.Double != nil && *v.Double > 0:
			return int(*v.Double)
		}
	}
	return int(model.MaxOutputTokens)
}

// withTargetTimeout applies the target's explicit timeout (if any) as a context
// deadline. The ProviderBridge derives Invocation.timeout_ms from this deadline.
func withTargetTimeout(ctx context.Context, target common.LLMTarget) (context.Context, context.CancelFunc) {
	if target.TimeoutMs != nil && *target.TimeoutMs > 0 {
		return context.WithTimeout(ctx, time.Duration(*target.TimeoutMs)*time.Millisecond)
	}
	return ctx, func() {}
}

func (s *service) ModelDescriptors(providerUID, modelUID string) ([]ConfigParamDescriptor, bool, bool) {
	provider, ok := s.registry.Get(providerUID)
	if !ok {
		return nil, false, false
	}
	info := provider.Info()
	idx := slices.IndexFunc(info.Models, func(m ModelInfo) bool { return m.UID == modelUID })
	if idx < 0 {
		return nil, false, false
	}
	return info.Models[idx].ConfigParams, info.Models[idx].Reasoning, true
}

func (s *service) MaxImages(providerUID, modelUID string) (uint32, bool) {
	provider, ok := s.registry.Get(providerUID)
	if !ok {
		return 0, false
	}
	info := provider.Info()
	idx := slices.IndexFunc(info.Models, func(m ModelInfo) bool { return m.UID == modelUID })
	if idx < 0 {
		return 0, false
	}
	return info.Models[idx].MaxImages, true
}

func (s *service) ValidateTarget(target common.LLMTarget, purpose common.ModelCapability) error {
	if _, _, err := s.resolve(target, purpose, ""); err != nil {
		return err
	}
	if target.TimeoutMs != nil && *target.TimeoutMs == 0 {
		return ErrInvalidTimeout
	}
	return nil
}
