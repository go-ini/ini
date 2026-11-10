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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// parserInputs returns the inputs in testdata/inputs.txt.
func parserInputs(tb testing.TB) []string {
	tb.Helper()
	data, err := os.ReadFile("testdata/inputs.txt")
	if err != nil {
		tb.Fatal(err)
	}
	var inputs []string
	for i, line := range strings.Split(string(data), "\n") {
		// Git on Windows may check the file out with CRLF line endings.
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		quoted, err := strconv.QuotedPrefix(line)
		if err == nil {
			var in string
			in, err = strconv.Unquote(quoted)
			inputs = append(inputs, in)
		}
		if err != nil {
			tb.Fatalf("testdata/inputs.txt:%d: %v", i+1, err)
		}
	}
	return inputs
}

// fuzzInputs returns the seed inputs of the fuzzers: confData, the parser
// inputs and the testdata files.
func fuzzInputs(tb testing.TB) []string {
	tb.Helper()
	inputs := append([]string{confData}, parserInputs(tb)...)
	files, _ := filepath.Glob("testdata/*.ini")
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			tb.Fatal(err)
		}
		inputs = append(inputs, string(data))
	}
	return inputs
}

// optionsFromBits maps the bits of flags to load options, so that fuzzers
// explore option combinations along with inputs.
func optionsFromBits(flags uint32) []LoadOption {
	bit := func(i uint) bool { return flags&(1<<i) != 0 }
	opts := []LoadOption{
		CaseInsensitive(bit(0)),
		CaseInsensitiveSections(bit(1)),
		CaseInsensitiveKeys(bit(2)),
		IgnoreContinuation(bit(3)),
		IgnoreInlineComment(bit(4)),
		SkipUnrecognizableLines(bit(5)),
		AllowBooleanKeys(bit(6)),
		AllowShadows(bit(7)),
		AllowNestedValues(bit(8)),
		AllowPythonMultilineValues(bit(9)),
		SpaceBeforeInlineComment(bit(10)),
		UnescapeValueDoubleQuotes(bit(11)),
		UnescapeValueCommentSymbols(bit(12)),
		PreserveSurroundingQuotes(bit(13)),
		AllowNonUniqueSections(bit(14)),
		AllowDuplicateShadowValues(bit(15)),
	}
	if bit(16) {
		opts = append(opts, RawSections("raw", "COMMENTS"))
	}
	return append(opts, KeyValueDelimiters([]string{"", "=", ":", "→=", " ", "?!", "=:", ""}[(flags>>17)&7]))
}
