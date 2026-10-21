package common

import (
	"go.mongodb.org/mongo-driver/v2/bson"
)

// NormalizeValue recursively converts bson container/primitive types produced by
// driver decoding into plain Go values, so no mongo type ever crosses the
// repository boundary inside an interface{} field:
//
//	bson.D / bson.M  -> map[string]interface{}
//	bson.A           -> []interface{}
//	bson.DateTime    -> time.Time
//	everything else  -> unchanged
//
// Maps and slices are rebuilt (never aliased) and their elements normalized.
func NormalizeValue(v interface{}) interface{} {
	switch t := v.(type) {
	case bson.D:
		m := make(map[string]interface{}, len(t))
		for _, e := range t {
			m[e.Key] = NormalizeValue(e.Value)
		}
		return m
	case bson.M:
		m := make(map[string]interface{}, len(t))
		for k, val := range t {
			m[k] = NormalizeValue(val)
		}
		return m
	case bson.A:
		s := make([]interface{}, len(t))
		for i, val := range t {
			s[i] = NormalizeValue(val)
		}
		return s
	case bson.DateTime:
		return t.Time()
	case map[string]interface{}:
		m := make(map[string]interface{}, len(t))
		for k, val := range t {
			m[k] = NormalizeValue(val)
		}
		return m
	case []interface{}:
		s := make([]interface{}, len(t))
		for i, val := range t {
			s[i] = NormalizeValue(val)
		}
		return s
	default:
		return v
	}
}

// NormalizeMap applies NormalizeValue to a decoded map, preserving nil.
// The type assertion is safe by invariant: NormalizeValue always returns
// map[string]interface{} for a map[string]interface{} input — keep that case
// arm intact when extending the switch.
func NormalizeMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	return NormalizeValue(m).(map[string]interface{})
}

// NormalizeSlice applies NormalizeValue to each element, preserving nil.
// Safe by the same invariant as NormalizeMap: a []interface{} input always
// yields a []interface{} result.
func NormalizeSlice(s []interface{}) []interface{} {
	if s == nil {
		return nil
	}
	return NormalizeValue(s).([]interface{})
}

// CopyMap shallow-copies an entity map for writing, so the stored document never
// aliases the caller's map. Values are written as-is (plain Go values encode fine).
func CopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
