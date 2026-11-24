package ini

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_RejectLineBreakWithoutLF(t *testing.T) {
	for _, lineBreak := range []string{"", "\r", "\r\r", " "} {
		t.Run("line-break="+strconv.Quote(lineBreak), func(t *testing.T) {
			f, err := Load([]byte("a=1\nb=2\n[service]\nhost=localhost\n"))
			require.NoError(t, err)
			const previous = "keep = original\n"
			buf := bytes.NewBufferString(previous)
			n, err := f.Write(buf, LineBreak(lineBreak))
			if err == nil {
				written := buf.Bytes()[len(previous):]
				t.Logf("Write accepted %q", written)
				reloaded, reloadErr := Load(written)
				if reloadErr == nil {
					t.Logf("reloaded default keys: %v", reloaded.Section("").KeyValues())
				}
			}
			assert.Error(t, err)
			assert.Zero(t, n)
			assert.Equal(t, previous, buf.String(), "a rejected write must not touch its writer")

			path := filepath.Join(t.TempDir(), "config.ini")
			require.NoError(t, os.WriteFile(path, []byte(previous), 0600))
			assert.Error(t, f.SaveTo(path, LineBreak(lineBreak)))
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, previous, string(got), "a rejected save must not replace the existing file")
		})
	}
}

func TestWrite_LineBreakWithLF(t *testing.T) {
	for _, lineBreak := range []string{"\n", "\r\n", "\n\n", "\n ", "\n\t"} {
		t.Run("line-break="+strconv.Quote(lineBreak), func(t *testing.T) {
			f, err := Load([]byte("a=1\nb=2\n[service]\nhost=localhost\n"))
			require.NoError(t, err)
			var buf bytes.Buffer
			_, err = f.Write(&buf, LineBreak(lineBreak))
			require.NoError(t, err)
			reloaded, err := Load(buf.Bytes())
			require.NoError(t, err)
			assert.Equal(t, f.SectionNames(), reloaded.SectionNames())
			for _, name := range f.SectionNames() {
				assert.Equal(t, f.Section(name).KeyValues(), reloaded.Section(name).KeyValues())
			}
		})
	}
}
