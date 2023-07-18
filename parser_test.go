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
	"io/ioutil"
	"os"
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

func FuzzLoad(f *testing.F) {

	files := []string{
		"testdata/full.ini",
		"testdata/minimal.ini",
		"testdata/multiline_eof.ini",
		"testdata/multiline.ini",
		"testdata/TestFile_WriteTo.golden",
		"testdata/UTF-16-BE-BOM.ini",
		"testdata/UTF-16-LE-BOM.ini",
		"testdata/UTF-8-BOM.ini",
	}

	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			f.Fatalf("failed to read file %s: %v", file, err)
			os.Exit(1)
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Load(data)
		_, _ = LooseLoad(data)
		_, _ = InsensitiveLoad(data)
		_, _ = ShadowLoad(data)
	})
}
