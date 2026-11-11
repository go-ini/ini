package ini

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmpty(t *testing.T) {
	f := Empty()
	require.NotNil(t, f)

	// Should only have the default section
	assert.Len(t, f.Sections(), 1)

	// Default section should not contain any key
	assert.Len(t, f.Section("").Keys(), 0)
}

func TestFile_NewSection(t *testing.T) {
	f := Empty()
	require.NotNil(t, f)

	sec, err := f.NewSection("author")
	require.NoError(t, err)
	require.NotNil(t, sec)
	assert.Equal(t, "author", sec.Name())

	assert.Equal(t, []string{DefaultSection, "author"}, f.SectionNames())

	t.Run("with duplicated name", func(t *testing.T) {
		sec, err := f.NewSection("author")
		require.NoError(t, err)
		require.NotNil(t, sec)

		// Does nothing if section already exists
		assert.Equal(t, []string{DefaultSection, "author"}, f.SectionNames())
	})

	t.Run("with empty string", func(t *testing.T) {
		_, err := f.NewSection("")
		require.Error(t, err)
	})
}

func TestFile_NonUniqueSection(t *testing.T) {
	t.Run("read and write non-unique sections", func(t *testing.T) {
		f, err := Load([]byte(`[Interface]
Address = 192.168.2.1
PrivateKey = <server's privatekey>
ListenPort = 51820

[Peer]
PublicKey = <client's publickey>
AllowedIPs = 192.168.2.2/32

[Peer]
PublicKey = <client2's publickey>
AllowedIPs = 192.168.2.3/32`), AllowNonUniqueSections(true))
		require.NoError(t, err)
		require.NotNil(t, f)

		sec, err := f.NewSection("Peer")
		require.NoError(t, err)
		require.NotNil(t, f)

		_, _ = sec.NewKey("PublicKey", "<client3's publickey>")
		_, _ = sec.NewKey("AllowedIPs", "192.168.2.4/32")

		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		require.NoError(t, err)
		str := buf.String()
		assert.Equal(t, `[Interface]
Address    = 192.168.2.1
PrivateKey = <server's privatekey>
ListenPort = 51820

[Peer]
PublicKey  = <client's publickey>
AllowedIPs = 192.168.2.2/32

[Peer]
PublicKey  = <client2's publickey>
AllowedIPs = 192.168.2.3/32

[Peer]
PublicKey  = <client3's publickey>
AllowedIPs = 192.168.2.4/32
`, str)
	})

	t.Run("delete non-unique section", func(t *testing.T) {
		f, err := Load([]byte(`[Interface]
Address    = 192.168.2.1
PrivateKey = <server's privatekey>
ListenPort = 51820

[Peer]
PublicKey  = <client's publickey>
AllowedIPs = 192.168.2.2/32

[Peer]
PublicKey  = <client2's publickey>
AllowedIPs = 192.168.2.3/32

[Peer]
PublicKey  = <client3's publickey>
AllowedIPs = 192.168.2.4/32

`), AllowNonUniqueSections(true))
		require.NoError(t, err)
		require.NotNil(t, f)

		err = f.DeleteSectionWithIndex("Peer", 1)
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		require.NoError(t, err)
		str := buf.String()
		assert.Equal(t, `[Interface]
Address    = 192.168.2.1
PrivateKey = <server's privatekey>
ListenPort = 51820

[Peer]
PublicKey  = <client's publickey>
AllowedIPs = 192.168.2.2/32

[Peer]
PublicKey  = <client3's publickey>
AllowedIPs = 192.168.2.4/32
`, str)
	})

	t.Run("delete all sections", func(t *testing.T) {
		f := Empty(AllowNonUniqueSections(true))
		require.NotNil(t, f)

		_ = f.NewSections("Interface", "Peer", "Peer")
		assert.Equal(t, []string{DefaultSection, "Interface", "Peer", "Peer"}, f.SectionNames())
		f.DeleteSection("Peer")
		assert.Equal(t, []string{DefaultSection, "Interface"}, f.SectionNames())
	})
}

func TestFile_NewRawSection(t *testing.T) {
	f := Empty()
	require.NotNil(t, f)

	sec, err := f.NewRawSection("comments", `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`)
	require.NoError(t, err)
	require.NotNil(t, sec)
	assert.Equal(t, "comments", sec.Name())

	assert.Equal(t, []string{DefaultSection, "comments"}, f.SectionNames())
	assert.Equal(t, `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`, f.Section("comments").Body())

	t.Run("with duplicated name", func(t *testing.T) {
		sec, err := f.NewRawSection("comments", `1111111111111111111000000000000000001110000`)
		require.NoError(t, err)
		require.NotNil(t, sec)
		assert.Equal(t, []string{DefaultSection, "comments"}, f.SectionNames())

		// Overwrite previous existed section
		assert.Equal(t, `1111111111111111111000000000000000001110000`, f.Section("comments").Body())
	})

	t.Run("with empty string", func(t *testing.T) {
		_, err := f.NewRawSection("", "")
		require.Error(t, err)
	})
}

func TestFile_NewSections(t *testing.T) {
	f := Empty()
	require.NotNil(t, f)

	assert.NoError(t, f.NewSections("package", "author"))
	assert.Equal(t, []string{DefaultSection, "package", "author"}, f.SectionNames())

	t.Run("with duplicated name", func(t *testing.T) {
		assert.NoError(t, f.NewSections("author", "features"))

		// Ignore section already exists
		assert.Equal(t, []string{DefaultSection, "package", "author", "features"}, f.SectionNames())
	})

	t.Run("with empty string", func(t *testing.T) {
		assert.Error(t, f.NewSections("", ""))
	})
}

func TestFile_GetSection(t *testing.T) {
	f, err := Load(fullConf)
	require.NoError(t, err)
	require.NotNil(t, f)

	sec, err := f.GetSection("author")
	require.NoError(t, err)
	require.NotNil(t, sec)
	assert.Equal(t, "author", sec.Name())

	t.Run("section not exists", func(t *testing.T) {
		_, err := f.GetSection("404")
		require.Error(t, err)
	})
}

func TestFile_HasSection(t *testing.T) {
	f, err := Load(fullConf)
	require.NoError(t, err)
	require.NotNil(t, f)

	sec := f.HasSection("author")
	assert.True(t, sec)

	t.Run("section not exists", func(t *testing.T) {
		nonexistent := f.HasSection("404")
		assert.False(t, nonexistent)
	})
}

func TestFile_Section(t *testing.T) {
	t.Run("get a section", func(t *testing.T) {
		f, err := Load(fullConf)
		require.NoError(t, err)
		require.NotNil(t, f)

		sec := f.Section("author")
		require.NotNil(t, sec)
		assert.Equal(t, "author", sec.Name())

		t.Run("section not exists", func(t *testing.T) {
			sec := f.Section("404")
			require.NotNil(t, sec)
			assert.Equal(t, "404", sec.Name())
		})
	})

	t.Run("get default section in lower case with insensitive load", func(t *testing.T) {
		f, err := Load([]byte(`
[default]
NAME = ini
VERSION = v1`), CaseInsensitive(true))
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "ini", f.Section("").Key("name").String())
		assert.Equal(t, "v1", f.Section("").Key("version").String())
	})

	t.Run("get sections after deletion", func(t *testing.T) {
		f, err := Load([]byte(`
[RANDOM]
`))
		require.NoError(t, err)
		require.NotNil(t, f)

		sectionNames := f.SectionNames()
		sort.Strings(sectionNames)
		assert.Equal(t, []string{DefaultSection, "RANDOM"}, sectionNames)

		for _, currentSection := range sectionNames {
			f.DeleteSection(currentSection)
		}

		for sectionParam, expectedSectionName := range map[string]string{
			"":       DefaultSection,
			"RANDOM": "RANDOM",
		} {
			sec := f.Section(sectionParam)
			require.NotNil(t, sec)
			assert.Equal(t, expectedSectionName, sec.Name())
		}
	})

}

func TestFile_Sections(t *testing.T) {
	f, err := Load(fullConf)
	require.NoError(t, err)
	require.NotNil(t, f)

	secs := f.Sections()
	names := []string{DefaultSection, "author", "package", "package.sub", "features", "types", "array", "note", "comments", "string escapes", "advance"}
	assert.Len(t, secs, len(names))
	for i, name := range names {
		assert.Equal(t, name, secs[i].Name())
	}
}

func TestFile_ChildSections(t *testing.T) {
	f, err := Load([]byte(`
[node]
[node.biz1]
[node.biz2]
[node.biz3]
[node.bizN]
`))
	require.NoError(t, err)
	require.NotNil(t, f)

	children := f.ChildSections("node")
	names := []string{"node.biz1", "node.biz2", "node.biz3", "node.bizN"}
	assert.Len(t, children, len(names))
	for i, name := range names {
		assert.Equal(t, name, children[i].Name())
	}
}

func TestFile_SectionStrings(t *testing.T) {
	f, err := Load(fullConf)
	require.NoError(t, err)
	require.NotNil(t, f)

	assert.Equal(t, []string{DefaultSection, "author", "package", "package.sub", "features", "types", "array", "note", "comments", "string escapes", "advance"}, f.SectionNames())
}

func TestFile_DeleteSection(t *testing.T) {
	t.Run("delete a section", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		_ = f.NewSections("author", "package", "features")
		f.DeleteSection("features")
		f.DeleteSection("")
		assert.Equal(t, []string{"author", "package"}, f.SectionNames())
	})

	t.Run("delete default section", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f)

		f.Section("").Key("foo").SetValue("bar")
		f.Section("section1").Key("key1").SetValue("value1")
		f.DeleteSection("")
		assert.Equal(t, []string{"section1"}, f.SectionNames())

		var buf bytes.Buffer
		_, err := f.WriteTo(&buf)
		require.NoError(t, err)

		assert.Equal(t, `[section1]
key1 = value1
`, buf.String())
	})

	t.Run("delete a section with CaseInsensitiveSections", func(t *testing.T) {
		f := Empty(CaseInsensitiveSections(true))
		require.NotNil(t, f)

		_ = f.NewSections("author", "package", "features")
		f.DeleteSection("FEATURES")
		f.DeleteSection("")
		assert.Equal(t, []string{"author", "package"}, f.SectionNames())
	})
}

func TestFile_Append(t *testing.T) {
	f := Empty()
	require.NotNil(t, f)

	assert.NoError(t, f.Append(minimalConf, []byte(`
[author]
NAME = Unknwon`)))

	t.Run("with bad input", func(t *testing.T) {
		assert.Error(t, f.Append(123))
		assert.Error(t, f.Append(minimalConf, 123))
	})
}

func TestFile_WriteTo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping testing on Windows")
	}

	t.Run("write content to somewhere", func(t *testing.T) {
		f, err := Load(fullConf)
		require.NoError(t, err)
		require.NotNil(t, f)

		f.Section("author").SetComment(`Information about package author
# Bio can be written in multiple lines.`)
		f.Section("author").Key("NAME").SetComment("This is author name")
		_, _ = f.Section("note").NewBooleanKey("boolean_key")
		_, _ = f.Section("note").NewKey("more", "notes")

		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		require.NoError(t, err)

		golden := "testdata/TestFile_WriteTo.golden"
		if *update {
			require.NoError(t, os.WriteFile(golden, buf.Bytes(), 0644))
		}

		expected, err := os.ReadFile(golden)
		require.NoError(t, err)
		assert.Equal(t, string(expected), buf.String())
	})

	t.Run("support multiline comments", func(t *testing.T) {
		f, err := Load([]byte(`
# 
# general.domain
# 
# Domain name of XX system.
domain      = mydomain.com
`))
		require.NoError(t, err)

		f.Section("").Key("test").SetComment("Multiline\nComment")

		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		require.NoError(t, err)

		assert.Equal(t, `# 
# general.domain
# 
# Domain name of XX system.
domain = mydomain.com
; Multiline
; Comment
test   = 
`, buf.String())

	})

	t.Run("keep leading and trailing spaces in value", func(t *testing.T) {
		f, _ := Load([]byte(`[foo]
bar1 = '  val ue1 '
bar2 = """  val ue2 """
bar3 = "  val ue3 "
`))
		require.NotNil(t, f)

		var buf bytes.Buffer
		_, err := f.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, `[foo]
bar1 = "  val ue1 "
bar2 = "  val ue2 "
bar3 = "  val ue3 "
`, buf.String())
	})
}

func TestFile_SaveTo(t *testing.T) {
	f, err := Load(fullConf)
	require.NoError(t, err)
	require.NotNil(t, f)

	assert.NoError(t, f.SaveTo("testdata/conf_out.ini"))
	assert.NoError(t, f.SaveTo("testdata/conf_out.ini", Indent("\t")))
}

func TestFile_WriteToWithOutputDelimiter(t *testing.T) {
	f, err := Load([]byte(`[Others]
Cities = HangZhou|Boston
Visits = 1993-10-07T20:17:05Z, 1993-10-07T20:17:05Z
Years = 1993,1994
Numbers = 10010,10086
Ages = 18,19
Populations = 12345678,98765432
Coordinates = 192.168,10.11
Flags       = true,false
Note = Hello world!`))
	require.NoError(t, err)
	require.NotNil(t, f)

	var actual bytes.Buffer
	var expected = []byte(`[Others]
Cities      -> HangZhou|Boston
Visits      -> 1993-10-07T20:17:05Z, 1993-10-07T20:17:05Z
Years       -> 1993,1994
Numbers     -> 10010,10086
Ages        -> 18,19
Populations -> 12345678,98765432
Coordinates -> 192.168,10.11
Flags       -> true,false
Note        -> Hello world!
`)
	_, err = f.Write(&actual, WriteDelimiter("->"))
	require.NoError(t, err)

	assert.Equal(t, expected, actual.Bytes())
}

// Inspired by https://github.com/go-ini/ini/issues/207
func TestReloadAfterShadowLoad(t *testing.T) {
	f, err := Load([]byte(`
[slice]
v = 1
v = 2
v = 3
`), AllowShadows(true))
	require.NoError(t, err)
	require.NotNil(t, f)

	assert.Equal(t, []string{"1", "2", "3"}, f.Section("slice").Key("v").Values())

	require.NoError(t, f.Reload())
	assert.Equal(t, []string{"1", "2", "3"}, f.Section("slice").Key("v").Values())
}

func TestFile_Reload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ini")
	write := func(s string) { require.NoError(t, os.WriteFile(path, []byte(s), 0o600)) }

	// https://github.com/go-ini/ini/issues/418
	t.Run("removed keys and sections are gone", func(t *testing.T) {
		write("[app]\nold = 1\nkeep = 2\n[gone]\nk = v\n")
		f, err := Load(path)
		require.NoError(t, err)
		sec, key := f.Section("app"), f.Section("app").Key("keep")

		write("[app]\nkeep = 3\n")
		require.NoError(t, f.Reload())
		assert.False(t, f.Section("app").HasKey("old"))
		assert.False(t, f.HasSection("gone"))
		assert.Equal(t, map[string]string{"keep": "3"}, f.Section("app").KeyValues())
		// Pointers taken before the reload see the new contents.
		assert.Equal(t, "3", key.String())
		assert.Same(t, sec, f.Section("app"))
	})

	// https://github.com/go-ini/ini/issues/273
	t.Run("shadows don't pile up", func(t *testing.T) {
		write("[log]\nlevel = info\nlevel = warn\n")
		f, err := Load(path, AllowShadows(true), AllowDuplicateShadowValues(true))
		require.NoError(t, err)

		write("[log]\nlevel = debug\n")
		require.NoError(t, f.Reload())
		assert.Equal(t, []string{"debug"}, f.Section("log").Key("level").Values())
	})

	t.Run("error leaves the file unchanged", func(t *testing.T) {
		write("[app]\nk = 1\n")
		f, err := Load(path)
		require.NoError(t, err)

		write("[app]\nk = 2\nbroken\n")
		require.Error(t, f.Reload())
		assert.Equal(t, "1", f.Section("app").Key("k").String())
	})
}

// https://github.com/go-ini/ini/issues/324
func TestFile_AppendReaders(t *testing.T) {
	f, err := Load(strings.NewReader("[a]\nk = 1\n"))
	require.NoError(t, err)
	f.Section("a").Key("edited").SetValue("yes")

	require.NoError(t, f.Append(strings.NewReader("[b]\nk = 2\n")))
	require.NoError(t, f.Append(strings.NewReader("[a]\nk = 3\n")))
	assert.Equal(t, "3", f.Section("a").Key("k").String())
	assert.Equal(t, "2", f.Section("b").Key("k").String())
	assert.Equal(t, "yes", f.Section("a").Key("edited").String(), "Append keeps changes made in memory")

	// Readers are read once and kept for reloading.
	require.NoError(t, f.Reload())
	assert.Equal(t, "3", f.Section("a").Key("k").String())
	assert.False(t, f.Section("a").HasKey("edited"))
}

// Misuse and option combinations that used to panic, hang or lose data.
func TestFile_EdgeCases(t *testing.T) {
	t.Run("self-reference in the default section", func(t *testing.T) {
		f, err := Load([]byte("a = %(a)s\n"))
		require.NoError(t, err)
		assert.Equal(t, "%(a)s", f.Section("").Key("a").String())
	})

	t.Run("Strings with an empty delimiter", func(t *testing.T) {
		f, err := Load([]byte("a = x,y\n"))
		require.NoError(t, err)
		assert.Equal(t, []string{"x,y"}, f.Section("").Key("a").Strings(""))
	})

	t.Run("negative and out-of-range section indexes", func(t *testing.T) {
		f, err := Load([]byte("a = 1\n[x]\nb = 2\n"), AllowNonUniqueSections(true))
		require.NoError(t, err)
		assert.NotNil(t, f.SectionWithIndex("x", -1))
		assert.Error(t, f.DeleteSectionWithIndex("x", -1))
		assert.NotNil(t, f.SectionWithIndex("", 1))
		assert.NotPanics(t, func() { _ = f.Sections() })
	})

	t.Run("default section after deleting it, case-insensitive", func(t *testing.T) {
		f, err := Load([]byte("a = 1\n"), CaseInsensitive(true), AllowNonUniqueSections(true))
		require.NoError(t, err)
		f.DeleteSection("")
		f.Section("").Key("k").SetValue("v")
		assert.Equal(t, "v", f.Section("").Key("k").String())
		assert.Len(t, f.Sections(), 1)
	})

	// https://github.com/go-ini/ini/issues/340
	t.Run("DeleteKey with CaseInsensitive", func(t *testing.T) {
		f, err := Load([]byte("TestKey = 1\n"), CaseInsensitive(true))
		require.NoError(t, err)
		f.Section("").DeleteKey("TestKey")
		assert.False(t, f.Section("").HasKey("TestKey"))
	})

	// https://github.com/go-ini/ini/issues/358
	t.Run("key with an empty name", func(t *testing.T) {
		f := Empty()
		require.NotNil(t, f.Section("").Key(""))
		assert.Equal(t, "", f.Section("").Key("").String())
		assert.Empty(t, f.Section("").Keys())
	})

	t.Run("child sections with non-unique names", func(t *testing.T) {
		f, err := Load([]byte("[a]\n[a.b]\nx = 1\n[a.b]\nx = 2\n"), AllowNonUniqueSections(true))
		require.NoError(t, err)
		children := f.Section("a").ChildSections()
		require.Len(t, children, 2)
		assert.Equal(t, "1", children[0].Key("x").String())
		assert.Equal(t, "2", children[1].Key("x").String())
	})
}

// Unless loaded with Locking(false), a File can be read while other goroutines
// change it. Run with -race.
func TestFile_Concurrency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ini")
	require.NoError(t, os.WriteFile(path, []byte("name = %(base)s/x\nbase = /srv\n[s]\nk = a\nk = b\n[s.child]\nc = 1\n[raw]\nbody\n"), 0o600))
	f, err := Load(path, AllowShadows(true), RawSections("raw"))
	require.NoError(t, err)

	type cfg struct {
		Name string `ini:"name"`
		S    struct {
			K []string `ini:"k,,allowshadow"`
		} `ini:"s"`
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	run := func(fn func(i int)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
					fn(i)
				}
			}
		}()
	}

	// Readers.
	run(func(int) {
		for _, sec := range f.Sections() {
			_ = sec.Body()
			_ = sec.Comment()
			_ = sec.KeyNames()
			_ = sec.KeyValues()
			_ = sec.ChildSections()
			for _, k := range sec.Keys() {
				_ = k.String()
				_ = k.Value()
				_ = k.Comment()
				_ = k.Values()
				_ = k.IsBoolean()
				_ = k.NestedValues()
			}
		}
	})
	run(func(int) {
		_, _ = f.Write(io.Discard)
		_ = f.SectionNames()
		var c cfg
		_ = f.MapTo(&c)
	})

	// Writers.
	run(func(int) { _ = f.Reload() })
	run(func(i int) {
		sec := f.Section("s")
		sec.Key("k").SetValue(strconv.Itoa(i))
		sec.SetComment("; c" + strconv.Itoa(i))
		sec.Key("k").SetComment("; c" + strconv.Itoa(i))
		_ = sec.Key("k").AddShadow("z")
		_ = sec.Key("m").MustInt(3)
		_, _ = f.Section("new").NewKey("n", "v")
		f.Section("new").DeleteKey("n")
		f.DeleteSection("new")
		f.Section("raw").SetBody("other")
		_ = f.ReflectFrom(&cfg{Name: "n"})
	})
	run(func(i int) {
		if i%50 == 0 {
			_ = f.Append([]byte("[more]\nx = 1\n"))
		}
	})

	time.Sleep(time.Second)
	close(stop)
	wg.Wait()
}

// The round-trip tests write a loaded File and load the output again with
// the same options. Whatever was loaded must come back: section and key
// names, values, shadows, nested values, raw bodies, flags and comments.

// describeFile lists everything a File holds that writing must preserve.
// Comments are compared after the normalization the writer applies.
func describeFile(f *File) string {
	var b strings.Builder
	for _, s := range f.Sections() {
		// A raw body gets a final line break when a section follows it,
		// and Body trims it either way.
		fmt.Fprintf(&b, "[%q] comment=%q raw=%v body=%q\n", s.name, normalizeComment(s.comment), s.isRawSection, strings.TrimRight(s.rawBody, "\n"))
		for _, k := range s.Keys() {
			vals := append([]string{k.value}, k.shadows...)
			fmt.Fprintf(&b, "  %q=%q comment=%q auto=%v bool=%v nested=%q\n",
				k.name, vals, normalizeComment(k.comment), k.isAutoIncrement, k.isBooleanType, k.nestedValues)
		}
	}
	return b.String()
}

// normalizeComment trims every line and puts one space after its comment
// symbol, as the writer does.
func normalizeComment(c string) string {
	lines := strings.Split(c, "\n")
	for i, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			lines[i] = line[:1] + " " + strings.TrimSpace(line[1:])
		}
	}
	return strings.Join(lines, "\n")
}

func checkRoundTrip(t *testing.T, data []byte, opts []LoadOption, wopts ...WriteOption) {
	t.Helper()
	f, err := Load(data, opts...)
	if err != nil {
		return
	}
	if !strings.Contains(f.options.KeyValueDelimiters, "=") {
		// Written with "=" for other readers, and not checked.
		return
	}
	want := describeFile(f)

	var out bytes.Buffer
	if _, err = f.Write(&out, wopts...); err != nil {
		// Some files can't be written. That must be reported, not
		// written wrongly.
		return
	}
	g, err := Load(out.Bytes(), opts...)
	require.NoError(t, err, "written file doesn't load\ninput: %q\nwritten: %q", data, out.String())
	require.Equal(t, want, describeFile(g), "written file loads differently\ninput: %q\nwritten: %q", data, out.String())
}

func FuzzWriteRoundTrip(f *testing.F) {
	for _, in := range fuzzInputs(f) {
		for _, flags := range []uint32{0, 1 << 9, 1<<8 | 1<<6, 1<<11 | 1<<10 | 1<<12, 1<<16 | 1<<7 | 1<<5, 1<<20 - 1} {
			f.Add([]byte(in), flags)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, flags uint32) {
		checkRoundTrip(t, data, optionsFromBits(flags))
	})
}

func TestWriteRoundTripRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, in := range parserInputs(t) {
		for i := 0; i < 200; i++ {
			checkRoundTrip(t, []byte(in), optionsFromBits(rng.Uint32()))
		}
	}
}

// Each of these files used to be written so that it loaded differently.
func TestWriteReadsBack(t *testing.T) {
	for _, tc := range []struct {
		data string
		opts []LoadOption
	}{
		{"a = 'x\\'\nb = 1\n", nil},                                          // value ending in a backslash
		{"k = `a\"\"\"\nb`\n", nil},                                          // value containing """
		{"k=`= v`\n", []LoadOption{KeyValueDelimiters(" =")}},                // value starting with a delimiter
		{"[u]\nExecStart=\nExecStart=a\n", []LoadOption{AllowShadows(true)}}, // empty value resets
		{"k = a\n; c\nk = b\n", []LoadOption{AllowShadows(true)}},            // comment of a shadowed key
		{"k =\n  n\nk\n", []LoadOption{AllowNestedValues(true), AllowBooleanKeys(true)}},
		{"k = \"\"\n  n\n", []LoadOption{AllowNestedValues(true), AllowPythonMultilineValues(true)}},
		{"NAME = x\n[DEFAULT]\nk1 = v1\n", []LoadOption{CaseInsensitive(true), AllowNonUniqueSections(true)}},
		{"[DEFAULT] ; c\n[s]\nk = v\n", nil}, // comment of an empty default section
	} {
		f, err := Load([]byte(tc.data), tc.opts...)
		require.NoError(t, err)
		_, err = f.Write(io.Discard)
		require.NoError(t, err, "%q", tc.data)
		checkRoundTrip(t, []byte(tc.data), tc.opts)
	}

	t.Run("raw body without line break", func(t *testing.T) {
		f := Empty(RawSections("raw"))
		_, _ = f.NewRawSection("raw", "body")
		_, _ = f.Section("s").NewKey("k", "v")
		var b bytes.Buffer
		_, err := f.Write(&b)
		require.NoError(t, err)
		assert.Equal(t, "[raw]\nbody\n\n[s]\nk = v\n", b.String())
	})

	t.Run("unwritable key", func(t *testing.T) {
		f := Empty()
		_, _ = f.Section("").NewKey("a\nb", "v")
		_, err := f.Write(io.Discard)
		assert.Error(t, err, "a key name with a line break")
	})

	t.Run("comments with \\n written with \\r\\n", func(t *testing.T) {
		f := Empty()
		sec := f.Section("s")
		sec.SetComment("c1\nc2")
		k, _ := sec.NewKey("a", "1")
		k.SetComment("k1\r\nk2\n[admin]")
		var b bytes.Buffer
		_, err := f.Write(&b, LineBreak("\r\n"))
		require.NoError(t, err)
		assert.Equal(t, "; c1\r\n; c2\r\n[s]\r\n; k1\r\n; k2\r\n; [admin]\r\na = 1\r\n", b.String())

		f2, err := Load(b.Bytes())
		require.NoError(t, err)
		assert.Equal(t, []string{DefaultSection, "s"}, f2.SectionNames())
		assert.Equal(t, "1", f2.Section("s").Key("a").String())
	})

	t.Run("raw body that doesn't read back", func(t *testing.T) {
		for _, body := range []string{"a\n[x]\nb", "a\n; c\n"} {
			f := Empty(RawSections("r"))
			_, _ = f.NewRawSection("r", body)
			_, err := f.Write(io.Discard)
			assert.Error(t, err, "%q", body)
		}

		// Blank lines and indentation are dropped, as when loading.
		f := Empty(RawSections("r"))
		_, _ = f.NewRawSection("r", "a\n\n  b\n")
		_, err := f.Write(io.Discard)
		assert.NoError(t, err)

		// Without RawSections, the body is written for other readers.
		f = Empty()
		_, _ = f.NewRawSection("r", "a\n[x]\n")
		_, err = f.Write(io.Discard)
		assert.NoError(t, err)
	})

	t.Run("raw default section", func(t *testing.T) {
		f := Empty(RawSections(DefaultSection))
		_, _ = f.NewRawSection(DefaultSection, "injected = 1\n")
		var b bytes.Buffer
		_, err := f.Write(&b)
		require.NoError(t, err)
		assert.Equal(t, "[DEFAULT]\ninjected = 1\n", b.String())
		f2, err := Load(b.Bytes(), RawSections(DefaultSection))
		require.NoError(t, err)
		assert.Equal(t, "injected = 1", f2.Section("").Body())
		assert.False(t, f2.Section("").HasKey("injected"))

		// A raw default section that was loaded isn't dropped.
		data := "[DEFAULT]\nsome raw text\n[s]\nk = v\n"
		f, err = Load([]byte(data), RawSections(DefaultSection))
		require.NoError(t, err)
		b.Reset()
		_, err = f.Write(&b)
		require.NoError(t, err)
		assert.Equal(t, "[DEFAULT]\nsome raw text\n\n[s]\nk = v\n", b.String())
	})

	t.Run("raw section with keys", func(t *testing.T) {
		f, err := Load([]byte("a=1\n[DEFAULT]\nbody\n"), RawSections(DefaultSection))
		require.NoError(t, err)
		_, err = f.Write(io.Discard)
		assert.Error(t, err)

		f = Empty(RawSections("r"))
		_, _ = f.NewRawSection("r", "body\n")
		_, _ = f.Section("r").NewKey("k", "v")
		_, err = f.Write(io.Discard)
		assert.Error(t, err)
	})

	t.Run("default section renamed in lower case and read as raw", func(t *testing.T) {
		data := "a = 1\n\n[s]\nk = v\n"
		f, err := Load([]byte(data), RawSections("default"), DefaultSectionName("default"))
		require.NoError(t, err)
		var b bytes.Buffer
		_, err = f.Write(&b)
		require.NoError(t, err)
		assert.Equal(t, data, b.String())
	})

	t.Run("keys of a section read as raw", func(t *testing.T) {
		f := Empty(RawSections("notes"))
		_, _ = f.Section("notes").NewKey("k", "x\n[evil]\ninjected = 1")
		_, err := f.Write(io.Discard)
		assert.Error(t, err)

		// Without keys, the header alone reads back.
		f = Empty(RawSections("notes"))
		_, _ = f.NewSection("notes")
		_, err = f.Write(io.Discard)
		assert.NoError(t, err)
	})

	t.Run("key named -", func(t *testing.T) {
		f := Empty()
		_, _ = f.Section("s").NewKey("-", "x")
		_, err := f.Write(io.Discard)
		assert.Error(t, err)
	})

	t.Run("section named like the deleted default section", func(t *testing.T) {
		f := Empty()
		f.DeleteSection("")
		_, _ = f.Section("Default").NewKey("k", "v")
		var b bytes.Buffer
		_, err := f.Write(&b)
		require.NoError(t, err)
		assert.Equal(t, "[Default]\nk = v\n", b.String())

		f = Empty()
		f.DeleteSection("")
		_, _ = f.NewSection("Default")
		b.Reset()
		_, err = f.Write(&b)
		require.NoError(t, err)
		assert.Equal(t, "[Default]\n", b.String())
	})
}
