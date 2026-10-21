package httpapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nestedTarget struct {
	ProviderUID string `json:"provider_uid" validate:"required,min=1,max=100"`
	ModelUID    string `json:"model_uid" validate:"required,min=1,max=100"`
	Effort      string `json:"effort,omitempty" validate:"omitempty,oneof=minimal low medium high max"`
}

type testCreateReq struct {
	Name      string        `json:"name" validate:"omitempty,max=100"`
	Content   string        `json:"content" validate:"required,notblank,max=20"`
	LLMTarget *nestedTarget `json:"llm_target" validate:"required"`
}

func TestValidate_TranslatesToAPIErrorWithJSONFieldNames(t *testing.T) {
	v := NewGoPlaygroundValidator()

	err := v.Validate(&testCreateReq{Content: "", LLMTarget: &nestedTarget{ModelUID: "gpt-4o", Effort: "wrong"}})
	require.Error(t, err)

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 422, apiErr.Status)
	assert.Equal(t, "validation_failed", apiErr.Code)
	assert.Equal(t, "required", apiErr.Fields["content"])
	assert.Equal(t, "required", apiErr.Fields["llm_target.provider_uid"])
	assert.Contains(t, apiErr.Fields["llm_target.effort"], "oneof")
}

func TestValidate_NotBlankRejectsWhitespace(t *testing.T) {
	v := NewGoPlaygroundValidator()
	err := v.Validate(&testCreateReq{Content: "   ", LLMTarget: &nestedTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "notblank", apiErr.Fields["content"])
}

func TestValidate_PassesValidStruct(t *testing.T) {
	v := NewGoPlaygroundValidator()
	err := v.Validate(&testCreateReq{Content: "hi", LLMTarget: &nestedTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}})
	assert.NoError(t, err)
}

// TestValidate_NonValidationErrorFallbackIs422 exercises the defensive `!ok`
// branch: go-playground/validator returns *InvalidValidationError (not
// ValidationErrors) when handed a nil struct pointer. Policy is that
// "validation_failed" is 422 everywhere, including this fallback path.
func TestValidate_NonValidationErrorFallbackIs422(t *testing.T) {
	v := NewGoPlaygroundValidator()

	err := v.Validate((*testCreateReq)(nil))
	require.Error(t, err)

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 422, apiErr.Status)
	assert.Equal(t, "validation_failed", apiErr.Code)
}
