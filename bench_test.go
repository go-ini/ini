package ini

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestFile(block bool) *File {
	var opts []LoadOption
	if !block {
		opts = append(opts, Locking(false))
	}
	c, _ := Load([]byte(confData), opts...)
	return c
}

func Benchmark_Key_Value(b *testing.B) {
	c := newTestFile(true)
	for i := 0; i < b.N; i++ {
		c.Section("").Key("NAME").Value()
	}
}

func Benchmark_Key_Value_NonBlock(b *testing.B) {
	c := newTestFile(false)
	for i := 0; i < b.N; i++ {
		c.Section("").Key("NAME").Value()
	}
}

func Benchmark_Key_Value_ViaSection(b *testing.B) {
	c := newTestFile(true)
	sec := c.Section("")
	for i := 0; i < b.N; i++ {
		sec.Key("NAME").Value()
	}
}

func Benchmark_Key_Value_ViaSection_NonBlock(b *testing.B) {
	c := newTestFile(false)
	sec := c.Section("")
	for i := 0; i < b.N; i++ {
		sec.Key("NAME").Value()
	}
}

func Benchmark_Key_Value_Direct(b *testing.B) {
	c := newTestFile(true)
	key := c.Section("").Key("NAME")
	for i := 0; i < b.N; i++ {
		key.Value()
	}
}

func Benchmark_Key_Value_Direct_NonBlock(b *testing.B) {
	c := newTestFile(false)
	key := c.Section("").Key("NAME")
	for i := 0; i < b.N; i++ {
		key.Value()
	}
}

func Benchmark_Key_String(b *testing.B) {
	c := newTestFile(true)
	for i := 0; i < b.N; i++ {
		_ = c.Section("").Key("NAME").String()
	}
}

func Benchmark_Key_String_NonBlock(b *testing.B) {
	c := newTestFile(false)
	for i := 0; i < b.N; i++ {
		_ = c.Section("").Key("NAME").String()
	}
}

func Benchmark_Key_String_ViaSection(b *testing.B) {
	c := newTestFile(true)
	sec := c.Section("")
	for i := 0; i < b.N; i++ {
		_ = sec.Key("NAME").String()
	}
}

func Benchmark_Key_String_ViaSection_NonBlock(b *testing.B) {
	c := newTestFile(false)
	sec := c.Section("")
	for i := 0; i < b.N; i++ {
		_ = sec.Key("NAME").String()
	}
}

func Benchmark_Key_SetValue(b *testing.B) {
	c := newTestFile(true)
	for i := 0; i < b.N; i++ {
		c.Section("").Key("NAME").SetValue("10")
	}
}

func Benchmark_Key_SetValue_VisSection(b *testing.B) {
	c := newTestFile(true)
	sec := c.Section("")
	for i := 0; i < b.N; i++ {
		sec.Key("NAME").SetValue("10")
	}
}

func Benchmark_Key_Strings_LargeArray(b *testing.B) {
	c, err := Load("testdata/large_array.ini")
	require.NoError(b, err)
	key := c.Section("").Key("ARRAY")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = key.Strings(",")
	}
}

// benchConfig is a typical large config: sections with comments, plain,
// quoted and commented values.
var benchConfig = func() []byte {
	var b strings.Builder
	b.WriteString("; global settings\nAPP_NAME = bench\nRUN_MODE = prod\n\n")
	for i := range 500 {
		fmt.Fprintf(&b, "# Section %d\n[section_%d]\n", i, i)
		for j := range 20 {
			switch j % 4 {
			case 0:
				fmt.Fprintf(&b, "key_%d = value_%d_%d\n", j, i, j)
			case 1:
				fmt.Fprintf(&b, "; comment for key %d\nkey_%d = %d ; inline comment\n", j, j, i*j)
			case 2:
				fmt.Fprintf(&b, "key_%d = \"quoted value %d\"\n", j, j)
			case 3:
				fmt.Fprintf(&b, "  key_%d   =   https://example.com/path/%d?x=%d  \n", j, i, j)
			}
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}()

func benchmarkLoad(b *testing.B, data []byte, opts ...LoadOption) {
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Load(data, opts...); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Load(b *testing.B) {
	benchmarkLoad(b, benchConfig)
}

func Benchmark_Load_Insensitive(b *testing.B) {
	benchmarkLoad(b, benchConfig, CaseInsensitive(true))
}

func Benchmark_Load_Full(b *testing.B) {
	data, err := os.ReadFile("testdata/full.ini")
	require.NoError(b, err)
	benchmarkLoad(b, data, AllowBooleanKeys(true))
}

func Benchmark_Load_Multiline(b *testing.B) {
	data, err := os.ReadFile("testdata/multiline.ini")
	require.NoError(b, err)
	benchmarkLoad(b, data, AllowPythonMultilineValues(true))
}
