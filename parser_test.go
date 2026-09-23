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

func TestInlineCommentMarkersInsideQuotedValues(t *testing.T) {
	f, err := Load([]byte("[background]\nprimary = '#e18477'\nsecondary = \"#000000\"\nurl = 'https://example.test/#fragment' ; trailing comment\nplain = value # actual comment\n"))
	require.NoError(t, err)
	section := f.Section("background")
	assert.Equal(t, "#e18477", section.Key("primary").String())
	assert.Equal(t, "#000000", section.Key("secondary").String())
	assert.Equal(t, "https://example.test/#fragment", section.Key("url").String())
	assert.Equal(t, "value", section.Key("plain").String())
}

func TestInlineCommentIndexRespectsQuoteAndSpaceOptions(t *testing.T) {
	assert.Equal(t, -1, inlineCommentIndex(`'#a;b'`, false))
	assert.Equal(t, 7, inlineCommentIndex(`'#a;b' # trailing`, false))
	assert.Equal(t, 6, inlineCommentIndex(`'#a;b' # trailing`, true))
	assert.Equal(t, -1, inlineCommentIndex(`value#literal`, true))
	assert.Equal(t, 5, inlineCommentIndex(`value#comment`, false))
	assert.Equal(t, 5, inlineCommentIndex(`value ; comment`, true))
	assert.Equal(t, 5, inlineCommentIndex(`don't # comment`, true))
}
