package httpapi

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type GoPlaygroundValidator struct {
	validator *validator.Validate
}

// Validate translates go-playground failures into the shared APIError envelope
// (code "validation_failed") with snake_case JSON field paths, e.g.
// "llm_target.provider_uid": "required".
func (v *GoPlaygroundValidator) Validate(i any) error {
	err := v.validator.Struct(i)
	if err == nil {
		return nil
	}
	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		// Defensive fallback: go-playground returns ValidationErrors for all
		// struct failures, so this branch should not be reached in practice.
		// Policy: "validation_failed" is always 422, even on this fallback path.
		return NewAPIError(http.StatusUnprocessableEntity, "validation_failed", err.Error())
	}
	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[jsonFieldPath(fe.Namespace())] = constraintText(fe)
	}
	return NewValidationError(fields)
}

// jsonFieldPath strips the root struct name from a namespace like
// "CreateChatRequest.llm_target.provider_uid".
func jsonFieldPath(namespace string) string {
	if _, after, found := strings.Cut(namespace, "."); found {
		return after
	}
	return namespace
}

func constraintText(fe validator.FieldError) string {
	if fe.Param() != "" {
		return fe.Tag() + "=" + fe.Param()
	}
	return fe.Tag()
}

func NewGoPlaygroundValidator() *GoPlaygroundValidator {
	v := validator.New()

	// Report fields by their snake_case json names, not Go struct names.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name, _, _ := strings.Cut(fld.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return fld.Name
		}
		return name
	})

	// notblank: non-empty after trimming whitespace (use with required on strings).
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != ""
	})

	return &GoPlaygroundValidator{validator: v}
}
