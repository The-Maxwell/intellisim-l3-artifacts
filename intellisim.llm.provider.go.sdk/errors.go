package sdk

import "fmt"

// ErrorCode mirrors the wire-level error codes. Returning a *Error from a
// handler sends the exact code to core; any other error becomes ErrCodeUpstreamError.
type ErrorCode string

const (
	ErrCodeInvalidRequest         ErrorCode = "invalid_request"
	ErrCodeModelNotSupported      ErrorCode = "model_not_supported"
	ErrCodeEffortNotSupported     ErrorCode = "effort_not_supported"
	ErrCodeCapabilityNotSupported ErrorCode = "capability_not_supported"
	ErrCodeUpstreamError          ErrorCode = "upstream_error"
	ErrCodeTimeout                ErrorCode = "timeout"
	ErrCodeInternal               ErrorCode = "internal"
)

// Error is a typed handler error carrying a wire-level code.
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Errorf builds a typed *Error.
func Errorf(code ErrorCode, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
