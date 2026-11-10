// Copyright 2017 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package ini

import (
	"bytes"
	"fmt"
	"testing"

	v1 "gopkg.in/ini.v1"
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
	if err != nil {
		b.Fatal(err)
	}
	key := c.Section("").Key("ARRAY")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = key.Strings(",")
	}
}

// genKeys10k returns INI input with 10,000 keys: in one section, in 1,000
// sections of 10 keys, and in one section using every value syntax.
func genKeys10k(layout string) []byte {
	var b bytes.Buffer
	b.WriteString("; generated\n[s]\n")
	for i := 0; i < 10000; i++ {
		switch layout {
		case "flat":
			fmt.Fprintf(&b, "key_%d = some value number %d ; trailing comment\n", i, i)
		case "sections":
			if i%10 == 0 {
				fmt.Fprintf(&b, "\n# section %d\n[section_%d]\n", i/10, i/10)
			}
			fmt.Fprintf(&b, "key_%d = value %d\n", i%10, i)
		case "mixed":
			switch i % 8 {
			case 0:
				fmt.Fprintf(&b, "# comment for key %d\nkey_%d = plain value %d\n", i, i, i)
			case 1:
				fmt.Fprintf(&b, "key_%d = \"double quoted %d\"\n", i, i)
			case 2:
				fmt.Fprintf(&b, "key_%d = 'single quoted %d' # comment\n", i, i)
			case 3:
				fmt.Fprintf(&b, "key_%d = `raw #%d; value`\n", i, i)
			case 4:
				fmt.Fprintf(&b, "key_%d = \"\"\"multi\nline %d\nvalue\"\"\"\n", i, i)
			case 5:
				fmt.Fprintf(&b, "key_%d = first part \\\n\tsecond part %d\n", i, i)
			case 6:
				fmt.Fprintf(&b, "- = auto increment %d\n", i)
			case 7:
				fmt.Fprintf(&b, "\"quoted key %d\" : value\r\n", i)
			}
		}
	}
	return b.Bytes()
}

// BenchmarkLoad10kKeys compares loading with v2 and with the released v1.
func BenchmarkLoad10kKeys(b *testing.B) {
	loaders := []struct {
		name string
		load func([]byte) error
	}{
		{"v2", func(data []byte) error { _, err := Load(data); return err }},
		{"v1", func(data []byte) error { _, err := v1.Load(data); return err }},
	}
	for _, layout := range []string{"flat", "sections", "mixed"} {
		data := genKeys10k(layout)
		for _, l := range loaders {
			b.Run(layout+"/"+l.name, func(b *testing.B) {
				b.SetBytes(int64(len(data)))
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if err := l.load(data); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
