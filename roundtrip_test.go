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
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The round-trip tests write a loaded File and load the output again with
// the same options. Whatever was loaded must come back: section and key
// names, values, shadows, nested values, raw bodies, flags and comments.

// describeFile lists everything a File holds that writing must preserve.
// Comments are compared after the normalization the writer applies.
func describeFile(f *File) string {
	var b strings.Builder
	for _, s := range f.Sections() {
		fmt.Fprintf(&b, "[%q] comment=%q raw=%v body=%q\n", s.name, normalizeComment(s.Comment), s.isRawSection, s.rawBody)
		for _, k := range s.Keys() {
			vals := []string{k.value}
			for _, s := range k.shadows {
				vals = append(vals, s.value)
			}
			fmt.Fprintf(&b, "  %q=%q comment=%q auto=%v bool=%v nested=%q\n",
				k.name, vals, normalizeComment(k.Comment), k.isAutoIncrement, k.isBooleanType, k.nestedValues)
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
		// Written with "=" for other readers, as v1 did.
		return
	}
	want := describeFile(f)

	var out bytes.Buffer
	if _, err = f.Write(&out, wopts...); err != nil {
		// Some files can't be written; that must be reported, not
		// written wrongly.
		return
	}
	g, err := Load(out.Bytes(), opts...)
	if err != nil {
		t.Fatalf("written file doesn't load: %v\ninput: %q\nwritten: %q", err, data, out.String())
	}
	if got := describeFile(g); got != want {
		t.Fatalf("written file loads differently\ninput: %q\nwritten: %q\nloaded:\n%s\nreloaded:\n%s", data, out.String(), want, got)
	}
}

func FuzzWriteRoundTrip(f *testing.F) {
	inputs := append([]string{confData}, paritySeeds...)
	files, _ := filepath.Glob("testdata/*.ini")
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			f.Fatal(err)
		}
		inputs = append(inputs, string(data))
	}
	for _, in := range inputs {
		for _, flags := range []uint32{0, 1 << 9, 1<<8 | 1<<6, 1<<11 | 1<<10 | 1<<12, 1<<16 | 1<<7 | 1<<5, 1<<20 - 1} {
			f.Add([]byte(in), flags)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, flags uint32) {
		checkRoundTrip(t, data, v2Options(parityOptions(flags)))
	})
}

func TestWriteRoundTripRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, in := range paritySeeds {
		for i := 0; i < 200; i++ {
			checkRoundTrip(t, []byte(in), v2Options(parityOptions(rng.Uint32())))
		}
	}
}

// v1 wrote each of these files so that they loaded differently.
func TestWriteReadsBack(t *testing.T) {
	for _, tc := range []struct {
		data string
		opts []LoadOption
	}{
		{"a = 'x\\'\nb = 1\n", nil},                                      // value ending in a backslash
		{"k = `a\"\"\"\nb`\n", nil},                                      // value containing """
		{"k=`= v`\n", []LoadOption{KeyValueDelimiters(" =")}},            // value starting with a delimiter
		{"[u]\nExecStart=\nExecStart=a\n", []LoadOption{AllowShadows()}}, // empty value resets
		{"k = a\n; c\nk = b\n", []LoadOption{AllowShadows()}},            // comment of a shadowed key
		{"k =\n  n\nk\n", []LoadOption{AllowNestedValues(), AllowBooleanKeys()}},
		{"k = \"\"\n  n\n", []LoadOption{AllowNestedValues(), AllowPythonMultilineValues()}},
		{"NAME = x\n[DEFAULT]\nk1 = v1\n", []LoadOption{Insensitive(), AllowNonUniqueSections()}},
		{"[DEFAULT] ; c\n[s]\nk = v\n", nil}, // comment of an empty default section
	} {
		f, err := Load([]byte(tc.data), tc.opts...)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(io.Discard); err != nil {
			t.Fatalf("%q: %v", tc.data, err)
		}
		checkRoundTrip(t, []byte(tc.data), tc.opts)
	}

	t.Run("raw body without line break", func(t *testing.T) {
		f := Empty(UnparseableSections("raw"))
		_, _ = f.NewRawSection("raw", "body")
		_, _ = f.Section("s").NewKey("k", "v")
		var b bytes.Buffer
		if _, err := f.Write(&b); err != nil {
			t.Fatal(err)
		}
		if want := "[raw]\nbody\n\n[s]\nk = v\n"; b.String() != want {
			t.Fatalf("got %q, want %q", b.String(), want)
		}
	})

	t.Run("unwritable key", func(t *testing.T) {
		f := Empty()
		_, _ = f.Section("").NewKey("a\nb", "v")
		if _, err := f.Write(io.Discard); err == nil {
			t.Fatal("want an error for a key name with a line break")
		}
	})
}
