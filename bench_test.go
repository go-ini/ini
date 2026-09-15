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
	"fmt"
	"testing"
)

func newTestFile(block bool) *File {
	c, _ := Load([]byte(confData))
	c.BlockMode = block
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

func Benchmark_NewKey_With_N_Keys(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000} {
		b.Run(fmt.Sprintf("keys=%d", n), func(b *testing.B) {
			f := Empty()
			sec, _ := f.NewSection("bench")
			for i := 0; i < n; i++ {
				sec.NewKey(fmt.Sprintf("key%d", i), "val")
			}
			target := fmt.Sprintf("key%d", n-1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sec.NewKey(target, "newval")
			}
		})
	}
}

func Benchmark_NewSection_With_N_Sections(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000} {
		b.Run(fmt.Sprintf("sections=%d", n), func(b *testing.B) {
			f := Empty()
			for i := 0; i < n; i++ {
				f.NewSection(fmt.Sprintf("sec%d", i))
			}
			target := fmt.Sprintf("sec%d", n-1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				f.NewSection(target)
			}
		})
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
