package sdk

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorf(t *testing.T) {
	err := Errorf(ErrCodeModelNotSupported, "model %q unknown", "x")
	assert.Equal(t, "model_not_supported: model \"x\" unknown", err.Error())

	var typed *Error
	assert.True(t, errors.As(error(err), &typed))
	assert.Equal(t, ErrCodeModelNotSupported, typed.Code)
}
