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
	"cmp"
	"reflect"
	"slices"
	"time"
)

// Scalar is a type a value can be parsed as, as struct mapping parses
// fields: strings, booleans, integers and floats of any size, named types of
// those, time.Duration (as "1s" or nanoseconds) and time.Time (RFC 3339).
// A number that doesn't fit the type is an error.
type Scalar interface {
	~string | ~bool |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | time.Time
}

// parse parses s as a T.
func parse[T Scalar](s string) (T, error) {
	v, err := parseValue(reflect.TypeFor[T](), s)
	if err != nil {
		var zero T
		return zero, err
	}
	return v.Interface().(T), nil
}

// List splits the value of k at delim, as Key.Strings does, and parses each
// element as a T. It returns an error for the first element that doesn't
// parse.
func List[T Scalar](k *Key, delim string) ([]T, error) {
	strs := k.Strings(delim)
	vals := make([]T, len(strs))
	for i, s := range strs {
		v, err := parse[T](s)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	return vals, nil
}

// In returns the value of k parsed as a T when it is one of candidates, and
// defaultVal otherwise. T comes from candidates, as in
// In(k, "info", []string{"debug", "info"}).
func In[T Scalar](k *Key, defaultVal T, candidates []T) T {
	if v, err := parse[T](k.String()); err == nil && slices.Contains(candidates, v) {
		return v
	}
	return defaultVal
}

// Range returns the value of k parsed as a T when it is between lo and hi
// inclusive, and defaultVal otherwise. Untyped constants make T int, so give
// another type explicitly: Range(k, 0.5, 0, 1) or Range(k, uint8(0), 0, 9).
func Range[T interface {
	Scalar
	cmp.Ordered
}](k *Key, defaultVal, lo, hi T) T {
	if v, err := parse[T](k.String()); err == nil && v >= lo && v <= hi {
		return v
	}
	return defaultVal
}
