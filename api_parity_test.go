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
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "gopkg.in/ini.v1"
)

func init() {
	// v1 writes "\r\n" on Windows; v2 keeps "\n" in this package's tests.
	v1.LineBreak = "\n"
}

// The API parity tests call the public API of v1 and v2 on the same files
// and values. Where v1 returns without panicking, v2 must return the same,
// unless the difference is one of the deliberate changes in the README.

type apiSub struct {
	Name  string
	Port  int
	Hosts []string
}

// APIEmbed is exported so that MapTo can set the fields it promotes.
type APIEmbed struct{ Shared string }

// apiCfg has a field of every kind v1 maps.
type apiCfg struct {
	APIEmbed
	Str    string
	Lower  string `ini:"lower"`
	Bool   bool
	Int    int
	Int64  int64
	Uint   uint
	Uint64 uint64
	Float  float64
	Dur    time.Duration
	Time   time.Time
	PStr   *string
	PInt   *int
	PBool  *bool
	PFloat *float64
	Strs   []string
	Ints   []int `delim:"|"`
	Int64s []int64
	Uints  []uint
	Floats []float64
	Bools  []bool
	Times  []time.Time
	Shadow []string `ini:"shadow,,allowshadow"`
	Omit   string   `ini:"omit,omitempty"`
	Skip   string   `ini:"-"`
	Sub    apiSub   `ini:"sub"`
	PSub   *apiSub  `ini:"psub"`
}

type apiPeers struct {
	Peers []apiSub `ini:"peer,,,nonunique"`
}

// newAPICfg returns an apiCfg with every field set, so the tests see which
// fields mapping changes and which it leaves.
func newAPICfg() *apiCfg {
	s, i, b, f := "p", 4, true, 2.5
	return &apiCfg{
		APIEmbed: APIEmbed{"shared"}, Str: "def", Lower: "low", Bool: true, Int: 3, Int64: -3, Uint: 3, Uint64: 3,
		Float: 1.5, Dur: 7 * time.Second, Time: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
		PStr: &s, PInt: &i, PBool: &b, PFloat: &f,
		Strs: []string{"x"}, Ints: []int{9}, Int64s: []int64{9}, Uints: []uint{9}, Floats: []float64{9}, Bools: []bool{true},
		Times: []time.Time{time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)}, Shadow: []string{"s"}, Omit: "o", Skip: "skip",
		Sub: apiSub{"n", 1, []string{"h"}}, PSub: &apiSub{"pn", 2, []string{"ph"}},
	}
}

// try calls fn and reports whether it panicked.
func try(fn func()) (panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	fn()
	return false
}

func errStr(err error) string {
	if err == nil {
		return "<nil>"
	}
	return "error"
}

type apiChecker struct {
	t     *testing.T
	input []byte
	flags uint32
}

func (c *apiChecker) same(what string, want, got any) {
	c.t.Helper()
	if !reflect.DeepEqual(want, got) {
		c.t.Fatalf("v1 and v2 differ in %s\ninput: %q\nflags: %#x\nv1: %#v\nv2: %#v", what, c.input, c.flags, want, got)
	}
}

// checkAPIParity loads data with v1 and v2 and compares what the public API
// returns for it.
func checkAPIParity(t *testing.T, data []byte, flags uint32) {
	t.Helper()
	opts := parityOptions(flags)
	if opts.AllowPythonMultilineValues && len(data) > 4096 {
		return
	}
	c := &apiChecker{t: t, input: data, flags: flags}

	var f1 *v1.File
	var err1 error
	if try(func() { f1, err1 = v1.LoadSources(opts, data) }) || err1 != nil {
		return // The parse parity tests cover errors.
	}
	f2, err := LoadSources([]any{data}, v2Options(opts)...)
	c.same("load error", "<nil>", errStr(err))

	expand := v1ExpandsSafely(f1)

	c.same("SectionNames", f1.SectionStrings(), f2.SectionNames())
	secs1, secs2 := f1.Sections(), f2.Sections()
	for i, s1 := range secs1 {
		s2, sname := secs2[i], s1.Name()
		c.same(sname+" KeyNames", s1.KeyStrings(), s2.KeyNames())
		c.same(sname+" KeyValues", s1.KeysHash(), s2.KeyValues())
		c.same(sname+" Body", s1.Body(), s2.Body())
		c.same(sname+" Comment", s1.Comment, s2.Comment())
		c.same(sname+" ChildSections", len(s1.ChildSections()), len(s2.ChildSections()))
		c.same(sname+" ParentKeys", len(s1.ParentKeys()), len(s2.ParentKeys()))
		for _, kname := range s1.KeyStrings() {
			c.checkKey(sname+"."+kname, s1.Key(kname), s2.Key(kname), expand)
		}
	}
	c.checkWrite(opts, f1, f2)
	if expand {
		c.checkMapping(f1, f2)
	}
}

func (c *apiChecker) checkKey(name string, k1 *v1.Key, k2 *Key, expand bool) {
	c.t.Helper()
	c.same(name+" Value", k1.Value(), k2.Value())
	c.same(name+" Values", k1.ValueWithShadows(), k2.Values())
	c.same(name+" NestedValues", k1.NestedValues(), k2.NestedValues())
	c.same(name+" Comment", k1.Comment, k2.Comment())
	if !expand {
		return
	}
	pair := func(what string, v1v any, e1 error, v2v any, e2 error) {
		c.t.Helper()
		c.same(name+" "+what+" error", errStr(e1), errStr(e2))
		if e1 == nil {
			c.same(name+" "+what, v1v, v2v)
		}
	}
	c.same(name+" String", k1.String(), k2.String())
	b1, e1 := k1.Bool()
	b2, e2 := k2.Bool()
	pair("Bool", b1, e1, b2, e2)
	i1, e1 := k1.Int()
	i2, e2 := k2.Int()
	pair("Int", i1, e1, i2, e2)
	j1, e1 := k1.Int64()
	j2, e2 := k2.Int64()
	pair("Int64", j1, e1, j2, e2)
	u1, e1 := k1.Uint()
	u2, e2 := k2.Uint()
	pair("Uint", u1, e1, u2, e2)
	w1, e1 := k1.Uint64()
	w2, e2 := k2.Uint64()
	pair("Uint64", w1, e1, w2, e2)
	f1, e1 := k1.Float64()
	f2, e2 := k2.Float64()
	if e1 == nil && f1 != f1 { // NaN
		f1, f2 = 0, 0
	}
	pair("Float64", f1, e1, f2, e2)
	d1, e1 := k1.Duration()
	d2, e2 := k2.Duration()
	pair("Duration", d1, e1, d2, e2)
	t1, e1 := k1.Time()
	t2, e2 := k2.Time()
	pair("Time", t1, e1, t2, e2)
	t1, e1 = k1.TimeFormat(time.DateOnly)
	t2, e2 = k2.TimeFormat(time.DateOnly)
	pair("TimeFormat", t1, e1, t2, e2)

	for _, delim := range []string{",", "|", " "} {
		d := fmt.Sprintf("(%q)", delim)
		c.same(name+" Strings"+d, k1.Strings(delim), k2.Strings(delim))
		c.same(name+" StringsWithShadows"+d, k1.StringsWithShadows(delim), k2.StringsWithShadows(delim))
		c.same(name+" Bools"+d, k1.Bools(delim), List[bool](k2, delim))
		c.same(name+" Ints"+d, outOfRangeToZero(k1, delim, k1.Ints(delim)), List[int](k2, delim))
		c.same(name+" Int64s"+d, outOfRangeToZero(k1, delim, k1.Int64s(delim)), List[int64](k2, delim))
		c.same(name+" Uints"+d, outOfRangeToZero(k1, delim, k1.Uints(delim)), List[uint](k2, delim))
		c.same(name+" Uint64s"+d, outOfRangeToZero(k1, delim, k1.Uint64s(delim)), List[uint64](k2, delim))
		c.same(name+" Float64s"+d, nanToZero(outOfRangeToZero(k1, delim, k1.Float64s(delim))), nanToZero(List[float64](k2, delim)))
		c.same(name+" Times"+d, k1.Times(delim), List[time.Time](k2, delim))
		c.same(name+" TimesFormat"+d, k1.TimesFormat(time.DateOnly, delim), k2.TimesFormat(time.DateOnly, delim))
		c.same(name+" ValidBools"+d, k1.ValidBools(delim), ValidList[bool](k2, delim))
		c.same(name+" ValidInts"+d, k1.ValidInts(delim), ValidList[int](k2, delim))
		c.same(name+" ValidInt64s"+d, k1.ValidInt64s(delim), ValidList[int64](k2, delim))
		c.same(name+" ValidUints"+d, k1.ValidUints(delim), ValidList[uint](k2, delim))
		c.same(name+" ValidUint64s"+d, k1.ValidUint64s(delim), ValidList[uint64](k2, delim))
		c.same(name+" ValidFloat64s"+d, nanToZero(k1.ValidFloat64s(delim)), nanToZero(ValidList[float64](k2, delim)))
		c.same(name+" ValidTimes"+d, k1.ValidTimes(delim), ValidList[time.Time](k2, delim))
		c.same(name+" ValidTimesFormat"+d, k1.ValidTimesFormat(time.DateOnly, delim), k2.ValidTimesFormat(time.DateOnly, delim))
		strict := func(what string, a any, e1 error, b any, e2 error) {
			c.t.Helper()
			c.same(name+" "+what+d+" error", errStr(e1), errStr(e2))
			if e1 == nil {
				c.same(name+" "+what+d, a, b)
			}
		}
		sb1, e1 := k1.StrictBools(delim)
		sb2, e2 := StrictList[bool](k2, delim)
		strict("StrictBools", sb1, e1, sb2, e2)
		si1, e1 := k1.StrictInts(delim)
		si2, e2 := StrictList[int](k2, delim)
		strict("StrictInts", si1, e1, si2, e2)
		sj1, e1 := k1.StrictInt64s(delim)
		sj2, e2 := StrictList[int64](k2, delim)
		strict("StrictInt64s", sj1, e1, sj2, e2)
		su1, e1 := k1.StrictUints(delim)
		su2, e2 := StrictList[uint](k2, delim)
		strict("StrictUints", su1, e1, su2, e2)
		sw1, e1 := k1.StrictUint64s(delim)
		sw2, e2 := StrictList[uint64](k2, delim)
		strict("StrictUint64s", sw1, e1, sw2, e2)
		sf1, e1 := k1.StrictFloat64s(delim)
		sf2, e2 := StrictList[float64](k2, delim)
		strict("StrictFloat64s", nanToZero(sf1), e1, nanToZero(sf2), e2)
		st1, e1 := k1.StrictTimes(delim)
		st2, e2 := StrictList[time.Time](k2, delim)
		strict("StrictTimes", st1, e1, st2, e2)
		sf3, e1 := k1.StrictTimesFormat(time.DateOnly, delim)
		sf4, e2 := k2.StrictTimesFormat(time.DateOnly, delim)
		strict("StrictTimesFormat", sf3, e1, sf4, e2)
	}

	day := func(d int) time.Time { return time.Date(2006, 1, d, 0, 0, 0, 0, time.UTC) }
	c.same(name+" In", k1.In("d", []string{"a", "b", ""}), In(k2, "d", []string{"a", "b", ""}))
	c.same(name+" InTimeFormat", k1.InTimeFormat(time.DateOnly, day(9), []time.Time{day(2)}), k2.InTimeFormat(time.DateOnly, day(9), []time.Time{day(2)}))
	c.same(name+" RangeTimeFormat", k1.RangeTimeFormat(time.DateOnly, day(9), day(1), day(3)), k2.RangeTimeFormat(time.DateOnly, day(9), day(1), day(3)))
	if _, err := k1.Int64(); err == nil {
		c.same(name+" InInt", k1.InInt(7, []int{0, 1, 5}), In(k2, 7, []int{0, 1, 5}))
		c.same(name+" InInt64", k1.InInt64(7, []int64{0, 1, 5}), In(k2, int64(7), []int64{0, 1, 5}))
		c.same(name+" RangeInt", k1.RangeInt(7, -1, 3), Range(k2, 7, -1, 3))
		c.same(name+" RangeInt64", k1.RangeInt64(7, -1, 3), Range(k2, int64(7), -1, 3))
	}
	if _, err := k1.Uint64(); err == nil {
		c.same(name+" InUint", k1.InUint(7, []uint{0, 1, 5}), In(k2, uint(7), []uint{0, 1, 5}))
		c.same(name+" InUint64", k1.InUint64(7, []uint64{0, 1, 5}), In(k2, uint64(7), []uint64{0, 1, 5}))
	}
	if v, err := k1.Float64(); err == nil && v == v {
		c.same(name+" InFloat64", k1.InFloat64(7, []float64{0, 1.5}), In(k2, 7.0, []float64{0, 1.5}))
		c.same(name+" RangeFloat64", k1.RangeFloat64(7, -1, 3), Range(k2, 7.0, -1, 3))
	}
	if _, err := k1.Time(); err == nil {
		c.same(name+" InTime", k1.InTime(day(9), []time.Time{day(2)}), In(k2, day(9), []time.Time{day(2)}))
	}

	// v1's Must* stored the default in the key when the value didn't parse;
	// v2's only read. Put v1's value back after each call so both see the
	// same value, then check that v2 left the key as it was.
	v1Value := k1.Value()
	must := func(what string, v1, v2 any) {
		c.same(name+" "+what, v1, v2)
		k1.SetValue(v1Value)
	}
	must("MustString", k1.MustString("d"), k2.MustString("d"))
	must("MustBool", k1.MustBool(true), k2.MustBool(true))
	must("MustInt", k1.MustInt(7), k2.MustInt(7))
	must("MustInt64", k1.MustInt64(7), k2.MustInt64(7))
	must("MustUint", k1.MustUint(7), k2.MustUint(7))
	must("MustUint64", k1.MustUint64(7), k2.MustUint64(7))
	if v, err := k1.Float64(); err != nil || v == v {
		must("MustFloat64", k1.MustFloat64(7), k2.MustFloat64(7))
	}
	must("MustDuration", k1.MustDuration(7), k2.MustDuration(7))
	must("MustTime", k1.MustTime(day(9)), k2.MustTime(day(9)))
	c.same(name+" String after Must", k1.String(), k2.String())
}

// outOfRangeToZero applies a deliberate change to v1's lenient list
// result: v1 kept the clamped value strconv returns for an out-of-range
// element (MaxInt64, +Inf), v2 gives the zero value as for any element that
// doesn't parse.
func outOfRangeToZero[T int | int64 | uint | uint64 | float64](k1 *v1.Key, delim string, vals []T) []T {
	for i, s := range k1.Strings(delim) {
		var err error
		var zero T
		switch any(zero).(type) {
		case int, int64:
			_, err = strconv.ParseInt(s, 0, 64)
		case uint, uint64:
			_, err = strconv.ParseUint(s, 0, 64)
		case float64:
			_, err = strconv.ParseFloat(s, 64)
		}
		if errors.Is(err, strconv.ErrRange) && i < len(vals) {
			vals[i] = zero
		}
	}
	return vals
}

func nanToZero(vals []float64) []float64 {
	for i, v := range vals {
		if v != v {
			vals[i] = 0
		}
	}
	return vals
}

var refPattern = regexp.MustCompile(`%\(([^)]*)\)s`)

// v1ExpandsSafely reports whether v1 can expand every %(name)s reference
// in f. v1 follows references without a limit, so a cycle overflows the
// stack and a chain of doubling values runs out of memory. A reference may
// resolve in the key's section, a parent section or the default section,
// so this treats a reference to a name as one to every key of that name.
func v1ExpandsSafely(f *v1.File) bool {
	refs := map[string][]string{}
	n := 0
	for _, sec := range f.Sections() {
		for _, k := range sec.Keys() {
			for _, v := range k.ValueWithShadows() {
				for _, m := range refPattern.FindAllStringSubmatch(v, -1) {
					from, to := strings.ToLower(k.Name()), strings.ToLower(m[1])
					refs[from] = append(refs[from], to)
					n++
				}
			}
		}
	}
	if n > 16 {
		return false
	}
	// No cycles: every name finishes its depth-first walk.
	state := map[string]int{} // 1 visiting, 2 done
	var visit func(string) bool
	visit = func(name string) bool {
		switch state[name] {
		case 1:
			return false
		case 2:
			return true
		}
		state[name] = 1
		for _, to := range refs[name] {
			if !visit(to) {
				return false
			}
		}
		state[name] = 2
		return true
	}
	for name := range refs {
		if !visit(name) {
			return false
		}
	}
	return true
}

// v1WriteReadsBack reports whether v1's output for f1 loads with v1 to the
// same contents. Where it does, v2 must write the same bytes; where it
// doesn't, v2 deliberately writes something else that reads back.
func v1WriteReadsBack(opts v1.LoadOptions, f1 *v1.File, out []byte) bool {
	g, err := v1.LoadSources(opts, out)
	return err == nil && dumpV1(g) == dumpV1(f1)
}

func (c *apiChecker) checkWrite(opts v1.LoadOptions, f1 *v1.File, f2 *File) {
	c.t.Helper()
	var b1, b2 bytes.Buffer
	if try(func() { _, _ = f1.WriteTo(&b1) }) || !v1WriteReadsBack(opts, f1, b1.Bytes()) {
		return
	}
	_, err := f2.Write(&b2)
	c.same("Write error", "<nil>", errStr(err))
	c.same("Write", b1.String(), b2.String())
}

// checkMapping maps the default section and the peer sections with both
// versions, then writes the mapped struct back with ReflectFrom.
func (c *apiChecker) checkMapping(f1 *v1.File, f2 *File) {
	c.t.Helper()
	for _, strict := range []bool{false, true} {
		m1, m2 := newAPICfg(), newAPICfg()
		var e1 error
		if try(func() {
			if strict {
				e1 = f1.StrictMapTo(m1)
			} else {
				e1 = f1.MapTo(m1)
			}
		}) {
			continue
		}
		var e2 error
		if strict {
			e2 = f2.StrictMapTo(m2)
		} else {
			e2 = f2.MapTo(m2)
		}
		what := fmt.Sprintf("MapTo (strict %v)", strict)
		if strict && e1 == nil && e2 != nil && hasInvalidSliceElem(f2) {
			// Deliberate: StrictMapTo reports invalid slice elements, which
			// v1 left as zero values.
			continue
		}
		c.same(what+" error", errStr(e1), errStr(e2))
		if m2.Dur <= 0 && m1.Dur > 0 {
			if d, err := f2.Section("").Key("Dur").Duration(); err == nil && d == m2.Dur {
				// Deliberate: v2 applies zero and negative durations, which
				// v1 skipped.
				m1.Dur = m2.Dur
			}
		}
		if !strict {
			// Deliberate: an out-of-range list element maps to zero, as in
			// ini.List.
			sec := f1.Section("")
			if k, err := sec.GetKey("Ints"); err == nil && len(m1.Ints) == len(m2.Ints) {
				m1.Ints = outOfRangeToZero(k, "|", m1.Ints)
			}
			if k, err := sec.GetKey("Int64s"); err == nil && len(m1.Int64s) == len(m2.Int64s) {
				m1.Int64s = outOfRangeToZero(k, ",", m1.Int64s)
			}
			if k, err := sec.GetKey("Uints"); err == nil && len(m1.Uints) == len(m2.Uints) {
				m1.Uints = outOfRangeToZero(k, ",", m1.Uints)
			}
			if k, err := sec.GetKey("Floats"); err == nil && len(m1.Floats) == len(m2.Floats) {
				m1.Floats = outOfRangeToZero(k, ",", m1.Floats)
			}
		}
		// NaN never equals itself.
		m1.Floats, m2.Floats = nanToZero(m1.Floats), nanToZero(m2.Floats)
		for _, m := range []*apiCfg{m1, m2} {
			if m.Float != m.Float {
				m.Float = 0
			}
			if m.PFloat != nil && *m.PFloat != *m.PFloat {
				*m.PFloat = 0
			}
		}
		if e1 == nil {
			c.same(what, m1, m2)
		}
		if !strict && e1 == nil {
			c.checkReflect(m1)
		}

		p1, p2 := &apiPeers{}, &apiPeers{}
		if try(func() { e1 = f1.MapTo(p1) }) || e1 != nil {
			continue
		}
		c.same("MapTo peers error", "<nil>", errStr(f2.MapTo(p2)))
		c.same("MapTo peers", p1, p2)
	}
}

// hasInvalidSliceElem reports whether a typed slice field of apiCfg has an
// element that doesn't parse.
func hasInvalidSliceElem(f *File) bool {
	sec, err := f.GetSection("")
	if err != nil {
		return false
	}
	invalid := func(name string, list func(k *Key) error) bool {
		k, err := sec.GetKey(name)
		return err == nil && list(k) != nil
	}
	return invalid("Ints", func(k *Key) error { _, err := StrictList[int](k, "|"); return err }) ||
		invalid("Int64s", func(k *Key) error { _, err := StrictList[int64](k, ","); return err }) ||
		invalid("Uints", func(k *Key) error { _, err := StrictList[uint](k, ","); return err }) ||
		invalid("Floats", func(k *Key) error { _, err := StrictList[float64](k, ","); return err }) ||
		invalid("Bools", func(k *Key) error { _, err := StrictList[bool](k, ","); return err }) ||
		invalid("Times", func(k *Key) error { _, err := StrictList[time.Time](k, ","); return err })
}

// checkReflect writes v with ReflectFrom from both versions. Where v1's
// output maps back to v, v2 must write the same bytes.
func (c *apiChecker) checkReflect(v *apiCfg) {
	c.t.Helper()
	f1 := v1.Empty()
	var b1 bytes.Buffer
	var err error
	if try(func() { err = f1.ReflectFrom(v) }) || err != nil {
		return
	}
	if try(func() { _, err = f1.WriteTo(&b1) }) || err != nil {
		return
	}
	back := newAPICfg()
	g, err := v1.Load(b1.Bytes())
	if err != nil || try(func() { err = g.MapTo(back) }) || err != nil || !reflect.DeepEqual(back, v) {
		return
	}
	f2 := Empty()
	c.same("ReflectFrom error", "<nil>", errStr(f2.ReflectFrom(v)))
	var b2 bytes.Buffer
	_, err = f2.Write(&b2)
	c.same("ReflectFrom Write error", "<nil>", errStr(err))
	c.same("ReflectFrom Write", b1.String(), b2.String())
}

// apiValues are values on the boundaries of the accessors and struct
// mapping.
var apiValues = []string{
	"", "0", "-0", "1", "-1", "+5", "007", "0x1F", "0o17", "0b101", "1_000", "1e3", "1.5", "-2.5", ".5", "5.",
	"9223372036854775807", "9223372036854775808", "-9223372036854775808", "-9223372036854775809",
	"18446744073709551615", "18446744073709551616", "4294967296", "1e400", "-1e400", "NaN", "Inf", "-Inf", "0x1p-2",
	"true", "TRUE", "True", "yes", "on", "OFF", "y", "n", "t", "F", "maybe", "  1  ",
	"1s", "-5s", "0s", "1h2m3.5s", "5", "1.5h", "300ms", "1us", "1µs", "-0s",
	"2006-01-02T15:04:05Z", "2006-01-02T15:04:05.123456789+08:00", "2006-01-02", "2006-01-02T15:04:05", "2006-01-02 15:04:05Z",
	"a,b", "a, b ,c", "a\\,b", "a\\\\,b", "a\\b", "a,", ",a", ",", ",,", "1|2|x", "1|99999999999999999999|-3", "1, 2, 3",
	"1s,2s", "true,no,x", "2006-01-02,2006-01-03", "2006-01-02T15:04:05Z,bad", "1.5,NaN,2", "a b", "x y z",
	"`quoted`", "\"q\"", "'s'", "\"\"\"tq\"\"\"", "v ; c", "v # c", "a\\", "\\", "é,ü", "\t",
}

var apiKeys = []string{
	"Shared", "Str", "lower", "Bool", "Int", "Int64", "Uint", "Uint64", "Float", "Dur", "Time",
	"PStr", "PInt", "PBool", "PFloat", "Strs", "Ints", "Int64s", "Uints", "Floats", "Bools", "Times",
	"shadow", "shadow", "omit", "Skip", "Name", "Port", "Hosts",
}

// apiDoc returns a document whose keys are apiCfg's, with random values.
func apiDoc(rng *rand.Rand) string {
	var b strings.Builder
	order := map[string]int{}
	for i, k := range apiKeys {
		order[k] = i
	}
	value := func(key string) string {
		v := apiValues[rng.Intn(len(apiValues))]
		// A reference only to a key earlier in apiKeys, so that there are
		// no cycles and chains of references form.
		if i := order[key]; i > 0 && rng.Intn(6) == 0 {
			ref := apiKeys[rng.Intn(i)]
			if rng.Intn(4) == 0 {
				ref = "missing"
			}
			v = []string{"", v, "x,"}[rng.Intn(3)] + "%(" + ref + ")s" + []string{"", ",y", "|2"}[rng.Intn(3)]
		}
		return v
	}
	keys := func(names []string) {
		for _, k := range names {
			for n := rng.Intn(3); n > 0; n-- {
				fmt.Fprintf(&b, "%s = %s\n", k, value(k))
			}
		}
	}
	keys(apiKeys)
	for _, sec := range []string{"sub", "psub", "peer", "peer", "sub.child"} {
		if rng.Intn(2) == 0 {
			fmt.Fprintf(&b, "[%s]\n", sec)
			keys([]string{"Name", "Port", "Hosts", "Str"})
		}
	}
	return b.String()
}

// apiSeeds are documents for the fuzz test and the random test.
func apiSeeds() []string {
	rng := rand.New(rand.NewSource(2))
	seeds := []string{
		"Str = a\nlower = %(Str)s-x\nInt = [%(lower)s]\n[sub]\nName = %(Int)s,%(Str)s\nPort = %(Name)s\n",
		"Str = ${HOME}\nInts = 1|2|3\nDur = 1m\nTime = 2006-01-02T15:04:05Z\n",
	}
	for i := 0; i < 50; i++ {
		seeds = append(seeds, apiDoc(rng))
	}
	return seeds
}

// TestAPIParityShapes passes values of every shape to the struct API.
func TestAPIParityShapes(t *testing.T) {
	data := []byte("Str = a\nInt = 2\nshadow = x\nshadow = y\n[sub]\nName = n\nPort = 3\n[peer]\nName = p1\n[peer]\nName = p2\n")
	shapes := map[string]func() any{
		"*T":          func() any { return newAPICfg() },
		"**T":         func() any { p := newAPICfg(); return &p },
		"**T nil":     func() any { var p *apiCfg; return &p },
		"***T":        func() any { p := newAPICfg(); pp := &p; return &pp },
		"T":           func() any { return *newAPICfg() },
		"nil":         func() any { return nil },
		"*T nil":      func() any { return (*apiCfg)(nil) },
		"*int":        func() any { return new(int) },
		"*[]*sub":     func() any { return &[]*apiSub{} },
		"[]*sub":      func() any { return []*apiSub{{Name: "a"}, {Name: "b"}} },
		"[]sub":       func() any { return []apiSub{{Name: "a"}} },
		"*sub":        func() any { return &apiSub{Name: "a"} },
		"*peers":      func() any { return &apiPeers{} },
		"*peers full": func() any { return &apiPeers{Peers: []apiSub{{Name: "a"}, {Name: "b"}}} },
		"*map":        func() any { return &map[string]string{} },
	}
	for _, flags := range []uint32{0, 1 << 7, 1 << 14, 1<<7 | 1<<14} {
		opts := parityOptions(flags)
		for name, shape := range shapes {
			for _, call := range []string{"File.MapTo", "File.StrictMapTo", "Section.MapTo", "MapTo", "StrictMapTo", "File.ReflectFrom", "Section.ReflectFrom"} {
				c := &apiChecker{t: t, input: data, flags: flags}
				what := fmt.Sprintf("%s(%s)", call, name)
				f1, err := v1.LoadSources(opts, data)
				if err != nil {
					t.Fatal(err)
				}
				f2, err := LoadSources([]any{data}, v2Options(opts)...)
				if err != nil {
					t.Fatal(err)
				}
				t1, t2 := shape(), shape()
				var e1, e2 error
				if try(func() {
					switch call {
					case "File.MapTo":
						e1 = f1.MapTo(t1)
					case "File.StrictMapTo":
						e1 = f1.StrictMapTo(t1)
					case "Section.MapTo":
						e1 = f1.Section("peer").MapTo(t1)
					case "MapTo":
						e1 = v1.MapTo(t1, data)
					case "StrictMapTo":
						e1 = v1.StrictMapTo(t1, data)
					case "File.ReflectFrom":
						e1 = f1.ReflectFrom(t1)
					case "Section.ReflectFrom":
						e1 = f1.Section("peer").ReflectFrom(t1)
					}
				}) {
					continue
				}
				switch call {
				case "File.MapTo":
					e2 = f2.MapTo(t2)
				case "File.StrictMapTo":
					e2 = f2.StrictMapTo(t2)
				case "Section.MapTo":
					e2 = f2.Section("peer").MapTo(t2)
				case "MapTo":
					e2 = MapTo(t2, data)
				case "StrictMapTo":
					e2 = StrictMapTo(t2, data)
				case "File.ReflectFrom":
					e2 = f2.ReflectFrom(t2)
				case "Section.ReflectFrom":
					e2 = f2.Section("peer").ReflectFrom(t2)
				}
				c.same(what+" error", errStr(e1), errStr(e2))
				c.same(what, t1, t2)
				if e1 != nil {
					// v1 could change the file before failing; v2 checks
					// first.
					continue
				}
				var b1, b2 bytes.Buffer
				_, _ = f1.WriteTo(&b1)
				_, _ = f2.Write(&b2)
				c.same(what+" written", b1.String(), b2.String())
			}
		}
	}
}

func TestAPIParity(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, in := range append(apiSeeds(), paritySeeds...) {
		for i := 0; i < 3; i++ {
			checkAPIParity(t, []byte(in), rng.Uint32())
		}
		checkAPIParity(t, []byte(in), 0)
	}
	for i := 0; i < 300; i++ {
		checkAPIParity(t, []byte(apiDoc(rng)), rng.Uint32()&(1<<16|1<<7|1<<14|1<<15|1<<1|1<<2|1<<6|1<<8))
	}
}

func FuzzAPIParity(f *testing.F) {
	for _, in := range append(apiSeeds(), paritySeeds...) {
		f.Add([]byte(in), uint32(0))
		f.Add([]byte(in), uint32(1<<7))
	}
	f.Fuzz(func(t *testing.T, data []byte, flags uint32) {
		checkAPIParity(t, data, flags)
	})
}
