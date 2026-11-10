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

package ini

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Key represents a key under a section.
type Key struct {
	s       *Section
	name    string
	comment string
	value   string
	// shadows are the values after the first of a key that appears more
	// than once, with AllowShadows.
	shadows         []string
	isAutoIncrement bool
	isBooleanType   bool

	nestedValues []string
}

// newKey simply return a key object with given values.
func newKey(s *Section, name, val string) *Key {
	return &Key{
		s:     s,
		name:  name,
		value: val,
	}
}

func (k *Key) addShadow(val string) error {
	if k.isAutoIncrement || k.isBooleanType {
		return errors.New("cannot add shadow to auto-increment or boolean key")
	}

	// Deduplicate shadows based on their values. Empty values kept with
	// PreserveEmptyShadowValues each reset the values before them, so they stay.
	keptEmpty := val == "" && k.s.f.options.PreserveEmptyShadowValues
	if !k.s.f.options.AllowDuplicateShadowValues && !keptEmpty {
		if k.value == val || slices.Contains(k.shadows, val) {
			return nil
		}
	}

	k.shadows = append(k.shadows, val)
	return nil
}

// AddShadow adds a new shadow key to itself.
func (k *Key) AddShadow(val string) error {
	if !k.s.f.options.AllowShadows {
		return errors.New("shadow key is not allowed")
	}
	k.s.f.wlock()
	defer k.s.f.wunlock()
	return k.addShadow(val)
}

func (k *Key) addNestedValue(val string) error {
	if k.isAutoIncrement || k.isBooleanType {
		return errors.New("cannot add nested value to auto-increment or boolean key")
	}

	k.nestedValues = append(k.nestedValues, val)
	return nil
}

// AddNestedValue adds a nested value to the key.
func (k *Key) AddNestedValue(val string) error {
	if !k.s.f.options.AllowNestedValues {
		return errors.New("nested value is not allowed")
	}
	k.s.f.wlock()
	defer k.s.f.wunlock()
	return k.addNestedValue(val)
}

// ValueMapper represents a mapping function for values, e.g. os.ExpandEnv
type ValueMapper func(string) string

// Name returns name of key.
func (k *Key) Name() string {
	return k.name
}

// Comment returns the comment lines before the key, with their comment
// symbols.
func (k *Key) Comment() string {
	k.s.f.rlock()
	defer k.s.f.runlock()
	return k.comment
}

// SetComment sets the comment lines written before the key. Lines without a
// comment symbol get one.
func (k *Key) SetComment(comment string) {
	k.s.f.wlock()
	defer k.s.f.wunlock()
	k.comment = comment
}

// Value returns raw value of key for performance purpose.
func (k *Key) Value() string {
	f := k.s.f
	if !f.locking {
		return k.value
	}
	f.lock.RLock()
	v := k.value
	f.lock.RUnlock()
	return v
}

// Values returns raw values of key and its shadows if any. Empty
// values are left out, unless the file was loaded with
// PreserveEmptyShadowValues.
func (k *Key) Values() []string {
	k.s.f.rlock()
	defer k.s.f.runlock()
	keepEmpty := k.s.f.options.PreserveEmptyShadowValues
	vals := make([]string, 0, len(k.shadows)+1)
	if keepEmpty || k.value != "" {
		vals = append(vals, k.value)
	}
	for _, v := range k.shadows {
		if keepEmpty || v != "" {
			vals = append(vals, v)
		}
	}
	return vals
}

// IsBoolean reports whether the key was read as a boolean key, a line
// without a delimiter loaded with AllowBooleanKeys.
func (k *Key) IsBoolean() bool {
	k.s.f.rlock()
	defer k.s.f.runlock()
	return k.isBooleanType
}

// NestedValues returns nested values stored in the key.
// It is possible returned value is nil if no nested values stored in the key.
func (k *Key) NestedValues() []string {
	k.s.f.rlock()
	defer k.s.f.runlock()
	return k.nestedValues
}

// expandValue takes a raw value and transforms to its final string.
func (k *Key) expandValue(val string) string {
	return k.expand(val, 0, new(expansion))
}

// expansion counts the work of one %(name)s expansion.
type expansion struct {
	substitutions int
	bytes         int // total length of the values substitutions made
}

// expand applies the ValueMapper and replaces %(name)s references in val
// with the expanded values of those keys. References nest at most
// depthValues deep, and one expansion makes at most maxSubstitutions
// substitutions producing at most maxExpandedLen bytes in total, so
// reference cycles and values that double at every level stop instead of
// overflowing the stack or using up time and memory.
func (k *Key) expand(val string, depth int, e *expansion) string {
	if k.s.f.options.ValueMapper != nil {
		val = k.s.f.options.ValueMapper(val)
	}

	// Fail-fast if no indicate char found for recursive value
	if !strings.Contains(val, "%") {
		return val
	}
	for i := 0; i < depthValues && depth < depthValues && e.substitutions < maxSubstitutions; i++ {
		vr := varPattern.FindString(val)
		if len(vr) == 0 {
			break
		}

		// Take off leading '%(' and trailing ')s'.
		noption := vr[2 : len(vr)-2]

		// Search in the same section.
		// If not found or found the key itself, then search again in default section.
		nk, err := k.s.GetKey(noption)
		if err != nil || k == nk {
			nk, _ = k.s.f.Section("").GetKey(noption)
			if nk == nil || nk == k {
				// Stop when no results found in the default section, or only
				// the key itself, and returns the value as-is.
				break
			}
		}

		e.substitutions++
		sub := nk.expand(nk.Value(), depth+1, e)
		n := len(val) + strings.Count(val, vr)*(len(sub)-len(vr))
		if e.bytes+n > maxExpandedLen {
			break
		}
		e.bytes += n
		val = strings.ReplaceAll(val, vr, sub)
	}
	return val
}

// String returns string representation of value.
func (k *Key) String() string {
	return k.expandValue(k.Value())
}

// Validate accepts a validate function which can
// return modifed result as key value.
func (k *Key) Validate(fn func(string) string) string {
	return fn(k.String())
}

// parseBool returns the boolean value represented by the string.
//
// It accepts 1, t, T, TRUE, true, True, YES, yes, Yes, y, ON, on, On,
// 0, f, F, FALSE, false, False, NO, no, No, n, OFF, off, Off.
// Any other value returns an error.
func parseBool(str string) (value bool, err error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "YES", "yes", "Yes", "y", "ON", "on", "On":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False", "NO", "no", "No", "n", "OFF", "off", "Off":
		return false, nil
	}
	return false, fmt.Errorf("parsing \"%s\": invalid syntax", str)
}

// Bool returns bool type value.
func (k *Key) Bool() (bool, error) {
	return parseBool(k.String())
}

// Float64 returns float64 type value.
func (k *Key) Float64() (float64, error) {
	return strconv.ParseFloat(k.String(), 64)
}

// Int returns int type value.
func (k *Key) Int() (int, error) {
	v, err := strconv.ParseInt(k.String(), 0, 64)
	return int(v), err
}

// Int64 returns int64 type value.
func (k *Key) Int64() (int64, error) {
	return strconv.ParseInt(k.String(), 0, 64)
}

// Uint returns uint type valued.
func (k *Key) Uint() (uint, error) {
	u, e := strconv.ParseUint(k.String(), 0, 64)
	return uint(u), e
}

// Uint64 returns uint64 type value.
func (k *Key) Uint64() (uint64, error) {
	return strconv.ParseUint(k.String(), 0, 64)
}

// Duration returns time.Duration type value.
func (k *Key) Duration() (time.Duration, error) {
	return time.ParseDuration(k.String())
}

// TimeFormat parses with given format and returns time.Time type value.
func (k *Key) TimeFormat(format string) (time.Time, error) {
	return time.Parse(format, k.String())
}

// TimesFormat parses the elements of the value, split at delim, with the
// given layout. An element that doesn't parse is the zero time. For RFC 3339
// times, use List[time.Time].
func (k *Key) TimesFormat(format, delim string) []time.Time {
	vals, _ := parseList(k.Strings(delim), timeParser(format), true, false)
	return vals
}

// ValidTimesFormat is TimesFormat without the elements that don't parse.
func (k *Key) ValidTimesFormat(format, delim string) []time.Time {
	vals, _ := parseList(k.Strings(delim), timeParser(format), false, false)
	return vals
}

// StrictTimesFormat is TimesFormat that returns an error for the first
// element that doesn't parse.
func (k *Key) StrictTimesFormat(format, delim string) ([]time.Time, error) {
	return parseList(k.Strings(delim), timeParser(format), false, true)
}

// InTimeFormat returns the value parsed with the given layout when it is the
// same instant as one of candidates, and defaultVal otherwise.
func (k *Key) InTimeFormat(format string, defaultVal time.Time, candidates []time.Time) time.Time {
	if v, err := k.TimeFormat(format); err == nil && slices.ContainsFunc(candidates, v.Equal) {
		return v
	}
	return defaultVal
}

// RangeTimeFormat returns the value parsed with the given layout when it is
// between min and max inclusive, and defaultVal otherwise.
func (k *Key) RangeTimeFormat(format string, defaultVal, min, max time.Time) time.Time {
	if v, err := k.TimeFormat(format); err == nil && !v.Before(min) && !v.After(max) {
		return v
	}
	return defaultVal
}

func timeParser(format string) func(string) (time.Time, error) {
	return func(s string) (time.Time, error) { return time.Parse(format, s) }
}

// Time parses with RFC3339 format and returns time.Time type value.
func (k *Key) Time() (time.Time, error) {
	return k.TimeFormat(time.RFC3339)
}

// MustString returns default value if key value is empty.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustString(defaultVal string) string {
	val := k.String()
	if len(val) == 0 {
		return defaultVal
	}
	return val
}

// MustBool always returns value without error,
// it returns false if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustBool(defaultVal ...bool) bool {
	val, err := k.Bool()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustFloat64 always returns value without error,
// it returns 0.0 if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustFloat64(defaultVal ...float64) float64 {
	val, err := k.Float64()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustInt always returns value without error,
// it returns 0 if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustInt(defaultVal ...int) int {
	val, err := k.Int()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustInt64 always returns value without error,
// it returns 0 if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustInt64(defaultVal ...int64) int64 {
	val, err := k.Int64()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustUint always returns value without error,
// it returns 0 if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustUint(defaultVal ...uint) uint {
	val, err := k.Uint()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustUint64 always returns value without error,
// it returns 0 if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustUint64(defaultVal ...uint64) uint64 {
	val, err := k.Uint64()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustDuration always returns value without error,
// it returns zero value if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustDuration(defaultVal ...time.Duration) time.Duration {
	val, err := k.Duration()
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustTimeFormat always parses with given format and returns value without error,
// it returns zero value if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustTimeFormat(format string, defaultVal ...time.Time) time.Time {
	val, err := k.TimeFormat(format)
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustTime always parses with RFC3339 format and returns value without error,
// it returns zero value if error occurs.
// It doesn't store the default in the key; call SetValue for that.
func (k *Key) MustTime(defaultVal ...time.Time) time.Time {
	return k.MustTimeFormat(time.RFC3339, defaultVal...)
}

// Strings returns list of string divided by given delimiter.
func (k *Key) Strings(delim string) []string {
	return splitList(k.String(), delim)
}

// splitList splits str at delim into trimmed elements. A backslash escapes
// delim and another backslash; before anything else it is kept.
func splitList(str, delim string) []string {
	if len(str) == 0 {
		return []string{}
	}
	if delim == "" {
		return []string{strings.TrimSpace(str)}
	}

	maxParts := strings.Count(str, delim) + 1
	vals := make([]string, 0, maxParts)

	var buf strings.Builder
	buf.Grow(len(str))

	i := 0
	for {
		if str[i] == '\\' {
			i++
			if i >= len(str) {
				break
			}

			if str[i] != '\\' && !strings.HasPrefix(str[i:], delim) {
				buf.WriteRune('\\')
			}

			r, size := utf8.DecodeRuneInString(str[i:])
			i += size
			buf.WriteRune(r)
		} else if strings.HasPrefix(str[i:], delim) {
			i += len(delim)
			vals = append(vals, strings.TrimSpace(buf.String()))
			buf.Reset()
		} else {
			r, size := utf8.DecodeRuneInString(str[i:])
			i += size
			buf.WriteRune(r)
		}

		if i >= len(str) {
			break
		}
	}

	if buf.Len() > 0 {
		vals = append(vals, strings.TrimSpace(buf.String()))
	}

	return vals
}

// StringsWithShadows returns list of string divided by given delimiter.
// Shadows will also be appended if any.
func (k *Key) StringsWithShadows(delim string) []string {
	vals := k.Values()
	results := make([]string, 0, len(vals)*2)
	for i := range vals {
		if len(vals) == 0 {
			continue
		}

		results = append(results, strings.Split(vals[i], delim)...)
	}

	for i := range results {
		results[i] = k.expandValue(strings.TrimSpace(results[i]))
	}
	return results
}

// SetValue changes key value.
func (k *Key) SetValue(v string) {
	k.s.f.wlock()
	defer k.s.f.wunlock()
	k.value = v
}
