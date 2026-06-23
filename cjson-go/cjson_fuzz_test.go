package cjsongo

import "testing"

// FuzzUnmarshal ensures the unmarshal/parallel/util entry points survive
// arbitrary input without panicking (including the concurrent array path).
func FuzzUnmarshal(f *testing.F) {
	f.Add([]byte(`[{"a":1},{"b":2},{"c":[1,2,3]}]`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`[1,2,3]`)) // non-object elements -> error path
	f.Add([]byte(`{"k":"v"}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = UnmarshalArrayParallel(data)
		_, _ = UnmarshalToMap(data)
		_, _ = UnmarshalToSlice(data)
		_ = Valid(data)
		_, _ = Compact(data)
	})
}
