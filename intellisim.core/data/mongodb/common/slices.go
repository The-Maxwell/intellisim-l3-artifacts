package common

// MapSlice converts a slice via f, preserving nil (so bson `omitempty` semantics
// survive the mapping round-trip).
func MapSlice[T, U any](in []T, f func(T) U) []U {
	if in == nil {
		return nil
	}
	out := make([]U, len(in))
	for i := range in {
		out[i] = f(in[i])
	}
	return out
}

// MapSliceErr is MapSlice for fallible converters.
func MapSliceErr[T, U any](in []T, f func(T) (U, error)) ([]U, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]U, len(in))
	for i := range in {
		u, err := f(in[i])
		if err != nil {
			return nil, err
		}
		out[i] = u
	}
	return out, nil
}
