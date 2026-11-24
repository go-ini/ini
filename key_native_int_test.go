package ini

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKey_NativeIntegerOverflow(t *testing.T) {
	aboveInt := "9223372036854775808"
	belowInt := "-9223372036854775809"
	aboveUint := "18446744073709551616"
	if strconv.IntSize == 32 {
		aboveInt = "2147483648"
		belowInt = "-2147483649"
		aboveUint = "4294967296"
	}
	key := Empty().Section("").Key("size")
	for _, value := range []string{aboveInt, belowInt} {
		t.Run(value, func(t *testing.T) {
			key.SetValue(value)
			_, err := key.Int()
			assert.ErrorIs(t, err, strconv.ErrRange)
			assert.Equal(t, 7, key.MustInt(7))
			if strconv.IntSize == 32 {
				_, err := key.Int64()
				assert.NoError(t, err, "the value still fits an explicit int64")
			}
		})
	}
	key.SetValue(aboveUint)
	_, err := key.Uint()
	assert.ErrorIs(t, err, strconv.ErrRange)
	assert.Equal(t, uint(7), key.MustUint(7))
	if strconv.IntSize == 32 {
		_, err := key.Uint64()
		assert.NoError(t, err, "the value still fits an explicit uint64")
	}
}

func TestKey_NativeIntegerLimits(t *testing.T) {
	key := Empty().Section("").Key("size")
	maxInt := int(^uint(0) >> 1)
	for _, want := range []int{0, maxInt, -maxInt - 1} {
		key.SetValue(strconv.FormatInt(int64(want), 10))
		got, err := key.Int()
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	maxUint := ^uint(0)
	key.SetValue(strconv.FormatUint(uint64(maxUint), 10))
	got, err := key.Uint()
	require.NoError(t, err)
	assert.Equal(t, maxUint, got)
}
