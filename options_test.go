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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteOptions(t *testing.T) {
	f, err := Load([]byte("a = 1\n[s]\nbb = 2\n"))
	require.NoError(t, err)

	// Expected output is what v1 wrote with the same package globals set.
	var b bytes.Buffer
	_, err = f.Write(&b, PrettyFormat(false), PrettySection(false), DelimiterSpaces("", " "), LineBreak("\r\n"), DefaultHeader())
	require.NoError(t, err)
	assert.Equal(t, "[DEFAULT]\r\na= 1\r\n[s]\r\nbb= 2\r\n", b.String())

	// Options apply to one call only.
	b.Reset()
	_, err = f.WriteTo(&b)
	require.NoError(t, err)
	assert.Equal(t, "a = 1\n\n[s]\nbb = 2\n", b.String())
}

func TestDefaultSectionName(t *testing.T) {
	f, err := Load([]byte("a = 1\n[default]\nb = 2\n[profile x]\nc = 3\n"), DefaultSectionName("default"))
	require.NoError(t, err)

	assert.Equal(t, []string{"default", "profile x"}, f.SectionStrings())
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, f.Section("").KeysHash())

	var b bytes.Buffer
	_, err = f.WriteTo(&b)
	require.NoError(t, err)
	assert.Equal(t, "[default]\na = 1\nb = 2\n\n[profile x]\nc = 3\n", b.String())

	t.Run("empty default section is not written", func(t *testing.T) {
		f, err := Load([]byte("[s]\nk = v\n"), DefaultSectionName("default"))
		require.NoError(t, err)

		var b bytes.Buffer
		_, err = f.WriteTo(&b)
		require.NoError(t, err)
		assert.Equal(t, "[s]\nk = v\n", b.String())
	})
}

func TestCommentSymbols(t *testing.T) {
	f, err := Load([]byte("// header\n[s] // section\nurl = http://x/#a;b // note\nk = a\\/\\/b\n"),
		CommentSymbols("/"), UnescapeValueCommentSymbols(), SpaceBeforeInlineComment())
	require.NoError(t, err)

	sec := f.Section("s")
	assert.Equal(t, "// header\n// section", sec.Comment)
	assert.Equal(t, "http://x/#a;b", sec.Key("url").String())
	assert.Equal(t, "// note", sec.Key("url").Comment)
	assert.Equal(t, "a//b", sec.Key("k").String())

	t.Run("multi-byte symbols", func(t *testing.T) {
		f, err := Load([]byte("€ head\n→ arrow = 1\nk = a €c\nj = x\\€y\nu = \\→\n"),
			CommentSymbols("€"), SpaceBeforeInlineComment(), UnescapeValueCommentSymbols())
		require.NoError(t, err)

		sec := f.Section("")
		assert.Equal(t, []string{"→ arrow", "k", "j", "u"}, sec.KeyStrings())
		assert.Equal(t, "€ head", sec.Key("→ arrow").Comment)
		assert.Equal(t, "a", sec.Key("k").String())
		assert.Equal(t, "€c", sec.Key("k").Comment)
		assert.Equal(t, "x€y", sec.Key("j").String())
		assert.Equal(t, `\→`, sec.Key("u").String())

		f, err = Load([]byte("§  head\nk = v\n"), CommentSymbols("§"))
		require.NoError(t, err)
		_, err = f.Section("").NewKey("n", "1")
		require.NoError(t, err)
		f.Section("").Key("n").Comment = "note"
		var b bytes.Buffer
		_, err = f.WriteTo(&b)
		require.NoError(t, err)
		assert.Equal(t, "§ head\nk = v\n§ note\nn = 1\n", b.String())
	})

	t.Run("no symbols", func(t *testing.T) {
		f := Empty(CommentSymbols(""))
		k, err := f.Section("").NewKey("k", "v")
		require.NoError(t, err)

		var b bytes.Buffer
		_, err = f.WriteTo(&b)
		require.NoError(t, err)
		assert.Equal(t, "k = v\n", b.String())

		// A comment can't be written so that it reads back as a comment.
		k.Comment = "set x = y"
		_, err = f.WriteTo(&b)
		assert.Error(t, err)
	})

	t.Run("written comments use the configured symbol", func(t *testing.T) {
		f := Empty(CommentSymbols("#"))
		k, err := f.Section("").NewKey("k", "v;w")
		require.NoError(t, err)
		k.Comment = "note"

		var b bytes.Buffer
		_, err = f.WriteTo(&b)
		require.NoError(t, err)
		assert.Equal(t, "# note\nk = v;w\n", b.String())
	})
}

func TestParseErrorPosition(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		line, column int
		is           func(error) bool
	}{
		{"missing delimiter", "a = 1\n[s]\n\u00a0\u00a0été x\n", 3, 3, IsErrDelimiterNotFound},
		{"empty key name", "a = 1\n\n= 2\n", 3, 1, IsErrEmptyKeyName},
		{"unclosed section", "a = 1\n\t[s\n", 2, 2, nil},
		{"unclosed quote", "a = 1\nk = `x\ny\n", 2, 1, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load([]byte(test.data))
			var perr *ParseError
			require.True(t, errors.As(err, &perr), "error %v", err)
			assert.Equal(t, test.line, perr.Line)
			assert.Equal(t, test.column, perr.Column)
			if test.is != nil {
				assert.True(t, test.is(err))
			}
		})
	}
}

func TestKeepEmptyShadowValues(t *testing.T) {
	data := []byte("[Service]\nExecStart=/bin/a\nExecStart=\nExecStart=/bin/b\n")

	f, err := Load(data, AllowShadows())
	require.NoError(t, err)
	assert.Equal(t, []string{"/bin/a", "/bin/b"}, f.Section("Service").Key("ExecStart").ValueWithShadows())

	f, err = Load(data, AllowShadows(), KeepEmptyShadowValues())
	require.NoError(t, err)
	assert.Equal(t, []string{"/bin/a", "", "/bin/b"}, f.Section("Service").Key("ExecStart").ValueWithShadows())

	var b bytes.Buffer
	_, err = f.WriteTo(&b)
	require.NoError(t, err)
	assert.Equal(t, "[Service]\nExecStart = /bin/a\nExecStart = \nExecStart = /bin/b\n", b.String())

	t.Run("every empty value is kept", func(t *testing.T) {
		f, err := Load([]byte("[u]\nExecStart=\nExecStart=a\nExecStart=\nExecStart=b\nExecStart=a\n"),
			AllowShadows(), KeepEmptyShadowValues())
		require.NoError(t, err)
		assert.Equal(t, []string{"", "a", "", "b"}, f.Section("u").Key("ExecStart").ValueWithShadows())
	})
}

func TestKey_IsBooleanType(t *testing.T) {
	f, err := Load([]byte("[mysqld]\nskip-name-resolve\nuser = mysql\n"), AllowBooleanKeys())
	require.NoError(t, err)
	assert.True(t, f.Section("mysqld").Key("skip-name-resolve").IsBooleanType())
	assert.False(t, f.Section("mysqld").Key("user").IsBooleanType())
}
