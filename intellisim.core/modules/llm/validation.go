package llm

import (
	"fmt"
	"slices"

	"intelli.sim.core/data/common"
)

// configValueFieldsSet counts how many of the union's variants are populated.
func configValueFieldsSet(v common.ConfigValue) int {
	n := 0
	if v.Double != nil {
		n++
	}
	if v.Int != nil {
		n++
	}
	if v.Uint != nil {
		n++
	}
	if v.Bool != nil {
		n++
	}
	if v.String != nil {
		n++
	}
	if v.List != nil {
		n++
	}
	return n
}

// ValidateConfigValue checks one value against one descriptor.
func ValidateConfigValue(desc ConfigParamDescriptor, v common.ConfigValue) error {
	// A ConfigValue is a tagged union: exactly one variant must be set. Reject
	// malformed values (none or several set) before the type-specific checks.
	if n := configValueFieldsSet(v); n != 1 {
		return fmt.Errorf("%w: %q must set exactly one value field (got %d)", ErrConfigParamType, desc.Key, n)
	}
	switch desc.Type {
	case ConfigParamDouble:
		if v.Double == nil {
			return fmt.Errorf("%w: %q expects double", ErrConfigParamType, desc.Key)
		}
		return checkRange(desc, *v.Double)
	case ConfigParamInt:
		if v.Int == nil {
			return fmt.Errorf("%w: %q expects int", ErrConfigParamType, desc.Key)
		}
		return checkRange(desc, float64(*v.Int))
	case ConfigParamUint:
		if v.Uint == nil {
			return fmt.Errorf("%w: %q expects uint", ErrConfigParamType, desc.Key)
		}
		return checkRange(desc, float64(*v.Uint))
	case ConfigParamBool:
		if v.Bool == nil {
			return fmt.Errorf("%w: %q expects bool", ErrConfigParamType, desc.Key)
		}
	case ConfigParamString:
		if v.String == nil {
			return fmt.Errorf("%w: %q expects string", ErrConfigParamType, desc.Key)
		}
	case ConfigParamEnum:
		if v.String == nil {
			return fmt.Errorf("%w: %q expects enum string", ErrConfigParamType, desc.Key)
		}
		if !slices.Contains(desc.EnumValues, *v.String) {
			return fmt.Errorf("%w: %q=%q (allowed: %v)", ErrConfigParamEnum, desc.Key, *v.String, desc.EnumValues)
		}
	case ConfigParamStringList:
		if v.List == nil {
			return fmt.Errorf("%w: %q expects string list", ErrConfigParamType, desc.Key)
		}
	default:
		return fmt.Errorf("%w: %q has unknown descriptor type", ErrConfigParamType, desc.Key)
	}
	return nil
}

func checkRange(desc ConfigParamDescriptor, f float64) error {
	if desc.Min != nil && f < *desc.Min {
		return fmt.Errorf("%w: %q=%v < min %v", ErrConfigParamRange, desc.Key, f, *desc.Min)
	}
	if desc.Max != nil && f > *desc.Max {
		return fmt.Errorf("%w: %q=%v > max %v", ErrConfigParamRange, desc.Key, f, *desc.Max)
	}
	return nil
}

// ValidateGenerationParams validates a dynamic config map against descriptors:
// unknown keys, per-value type/range/enum, and required-descriptor presence.
func ValidateGenerationParams(params common.GenerationParams, descriptors []ConfigParamDescriptor) error {
	byKey := make(map[string]ConfigParamDescriptor, len(descriptors))
	for _, d := range descriptors {
		byKey[d.Key] = d
	}
	for key, val := range params {
		d, ok := byKey[key]
		if !ok {
			return fmt.Errorf("%w: %q", ErrConfigParamUnknown, key)
		}
		if err := ValidateConfigValue(d, val); err != nil {
			return err
		}
	}
	for _, d := range descriptors {
		if d.Required {
			if _, ok := params[d.Key]; !ok {
				return fmt.Errorf("%w: %q", ErrConfigParamRequired, d.Key)
			}
		}
	}
	return nil
}
