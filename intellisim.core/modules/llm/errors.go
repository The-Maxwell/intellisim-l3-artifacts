package llm

import (
	"errors"
	"fmt"
)

// Validation/routing sentinels — wrap with %w and match with errors.Is.
var (
	ErrProviderNotFound          = errors.New("llm: provider not found or not connected")
	ErrModelNotFound             = errors.New("llm: model not registered by provider")
	ErrEffortNotAllowed          = errors.New("llm: effort not allowed for model")
	ErrCapabilityNotSupported    = errors.New("llm: capability not supported by model")
	ErrProviderDisconnected      = errors.New("llm: provider disconnected while waiting for response")
	ErrMalformedProviderResponse = errors.New("llm: provider returned malformed or unexpected result type")

	ErrConfigParamUnknown  = errors.New("llm: unknown generation config parameter")
	ErrConfigParamType     = errors.New("llm: generation config parameter has wrong type")
	ErrConfigParamRange    = errors.New("llm: generation config parameter out of range")
	ErrConfigParamEnum     = errors.New("llm: generation config parameter not an allowed value")
	ErrConfigParamRequired = errors.New("llm: required generation config parameter missing")
	ErrInvalidTimeout      = errors.New("llm: timeout_ms must be greater than zero when set")
)

// ProviderErrorCode mirrors the wire-level error codes a provider can return.
type ProviderErrorCode string

const (
	ProviderErrInvalidRequest         ProviderErrorCode = "invalid_request"
	ProviderErrModelNotSupported      ProviderErrorCode = "model_not_supported"
	ProviderErrEffortNotSupported     ProviderErrorCode = "effort_not_supported"
	ProviderErrCapabilityNotSupported ProviderErrorCode = "capability_not_supported"
	ProviderErrUpstreamError          ProviderErrorCode = "upstream_error"
	ProviderErrTimeout                ProviderErrorCode = "timeout"
	ProviderErrInternal               ProviderErrorCode = "internal"
	ProviderErrUnspecified            ProviderErrorCode = "unspecified"
)

// ProviderError is a typed failure reported BY the provider (vs. core-side
// validation sentinels above). Distinguish with errors.As.
type ProviderError struct {
	Code    ProviderErrorCode
	Message string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("llm provider error (%s): %s", e.Code, e.Message)
}
