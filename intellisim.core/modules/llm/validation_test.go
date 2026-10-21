package llm

import (
	"errors"
	"testing"

	"intelli.sim.core/data/common"
)

func descTemp() ConfigParamDescriptor {
	min, max := 0.0, 2.0
	return ConfigParamDescriptor{Key: "temperature", Type: ConfigParamDouble, Min: &min, Max: &max}
}

func TestValidateGenerationParams_Range(t *testing.T) {
	hot := 3.0
	err := ValidateGenerationParams(common.GenerationParams{"temperature": {Double: &hot}}, []ConfigParamDescriptor{descTemp()})
	if !errors.Is(err, ErrConfigParamRange) {
		t.Fatalf("expected range error, got %v", err)
	}
}

func TestValidateGenerationParams_UnknownKey(t *testing.T) {
	v := 1.0
	err := ValidateGenerationParams(common.GenerationParams{"nope": {Double: &v}}, []ConfigParamDescriptor{descTemp()})
	if !errors.Is(err, ErrConfigParamUnknown) {
		t.Fatalf("expected unknown error, got %v", err)
	}
}

func TestValidateGenerationParams_WrongType(t *testing.T) {
	s := "hot"
	err := ValidateGenerationParams(common.GenerationParams{"temperature": {String: &s}}, []ConfigParamDescriptor{descTemp()})
	if !errors.Is(err, ErrConfigParamType) {
		t.Fatalf("expected type error, got %v", err)
	}
}

func TestValidateGenerationParams_Enum(t *testing.T) {
	d := ConfigParamDescriptor{Key: "mode", Type: ConfigParamEnum, EnumValues: []string{"a", "b"}}
	bad := "c"
	err := ValidateGenerationParams(common.GenerationParams{"mode": {String: &bad}}, []ConfigParamDescriptor{d})
	if !errors.Is(err, ErrConfigParamEnum) {
		t.Fatalf("expected enum error, got %v", err)
	}
}

func TestValidateConfigValue_RequiresExactlyOneField(t *testing.T) {
	d := descTemp()
	// none set
	if err := ValidateConfigValue(d, common.ConfigValue{}); !errors.Is(err, ErrConfigParamType) {
		t.Fatalf("empty value should fail with type error, got %v", err)
	}
	// two set
	x, y := 0.5, "extra"
	if err := ValidateConfigValue(d, common.ConfigValue{Double: &x, String: &y}); !errors.Is(err, ErrConfigParamType) {
		t.Fatalf("multi-field value should fail with type error, got %v", err)
	}
}

func TestValidateGenerationParams_OKAndNil(t *testing.T) {
	ok := 0.5
	if err := ValidateGenerationParams(common.GenerationParams{"temperature": {Double: &ok}}, []ConfigParamDescriptor{descTemp()}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateGenerationParams(nil, []ConfigParamDescriptor{descTemp()}); err != nil {
		t.Fatalf("nil params should pass: %v", err)
	}
}
