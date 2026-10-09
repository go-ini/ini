// Copyright 2016 Unknwon
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
	})
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

func Test_findInlineComment(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		spaceBefore bool
		expected    int
	}{
		{
			name:        "no comment",
			line:        "hello world",
			spaceBefore: false,
			expected:    -1,
		},
		{
			name:        "hash comment without space requirement",
			line:        "hello#world",
			spaceBefore: false,
			expected:    5,
		},
		{
			name:        "hash comment with space before",
			line:        "hello #world",
			spaceBefore: true,
			expected:    5,
		},
		{
			name:        "hash without space when space required",
			line:        "hello#world",
			spaceBefore: true,
			expected:    -1,
		},
		{
			name:        "semicolon comment without space requirement",
			line:        "hello;world",
			spaceBefore: false,
			expected:    5,
		},
		{
			name:        "semicolon comment with space before",
			line:        "hello ;world",
			spaceBefore: true,
			expected:    5,
		},
		{
			name:        "hash inside single quotes",
			line:        "'#e18477'",
			spaceBefore: false,
			expected:    -1,
		},
		{
			name:        "hash inside double quotes",
			line:        `"#e18477"`,
			spaceBefore: false,
			expected:    -1,
		},
		{
			name:        "semicolon inside single quotes",
			line:        "'val;ue'",
			spaceBefore: false,
			expected:    -1,
		},
		{
			name:        "semicolon inside double quotes",
			line:        `"val;ue"`,
			spaceBefore: false,
			expected:    -1,
		},
		{
			name:        "comment after single quoted string",
			line:        "'#e18477' # comment",
			spaceBefore: false,
			expected:    10,
		},
		{
			name:        "comment after single quoted string with space required",
			line:        "'#e18477' # comment",
			spaceBefore: true,
			expected:    9,
		},
		{
			name:        "comment after double quoted string",
			line:        `"#e18477" ; comment`,
			spaceBefore: false,
			expected:    10,
		},
		{
			name:        "comment after double quoted string with space required",
			line:        `"#e18477" ; comment`,
			spaceBefore: true,
			expected:    9,
		},
		{
			name:        "multiple quoted strings with comment",
			line:        `'#123', "#456" # comment`,
			spaceBefore: false,
			expected:    15,
		},
		{
			name:        "escaped quote inside quotes",
			line:        `"hello \"#\" world" # real comment`,
			spaceBefore: false,
			expected:    20,
		},
		{
			name:        "apostrophe in word followed by comment",
			line:        "don't touch # comment",
			spaceBefore: false,
			expected:    12,
		},
		{
			name:        "unclosed quote falls back to scanning",
			line:        `"unclosed # comment`,
			spaceBefore: false,
			expected:    10,
		},
		{
			name:        "escaped comment symbol outside quotes",
			line:        `value\#not_comment # real`,
			spaceBefore: false,
			expected:    19,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := findInlineComment(tt.line, tt.spaceBefore)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

