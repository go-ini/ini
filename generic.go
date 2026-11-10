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
// A number that doesn't fit the type doesn't parse.
type Scalar interface {
	~string | ~bool |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | time.Time
}

// Ordered is a Scalar that Range can compare: any but booleans.
type Ordered interface {
	Scalar
	cmp.Ordered | time.Time
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

// parseList parses strs with parse. An element that doesn't parse is an
// error when strict, the zero value when keepInvalid, and left out
// otherwise.
func parseList[T any](strs []string, parse func(string) (T, error), keepInvalid, strict bool) ([]T, error) {
	vals := make([]T, 0, len(strs))
	for _, s := range strs {
		v, err := parse(s)
		if err != nil {
			if strict {
				return nil, err
			}
			if !keepInvalid {
				continue
			}
			var zero T
			v = zero
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// List splits the value of k at delim, as Key.Strings does, and parses each
// element as a T. An element that doesn't parse is the zero value, as in
// List[int] of "1, x, 3", which is [1 0 3].
func List[T Scalar](k *Key, delim string) []T {
	vals, _ := parseList(k.Strings(delim), parse[T], true, false)
	return vals
}

// ValidList is List without the elements that don't parse: ValidList[int]
// of "1, x, 3" is [1 3].
func ValidList[T Scalar](k *Key, delim string) []T {
	vals, _ := parseList(k.Strings(delim), parse[T], false, false)
	return vals
}

// StrictList is List that returns an error for the first element that
// doesn't parse.
func StrictList[T Scalar](k *Key, delim string) ([]T, error) {
	return parseList(k.Strings(delim), parse[T], false, true)
}

// In returns the value of k parsed as a T when it is one of candidates, and
// defaultVal otherwise. T comes from candidates, as in
// In(k, "info", []string{"debug", "info"}). Times match when they are the
// same instant.
func In[T Scalar](k *Key, defaultVal T, candidates []T) T {
	if v, err := parse[T](k.String()); err == nil && slices.ContainsFunc(candidates, func(c T) bool { return equal(v, c) }) {
		return v
	}
	return defaultVal
}

// Range returns the value of k parsed as a T when it is between lo and hi
// inclusive, and defaultVal otherwise. Untyped constants make T int, so give
// another type explicitly: Range(k, 0.5, 0, 1) or Range(k, uint8(0), 0, 9).
func Range[T Ordered](k *Key, defaultVal, lo, hi T) T {
	if v, err := parse[T](k.String()); err == nil && !less(v, lo) && !less(hi, v) {
		return v
	}
	return defaultVal
}

// equal compares times as instants and other values with ==.
func equal[T Scalar](a, b T) bool {
	if t, ok := any(a).(time.Time); ok {
		return t.Equal(any(b).(time.Time))
	}
	return a == b
}

// less reports whether a < b. Times compare as instants; the rest by kind,
// as T can't use < while time.Time is in its type set.
func less[T Ordered](a, b T) bool {
	if t, ok := any(a).(time.Time); ok {
		return t.Before(any(b).(time.Time))
	}
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	switch va.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return va.Int() < vb.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return va.Uint() < vb.Uint()
	case reflect.Float32, reflect.Float64:
		return va.Float() < vb.Float()
	default:
		return va.String() < vb.String()
	}
}
