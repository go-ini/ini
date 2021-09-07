// Copyright 2014 Unknwon
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

package ini_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/ini.v1"
)

const (
	confData = `
	; Package name
	NAME        = ini
	; Package version
	VERSION     = v1
	; Package import path
	IMPORT_PATH = gopkg.in/%(NAME)s.%(VERSION)s
	
	# Information about package author
	# Bio can be written in multiple lines.
	[author]
	NAME   = Unknwon  ; Succeeding comment
	E-MAIL = fake@localhost
	GITHUB = https://github.com/%(NAME)s
	BIO    = """Gopher.
	Coding addict.
	Good man.
	"""  # Succeeding comment`
	minimalConf  = "testdata/minimal.ini"
	fullConf     = "testdata/full.ini"
	notFoundConf = "testdata/404.ini"
)

var update = flag.Bool("update", false, "Update .golden files")

func TestLoad(t *testing.T) {
	t.Run("load from good data sources", func(t *testing.T) {
		f, err := ini.Load(
			"testdata/minimal.ini",
			[]byte("NAME = ini\nIMPORT_PATH = gopkg.in/%(NAME)s.%(VERSION)s"),
			bytes.NewReader([]byte(`VERSION = v1`)),
			ioutil.NopCloser(bytes.NewReader([]byte("[author]\nNAME = Unknwon"))),
		)
		require.NoError(t, err)
		require.NotNil(t, f)

		// Validate values make sure all sources are loaded correctly
		sec := f.Section("")
		assert.Equal(t, "ini", sec.Key("NAME").String())
		assert.Equal(t, "v1", sec.Key("VERSION").String())
		assert.Equal(t, "gopkg.in/ini.v1", sec.Key("IMPORT_PATH").String())

		sec = f.Section("author")
		assert.Equal(t, "Unknwon", sec.Key("NAME").String())
		assert.Equal(t, "u@gogs.io", sec.Key("E-MAIL").String())
	})

	t.Run("load from bad data sources", func(t *testing.T) {
		t.Run("invalid input", func(t *testing.T) {
			_, err := ini.Load(notFoundConf)
			require.Error(t, err)
		})

		t.Run("unsupported type", func(t *testing.T) {
			_, err := ini.Load(123)
			require.Error(t, err)
		})
	})

	t.Run("cannot properly parse INI files containing `#` or `;` in value", func(t *testing.T) {
		f, err := ini.Load([]byte(`
	[author]
	NAME = U#n#k#n#w#o#n
	GITHUB = U;n;k;n;w;o;n
	`))
		require.NoError(t, err)
		require.NotNil(t, f)

		sec := f.Section("author")
		nameValue := sec.Key("NAME").String()
		githubValue := sec.Key("GITHUB").String()
		assert.Equal(t, "U", nameValue)
		assert.Equal(t, "U", githubValue)
	})

	t.Run("cannot parse small python-compatible INI files", func(t *testing.T) {
		f, err := ini.Load([]byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
   foo
   bar
   foobar
   barfoo
   -----END RSA PRIVATE KEY-----
`))
		require.Error(t, err)
		assert.Nil(t, f)
		assert.Equal(t, "key-value delimiter not found: foo\n", err.Error())
	})

	t.Run("cannot parse big python-compatible INI files", func(t *testing.T) {
		f, err := ini.Load([]byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
   1foo
   2bar
   3foobar
   4barfoo
   5foo
   6bar
   7foobar
   8barfoo
   9foo
   10bar
   11foobar
   12barfoo
   13foo
   14bar
   15foobar
   16barfoo
   17foo
   18bar
   19foobar
   20barfoo
   21foo
   22bar
   23foobar
   24barfoo
   25foo
   26bar
   27foobar
   28barfoo
   29foo
   30bar
   31foobar
   32barfoo
   33foo
   34bar
   35foobar
   36barfoo
   37foo
   38bar
   39foobar
   40barfoo
   41foo
   42bar
   43foobar
   44barfoo
   45foo
   46bar
   47foobar
   48barfoo
   49foo
   50bar
   51foobar
   52barfoo
   53foo
   54bar
   55foobar
   56barfoo
   57foo
   58bar
   59foobar
   60barfoo
   61foo
   62bar
   63foobar
   64barfoo
   65foo
   66bar
   67foobar
   68barfoo
   69foo
   70bar
   71foobar
   72barfoo
   73foo
   74bar
   75foobar
   76barfoo
   77foo
   78bar
   79foobar
   80barfoo
   81foo
   82bar
   83foobar
   84barfoo
   85foo
   86bar
   87foobar
   88barfoo
   89foo
   90bar
   91foobar
   92barfoo
   93foo
   94bar
   95foobar
   96barfoo
   -----END RSA PRIVATE KEY-----
`))
		require.Error(t, err)
		assert.Nil(t, f)
		assert.Equal(t, "key-value delimiter not found: 1foo\n", err.Error())
	})
}

func TestLooseLoad(t *testing.T) {
	f, err := ini.LoadSources(ini.LoadOptions{Loose: true}, notFoundConf, minimalConf)
	require.NoError(t, err)
	require.NotNil(t, f)

	t.Run("inverse case", func(t *testing.T) {
		_, err = ini.Load(notFoundConf)
		require.Error(t, err)
	})
}

func TestInsensitiveLoad(t *testing.T) {
	t.Run("insensitive to section and key names", func(t *testing.T) {
		f, err := ini.InsensitiveLoad(minimalConf)
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "u@gogs.io", f.Section("Author").Key("e-mail").String())

		t.Run("write out", func(t *testing.T) {
			var buf bytes.Buffer
			_, err := f.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, `[author]
e-mail = u@gogs.io

`,
				buf.String(),
			)
		})

		t.Run("inverse case", func(t *testing.T) {
			f, err := ini.Load(minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Empty(t, f.Section("Author").Key("e-mail").String())
		})
	})

	// Ref: https://github.com/go-ini/ini/issues/198
	t.Run("insensitive load with default section", func(t *testing.T) {
		f, err := ini.InsensitiveLoad([]byte(`
user = unknwon
[profile]
email = unknwon@local
`))
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "unknwon", f.Section(ini.DefaultSection).Key("user").String())
	})
}

func TestLoadSources(t *testing.T) {
	t.Run("with true `AllowPythonMultilineValues`", func(t *testing.T) {
		t.Run("ignore nonexistent files", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true, Loose: true}, notFoundConf, minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			t.Run("inverse case", func(t *testing.T) {
				_, err = ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, notFoundConf)
				require.Error(t, err)
			})
		})

		t.Run("insensitive to section and key names", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true, Insensitive: true}, minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "u@gogs.io", f.Section("Author").Key("e-mail").String())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `[author]
e-mail = u@gogs.io

`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, minimalConf)
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Empty(t, f.Section("Author").Key("e-mail").String())
			})
		})

		t.Run("insensitive to sections and sensitive to key names", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{InsensitiveSections: true}, minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "u@gogs.io", f.Section("Author").Key("E-MAIL").String())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `[author]
E-MAIL = u@gogs.io

`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{}, minimalConf)
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Empty(t, f.Section("Author").Key("e-mail").String())
			})
		})

		t.Run("sensitive to sections and insensitive to key names", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{InsensitiveKeys: true}, minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "u@gogs.io", f.Section("author").Key("e-mail").String())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `[author]
e-mail = u@gogs.io

`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{}, minimalConf)
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Empty(t, f.Section("Author").Key("e-mail").String())
			})
		})

		t.Run("ignore continuation lines", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				IgnoreContinuation:         true,
			}, []byte(`
key1=a\b\
key2=c\d\
key3=value`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `a\b\`, f.Section("").Key("key1").String())
			assert.Equal(t, `c\d\`, f.Section("").Key("key2").String())
			assert.Equal(t, "value", f.Section("").Key("key3").String())

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
key1=a\b\
key2=c\d\`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, `a\bkey2=c\d`, f.Section("").Key("key1").String())
			})
		})

		t.Run("ignore inline comments", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				IgnoreInlineComment:        true,
			}, []byte(`
key1=value ;comment
key2=value2 #comment2
key3=val#ue #comment3`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `value ;comment`, f.Section("").Key("key1").String())
			assert.Equal(t, `value2 #comment2`, f.Section("").Key("key2").String())
			assert.Equal(t, `val#ue #comment3`, f.Section("").Key("key3").String())

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
key1=value ;comment
key2=value2 #comment2`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, `value`, f.Section("").Key("key1").String())
				assert.Equal(t, `;comment`, f.Section("").Key("key1").Comment)
				assert.Equal(t, `value2`, f.Section("").Key("key2").String())
				assert.Equal(t, `#comment2`, f.Section("").Key("key2").Comment)
			})
		})

		t.Run("skip unrecognizable lines", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				SkipUnrecognizableLines: true,
			}, []byte(`
GenerationDepth: 13

BiomeRarityScale: 100

################
# Biome Groups #
################

BiomeGroup(NormalBiomes, 3, 99, RoofedForestEnchanted, ForestSakura, FloatingJungle
BiomeGroup(IceBiomes, 4, 85, Ice Plains)
`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "13", f.Section("").Key("GenerationDepth").String())
			assert.Equal(t, "100", f.Section("").Key("BiomeRarityScale").String())
			assert.False(t, f.Section("").HasKey("BiomeGroup"))
		})

		t.Run("allow boolean type keys", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				AllowBooleanKeys:           true,
			}, []byte(`
key1=hello
#key2
key3`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, []string{"key1", "key3"}, f.Section("").KeyStrings())
			assert.True(t, f.Section("").Key("key3").MustBool(false))

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `key1 = hello
# key2
key3
`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
key1=hello
#key2
key3`))
				require.Error(t, err)
			})
		})

		t.Run("allow shadow keys", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{AllowShadows: true, AllowPythonMultilineValues: true}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "https://github.com/Antergone/test1.git", f.Section(`remote "origin"`).Key("url").String())
			assert.Equal(
				t,
				[]string{
					"https://github.com/Antergone/test1.git",
					"https://github.com/Antergone/test2.git",
				},
				f.Section(`remote "origin"`).Key("url").ValueWithShadows(),
			)
			assert.Equal(t, "+refs/heads/*:refs/remotes/origin/*", f.Section(`remote "origin"`).Key("fetch").String())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `[remote "origin"]
url   = https://github.com/Antergone/test1.git
url   = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*

`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, "https://github.com/Antergone/test2.git", f.Section(`remote "origin"`).Key("url").String())
			})
		})

		t.Run("unescape double quotes inside value", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				UnescapeValueDoubleQuotes:  true,
			}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `创建了仓库 <a href="%s">%s</a>`, f.Section("").Key("create_repo").String())

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, `"创建了仓库 <a href=\"%s\">%s</a>"`, f.Section("").Key("create_repo").String())
			})
		})

		t.Run("unescape comment symbols inside value", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues:  true,
				IgnoreInlineComment:         true,
				UnescapeValueCommentSymbols: true,
			}, []byte(`
key = test value <span style="color: %s\; background: %s">more text</span>
`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `test value <span style="color: %s; background: %s">more text</span>`, f.Section("").Key("key").String())
		})

		t.Run("can parse small python-compatible INI files", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				Insensitive:                true,
				UnparseableSections:        []string{"core_lesson", "comments"},
			}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
  foo
  bar
  foobar
  barfoo
  -----END RSA PRIVATE KEY-----
multiline_list =
  first
  second
  third
`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "-----BEGIN RSA PRIVATE KEY-----\nfoo\nbar\nfoobar\nbarfoo\n-----END RSA PRIVATE KEY-----", f.Section("long").Key("long_rsa_private_key").String())
			assert.Equal(t, "\nfirst\nsecond\nthird", f.Section("long").Key("multiline_list").String())
		})

		t.Run("can parse big python-compatible INI files", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				Insensitive:                true,
				UnparseableSections:        []string{"core_lesson", "comments"},
			}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
   1foo
   2bar
   3foobar
   4barfoo
   5foo
   6bar
   7foobar
   8barfoo
   9foo
   10bar
   11foobar
   12barfoo
   13foo
   14bar
   15foobar
   16barfoo
   17foo
   18bar
   19foobar
   20barfoo
   21foo
   22bar
   23foobar
   24barfoo
   25foo
   26bar
   27foobar
   28barfoo
   29foo
   30bar
   31foobar
   32barfoo
   33foo
   34bar
   35foobar
   36barfoo
   37foo
   38bar
   39foobar
   40barfoo
   41foo
   42bar
   43foobar
   44barfoo
   45foo
   46bar
   47foobar
   48barfoo
   49foo
   50bar
   51foobar
   52barfoo
   53foo
   54bar
   55foobar
   56barfoo
   57foo
   58bar
   59foobar
   60barfoo
   61foo
   62bar
   63foobar
   64barfoo
   65foo
   66bar
   67foobar
   68barfoo
   69foo
   70bar
   71foobar
   72barfoo
   73foo
   74bar
   75foobar
   76barfoo
   77foo
   78bar
   79foobar
   80barfoo
   81foo
   82bar
   83foobar
   84barfoo
   85foo
   86bar
   87foobar
   88barfoo
   89foo
   90bar
   91foobar
   92barfoo
   93foo
   94bar
   95foobar
   96barfoo
   -----END RSA PRIVATE KEY-----
`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `-----BEGIN RSA PRIVATE KEY-----
1foo
2bar
3foobar
4barfoo
5foo
6bar
7foobar
8barfoo
9foo
10bar
11foobar
12barfoo
13foo
14bar
15foobar
16barfoo
17foo
18bar
19foobar
20barfoo
21foo
22bar
23foobar
24barfoo
25foo
26bar
27foobar
28barfoo
29foo
30bar
31foobar
32barfoo
33foo
34bar
35foobar
36barfoo
37foo
38bar
39foobar
40barfoo
41foo
42bar
43foobar
44barfoo
45foo
46bar
47foobar
48barfoo
49foo
50bar
51foobar
52barfoo
53foo
54bar
55foobar
56barfoo
57foo
58bar
59foobar
60barfoo
61foo
62bar
63foobar
64barfoo
65foo
66bar
67foobar
68barfoo
69foo
70bar
71foobar
72barfoo
73foo
74bar
75foobar
76barfoo
77foo
78bar
79foobar
80barfoo
81foo
82bar
83foobar
84barfoo
85foo
86bar
87foobar
88barfoo
89foo
90bar
91foobar
92barfoo
93foo
94bar
95foobar
96barfoo
-----END RSA PRIVATE KEY-----`,
				f.Section("long").Key("long_rsa_private_key").String(),
			)
		})

		t.Run("allow unparsable sections", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: true,
				Insensitive:                true,
				UnparseableSections:        []string{"core_lesson", "comments"},
			}, []byte(`
Lesson_Location = 87
Lesson_Status = C
Score = 3
Time = 00:02:30

[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[COMMENTS]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "3", f.Section("").Key("score").String())
			assert.Empty(t, f.Section("").Body())
			assert.Equal(t, `my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`,
				f.Section("core_lesson").Body(),
			)
			assert.Equal(t, `<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`, f.Section("comments").Body())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `lesson_location = 87
lesson_status   = C
score           = 3
time            = 00:02:30

[core_lesson]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[comments]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>
`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`))
				require.Error(t, err)
			})
		})

		t.Run("and false `SpaceBeforeInlineComment`", func(t *testing.T) {
			t.Run("cannot parse INI files containing `#` or `;` in value", func(t *testing.T) {
				f, err := ini.LoadSources(
					ini.LoadOptions{AllowPythonMultilineValues: false, SpaceBeforeInlineComment: false},
					[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
				require.NoError(t, err)
				require.NotNil(t, f)
				sec := f.Section("author")
				nameValue := sec.Key("NAME").String()
				githubValue := sec.Key("GITHUB").String()
				assert.Equal(t, "U", nameValue)
				assert.Equal(t, "U", githubValue)
			})
		})

		t.Run("and true `SpaceBeforeInlineComment`", func(t *testing.T) {
			t.Run("can parse INI files containing `#` or `;` in value", func(t *testing.T) {
				f, err := ini.LoadSources(
					ini.LoadOptions{AllowPythonMultilineValues: false, SpaceBeforeInlineComment: true},
					[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
				require.NoError(t, err)
				require.NotNil(t, f)
				sec := f.Section("author")
				nameValue := sec.Key("NAME").String()
				githubValue := sec.Key("GITHUB").String()
				assert.Equal(t, "U#n#k#n#w#o#n", nameValue)
				assert.Equal(t, "U;n;k;n;w;o;n", githubValue)
			})
		})
	})

	t.Run("with false `AllowPythonMultilineValues`", func(t *testing.T) {
		t.Run("ignore nonexistent files", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				Loose:                      true,
			}, notFoundConf, minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			t.Run("inverse case", func(t *testing.T) {
				_, err = ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
				}, notFoundConf)
				require.Error(t, err)
			})
		})

		t.Run("insensitive to section and key names", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				Insensitive:                true,
			}, minimalConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "u@gogs.io", f.Section("Author").Key("e-mail").String())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `[author]
e-mail = u@gogs.io

`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
				}, minimalConf)
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Empty(t, f.Section("Author").Key("e-mail").String())
			})
		})

		t.Run("ignore continuation lines", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				IgnoreContinuation:         true,
			}, []byte(`
key1=a\b\
key2=c\d\
key3=value`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `a\b\`, f.Section("").Key("key1").String())
			assert.Equal(t, `c\d\`, f.Section("").Key("key2").String())
			assert.Equal(t, "value", f.Section("").Key("key3").String())

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
key1=a\b\
key2=c\d\`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, `a\bkey2=c\d`, f.Section("").Key("key1").String())
			})
		})

		t.Run("ignore inline comments", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				IgnoreInlineComment:        true,
			}, []byte(`
key1=value ;comment
key2=value2 #comment2
key3=val#ue #comment3`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `value ;comment`, f.Section("").Key("key1").String())
			assert.Equal(t, `value2 #comment2`, f.Section("").Key("key2").String())
			assert.Equal(t, `val#ue #comment3`, f.Section("").Key("key3").String())

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
key1=value ;comment
key2=value2 #comment2`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, `value`, f.Section("").Key("key1").String())
				assert.Equal(t, `;comment`, f.Section("").Key("key1").Comment)
				assert.Equal(t, `value2`, f.Section("").Key("key2").String())
				assert.Equal(t, `#comment2`, f.Section("").Key("key2").Comment)
			})
		})

		t.Run("allow boolean type keys", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				AllowBooleanKeys:           true,
			}, []byte(`
key1=hello
#key2
key3`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, []string{"key1", "key3"}, f.Section("").KeyStrings())
			assert.True(t, f.Section("").Key("key3").MustBool(false))

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `key1 = hello
# key2
key3
`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
key1=hello
#key2
key3`))
				require.Error(t, err)
			})
		})

		t.Run("allow shadow keys", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false, AllowShadows: true}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "https://github.com/Antergone/test1.git", f.Section(`remote "origin"`).Key("url").String())
			assert.Equal(
				t,
				[]string{
					"https://github.com/Antergone/test1.git",
					"https://github.com/Antergone/test2.git",
				},
				f.Section(`remote "origin"`).Key("url").ValueWithShadows(),
			)
			assert.Equal(t, "+refs/heads/*:refs/remotes/origin/*", f.Section(`remote "origin"`).Key("fetch").String())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `[remote "origin"]
url   = https://github.com/Antergone/test1.git
url   = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*

`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, "https://github.com/Antergone/test2.git", f.Section(`remote "origin"`).Key("url").String())
			})
		})

		t.Run("unescape double quotes inside value", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				UnescapeValueDoubleQuotes:  true,
			}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `创建了仓库 <a href="%s">%s</a>`, f.Section("").Key("create_repo").String())

			t.Run("inverse case", func(t *testing.T) {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
				require.NoError(t, err)
				require.NotNil(t, f)

				assert.Equal(t, `"创建了仓库 <a href=\"%s\">%s</a>"`, f.Section("").Key("create_repo").String())
			})
		})

		t.Run("unescape comment symbols inside value", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues:  false,
				IgnoreInlineComment:         true,
				UnescapeValueCommentSymbols: true,
			}, []byte(`
key = test value <span style="color: %s\; background: %s">more text</span>
`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, `test value <span style="color: %s; background: %s">more text</span>`, f.Section("").Key("key").String())
		})

		t.Run("cannot parse small python-compatible INI files", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
  foo
  bar
  foobar
  barfoo
  -----END RSA PRIVATE KEY-----
`))
			require.Error(t, err)
			assert.Nil(t, f)
			assert.Equal(t, "key-value delimiter not found: foo\n", err.Error())
		})

		t.Run("cannot parse big python-compatible INI files", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
  1foo
  2bar
  3foobar
  4barfoo
  5foo
  6bar
  7foobar
  8barfoo
  9foo
  10bar
  11foobar
  12barfoo
  13foo
  14bar
  15foobar
  16barfoo
  17foo
  18bar
  19foobar
  20barfoo
  21foo
  22bar
  23foobar
  24barfoo
  25foo
  26bar
  27foobar
  28barfoo
  29foo
  30bar
  31foobar
  32barfoo
  33foo
  34bar
  35foobar
  36barfoo
  37foo
  38bar
  39foobar
  40barfoo
  41foo
  42bar
  43foobar
  44barfoo
  45foo
  46bar
  47foobar
  48barfoo
  49foo
  50bar
  51foobar
  52barfoo
  53foo
  54bar
  55foobar
  56barfoo
  57foo
  58bar
  59foobar
  60barfoo
  61foo
  62bar
  63foobar
  64barfoo
  65foo
  66bar
  67foobar
  68barfoo
  69foo
  70bar
  71foobar
  72barfoo
  73foo
  74bar
  75foobar
  76barfoo
  77foo
  78bar
  79foobar
  80barfoo
  81foo
  82bar
  83foobar
  84barfoo
  85foo
  86bar
  87foobar
  88barfoo
  89foo
  90bar
  91foobar
  92barfoo
  93foo
  94bar
  95foobar
  96barfoo
  -----END RSA PRIVATE KEY-----
`))
			require.Error(t, err)
			assert.Nil(t, f)
			assert.Equal(t, "key-value delimiter not found: 1foo\n", err.Error())
		})

		t.Run("allow unparsable sections", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{
				AllowPythonMultilineValues: false,
				Insensitive:                true,
				UnparseableSections:        []string{"core_lesson", "comments"},
			}, []byte(`
Lesson_Location = 87
Lesson_Status = C
Score = 3
Time = 00:02:30

[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[COMMENTS]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Equal(t, "3", f.Section("").Key("score").String())
			assert.Empty(t, f.Section("").Body())
			assert.Equal(t, `my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`,
				f.Section("core_lesson").Body(),
			)
			assert.Equal(t, `<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`, f.Section("comments").Body())

			t.Run("write out", func(t *testing.T) {
				var buf bytes.Buffer
				_, err := f.WriteTo(&buf)
				require.NoError(t, err)
				assert.Equal(t, `lesson_location = 87
lesson_status   = C
score           = 3
time            = 00:02:30

[core_lesson]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[comments]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>
`,
					buf.String(),
				)
			})

			t.Run("inverse case", func(t *testing.T) {
				_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`))
				require.Error(t, err)
			})
		})

		t.Run("and false `SpaceBeforeInlineComment`", func(t *testing.T) {
			t.Run("cannot parse INI files containing `#` or `;` in value", func(t *testing.T) {
				f, err := ini.LoadSources(
					ini.LoadOptions{AllowPythonMultilineValues: true, SpaceBeforeInlineComment: false},
					[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
				require.NoError(t, err)
				require.NotNil(t, f)
				sec := f.Section("author")
				nameValue := sec.Key("NAME").String()
				githubValue := sec.Key("GITHUB").String()
				assert.Equal(t, "U", nameValue)
				assert.Equal(t, "U", githubValue)
			})
		})

		t.Run("and true `SpaceBeforeInlineComment`", func(t *testing.T) {
			t.Run("can parse INI files containing `#` or `;` in value", func(t *testing.T) {
				f, err := ini.LoadSources(
					ini.LoadOptions{AllowPythonMultilineValues: true, SpaceBeforeInlineComment: true},
					[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
				require.NoError(t, err)
				require.NotNil(t, f)
				sec := f.Section("author")
				nameValue := sec.Key("NAME").String()
				githubValue := sec.Key("GITHUB").String()
				assert.Equal(t, "U#n#k#n#w#o#n", nameValue)
				assert.Equal(t, "U;n;k;n;w;o;n", githubValue)
			})
		})
	})

	t.Run("with `ChildSectionDelimiter` ':'", func(t *testing.T) {
		t.Run("get all keys of parent sections", func(t *testing.T) {
			f := ini.Empty(ini.LoadOptions{ChildSectionDelimiter: ":"})
			require.NotNil(t, f)

			k, err := f.Section("package").NewKey("NAME", "ini")
			require.NoError(t, err)
			assert.NotNil(t, k)
			k, err = f.Section("package").NewKey("VERSION", "v1")
			require.NoError(t, err)
			assert.NotNil(t, k)
			k, err = f.Section("package").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
			require.NoError(t, err)
			assert.NotNil(t, k)

			keys := f.Section("package:sub:sub2").ParentKeys()
			names := []string{"NAME", "VERSION", "IMPORT_PATH"}
			assert.Equal(t, len(names), len(keys))
			for i, name := range names {
				assert.Equal(t, name, keys[i].Name())
			}
		})

		t.Run("getting and setting values", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{ChildSectionDelimiter: ":"}, fullConf)
			require.NoError(t, err)
			require.NotNil(t, f)

			t.Run("get parent-keys that are available to the child section", func(t *testing.T) {
				parentKeys := f.Section("package:sub").ParentKeys()
				assert.NotNil(t, parentKeys)
				for _, k := range parentKeys {
					assert.Equal(t, "CLONE_URL", k.Name())
				}
			})

			t.Run("get parent section value", func(t *testing.T) {
				assert.Equal(t, "https://gopkg.in/ini.v1", f.Section("package:sub").Key("CLONE_URL").String())
				assert.Equal(t, "https://gopkg.in/ini.v1", f.Section("package:fake:sub").Key("CLONE_URL").String())
			})
		})

		t.Run("get child sections by parent name", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{ChildSectionDelimiter: ":"}, []byte(`
[node]
[node:biz1]
[node:biz2]
[node.biz3]
[node.bizN]
`))
			require.NoError(t, err)
			require.NotNil(t, f)

			children := f.ChildSections("node")
			names := []string{"node:biz1", "node:biz2"}
			assert.Equal(t, len(names), len(children))
			for i, name := range names {
				assert.Equal(t, name, children[i].Name())
			}
		})
	})

	t.Run("ShortCircuit", func(t *testing.T) {
		t.Run("load the first available configuration, ignore other configuration", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{ShortCircuit: true}, minimalConf, []byte(`key1 = value1`))
			require.NotNil(t, f)
			require.NoError(t, err)
			var buf bytes.Buffer
			_, err = f.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, `[author]
E-MAIL = u@gogs.io

`,
				buf.String(),
			)
		})

		t.Run("return an error when fail to load", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{ShortCircuit: true}, notFoundConf, minimalConf)
			assert.Nil(t, f)
			require.Error(t, err)
		})

		t.Run("used with Loose to ignore errors that the file does not exist", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{ShortCircuit: true, Loose: true}, notFoundConf, minimalConf)
			require.NotNil(t, f)
			require.NoError(t, err)
			var buf bytes.Buffer
			_, err = f.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, `[author]
E-MAIL = u@gogs.io

`,
				buf.String(),
			)
		})

		t.Run("ensure all sources are loaded without ShortCircuit", func(t *testing.T) {
			f, err := ini.LoadSources(ini.LoadOptions{ShortCircuit: false}, minimalConf, []byte(`key1 = value1`))
			require.NotNil(t, f)
			require.NoError(t, err)
			var buf bytes.Buffer
			_, err = f.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, `key1 = value1

[author]
E-MAIL = u@gogs.io

`,
				buf.String(),
			)
		})
	})
}

func Test_KeyValueDelimiters(t *testing.T) {
	t.Run("custom key-value delimiters", func(t *testing.T) {
		f, err := ini.LoadSources(ini.LoadOptions{
			KeyValueDelimiters: "?!",
		}, []byte(`
[section]
key1?value1
key2!value2
`))
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "value1", f.Section("section").Key("key1").String())
		assert.Equal(t, "value2", f.Section("section").Key("key2").String())
	})
}

func Test_PreserveSurroundedQuote(t *testing.T) {
	t.Run("preserve surrounded quote test", func(t *testing.T) {
		f, err := ini.LoadSources(ini.LoadOptions{
			PreserveSurroundedQuote: true,
		}, []byte(`
[section]
key1 = "value1"
key2 = value2
`))
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "\"value1\"", f.Section("section").Key("key1").String())
		assert.Equal(t, "value2", f.Section("section").Key("key2").String())
	})

	t.Run("preserve surrounded quote test inverse test", func(t *testing.T) {
		f, err := ini.LoadSources(ini.LoadOptions{
			PreserveSurroundedQuote: false,
		}, []byte(`
[section]
key1 = "value1"
key2 = value2
`))
		require.NoError(t, err)
		require.NotNil(t, f)

		assert.Equal(t, "value1", f.Section("section").Key("key1").String())
		assert.Equal(t, "value2", f.Section("section").Key("key2").String())
	})
}
