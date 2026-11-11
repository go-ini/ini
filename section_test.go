package ini

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSection_SetBody(t *testing.T) {
	t.Run("set body of raw section", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		sec, err := f.NewRawSection("comments", `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`)
		require.NoError(t, err)
		require.NotNil(t, sec)
		assert.Equal(t, `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`, sec.Body())

		sec.SetBody("1111111111111111111000000000000000001110000")
		assert.Equal(t, `1111111111111111111000000000000000001110000`, sec.Body())

		t.Run("set for non-raw section", func(t *testing.T) {
			sec, err := f.NewSection("author")
			require.NoError(t, err)
			require.NotNil(t, sec)
			assert.Empty(t, sec.Body())

			sec.SetBody("1111111111111111111000000000000000001110000")
			assert.Empty(t, sec.Body())
		})
	})
}

func TestSection_NewKey(t *testing.T) {
	t.Run("create a new key", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)
		assert.Equal(t, "NAME", k.Name())
		assert.Equal(t, "ini", k.Value())

		t.Run("with duplicated name", func(t *testing.T) {
			k, err := f.Section("").NewKey("NAME", "ini.v1")
			require.NoError(t, err)
			require.NotNil(t, k)

			// Overwrite previous existed key
			assert.Equal(t, "ini.v1", k.Value())
		})

		t.Run("with empty string", func(t *testing.T) {
			_, err := f.Section("").NewKey("", "")
			require.Error(t, err)
		})
	})

	t.Run("create keys with same name and allow shadow", func(t *testing.T) {
		f, err := Load([]byte(""), AllowShadows(true))
		require.NoError(t, err)
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("").NewKey("NAME", "ini.v1")
		require.NoError(t, err)
		require.NotNil(t, k)

		assert.Equal(t, []string{"ini", "ini.v1"}, k.Values())
	})
}

func TestSection_NewBooleanKey(t *testing.T) {
	t.Run("create a new boolean key", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewBooleanKey("start-ssh-server")
		require.NoError(t, err)
		require.NotNil(t, k)
		assert.Equal(t, "start-ssh-server", k.Name())
		assert.Equal(t, "true", k.Value())

		t.Run("with empty string", func(t *testing.T) {
			_, err := f.Section("").NewBooleanKey("")
			require.Error(t, err)
		})
	})
}

func TestSection_GetKey(t *testing.T) {
	t.Run("get a key", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)

		k, err = f.Section("").GetKey("NAME")
		require.NoError(t, err)
		require.NotNil(t, k)
		assert.Equal(t, "NAME", k.Name())
		assert.Equal(t, "ini", k.Value())

		t.Run("key not exists", func(t *testing.T) {
			_, err := f.Section("").GetKey("404")
			require.Error(t, err)
		})

		t.Run("key exists in parent section", func(t *testing.T) {
			k, err := f.Section("parent").NewKey("AGE", "18")
			require.NoError(t, err)
			require.NotNil(t, k)

			k, err = f.Section("parent.child.son").GetKey("AGE")
			require.NoError(t, err)
			require.NotNil(t, k)
			assert.Equal(t, "18", k.Value())
		})
	})
}

func TestSection_HasKey(t *testing.T) {
	t.Run("check if a key exists", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)

		assert.True(t, f.Section("").HasKey("NAME"))
		assert.True(t, f.Section("").HasKey("NAME"))
		assert.False(t, f.Section("").HasKey("404"))
		assert.False(t, f.Section("").HasKey("404"))
	})
}

func TestSection_HasValue(t *testing.T) {
	t.Run("check if contains a value in any key", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)

		assert.True(t, f.Section("").HasValue("ini"))
		assert.False(t, f.Section("").HasValue("404"))
	})
}

func TestSection_Key(t *testing.T) {
	t.Run("get a key", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)

		k = f.Section("").Key("NAME")
		require.NotNil(t, k)
		assert.Equal(t, "NAME", k.Name())
		assert.Equal(t, "ini", k.Value())

		t.Run("key not exists", func(t *testing.T) {
			k := f.Section("").Key("404")
			require.NotNil(t, k)
			assert.Equal(t, "404", k.Name())
		})

		t.Run("key exists in parent section", func(t *testing.T) {
			k, err := f.Section("parent").NewKey("AGE", "18")
			require.NoError(t, err)
			require.NotNil(t, k)

			k = f.Section("parent.child.son").Key("AGE")
			require.NotNil(t, k)
			assert.Equal(t, "18", k.Value())
		})
	})

	t.Run("reading a missing key doesn't add it", func(t *testing.T) {
		const data = "[app]\nname = demo\n\n[raw]\nbody\n"
		f, err := Load([]byte(data), RawSections("raw"))
		require.NoError(t, err)
		app := f.Section("app")

		k := app.Key("port")
		assert.Equal(t, "port", k.Name())
		assert.Equal(t, 8080, k.MustInt(8080))
		assert.Empty(t, k.String())
		assert.Empty(t, k.Values())
		assert.Empty(t, k.Comment())
		assert.Empty(t, k.NestedValues())
		assert.False(t, k.IsBoolean())
		_ = f.Section("raw").Key("x").String()

		assert.False(t, app.HasKey("port"))
		assert.Equal(t, []string{"name"}, app.KeyNames())
		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		require.NoError(t, err, "a raw section read for a key stays writable")
		assert.Equal(t, data, buf.String())
	})

	t.Run("changing a missing key adds it", func(t *testing.T) {
		f, err := Load([]byte("[s]\n"), AllowShadows(true), AllowNestedValues(true))
		require.NoError(t, err)
		sec := f.Section("s")

		sec.Key("a").SetValue("1")
		sec.Key("b").SetComment("note")
		require.NoError(t, sec.Key("c").AddShadow("x"))
		require.NoError(t, sec.Key("d").AddNestedValue("n = 1"))
		assert.Equal(t, []string{"a", "b", "c", "d"}, sec.KeyNames())
		assert.Equal(t, "1", sec.Key("a").String())

		k := sec.Key("e")
		k.SetValue("2")
		k.SetValue("3")
		assert.Same(t, k, sec.Key("e"), "the key added is the one returned")
		assert.Equal(t, []string{"a", "b", "c", "d", "e"}, sec.KeyNames())
	})

	t.Run("a missing key follows the key its section gains", func(t *testing.T) {
		f, err := Load([]byte("[s]\n"), CaseInsensitiveKeys(true))
		require.NoError(t, err)
		sec := f.Section("s")

		k1, k2 := sec.Key("Port"), sec.Key("port")
		assert.Equal(t, "port", k1.Name())
		k1.SetValue("1")
		assert.Equal(t, "1", k2.String())
		k2.SetValue("2")
		assert.Equal(t, "2", k1.String())
		assert.Equal(t, []string{"port"}, sec.KeyNames())

		k3 := sec.Key("host")
		_, err = sec.NewKey("host", "example.com")
		require.NoError(t, err)
		assert.Equal(t, "example.com", k3.String())
		k3.SetValue("localhost")
		assert.Equal(t, "localhost", sec.Key("host").String())
		assert.Equal(t, []string{"port", "host"}, sec.KeyNames())
	})

	t.Run("a missing key expands as the key it follows", func(t *testing.T) {
		f, err := Load([]byte("path = /usr\n[s]\n"))
		require.NoError(t, err)
		sec := f.Section("s")
		k := sec.Key("path")
		_, err = sec.NewKey("path", "%(path)s/bin")
		require.NoError(t, err)
		assert.Equal(t, "/usr/bin", k.String())
		assert.Equal(t, []string{"/usr/bin"}, k.StringsWithShadows(","))
	})

	t.Run("an empty name is never added", func(t *testing.T) {
		f := Empty()
		f.Section("").Key("").SetValue("v")
		assert.Empty(t, f.Section("").KeyNames())
	})
}

func TestSection_Keys(t *testing.T) {
	t.Run("get all keys in a section", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("").NewKey("VERSION", "v1")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		require.NoError(t, err)
		require.NotNil(t, k)

		keys := f.Section("").Keys()
		names := []string{"NAME", "VERSION", "IMPORT_PATH"}
		assert.Equal(t, len(names), len(keys))
		for i, name := range names {
			assert.Equal(t, name, keys[i].Name())
		}
	})
}

func TestSection_ParentKeys(t *testing.T) {
	t.Run("get all keys of parent sections", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("package").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("package").NewKey("VERSION", "v1")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("package").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		require.NoError(t, err)
		require.NotNil(t, k)

		keys := f.Section("package.sub.sub2").ParentKeys()
		names := []string{"NAME", "VERSION", "IMPORT_PATH"}
		assert.Equal(t, len(names), len(keys))
		for i, name := range names {
			assert.Equal(t, name, keys[i].Name())
		}
	})
}

func TestSection_KeyStrings(t *testing.T) {
	t.Run("get all key names in a section", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("").NewKey("VERSION", "v1")
		require.NoError(t, err)
		require.NotNil(t, k)
		k, err = f.Section("").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		require.NoError(t, err)
		require.NotNil(t, k)

		assert.Equal(t, []string{"NAME", "VERSION", "IMPORT_PATH"}, f.Section("").KeyNames())
	})
}

func TestSection_KeyHash(t *testing.T) {
	t.Run("get clone of key hash", func(t *testing.T) {
		f, err := LoadSources([]any{[]byte(`
key = one
[log]
name = app
file = a.log
`), []byte(`
key = two
[log]
name = app2
file = b.log
`)})
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "two", f.Section("").Key("key").String())

		hash := f.Section("log").KeyValues()
		relation := map[string]string{
			"name": "app2",
			"file": "b.log",
		}
		for k, v := range hash {
			assert.Equal(t, relation[k], v)
		}
	})

	t.Run("reports values as loaded or set", func(t *testing.T) {
		f, err := Load([]byte("empty =\nset = 1\ngone = 2\n"))
		require.NoError(t, err)
		sec := f.Section("")

		assert.Equal(t, "fallback", sec.Key("empty").MustString("fallback"))
		sec.Key("set").SetValue("3")
		sec.DeleteKey("gone")

		assert.Equal(t, map[string]string{"empty": "", "set": "3"}, sec.KeyValues())
	})
}

func TestSection_DeleteKey(t *testing.T) {
	t.Run("delete a key", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		k, err := f.Section("").NewKey("NAME", "ini")
		require.NoError(t, err)
		require.NotNil(t, k)

		assert.True(t, f.Section("").HasKey("NAME"))
		f.Section("").DeleteKey("NAME")
		assert.False(t, f.Section("").HasKey("NAME"))
	})
}
