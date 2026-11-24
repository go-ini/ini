package ini

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReflectFrom_EmptySliceOverwritesExistingValues(t *testing.T) {
	type config struct {
		Plain       []string `ini:"plain"`
		Shadow      []string `ini:"shadow,allowshadow"`
		Keep        []string `ini:"keep,omitempty"`
		KeepShadows []string `ini:"keep_shadows,omitempty,allowshadow"`
	}
	for _, preserveEmpty := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			in   config
		}{
			{name: "nil"},
			{name: "empty", in: config{Plain: []string{}, Shadow: []string{}, Keep: []string{}, KeepShadows: []string{}}},
		} {
			t.Run(tc.name+map[bool]string{false: "/drop-empty", true: "/preserve-empty"}[preserveEmpty], func(t *testing.T) {
				opts := []LoadOption{AllowShadows(true), PreserveEmptyShadowValues(preserveEmpty)}
				f, err := Load([]byte("plain = a,b\nshadow = a\nshadow = b\nkeep = x,y\nkeep_shadows = x\nkeep_shadows = y\n"), opts...)
				require.NoError(t, err)
				require.NoError(t, f.ReflectFrom(&tc.in))
				assert.Empty(t, f.Section("").Key("plain").String())
				assert.Empty(t, f.Section("").Key("shadow").String())
				assert.Equal(t, "x,y", f.Section("").Key("keep").String())
				assert.Equal(t, []string{"x", "y"}, f.Section("").Key("keep_shadows").Values())

				var buf bytes.Buffer
				_, err = f.WriteTo(&buf)
				require.NoError(t, err)
				var out config
				require.NoError(t, MapTo(&out, buf.Bytes(), opts...))
				assert.Empty(t, out.Plain)
				assert.Empty(t, out.Shadow)
				assert.Equal(t, []string{"x", "y"}, out.Keep)
				assert.Equal(t, []string{"x", "y"}, out.KeepShadows)
			})
		}
	}
}
