package ini

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRange_UnorderedFloats(t *testing.T) {
	type namedFloat float64
	t.Run("float32", testUnorderedFloatRange[float32])
	t.Run("float64", testUnorderedFloatRange[float64])
	t.Run("named float", testUnorderedFloatRange[namedFloat])
}

func testUnorderedFloatRange[T ~float32 | ~float64](t *testing.T) {
	nan := T(math.NaN())
	inf := T(math.Inf(1))
	for _, tc := range []struct {
		name  string
		value string
		lo    T
		hi    T
		want  T
	}{
		{"NaN value", "NaN", 0, 1, 9},
		{"NaN lower bound", "0.5", nan, 1, 9},
		{"NaN upper bound", "0.5", 0, nan, 9},
		{"NaN bounds", "0.5", nan, nan, 9},
		{"lower endpoint", "0", 0, 1, 0},
		{"upper endpoint", "1", 0, 1, 1},
		{"inside", "0.5", 0, 1, 0.5},
		{"outside", "2", 0, 1, 9},
		{"finite bounds exclude infinity", "+Inf", 0, 1, 9},
		{"infinite lower endpoint", "-Inf", -inf, inf, -inf},
		{"infinite upper endpoint", "+Inf", -inf, inf, inf},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := Empty().Section("").Key("rate")
			key.SetValue(tc.value)
			assert.Equal(t, tc.want, Range(key, T(9), tc.lo, tc.hi))
		})
	}
}
