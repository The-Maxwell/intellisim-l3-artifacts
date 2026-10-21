package httpapi

import "net/http"

// APIError is the shared error envelope for HTTP responses:
//
//	{ "code": "validation_failed", "message": "...", "fields": {"content": "max=10000"} }
//
// Handlers/middleware return *APIError from echo handlers; the custom
// HTTPErrorHandler (server.go) serializes it with the embedded status.
type APIError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// NewAPIError builds an APIError with an explicit HTTP status and machine-readable code.
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// NewValidationError reports per-field DTO validation failures (422).
func NewValidationError(fields map[string]string) *APIError {
	return &APIError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_failed",
		Message: "request validation failed",
		Fields:  fields,
	}
}
