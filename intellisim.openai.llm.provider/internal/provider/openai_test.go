package provider

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"
)

func uptr(v uint64) *uint64   { return &v }
func dptr(v float64) *float64 { return &v }

func TestBuildChatBody_StandardModel(t *testing.T) {
	params := map[string]sdk.ConfigValue{
		"temperature": {Double: dptr(0.5)},
		"max_tokens":  {Uint: uptr(256)},
		"seed":        {Uint: uptr(42)},
		"stop":        {List: []string{"END"}},
	}
	body := buildChatBody("gpt-4o", "", "hi", "", nil, params, sdk.ResponseFormatJSON)
	if body["temperature"] != 0.5 {
		t.Fatalf("temperature not applied: %v", body["temperature"])
	}
	if body["max_tokens"] != uint64(256) {
		t.Fatalf("expected max_tokens=256, got %v", body["max_tokens"])
	}
	if _, ok := body["max_completion_tokens"]; ok {
		t.Fatal("standard model must not set max_completion_tokens")
	}
	if body["seed"] != uint64(42) {
		t.Fatalf("expected seed=42, got %v", body["seed"])
	}
	if stops, ok := body["stop"].([]string); !ok || len(stops) != 1 || stops[0] != "END" {
		t.Fatalf("stop not applied: %v", body["stop"])
	}
	if body["response_format"] == nil {
		t.Fatal("response_format JSON not applied")
	}
}

func TestBuildChatBody_ReasoningModel(t *testing.T) {
	params := map[string]sdk.ConfigValue{
		"temperature": {Double: dptr(0.5)},
		"max_tokens":  {Uint: uptr(256)},
	}
	body := buildChatBody("o3", "high", "hi", "", nil, params, sdk.ResponseFormatDefault)
	if _, ok := body["temperature"]; ok {
		t.Fatal("reasoning model must reject temperature")
	}
	if body["max_completion_tokens"] != uint64(256) {
		t.Fatalf("expected max_completion_tokens=256, got %v", body["max_completion_tokens"])
	}
	if body["reasoning_effort"] != "high" {
		t.Fatalf("expected reasoning_effort=high, got %v", body["reasoning_effort"])
	}
}

func TestBuildChatBody_OmitsUnset(t *testing.T) {
	body := buildChatBody("gpt-4o", "", "hi", "", nil, nil, sdk.ResponseFormatDefault)
	for _, k := range []string{"temperature", "top_p", "max_tokens", "seed", "stop", "response_format"} {
		if _, ok := body[k]; ok {
			t.Fatalf("unset key %q must be omitted", k)
		}
	}
}

func TestBuildSimpleBody(t *testing.T) {
	params := map[string]sdk.ConfigValue{"max_tokens": {Uint: uptr(128)}, "temperature": {Double: dptr(0.2)}}
	body := buildSimpleBody("gpt-4o", "", "hi", "", params, sdk.ResponseFormatJSON)
	if body["max_output_tokens"] != uint64(128) {
		t.Fatalf("expected max_output_tokens=128, got %v", body["max_output_tokens"])
	}
	if body["temperature"] != 0.2 {
		t.Fatalf("expected temperature=0.2, got %v", body["temperature"])
	}
	if body["text"] == nil {
		t.Fatal("responses-API JSON format not applied")
	}
}

func TestBuildSimpleBody_ReasoningModel(t *testing.T) {
	params := map[string]sdk.ConfigValue{"temperature": {Double: dptr(0.5)}, "max_tokens": {Uint: uptr(128)}}
	body := buildSimpleBody("o4-mini", sdk.EffortLow, "hi", "", params, sdk.ResponseFormatDefault)
	if _, ok := body["temperature"]; ok {
		t.Fatal("reasoning model must reject temperature on the Simple path")
	}
	if body["max_output_tokens"] != uint64(128) {
		t.Fatalf("expected max_output_tokens=128, got %v", body["max_output_tokens"])
	}
	reasoningBlock, ok := body["reasoning"].(map[string]string)
	if !ok || reasoningBlock["effort"] != "low" {
		t.Fatalf("expected reasoning effort block, got %v", body["reasoning"])
	}
}

func TestBuildVisionBody(t *testing.T) {
	images := []sdk.VisionImage{
		{ImageData: []byte{1, 2, 3}, ImageFormat: "png"},
		{ImageData: []byte{4, 5, 6}, ImageFormat: "jpeg", Description: "obstacle layer"},
	}
	body := buildVisionBody("gpt-4o", "what is this", images, "", "sys", nil, sdk.ResponseFormatDefault)
	messages := body["messages"].([]any)
	require.Len(t, messages, 2) // system + user
	user := messages[1].(map[string]any)
	content := user["content"].([]map[string]any)
	// prompt text + image1 + (label + image2) = 4 parts
	require.Len(t, content, 4)
	assert.Equal(t, "what is this", content[0]["text"])
	img1 := content[1]["image_url"].(map[string]string)
	assert.True(t, strings.HasPrefix(img1["url"], "data:image/png;base64,"))
	assert.Equal(t, "high", img1["detail"], "empty detail defaults to high")
	// the labelled image is preceded by its description text part
	assert.Equal(t, "obstacle layer", content[2]["text"])
	img2 := content[3]["image_url"].(map[string]string)
	assert.True(t, strings.HasPrefix(img2["url"], "data:image/jpeg;base64,"))
}

func TestBuildReasoningBody(t *testing.T) {
	body := buildReasoningBody("o3", sdk.EffortMedium, "why", "the context", "", nil, sdk.ResponseFormatDefault)
	messages := body["messages"].([]map[string]string)
	require.Len(t, messages, 2)
	assert.Contains(t, messages[0]["content"], "expert analyst") // default system prompt
	assert.Contains(t, messages[1]["content"], "Context:\nthe context")
	assert.Contains(t, messages[1]["content"], "Analysis Request:\nwhy")
	assert.Equal(t, "medium", body["reasoning_effort"])
}

func TestCheckModel(t *testing.T) {
	assert.NoError(t, checkModel("gpt-4o"))

	err := checkModel("unknown-model")
	require.Error(t, err)
	var typed *sdk.Error
	require.True(t, errors.As(err, &typed))
	assert.Equal(t, sdk.ErrCodeModelNotSupported, typed.Code)
}

func TestModelRegistry(t *testing.T) {
	assert.True(t, IsModelSupported("o3"))
	assert.False(t, IsModelSupported("gpt-1"))
	assert.True(t, IsReasoningModel("o4-mini"))
	assert.False(t, IsReasoningModel("gpt-4o"))
}
