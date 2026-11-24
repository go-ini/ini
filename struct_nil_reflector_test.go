package ini

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type optionalReflector struct {
	Value string
}

func (r *optionalReflector) ReflectINIStruct(f *File) error {
	f.Section("custom").Key("value").SetValue(r.Value)
	return nil
}

func TestReflectFrom_NilStructReflector(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reflector *optionalReflector
		want      string
	}{
		{name: "nil", want: "After = kept\n"},
		{name: "non-nil", reflector: &optionalReflector{Value: "present"},
			want: "After = kept\n\n[custom]\nvalue = present\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := Empty()
			in := struct {
				Optional *optionalReflector
				After    string
			}{Optional: tc.reflector, After: "kept"}
			require.NotPanics(t, func() {
				require.NoError(t, f.ReflectFrom(&in))
			})
			var buf bytes.Buffer
			_, err := f.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
