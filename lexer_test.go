package ini

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBOM(t *testing.T) {
	t.Run("test handling BOM", func(t *testing.T) {
		t.Run("UTF-8-BOM", func(t *testing.T) {
			f, err := Load("testdata/UTF-8-BOM.ini")
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "example@email.com", f.Section("author").Key("E-MAIL").String())
		})

		t.Run("UTF-16-LE-BOM", func(t *testing.T) {
			f, err := Load("testdata/UTF-16-LE-BOM.ini")
			require.NoError(t, err)
			require.NotNil(t, f)
		})

		t.Run("UTF-16-BE-BOM", func(t *testing.T) {
		})
	})
}

func TestBadLoad(t *testing.T) {
	t.Run("load with bad data", func(t *testing.T) {
		t.Run("bad section name", func(t *testing.T) {
			_, err := Load([]byte("[]"))
			require.Error(t, err)

			_, err = Load([]byte("["))
			require.Error(t, err)
		})

		t.Run("bad keys", func(t *testing.T) {
			_, err := Load([]byte(`"""name`))
			require.Error(t, err)

			_, err = Load([]byte(`"""name"""`))
			require.Error(t, err)

			_, err = Load([]byte(`""=1`))
			require.Error(t, err)

			_, err = Load([]byte(`=`))
			require.Error(t, err)

			_, err = Load([]byte(`name`))
			require.Error(t, err)
		})

		t.Run("bad values", func(t *testing.T) {
			_, err := Load([]byte(`name="""Unknwon`))
			require.Error(t, err)
		})
	})
}

// A Python-style multiline value is read to its end however long it is. v1
// read only as far as one 4 KiB reader buffer looked ahead.
func TestParsePythonMultilineLongerThanBuffer(t *testing.T) {
	var b strings.Builder
	b.WriteString("[s]\nkey = first\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "  line %03d xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", i)
	}
	b.WriteString("next = ok\n")

	f, err := Load([]byte(b.String()), AllowPythonMultilineValues(true))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(f.Section("s").Key("key").Value(), "\n"); got != 200 {
		t.Fatalf("value has %d continuation lines, want 200", got)
	}
	if got := f.Section("s").Key("next").Value(); got != "ok" {
		t.Fatalf("next = %q, want ok", got)
	}
}
