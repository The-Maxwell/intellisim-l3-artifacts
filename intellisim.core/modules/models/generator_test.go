package models

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	"intelli.sim.core/modules/llm"
)

// fakeLLM is a minimal llm.Service double for generator tests. Chat returns a
// canned completion (text + optional error); Vision mirrors Chat via visionResp.
type fakeLLM struct {
	resp        string
	chatErr     error
	visionResp  string
	visionErr   error
	validateErr error
	maxImages   uint32 // 0 = unset (MaxImages reports ok=false)

	gotChat   *llm.ChatRequest
	gotVision *llm.VisionRequest
}

func (f *fakeLLM) Simple(context.Context, llm.SimpleRequest) (*llm.Completion, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.Completion, error) {
	r := req
	f.gotChat = &r
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	return &llm.Completion{
		Text:  f.resp,
		Usage: common.TokenUsage{LLMTarget: req.Target, TotalTokens: 11, Calls: 1},
	}, nil
}

func (f *fakeLLM) Vision(_ context.Context, req llm.VisionRequest) (*llm.Completion, error) {
	r := req
	f.gotVision = &r
	if f.visionErr != nil {
		return nil, f.visionErr
	}
	return &llm.Completion{
		Text:  f.visionResp,
		Usage: common.TokenUsage{LLMTarget: req.Target, TotalTokens: 5, Calls: 1},
	}, nil
}

func (f *fakeLLM) Reasoning(context.Context, llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeLLM) Providers() []llm.ProviderInfo { return nil }
func (f *fakeLLM) ProvidersByCapability(common.ModelCapability) []llm.ProviderInfo {
	return nil
}
func (f *fakeLLM) ProvidersByModel(string) []llm.ProviderInfo { return nil }
func (f *fakeLLM) ModelDescriptors(string, string) ([]llm.ConfigParamDescriptor, bool, bool) {
	return nil, false, false
}
func (f *fakeLLM) MaxImages(string, string) (uint32, bool) {
	if f.maxImages == 0 {
		return 0, false
	}
	return f.maxImages, true
}
func (f *fakeLLM) ValidateTarget(common.LLMTarget, common.ModelCapability) error {
	return f.validateErr
}

func TestGenerator_Generate_AppliesEnvelope(t *testing.T) {
	fake := &fakeLLM{resp: `{"display_message":"added prey","model":{"agents":[{"uid":"prey","name":"Prey","type":"agent","initialization":{"population_count":10}}]}}`}
	g := NewGenerator(zap.NewNop(), fake)

	out, err := g.Generate(context.Background(), GenerateInput{
		Model:       datamodels.Model{},
		UserMessage: "add prey",
		Scope:       EditScope{},
		Target:      common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
	})
	require.NoError(t, err)
	require.Equal(t, "added prey", out.DisplayMessage)
	prey := findAgent(out.Model, "prey")
	require.NotNil(t, prey)
	require.Equal(t, "Prey", prey.Name)
	require.NotEmpty(t, out.Raw) // raw json captured for RawContent
	require.Equal(t, uint64(11), out.Usage.TotalTokens)

	// The generator must route via the model-generation purpose with JSON format.
	require.NotNil(t, fake.gotChat)
	require.Equal(t, common.ModelCapabilityModelGeneration, fake.gotChat.Purpose)
	require.Equal(t, llm.ResponseFormatJSON, fake.gotChat.ResponseFormat)
	require.Equal(t, "model", fake.gotChat.ChatType)
}

func TestGenerator_Generate_PropagatesLLMError(t *testing.T) {
	fake := &fakeLLM{chatErr: llm.ErrProviderDisconnected}
	g := NewGenerator(zap.NewNop(), fake)

	_, err := g.Generate(context.Background(), GenerateInput{
		Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
	})
	require.ErrorIs(t, err, llm.ErrProviderDisconnected)
}

func TestGenerator_Generate_ParseErrorWrapped(t *testing.T) {
	fake := &fakeLLM{resp: "not json at all"}
	g := NewGenerator(zap.NewNop(), fake)

	_, err := g.Generate(context.Background(), GenerateInput{
		Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse llm response")
}

func TestGenerator_GenerateEnvironmentFromImage_MultiImageLabelled(t *testing.T) {
	fake := &fakeLLM{visionResp: `{"display_message":"env","model":{"environment":{"width":10,"height":10}}}`}
	g := NewGenerator(zap.NewNop(), fake)

	out, err := g.GenerateEnvironmentFromImage(context.Background(), VisionInput{
		Model: datamodels.Model{IsSpatial: true},
		Images: []datamodels.ModelImage{
			{Data: "AQID", MimeType: "image/png", Name: "base"},          // []byte{1,2,3}
			{Data: "AQID", MimeType: "image/jpeg", Description: "obstacle layer"},
		},
		Prompt: "extract",
		Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
	})
	require.NoError(t, err)
	require.Equal(t, "env", out.DisplayMessage)

	// ALL images are forwarded (not just the first), each decoded + labelled.
	require.NotNil(t, fake.gotVision)
	require.Len(t, fake.gotVision.Images, 2)
	require.Equal(t, common.ModelCapabilityVisionAnalysis, fake.gotVision.Purpose)
	require.Equal(t, llm.ResponseFormatJSON, fake.gotVision.ResponseFormat)
	require.Equal(t, "png", fake.gotVision.Images[0].Format)
	require.Equal(t, "jpeg", fake.gotVision.Images[1].Format)
	require.Equal(t, []byte{1, 2, 3}, fake.gotVision.Images[0].Data)
	require.Contains(t, fake.gotVision.Images[0].Description, "base")           // name → label
	require.Contains(t, fake.gotVision.Images[1].Description, "obstacle layer") // description → label
}

func TestResolveMaxImages(t *testing.T) {
	require.Equal(t, 1, resolveMaxImages(0, false)) // unregistered → 1
	require.Equal(t, 1, resolveMaxImages(0, true))  // unset → 1
	require.Equal(t, 5, resolveMaxImages(5, true))
}

func TestVisionImageLabel(t *testing.T) {
	require.Equal(t, "Reference image 1", visionImageLabel(0, datamodels.ModelImage{}))
	require.Equal(t, "Reference image 2 — floor", visionImageLabel(1, datamodels.ModelImage{Name: "floor"}))
	require.Equal(t, "Reference image 1: zones", visionImageLabel(0, datamodels.ModelImage{Description: "zones"}))
	require.Equal(t, "Reference image 1 — floor: zones", visionImageLabel(0, datamodels.ModelImage{Name: "floor", Description: "zones"}))
}

func TestParseEnvelope(t *testing.T) {
	t.Run("raw object", func(t *testing.T) {
		env, err := parseEnvelope(`{"display_message":"hi","model":{"description":"d"}}`)
		require.NoError(t, err)
		require.Equal(t, "hi", env.DisplayMessage)
		require.NotNil(t, env.Model)
		require.Equal(t, "d", env.Model.Description)
	})

	t.Run("fenced json", func(t *testing.T) {
		env, err := parseEnvelope("```json\n{\"display_message\":\"fenced\",\"model\":{}}\n```")
		require.NoError(t, err)
		require.Equal(t, "fenced", env.DisplayMessage)
	})

	t.Run("bare fence", func(t *testing.T) {
		env, err := parseEnvelope("```\n{\"display_message\":\"bare\"}\n```")
		require.NoError(t, err)
		require.Equal(t, "bare", env.DisplayMessage)
	})

	t.Run("prose then object", func(t *testing.T) {
		env, err := parseEnvelope("Sure! Here you go:\n{\"display_message\":\"prefixed\",\"model\":{}}\nHope that helps.")
		require.NoError(t, err)
		require.Equal(t, "prefixed", env.DisplayMessage)
	})

	t.Run("braces inside strings", func(t *testing.T) {
		env, err := parseEnvelope(`{"display_message":"a } brace { in text","model":{}}`)
		require.NoError(t, err)
		require.Equal(t, "a } brace { in text", env.DisplayMessage)
	})

	t.Run("garbage", func(t *testing.T) {
		_, err := parseEnvelope("the model could not produce JSON")
		require.Error(t, err)
	})

	t.Run("malformed object does not panic", func(t *testing.T) {
		_, err := parseEnvelope(`{"display_message": "unterminated`)
		require.Error(t, err)
	})
}
