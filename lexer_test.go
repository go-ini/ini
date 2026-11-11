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

		t.Run("two BOMs", func(t *testing.T) {
			f, err := Load([]byte("\ufeff\ufeff# comment\n[s]\nk = v\n"))
			require.NoError(t, err)
			assert.Equal(t, "v", f.Section("s").Key("k").String())
			assert.Equal(t, "# comment", f.Section("s").Comment())
		})
	})
}

func TestInlineComments(t *testing.T) {
	for _, tc := range []struct {
		in             string
		opts           []LoadOption
		value, comment string
	}{
		// Comment symbols inside quotes stay in the value.
		{in: "k = '#e18477'", value: "#e18477"},
		{in: `k = "XXX#123"`, value: "XXX#123"},
		{in: `k = "a;b" ; note`, value: "a;b", comment: "; note"},
		{in: `k = "a#b"`, opts: []LoadOption{PreserveSurroundingQuotes(true)}, value: `"a#b"`},
		// Text after the closing quote cuts at the first symbol, as before.
		{in: `k = "a;b" x`, value: `"a`, comment: `;b" x`},
		{in: "k = don't # note", value: "don't", comment: "# note"},
		// With SpaceBeforeInlineComment the first " #" or " ;" cuts.
		{in: "k = foo ; note # ticket", opts: []LoadOption{SpaceBeforeInlineComment(true)}, value: "foo", comment: "; note # ticket"},
		{in: "k = a#b ;c", opts: []LoadOption{SpaceBeforeInlineComment(true)}, value: "a#b", comment: ";c"},
	} {
		f, err := Load([]byte(tc.in+"\n"), tc.opts...)
		require.NoError(t, err, tc.in)
		k := f.Section("").Key("k")
		assert.Equal(t, tc.value, k.String(), tc.in)
		assert.Equal(t, tc.comment, strings.TrimSpace(k.Comment()), tc.in)
	}
}

func TestEmptyRawSection(t *testing.T) {
	for _, in := range []string{"[custom]\n", "[a]\nk = v\n[custom]\n", "[custom]\n[b]\nk = v\n"} {
		f, err := Load([]byte(in), RawSections("custom"))
		require.NoError(t, err)
		sec := f.Section("custom")
		sec.SetBody("x = 1")
		assert.Equal(t, "x = 1", sec.Body(), "an empty raw section is raw: %q", in)
	}
}

func TestBracketedValues(t *testing.T) {
	const in = "[server]\ntimeout = {\n    \"timeout\": 1000,\n    \"url\": \"http://x/#a}\"\n} ; note\nnext = 1\nopen = {\n"
	_, err := Load([]byte(in))
	require.Error(t, err)

	f, err := Load([]byte(in), AllowBracketedValues(true))
	require.NoError(t, err)
	sec := f.Section("server")
	assert.Equal(t, "{\n    \"timeout\": 1000,\n    \"url\": \"http://x/#a}\"\n}", sec.Key("timeout").String())
	assert.Equal(t, "; note", sec.Key("timeout").Comment())
	assert.Equal(t, "1", sec.Key("next").String())
	assert.Equal(t, "{", sec.Key("open").String(), "a bracket that never closes is read as usual")
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
	require.NoError(t, err)
	assert.Equal(t, 200, strings.Count(f.Section("s").Key("key").Value(), "\n"), "continuation lines")
	assert.Equal(t, "ok", f.Section("s").Key("next").Value())
}

// Empty lines inside a Python-style multiline value belong to it when an
// indented line follows, as in configparser.
func TestParsePythonMultilineEmptyLines(t *testing.T) {
	f, err := Load([]byte("notes = first\n\n  second\n\n\n  third\n\nnext = 1\n"), AllowPythonMultilineValues(true))
	require.NoError(t, err)
	assert.Equal(t, "first\n\n  second\n\n\n  third", f.Section("").Key("notes").String())
	assert.Equal(t, "1", f.Section("").Key("next").String())
}
