package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
)

// fakeProvider records the last request of each kind and returns canned responses.
type fakeProvider struct {
	info          ProviderInfo
	lastSimple    *SimpleRequest
	lastChat      *ChatRequest
	lastVision    *VisionRequest
	lastReasoning *ReasoningRequest
}

func (f *fakeProvider) UID() string        { return f.info.UID }
func (f *fakeProvider) Info() ProviderInfo { return f.info }
func (f *fakeProvider) Simple(ctx context.Context, req SimpleRequest) (*Completion, error) {
	f.lastSimple = &req
	return &Completion{Text: "simple-ok"}, nil
}
func (f *fakeProvider) Chat(ctx context.Context, req ChatRequest) (*Completion, error) {
	f.lastChat = &req
	return &Completion{Text: "chat-ok"}, nil
}
func (f *fakeProvider) Vision(ctx context.Context, req VisionRequest) (*Completion, error) {
	f.lastVision = &req
	return &Completion{Text: "vision-ok"}, nil
}
func (f *fakeProvider) Reasoning(ctx context.Context, req ReasoningRequest) (*ReasoningCompletion, error) {
	f.lastReasoning = &req
	return &ReasoningCompletion{Analysis: "reasoning-ok"}, nil
}

func newTestProvider() *fakeProvider {
	return &fakeProvider{info: ProviderInfo{
		UID: "p1", Name: "P1",
		Models: []ModelInfo{
			{
				UID: "general",
				Capabilities: []common.ModelCapability{
					common.ModelCapabilityChat, common.ModelCapabilitySimulationDecisions,
					common.ModelCapabilityVisionAnalysis,
				},
				Efforts:       []common.LLMEffort{common.LLMEffortLow, common.LLMEffortHigh},
				ContextWindow: 128000, MaxOutputTokens: 4096,
			},
			{
				UID:           "analyst",
				Capabilities:  []common.ModelCapability{common.ModelCapabilityResultsAnalysis},
				Efforts:       []common.LLMEffort{common.LLMEffortHigh},
				ContextWindow: 200000, MaxOutputTokens: 100000,
			},
		},
	}}
}

func newTestService(t *testing.T) (Service, *fakeProvider, Registry) {
	t.Helper()
	logger := zap.NewNop()
	reg := NewRegistry(logger)
	provider := newTestProvider()
	reg.Register(provider)
	fitter := NewContextFitter(logger, NewTokenEstimator(logger))
	return NewService(logger, reg, fitter), provider, reg
}

func target(model string, effort common.LLMEffort) common.LLMTarget {
	return common.LLMTarget{ProviderUID: "p1", ModelUID: model, Effort: effort}
}

// --- registry ---

func TestRegistry_RegisterGetUnregister(t *testing.T) {
	reg := NewRegistry(zap.NewNop())
	p := newTestProvider()
	reg.Register(p)

	got, ok := reg.Get("p1")
	require.True(t, ok)
	assert.Equal(t, "p1", got.UID())
	assert.Len(t, reg.List(), 1)

	reg.Unregister("p1")
	_, ok = reg.Get("p1")
	assert.False(t, ok)
	assert.Empty(t, reg.List())
}

func TestRegistry_Filters(t *testing.T) {
	reg := NewRegistry(zap.NewNop())
	reg.Register(newTestProvider())

	byCap := reg.ListByCapability(common.ModelCapabilityResultsAnalysis)
	require.Len(t, byCap, 1)
	require.Len(t, byCap[0].Models, 1)
	assert.Equal(t, "analyst", byCap[0].Models[0].UID)

	byModel := reg.ListByModel("general")
	require.Len(t, byModel, 1)
	require.Len(t, byModel[0].Models, 1)
	assert.Equal(t, "general", byModel[0].Models[0].UID)

	assert.Empty(t, reg.ListByCapability(common.ModelCapabilityModelGeneration))
	assert.Empty(t, reg.ListByModel("nope"))
}

// --- service validation matrix ---

func TestService_ValidationMatrix(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{"empty provider uid", func() error {
			_, err := svc.Simple(ctx, SimpleRequest{Target: common.LLMTarget{ModelUID: "general"}})
			return err
		}, ErrProviderNotFound},
		{"unknown provider", func() error {
			_, err := svc.Simple(ctx, SimpleRequest{Target: common.LLMTarget{ProviderUID: "ghost", ModelUID: "general"}})
			return err
		}, ErrProviderNotFound},
		{"empty model uid", func() error {
			_, err := svc.Simple(ctx, SimpleRequest{Target: common.LLMTarget{ProviderUID: "p1"}})
			return err
		}, ErrModelNotFound},
		{"unknown model", func() error {
			_, err := svc.Simple(ctx, SimpleRequest{Target: target("nope", "")})
			return err
		}, ErrModelNotFound},
		{"effort not allowed", func() error {
			_, err := svc.Simple(ctx, SimpleRequest{Target: target("general", common.LLMEffortMax)})
			return err
		}, ErrEffortNotAllowed},
		{"purpose not supported", func() error {
			_, err := svc.Simple(ctx, SimpleRequest{
				Target: target("general", ""), Purpose: common.ModelCapabilityResultsAnalysis,
			})
			return err
		}, ErrCapabilityNotSupported},
		{"vision requires vision_analysis", func() error {
			_, err := svc.Vision(ctx, VisionRequest{Target: target("analyst", "")})
			return err
		}, ErrCapabilityNotSupported},
		{"reasoning requires results_analysis", func() error {
			_, err := svc.Reasoning(ctx, ReasoningRequest{Target: target("general", "")})
			return err
		}, ErrCapabilityNotSupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(), tc.wantErr)
		})
	}
}

func TestValidateTarget_RejectsBadTemperature(t *testing.T) {
	min, max := 0.0, 2.0
	reg := NewRegistry(zap.NewNop())
	reg.Register(&fakeProvider{info: ProviderInfo{UID: "openai", Models: []ModelInfo{{
		UID: "gpt-4o", Capabilities: []common.ModelCapability{common.ModelCapabilityChat},
		ConfigParams: []ConfigParamDescriptor{{Key: "temperature", Type: ConfigParamDouble, Min: &min, Max: &max}},
	}}}})
	svc := NewService(zap.NewNop(), reg, nil)
	hot := 5.0
	err := svc.ValidateTarget(common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o",
		Generation: common.GenerationParams{"temperature": {Double: &hot}}}, common.ModelCapabilityChat)
	if !assert.ErrorIs(t, err, ErrConfigParamRange) {
		t.Fatalf("expected range error, got %v", err)
	}
}

func TestValidateTarget_RejectsZeroTimeout(t *testing.T) {
	reg := NewRegistry(zap.NewNop())
	reg.Register(&fakeProvider{info: ProviderInfo{UID: "openai", Models: []ModelInfo{{
		UID: "gpt-4o", Capabilities: []common.ModelCapability{common.ModelCapabilityChat},
	}}}})
	svc := NewService(zap.NewNop(), reg, nil)
	zero := uint64(0)
	err := svc.ValidateTarget(common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o", TimeoutMs: &zero},
		common.ModelCapabilityChat)
	assert.ErrorIs(t, err, ErrInvalidTimeout)
}

func TestModelDescriptors(t *testing.T) {
	min, max := 0.0, 2.0
	reg := NewRegistry(zap.NewNop())
	reg.Register(&fakeProvider{info: ProviderInfo{UID: "openai", Models: []ModelInfo{{
		UID: "gpt-4o", Capabilities: []common.ModelCapability{common.ModelCapabilityChat},
		Reasoning:    true,
		ConfigParams: []ConfigParamDescriptor{{Key: "temperature", Type: ConfigParamDouble, Min: &min, Max: &max}},
	}}}})
	svc := NewService(zap.NewNop(), reg, nil)

	descs, reasoning, ok := svc.ModelDescriptors("openai", "gpt-4o")
	require.True(t, ok)
	assert.True(t, reasoning)
	require.Len(t, descs, 1)
	assert.Equal(t, "temperature", descs[0].Key)

	_, _, ok = svc.ModelDescriptors("openai", "nope")
	assert.False(t, ok)
	_, _, ok = svc.ModelDescriptors("ghost", "gpt-4o")
	assert.False(t, ok)
}

func TestService_HappyPaths(t *testing.T) {
	svc, provider, _ := newTestService(t)
	ctx := context.Background()

	simple, err := svc.Simple(ctx, SimpleRequest{
		Target:  target("general", common.LLMEffortLow),
		Purpose: common.ModelCapabilitySimulationDecisions,
		Message: "decide",
	})
	require.NoError(t, err)
	assert.Equal(t, "simple-ok", simple.Text)
	require.NotNil(t, provider.lastSimple)
	assert.Equal(t, common.LLMEffortLow, provider.lastSimple.Target.Effort)

	chat, err := svc.Chat(ctx, ChatRequest{
		Target: target("general", ""), Purpose: common.ModelCapabilityChat, Message: "hi",
	})
	require.NoError(t, err)
	assert.Equal(t, "chat-ok", chat.Text)

	vision, err := svc.Vision(ctx, VisionRequest{
		Target: target("general", ""), Message: "what is this",
		Images: []VisionImage{{Data: []byte{1, 2}, Format: "png"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "vision-ok", vision.Text)

	reasoning, err := svc.Reasoning(ctx, ReasoningRequest{
		Target: target("analyst", common.LLMEffortHigh), Prompt: "why",
	})
	require.NoError(t, err)
	assert.Equal(t, "reasoning-ok", reasoning.Analysis)
}
