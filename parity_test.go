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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	v1 "gopkg.in/ini.v1"
)

// The parity tests load the same input with the released v1 module and with
// this package and compare everything both stored. v2 changes no parsing
// result unless an option that v1 doesn't have asks for it.

// parityOptions maps bits of flags to v1 load options, so the fuzzer explores
// option combinations along with inputs.
func parityOptions(flags uint32) v1.LoadOptions {
	bit := func(i uint) bool { return flags&(1<<i) != 0 }
	opts := v1.LoadOptions{
		Insensitive:                 bit(0),
		InsensitiveSections:         bit(1),
		InsensitiveKeys:             bit(2),
		IgnoreContinuation:          bit(3),
		IgnoreInlineComment:         bit(4),
		SkipUnrecognizableLines:     bit(5),
		AllowBooleanKeys:            bit(6),
		AllowShadows:                bit(7),
		AllowNestedValues:           bit(8),
		AllowPythonMultilineValues:  bit(9),
		SpaceBeforeInlineComment:    bit(10),
		UnescapeValueDoubleQuotes:   bit(11),
		UnescapeValueCommentSymbols: bit(12),
		PreserveSurroundedQuote:     bit(13),
		AllowNonUniqueSections:      bit(14),
		AllowDuplicateShadowValues:  bit(15),
	}
	if bit(16) {
		opts.UnparseableSections = []string{"raw", "COMMENTS"}
	}
	opts.KeyValueDelimiters = []string{"", "=", ":", "→=", " ", "?!", "=:", ""}[(flags>>17)&7]
	return opts
}

// v2Options returns the v2 equivalent of v1 load options.
func v2Options(o v1.LoadOptions) []LoadOption {
	opts := []LoadOption{
		CaseInsensitive(o.Insensitive),
		CaseInsensitiveSections(o.InsensitiveSections),
		CaseInsensitiveKeys(o.InsensitiveKeys),
		IgnoreContinuation(o.IgnoreContinuation),
		IgnoreInlineComment(o.IgnoreInlineComment),
		SkipUnrecognizableLines(o.SkipUnrecognizableLines),
		AllowBooleanKeys(o.AllowBooleanKeys),
		AllowShadows(o.AllowShadows),
		AllowNestedValues(o.AllowNestedValues),
		AllowPythonMultilineValues(o.AllowPythonMultilineValues),
		SpaceBeforeInlineComment(o.SpaceBeforeInlineComment),
		UnescapeValueDoubleQuotes(o.UnescapeValueDoubleQuotes),
		UnescapeValueCommentSymbols(o.UnescapeValueCommentSymbols),
		PreserveSurroundingQuotes(o.PreserveSurroundedQuote),
		AllowNonUniqueSections(o.AllowNonUniqueSections),
		AllowDuplicateShadowValues(o.AllowDuplicateShadowValues),
	}
	if o.UnparseableSections != nil {
		opts = append(opts, RawSections(o.UnparseableSections...))
	}
	return append(opts, KeyValueDelimiters(o.KeyValueDelimiters))
}

// dumpV1 describes everything the parser stores in a v1 *File, by
// reflection, so the comparison covers state the public API doesn't
// expose, such as empty shadow values. dumpV2 gives the same for v2.
func dumpV1(f any) string {
	v := reflect.ValueOf(f).Elem()
	names, indexes, sections := v.FieldByName("sectionList"), v.FieldByName("sectionIndexes"), v.FieldByName("sections")

	var b strings.Builder
	for i := 0; i < names.Len(); i++ {
		name := names.Index(i)
		s := sections.MapIndex(name).Index(int(indexes.Index(i).Int())).Elem()
		dumpSection(&b, s.FieldByName("name").String(), s.FieldByName("Comment").String(),
			s.FieldByName("isRawSection").Bool(), s.FieldByName("rawBody").String())
		keyList, keys := s.FieldByName("keyList"), s.FieldByName("keys")
		for j := 0; j < keyList.Len(); j++ {
			dumpV1Key(&b, "  ", keys.MapIndex(keyList.Index(j)).Elem())
		}
	}
	return b.String()
}

func dumpV1Key(b *strings.Builder, indent string, k reflect.Value) {
	nested := k.FieldByName("nestedValues")
	vals := make([]string, nested.Len())
	for i := range vals {
		vals[i] = nested.Index(i).String()
	}
	dumpKey(b, indent, k.FieldByName("name").String(), k.FieldByName("value").String(), k.FieldByName("Comment").String(),
		k.FieldByName("isAutoIncrement").Bool(), k.FieldByName("isBooleanType").Bool(), k.FieldByName("isShadow").Bool(), vals)
	shadows := k.FieldByName("shadows")
	for i := 0; i < shadows.Len(); i++ {
		dumpV1Key(b, indent+"  ", shadows.Index(i).Elem())
	}
}

// dumpV2 describes f in the format of dumpV1. v1 stored each shadow as a
// key with only a name and value.
func dumpV2(f *File) string {
	var b strings.Builder
	for _, s := range f.sections {
		dumpSection(&b, s.name, s.comment, s.isRawSection, s.rawBody)
		for _, k := range s.keys {
			dumpKey(&b, "  ", k.name, k.value, k.comment, k.isAutoIncrement, k.isBooleanType, false, k.nestedValues)
			for _, v := range k.shadows {
				dumpKey(&b, "    ", k.name, v, "", false, false, true, nil)
			}
		}
	}
	return b.String()
}

func dumpSection(b *strings.Builder, name, comment string, raw bool, body string) {
	fmt.Fprintf(b, "[%q] comment=%q raw=%v body=%q\n", name, comment, raw, body)
}

func dumpKey(b *strings.Builder, indent, name, value, comment string, auto, boolean, shadow bool, nested []string) {
	fmt.Fprintf(b, "%s%q=%q comment=%q auto=%v bool=%v shadow=%v nested=%d", indent, name, value, comment, auto, boolean, shadow, len(nested))
	for _, v := range nested {
		fmt.Fprintf(b, " %q", v)
	}
	b.WriteString("\n")
}

// loadResult describes the outcome of loading: the File dumped, or the error.
func loadResult[F any](load func() (F, error), dump func(F) string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("panic: %v", r)
		}
	}()
	f, err := load()
	// v2 adds the position to parse errors; compare the error under it.
	var perr *ParseError
	if errors.As(err, &perr) {
		err = perr.Err
	}
	if err != nil {
		return "error: " + err.Error()
	}
	return dump(f)
}

// checkParity loads data twice, as two sources of one File, with v1 and v2.
func checkParity(t *testing.T, data []byte, flags uint32) {
	t.Helper()
	opts := parityOptions(flags)
	// Known difference: v1 looks ahead at most one reader buffer (>= 4 KiB)
	// for Python-style multiline values; v2 has no such limit.
	if opts.AllowPythonMultilineValues && len(data) > 4096 {
		return
	}

	want := loadResult(func() (*v1.File, error) { return v1.LoadSources(opts, data, data) }, func(f *v1.File) string { return dumpV1(f) })
	got := loadResult(func() (*File, error) { return LoadSources([]any{data, data}, v2Options(opts)...) }, dumpV2)
	if want != got {
		t.Fatalf("v1 and v2 differ\ninput: %q\noptions: %+v\nv1:\n%s\nv2:\n%s", data, opts, want, got)
	}
}

// paritySeeds are inputs that exercise every parser feature, including the
// cases from the documentation and from GitHub issues and pull requests.
var paritySeeds = []string{
	"",
	"\xEF\xBB\xBFk = v\n",
	"\xFF\xFEk = v\n",
	"\xFE\xFFk = v\n",
	"\xEF\xBB",
	"k = v",
	"k = v\r\n[s]\r\nx = y\r\n",
	"# c1\n; c2\nkey = v\n# sec comment\n[sec]\n",
	"a = value # c\nb = value;c\nc = x#y\n",
	"[sec1] some words\nk = v\n[sec2] # hash words\n[sec3] ; semi\n[sec] words # c\n",
	"a = `v#1;2`\nb = \"\"\"v#1;2\"\"\"\nc = \"v#1\"\n",
	"a = a#b ;c\nb = x;y # z\nc = p#q\n",
	"a = x\\#y\\;z\n",
	"foo = \"some value\"\nbar = 'some value'\nbaz = \"a\" \"b\"\nq = 'it''s'\n",
	"create_repo=\"created repository <a href=\\\"%s\\\">%s</a>\"\n",
	"[advance]\nADDRESS = \"\"\"404 road,\nNotFound, State, 5000\nEarth\"\"\"\n",
	"[advance]\ntwo_lines = how about \\\n\tcontinuation lines?\nlots_of_lines = 1 \\\n\t2 \\\n\t3 \\\n\t4\n",
	"key1=a\\b\\\nkey2=c\\d\\\nkey3=value",
	"k = `line1\nline2` \\\nmore` ; c\n",
	"k = \"\"\"line1\nline2\"\"\" ; c\n",
	"k = `unterminated\n",
	"k = `unterminated",
	"name=\"\"\"Unknwon",
	"[long]\nlong_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----\n   foo\n   bar\n\tbaz\r\n\f-----END RSA PRIVATE KEY-----\nnext = v\n",
	"[s]\nk =\n  line2\n  line3\n\nnext = v\n",
	"[profile development]\ns3 =\n  max_concurrent_requests = 20\n  \n\tmax_queue_size = 10000\nregion = us-west-2\n",
	"- = a\n-: b\n[other]\n- = x\n-\n  nested\n",
	"[mysqld]\n...\nskip-host-cache\nskip-name-resolve # c\n\"quoted bool\"\n",
	"[remote \"origin\"]\nurl = a\nurl = b\nurl = a\nurl =\nfetch = +refs/heads/*:refs/remotes/origin/*\n",
	"[system]\nfoo = bar\n\n[system]\nbaz = gazonk\n[SYSTEM]\nfoo = 2\n",
	"[COMMENTS]\n<1><L.Slide#2> fuel ; units\n   indented\n# not body\n\n[raw]\nx\n[next]\nk = v\n",
	"[raw]\nbody",
	"GenerationDepth: 13\n\n################\nBiomeGroup(NormalBiomes, 3, 99\n= RainForest\n",
	"\"\"\"\"\"\"name\"\"\"=value\n\"\"\"key\"\"\" = v\n`bk` = v\n\"dq k\" junk = v\n\"\"=1\n=\nname\n\"\"\"name\n\"\"\"name\"\"\"\n",
	"a: 1\nb = 2\nc:d = 3\na ? 1\nk→v\n",
	"[a] ; note ]\nk=v\n[\n",
	"[]\n",
	"\u00a0k\u2003=\u00a0v\u00a0\n\v[s]\v\n",
	"k = %(x)s\nx = ${HOME}\n",
	"k = v\\\n\n",
	"k = \"\"\"\n\"\"\"",
	"k = \"\"\"",
	"k = \"x\"\"\"\n",
	"k = `x` y `z`\n",
	"k = \"quoted ; not comment\"\n",
	"k=\xff\xfe = \xc3\n\t\xff\n",

	// From GitHub issues and pull requests.
	"[background]\nprimary-color      = '#e18477'\nsecondary-color    = \"#000000\"\nhash-in-backticks  = `#e18477`\nsemi-single        = 'foo;bar'\nquoted-then-comment = '#e18477' ; trailing\n", // #393, #394
	"[background]\nurl = 'https://example.test/#fragment' ; trailing comment\nplain = value # actual comment\nk = don't # comment\n",                                                               // #411
	"#===== Start of the file =======\ntest_value=\"qwe#123\" \n#===== End of the file ======== \n",                                                                                                // #71
	"HOST = network=tcp,addr=:6379,password=`redis#$%12322`,db=15\nH2 = a=\"\"\"redis#1\"\"\",b\n",                                                                                                 // #141
	"\n[author]\nNAME = U#n#k#n#w#o#n\nGITHUB = U;n;k;n;w;o;n\nkey=value_with_#_in_it\nkey2=value with # in it\nk3 = a\t#c\nk4 = #abc\nj = ;\n",                                                    // #146, #277
	"\nkey = test value <span style=\"color: %s\\; background: %s\">more text</span>\nk = \"a\\;b\"\n",                                                                                             // #129
	"; Comment before the section\n[comments] ; This is a comment for the section too\n; Comment before key\nkey = \"value\"\nkey2 = \"value2\" ; c\n",                                             // #24
	"key1 = \"value\" ; comment\nkey2 = \"one\", \"two\", \"three\"\n",                                                                                                                             // #25
	"# BEGIN\n[profile test]\nsso_region              = us-east-1\noutput                  = json\n# END\n",                                                                                        // #362
	"Name=\"bruce\"\nAddress=\"\"\nSingle=''\na=\"b=c\"\ntime=2025-01-04T16:25:00+08:00\"\nc = _\"x\"_\nd = \"a\"b\"\ne = '\"x\"'\n",                                                               // #56, #112, #363, #389
	"hello = \"!f() { \\\n  echo \"hello world\"; \\\n};f\"",                                                                                                                                       // #373
	"key = \"a \\\nb\"\n[access_log]\nreg_gsid = ` (\\d{39,41}) `\n",                                                                                                                               // #172, #229
	"KEY = \"\"\"VALUE\"\"\"\n[ANOTHER SECTION]\nKEYS=VALUES\n; \"\"\"\"\"\"\n`A pathological `c\"as\"`e` = v\n`KEY`=FAKE_VALUE ;\"`` = ACTUAL_VALUE\n",                                            // #187
	"[Directories]\nSourceDir=C:\\App\\Source\\\nTargetDir=C:\\App\\Target\\\n[dirs]\ncert   = Y:\\\\_1\\\\\nserial = 12345678\n",                                                                  // #44, #347
	"[control]\naudio_path = \\\nvideo_path = \\\n\n\t\tA = C:\\\n\t\tB = D:\\\n\t\t",                                                                                                              // #243
	"[git]\nsubmodules_disabled = \n  .*secret.*\n  fix.*\n",                                                                                                                                       // #270
	"value1 = some text here\n\tsome more text here\n\t\n\tthere is an empty line above and below\n\t\n\nvalue2 = there is an empty line above\n    that is not indented\n",                        // #210, #299
	"[long]\nnotes = first\n  second\n\n  third\nnext = x\n",                                                                                                                                       // #205
	"value1 = some text here\n\tsome more text here 2",                                                                                                                                             // #309
	"k = a\r\n  b\r\nn = 1\r\nk = \"a\"\n  b\n",                                                                                                                                                    // Python CRLF
	"[profile testing]\nregion = us-west-2\ns3 =\n  max_concurrent_requests=10\n  max_queue_size=1000\n[section2]\nkey2 =\n[section3]\n  key3 = value3\n",                                          // #131, #306
	"[FILTER]\n  name            record_modifier\n  whitelist_key   _CMDLINE\n  whitelist_key   _EXE\n  record          filename systemd\n",                                                        // #367
	"keee:y=value\n\"keee:y\"=value\n\n[section]\nkey1?value1\nkey2!value2\n",                                                                                                                      // #166
	"[global]\nvfs objects      = catia fruit streams_xattr\nfruit:metadata = stream\n`fruit:metadata` = stream\n",                                                                                 // #355
	"-key=a\n_key=b\n.key=c\n:key=a\n",                                                                                        // #183
	"[third_section]\nphpversion[] = \"5.0\"\nphpversion[] = \"5.3\"\n",                                                       // #315
	"[我]\n姓名 = 张三\n[1234]\n我=abc\n有支付宝语句 = a\\n支付宝:{支付宝}\\n[红包]\n",                                                            // #165, #244
	"[server]\ntimeout = {\n    \"timeout\":1000\n}\n",                                                                        // #344
	"key1=hello\nkey2\n#key3\nkey4\nkey5",                                                                                     // #53, #78
	"localhost              ansible_connection=local\n\n[webservers]\nother1\nother2\nflag\nflag\n",                           // #159
	"[test]\n        Key1=Key1Value1\n        Key1=Key1Value2\n        Key1=Key1Value1\n\n[Network]\nDNS=\nDNS=\"1.2.3.4\"\n", // #283, #284, #294
	"[features]\n\t-: Support read/write comments\n\t-: Support auto-increment of key names\n[Agents]\n- = a\n- = b",          // #329, #240
	"NAME = Unknwon\n[DEFAULT]\nk1 = v1\n[default]\nk2 = v2\n",                                                                // #8, #37, #125, #198
	"[profile foo]\nMfa_Serial=xxxxxxx\n\n[profile FOO]\nmfa_serial=yyyyyyy\n",                                                // #238
	"[foo]bar]\nk = v\n[sec] ; see [1]\n[my section]\n[ padded ]\n",                                                           // #46, #212
	"[Interface]\nPrivateKey = a\n# Peer 1\n[Peer]\nPublicKey = b\nAllowedIPs = c\n# Peer 2\n[Peer]\nPublicKey = d\n",         // #190, #289
	"[Core]\n  Lesson_Location = 87\n\n[CORE_LESSON]\nmy lesson state data – 1111\n[COMMENTS]\n<1><L.Slide#2> units <e.1>\n",  // #72
	"[raw]", // #291
	"[RAW]\n  ; not a comment\n\n  indented line\n[next]\n",
	"[Version]\nSignature=\"$Windows NT$\"\n[Capture.2860.AddReg]\nHKR,,CLSID,,%ProxyVCap.CLSID%\nHKLM,\"%UNINSTKEY%\\%DRVNAME%\",\"caption\",0x0\n", // #173, #237
	"\xef\xbb\xbf\xef\xbb\xbf[author]\nE-MAIL = example@email.com",                                                                                   // #390
	"\xff\xfe[\x00c\x00o\x00n\x00f\x00]\x00\n\x00k\x00=\x00v\x00\n\x00",                                                                              // #66, #212
	"; c1\r\n; c2\r\n[sec] ; sc\r\nkey = value\r\nq = \"quoted\"\r\nbt = `a\r\nb`\r\ncont = a \\\r\n b\r\nflag\r\na = 1\rb = 2\r",                    // #216
	"k =\u00a0v\u00a0\n\u00a0[s]\n",
}

func FuzzParseParity(f *testing.F) {
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
		checkParity(t, data, flags)
	})
}

// TestParseParityRandom compares the parsers on documents assembled from
// fragments that sit on the boundaries between parser features.
func TestParseParityRandom(t *testing.T) {
	indents := []string{"", "", "", " ", "\t", "\f", "\v", "\u00a0"}
	fragments := []string{
		"[s]", "[raw]", "[COMMENTS]", "[S]", "[a] ; c ]", "[a] x", "[", "[]",
		"k = v", "K = v2", "k=", "k :", "-", "- = x", "k : v", "\"q k\" = v", "`b k` = v", "\"\"\"tk\"\"\" = v",
		"k = \"\"\"multi", "end\"\"\" ; c", "end\"\"\" \\", "k = `multi", "end` # c", "end`",
		"k = a \\", "k = \\", "cont \\", "a long continued line that is well over fifty bytes in total \\", "k = 'x'", "k = \"x\"", "k = \"a\\\"b\"", "k = \"a\" ; c",
		"k = x\\;y\\#z", "k = a #c", "k = a ;c", "k = a#b ;c", "k = a ;b #c",
		"# comment", "; comment", "#", "bool", "\"bool q\"", "bool ; c", "nested = 1",
		"k = `a` b `c`", "\"\"\"\"\"\"name\"\"\"=v", "k→v", "k = %(x)s", "k ? v", "= v", "k = '",
	}
	endings := []string{"\n", "\n", "\n", "\r\n", ""}

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 50000; i++ {
		var b strings.Builder
		for n := rng.Intn(10) + 1; n > 0; n-- {
			b.WriteString(indents[rng.Intn(len(indents))])
			b.WriteString(fragments[rng.Intn(len(fragments))])
			if rng.Intn(4) == 0 {
				b.WriteString(fragments[rng.Intn(len(fragments))])
			}
			b.WriteString(endings[rng.Intn(len(endings))])
		}
		checkParity(t, []byte(b.String()), rng.Uint32())
	}
	for _, in := range paritySeeds {
		for i := 0; i < 500; i++ {
			checkParity(t, []byte(in), rng.Uint32())
		}
	}
}

// v1 reads a Python-style multiline value only as far as one reader buffer
// looks ahead (#210); the lexer reads it to the end however long it is.
func TestParsePythonMultilineLongerThanBuffer(t *testing.T) {
	var b strings.Builder
	b.WriteString("[s]\nkey = first\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "  line %03d xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", i)
	}
	b.WriteString("next = ok\n")

	f, err := Load([]byte(b.String()), AllowPythonMultilineValues(true))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(f.Section("s").Key("key").Value(), "\n"); got != 200 {
		t.Fatalf("value has %d continuation lines, want 200", got)
	}
	if got := f.Section("s").Key("next").Value(); got != "ok" {
		t.Fatalf("next = %q, want ok", got)
	}
}
