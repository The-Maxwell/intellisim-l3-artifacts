package chats

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"intelli.sim.core/pkg/httpapi"
)

func validateDTO(t *testing.T, dto any) *httpapi.APIError {
	t.Helper()
	err := httpapi.NewGoPlaygroundValidator().Validate(dto)
	if err == nil {
		return nil
	}
	var apiErr *httpapi.APIError
	require.True(t, errors.As(err, &apiErr))
	return apiErr
}

func TestCreateChatRequest_Validation(t *testing.T) {
	assert.Nil(t, validateDTO(t, &CreateChatRequest{
		LLMTarget: &LLMTargetDTO{ProviderUID: "openai", ModelUID: "gpt-4o"},
	}))

	apiErr := validateDTO(t, &CreateChatRequest{})
	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Fields, "llm_target")

	apiErr = validateDTO(t, &CreateChatRequest{
		LLMTarget: &LLMTargetDTO{ProviderUID: "openai", ModelUID: "gpt-4o", Effort: "extreme"},
	})
	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Fields["llm_target.effort"], "oneof")
}

func TestSendMessageRequest_Validation(t *testing.T) {
	assert.Nil(t, validateDTO(t, &SendMessageRequest{Content: "hello"}))

	apiErr := validateDTO(t, &SendMessageRequest{Content: "   "})
	require.NotNil(t, apiErr)
	assert.Equal(t, "notblank", apiErr.Fields["content"])
}

func TestListMessagesQuery_Validation(t *testing.T) {
	assert.Nil(t, validateDTO(t, &ListMessagesQuery{Limit: 30, Before: "68bd5a3f2c1d4e0f9a7b3c2d"}))

	apiErr := validateDTO(t, &ListMessagesQuery{Before: "zz"})
	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Fields, "before")
}
