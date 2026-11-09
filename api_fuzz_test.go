// Copyright 2026 Unknwon
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
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fzName string

type fzSub struct {
	A int
	B string
	T time.Time
}

type fzNode struct {
	X     int
	Child *fzNode
}

type fzEmbed struct {
	E string
}

// fzCfg has a field of every kind struct mapping supports.
type fzCfg struct {
	S   string
	I   int
	I8  int8
	U   uint
	U8  uint8
	F32 float32
	F   float64
	B   bool
	D   time.Duration
	T   time.Time

	PS *string
	PI *int
	PT *time.Time
	PD *time.Duration

	SS []string
	SI []int
	SU []uint64
	SF []float64
	SB []bool
	ST []time.Time
	SP []string `delim:"|"`

	Named  fzName
	PNamed *fzName

	Sub     fzSub
	PSub    *fzSub
	Shadow  []string `ini:"shadow,,allowshadow"`
	Peers   []fzSub  `ini:"Peer,,,nonunique"`
	PPeers  []*fzSub `ini:"PPeer,,,nonunique"`
	Node    *fzNode
	fzEmbed `ini:",extends"`
	Skip    string `ini:"-"`
}

// FuzzStructMapping loads arbitrary input under random options, reads it
// with every Key accessor and maps it into fzCfg. Nothing may panic or hang,
// and a value written by Marshal must read back the same.
func FuzzStructMapping(f *testing.F) {
	inputs := append([]string{confData}, paritySeeds...)
	files, _ := filepath.Glob("testdata/*.ini")
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		inputs = append(inputs, string(data))
	}
	inputs = append(inputs,
		"S = s\nI = -3\nI8 = 127\nU = 4\nU8 = 255\nF32 = 1.5\nF = 2.5\nB = true\nD = 1s\nT = 2020-01-02T03:04:05.5Z\n"+
			"PS = p\nPI = 7\nPT = 2020-01-02T03:04:05Z\nPD = 3s\nSS = a,b\\, c,d\nSI = 1,2\nSU = 3,4\nSF = 1.5,2\nSB = true,false\n"+
			"ST = 2020-01-02T03:04:05Z\nSP = x|y\nNamed = n\nPNamed = m\nshadow = a\nshadow = b\nE = e\n"+
			"[Sub]\nA = 1\nB = b\n[PSub]\nA = 2\n[Peer]\nA = 3\n[Peer]\nA = 4\n[PPeer]\nB = q\n[Node]\nX = 1\n[Child]\nX = 2\n",
		"I8 = 300\nU8 = -1\nF32 = 1e300\nD = -5s\nSI = 1,x,3\nT = 2006-01-02T15:04:05.123456789+07:00\n",
		"a = %(b)s\nb = %(a)s\nS = %(S)s%(S)s\n",
	)
	for _, in := range inputs {
		for _, flags := range []uint32{0, 1 << 7, 1<<14 | 1<<7, 1<<0 | 1<<14, 1<<20 - 1} {
			f.Add([]byte(in), flags)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte, flags uint32) {
		opts := v2Options(parityOptions(flags))
		file, err := Load(data, opts...)
		if err != nil {
			return
		}

		for _, sec := range file.Sections() {
			_ = sec.ChildSections()
			_ = sec.ParentKeys()
			_ = sec.KeysHash()
			for _, k := range sec.Keys() {
				_ = k.String()
				for _, delim := range []string{",", "", "|", "\\", "::"} {
					_ = k.Strings(delim)
				}
				_ = k.StringsWithShadows(",")
				_ = k.Ints(",")
				_ = k.Uint64s(",")
				_ = k.Float64s(",")
				_ = k.Bools(",")
				_ = k.Times(",")
				_, _ = k.Duration()
				_, _ = k.Time()
				_ = k.MustInt(1)
				_ = k.In("a", []string{"a", "b"})
				_ = k.ValueWithShadows()
				_ = k.NestedValues()
			}
		}
		_ = file.SectionWithIndex("", 1)
		_ = file.SectionWithIndex("Peer", -1)

		var cfg fzCfg
		_ = file.MapTo(&cfg)
		var strict fzCfg
		_ = file.StrictMapTo(&strict)

		// What Marshal writes, Unmarshal reads back the same, except
		// for the limits of the format that are checked below.
		out, err := Marshal(cfg)
		if err != nil {
			return
		}
		var back fzCfg
		if err := Unmarshal(out, &back); err != nil {
			t.Fatalf("Unmarshal of Marshal output failed: %v\nmarshaled: %q", err, out)
		}
		normalizeFz(&cfg)
		normalizeFz(&back)
		if !reflect.DeepEqual(cfg, back) {
			t.Fatalf("Marshal/Unmarshal changed the value\nbefore: %+v\nafter:  %+v\nmarshaled: %q", cfg, back, out)
		}

		// ReflectFrom into the loaded file must not panic and must write.
		if err := file.ReflectFrom(&cfg); err == nil {
			var buf bytes.Buffer
			_, _ = file.Write(&buf)
		}
	})
}

// normalizeFz clears what the format can't carry: empty slices and nil
// versus empty, monotonic clock readings and locations of times.
func normalizeFz(c *fzCfg) {
	v := reflect.ValueOf(c).Elem()
	normalizeFzValue(v)
}

func normalizeFzValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			normalizeFzValue(v.Elem())
		}
	case reflect.Struct:
		if v.Type() == timeType {
			if v.CanSet() {
				t := v.Interface().(time.Time)
				v.Set(reflect.ValueOf(time.Unix(t.Unix(), int64(t.Nanosecond())).UTC()))
			}
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				normalizeFzValue(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.Len() == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		for i := 0; i < v.Len(); i++ {
			normalizeFzValue(v.Index(i))
		}
	}
}
