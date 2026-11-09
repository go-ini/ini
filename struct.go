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
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// NameMapper represents a ini tag name mapper.
type NameMapper func(string) string

// Built-in name getters.
var (
	// SnackCase converts to format SNACK_CASE.
	SnackCase NameMapper = func(raw string) string {
		newstr := make([]rune, 0, len(raw))
		for i, chr := range raw {
			if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
				if i > 0 {
					newstr = append(newstr, '_')
				}
			}
			newstr = append(newstr, unicode.ToUpper(chr))
		}
		return string(newstr)
	}
	// TitleUnderscore converts to format title_underscore.
	TitleUnderscore NameMapper = func(raw string) string {
		newstr := make([]rune, 0, len(raw))
		for i, chr := range raw {
			if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
				if i > 0 {
					newstr = append(newstr, '_')
				}
				chr -= 'A' - 'a'
			}
			newstr = append(newstr, chr)
		}
		return string(newstr)
	}
)

func (s *Section) parseFieldName(raw, actual string) string {
	if len(actual) > 0 {
		return actual
	}
	if s.f.NameMapper != nil {
		return s.f.NameMapper(raw)
	}
	return raw
}

func parseDelim(actual string) string {
	if len(actual) > 0 {
		return actual
	}
	return ","
}

var timeType = reflect.TypeOf(time.Time{})

// isStructType reports whether t is a struct type that maps to a section,
// which is any struct type but time.Time.
func isStructType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t != timeType
}

// isValueType reports whether a key value can be parsed as or formatted from
// a value of type t.
func isValueType(t reflect.Type) bool {
	if t == timeType {
		return true
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// isSliceElemType reports whether slices of t are supported.
func isSliceElemType(t reflect.Type) bool {
	if t == timeType {
		return true
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Float64:
		return true
	}
	return false
}

// parseValue parses str as a value of type t, for which isValueType is true.
func parseValue(t reflect.Type, str string) (reflect.Value, error) {
	v := reflect.New(t).Elem()
	if t == timeType {
		ti, err := time.Parse(time.RFC3339, str)
		if err != nil {
			return v, err
		}
		v.Set(reflect.ValueOf(ti))
		return v, nil
	}

	switch t.Kind() {
	case reflect.String:
		v.SetString(str)
	case reflect.Bool:
		b, err := parseBool(str)
		if err != nil {
			return v, err
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(str, 0, 64)
		// A duration can also be given as a number of nanoseconds.
		if t.Name() == "Duration" {
			if d, derr := time.ParseDuration(str); derr == nil {
				i, err = int64(d), nil
			}
		}
		if err != nil {
			return v, err
		}
		if v.OverflowInt(i) {
			return v, fmt.Errorf("value %q overflows %s", str, t)
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(str, 0, 64)
		if err != nil {
			return v, err
		}
		if v.OverflowUint(u) {
			return v, fmt.Errorf("value %q overflows %s", str, t)
		}
		v.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return v, err
		}
		if v.OverflowFloat(f) {
			return v, fmt.Errorf("value %q overflows %s", str, t)
		}
		v.SetFloat(f)
	default:
		return v, fmt.Errorf("unsupported type %q", t)
	}
	return v, nil
}

// formatValue does the opposite thing as parseValue.
func formatValue(v reflect.Value) string {
	if v.Type() == timeType {
		return v.Interface().(time.Time).Format(time.RFC3339Nano)
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return fmt.Sprint(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprint(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprint(v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprint(v.Float())
	}
	return ""
}

// defaultComment sets *c to comment if it is empty, under the file's lock.
func (f *File) defaultComment(c *string, comment string) {
	f.wlock()
	defer f.wunlock()
	if *c == "" {
		*c = comment
	}
}

// tooDeep reports whether path, the struct types being mapped or reflected
// from the outermost one, already holds t twice. Section names are not
// nested, so a recursive struct type would use the same sections forever: it
// is followed one level deep only.
func tooDeep(path []reflect.Type, t reflect.Type) bool {
	n := 0
	for _, p := range path {
		if p == t {
			n++
		}
	}
	return n >= 2
}

// setSliceWithProperType sets proper values to slice based on its type.
func setSliceWithProperType(key *Key, field reflect.Value, delim string, allowShadow, isStrict bool) error {
	elemType := field.Type().Elem()
	if !isSliceElemType(elemType) {
		return fmt.Errorf("unsupported type '[]%s'", elemType)
	}

	var strs []string
	if allowShadow {
		// Each value is split as Key.Strings splits one, so that escaped
		// elements read back as Marshal wrote them. An empty value, kept
		// with KeepEmptyShadowValues, is an empty element, unless it is the
		// only one: an empty slice is written as one empty value.
		vals := key.ValueWithShadows()
		if len(vals) == 1 && vals[0] == "" {
			vals = nil
		}
		for _, v := range vals {
			if v == "" {
				strs = append(strs, "")
				continue
			}
			strs = append(strs, splitList(key.transformValue(v), delim)...)
		}
	} else {
		strs = key.Strings(delim)
	}

	numVals := len(strs)
	if numVals == 0 {
		return nil
	}

	slice := reflect.MakeSlice(field.Type(), numVals, numVals)
	for i, str := range strs {
		val, err := parseValue(elemType, str)
		if err != nil {
			if isStrict {
				return err
			}
			// An invalid element is left as the zero value.
			continue
		}
		slice.Index(i).Set(val)
	}
	field.Set(slice)
	return nil
}

func wrapStrictError(err error, isStrict bool) error {
	if isStrict {
		return err
	}
	return nil
}

// setWithProperType sets proper value to field based on its type,
// but it does not return error for failing parsing,
// because we want to use default value that is already assigned to struct.
func setWithProperType(t reflect.Type, key *Key, field reflect.Value, delim string, allowShadow, isStrict bool) error {
	if t.Kind() == reflect.Slice {
		return setSliceWithProperType(key, field, delim, allowShadow, isStrict)
	}

	vt := t
	isPtr := t.Kind() == reflect.Pointer
	if isPtr {
		vt = t.Elem()
	}
	if !isValueType(vt) {
		return fmt.Errorf("unsupported type %q", t)
	}

	str := key.String()
	// An empty string keeps the default value of a string field.
	if vt.Kind() == reflect.String && len(str) == 0 && !isPtr {
		return nil
	}
	val, err := parseValue(vt, str)
	if err != nil {
		return wrapStrictError(err, isStrict)
	}
	if isPtr {
		ptr := reflect.New(vt)
		ptr.Elem().Set(val)
		val = ptr
	}
	field.Set(val)
	return nil
}

func parseTagOptions(tag string) (rawName string, omitEmpty bool, allowShadow bool, allowNonUnique bool, extends bool) {
	opts := strings.SplitN(tag, ",", 5)
	rawName = opts[0]
	for _, opt := range opts[1:] {
		omitEmpty = omitEmpty || (opt == "omitempty")
		allowShadow = allowShadow || (opt == "allowshadow")
		allowNonUnique = allowNonUnique || (opt == "nonunique")
		extends = extends || (opt == "extends")
	}
	return rawName, omitEmpty, allowShadow, allowNonUnique, extends
}

// mapToField maps the given value to the matching field of the given section.
// The sectionIndex is the index (if non unique sections are enabled) to which the value should be added.
// The path holds the struct types being mapped, see tooDeep.
func (s *Section) mapToField(val reflect.Value, isStrict bool, sectionIndex int, sectionName string, path []reflect.Type) error {
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	typ := val.Type()
	path = append(path, typ)

	for i := 0; i < typ.NumField(); i++ {
		field := val.Field(i)
		tpField := typ.Field(i)

		tag := tpField.Tag.Get("ini")
		if tag == "-" {
			continue
		}

		rawName, _, allowShadow, allowNonUnique, extends := parseTagOptions(tag)
		fieldName := s.parseFieldName(tpField.Name, rawName)
		if len(fieldName) == 0 || !field.CanSet() {
			continue
		}

		isStruct := isStructType(tpField.Type)
		isStructPtr := tpField.Type.Kind() == reflect.Pointer && isStructType(tpField.Type.Elem())
		isAnonymousPtr := isStructPtr && tpField.Anonymous
		structType := tpField.Type
		if isStructPtr {
			structType = tpField.Type.Elem()
		}
		if isAnonymousPtr && field.IsNil() && !tooDeep(path, structType) {
			field.Set(reflect.New(structType))
		}

		if extends && (isAnonymousPtr || (isStruct && tpField.Anonymous)) {
			if tooDeep(path, structType) {
				continue
			}
			fieldSection := s
			if rawName != "" {
				sectionName = s.name + s.f.options.ChildSectionDelimiter + rawName
				if secs, err := s.f.SectionsByName(sectionName); err == nil && sectionIndex < len(secs) {
					fieldSection = secs[sectionIndex]
				}
			}
			if err := fieldSection.mapToField(field, isStrict, sectionIndex, sectionName, path); err != nil {
				return fmt.Errorf("map to field %q: %v", fieldName, err)
			}
			continue
		} else if isStruct || isStructPtr {
			if secs, err := s.f.SectionsByName(fieldName); err == nil {
				if tooDeep(path, structType) {
					continue
				}
				if len(secs) <= sectionIndex {
					return fmt.Errorf("there are not enough sections (%d <= %d) for the field %q", len(secs), sectionIndex, fieldName)
				}
				// Only set the field to non-nil struct value if we have a section for it.
				// Otherwise, we end up with a non-nil struct ptr even though there is no data.
				if isStructPtr && field.IsNil() {
					field.Set(reflect.New(structType))
				}
				if err = secs[sectionIndex].mapToField(field, isStrict, sectionIndex, fieldName, path); err != nil {
					return fmt.Errorf("map to field %q: %v", fieldName, err)
				}
				continue
			}
		}

		// Map non-unique sections
		if allowNonUnique && tpField.Type.Kind() == reflect.Slice {
			newField, err := s.mapToSlice(fieldName, field, isStrict, path)
			if err != nil {
				return fmt.Errorf("map to slice %q: %v", fieldName, err)
			}

			field.Set(newField)
			continue
		}

		if key, err := s.GetKey(fieldName); err == nil {
			delim := parseDelim(tpField.Tag.Get("delim"))
			if err = setWithProperType(tpField.Type, key, field, delim, allowShadow, isStrict); err != nil {
				return fmt.Errorf("set field %q: %v", fieldName, err)
			}
		}
	}
	return nil
}

// mapToSlice maps all sections with the same name and returns the new value.
// The type of the Value must be a slice of structs or of pointers to structs.
func (s *Section) mapToSlice(secName string, val reflect.Value, isStrict bool, path []reflect.Type) (reflect.Value, error) {
	elemType := val.Type().Elem()
	isPtr := elemType.Kind() == reflect.Pointer
	structType := elemType
	if isPtr {
		structType = elemType.Elem()
	}
	if !isStructType(structType) {
		return reflect.Value{}, fmt.Errorf("unsupported type %q for non-unique sections", val.Type())
	}

	secs, err := s.f.SectionsByName(secName)
	if err != nil || tooDeep(path, structType) {
		// The slice is left as it is when there are no sections.
		return val, nil
	}

	for i, sec := range secs {
		elem := reflect.New(structType)
		if err = sec.mapToField(elem, isStrict, i, sec.name, path); err != nil {
			return reflect.Value{}, fmt.Errorf("map to field from section %q: %v", secName, err)
		}
		if !isPtr {
			elem = elem.Elem()
		}
		val = reflect.Append(val, elem)
	}
	return val, nil
}

// mapTo maps a section to object v.
func (s *Section) mapTo(v interface{}, isStrict bool) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return errors.New("not a pointer to a struct")
	}
	typ := val.Type().Elem()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct && typ.Kind() != reflect.Slice {
		return errors.New("not a pointer to a struct")
	}

	// Allocate the nil pointers v points through.
	val = val.Elem()
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}

	if typ.Kind() == reflect.Slice {
		newField, err := s.mapToSlice(s.name, val, isStrict, nil)
		if err != nil {
			return err
		}

		val.Set(newField)
		return nil
	}

	return s.mapToField(val, isStrict, 0, s.name, nil)
}

// MapTo maps section to given struct.
func (s *Section) MapTo(v interface{}) error {
	return s.mapTo(v, false)
}

// StrictMapTo maps section to given struct in strict mode,
// which returns all possible error including value parsing error.
func (s *Section) StrictMapTo(v interface{}) error {
	return s.mapTo(v, true)
}

// MapTo maps file to given struct.
func (f *File) MapTo(v interface{}) error {
	return f.Section("").MapTo(v)
}

// StrictMapTo maps file to given struct in strict mode,
// which returns all possible error including value parsing error.
func (f *File) StrictMapTo(v interface{}) error {
	return f.Section("").StrictMapTo(v)
}

// MapToWithMapper loads a data source with opts and maps it to the given
// struct with the name mapper.
func MapToWithMapper(v any, mapper NameMapper, source any, opts ...LoadOption) error {
	cfg, err := Load(source, opts...)
	if err != nil {
		return err
	}
	cfg.NameMapper = mapper
	return cfg.MapTo(v)
}

// StrictMapToWithMapper loads a data source with opts and maps it to the
// given struct with the name mapper in strict mode, which returns all
// possible error including value parsing error.
func StrictMapToWithMapper(v any, mapper NameMapper, source any, opts ...LoadOption) error {
	cfg, err := Load(source, opts...)
	if err != nil {
		return err
	}
	cfg.NameMapper = mapper
	return cfg.StrictMapTo(v)
}

// MapTo loads a data source with opts and maps it to the given struct.
func MapTo(v, source any, opts ...LoadOption) error {
	return MapToWithMapper(v, nil, source, opts...)
}

// StrictMapTo loads a data source with opts and maps it to the given struct
// in strict mode, which returns all possible error including value parsing
// error.
func StrictMapTo(v, source any, opts ...LoadOption) error {
	return StrictMapToWithMapper(v, nil, source, opts...)
}

// reflectSliceWithProperType does the opposite thing as setSliceWithProperType.
func reflectSliceWithProperType(key *Key, field reflect.Value, delim string, allowShadow bool) error {
	if field.Len() == 0 {
		return nil
	}
	elemType := field.Type().Elem()
	if !isSliceElemType(elemType) {
		return fmt.Errorf("unsupported type '[]%s'", elemType)
	}

	vals := make([]string, field.Len())
	for i := range vals {
		vals[i] = formatValue(field.Index(i))
	}

	for i := range vals {
		vals[i] = escapeSliceElem(vals[i], delim)
	}
	if allowShadow {
		keyWithShadows := newKey(key.s, key.name, vals[0])
		for _, val := range vals[1:] {
			if err := keyWithShadows.AddShadow(val); err != nil {
				return err
			}
		}
		key.s.f.wlock()
		key.value, key.hashValue, key.shadows = keyWithShadows.value, keyWithShadows.hashValue, keyWithShadows.shadows
		key.s.f.wunlock()
		return nil
	}
	key.SetValue(strings.Join(vals, delim))
	return nil
}

// escapeSliceElem escapes s so that Key.Strings reads it back as one element
// even when it contains delim. Key.Strings keeps a backslash unless it comes
// before another backslash, delim or the end, so only those are doubled and
// paths like C:\dir are written as they are.
func escapeSliceElem(s, delim string) string {
	if !strings.Contains(s, `\`) && !strings.Contains(s, delim) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		rest := s[i:]
		switch {
		case strings.HasPrefix(rest, delim):
			b.WriteString(`\` + delim)
			i += len(delim) - 1
		case s[i] == '\\' && (i+1 == len(s) || s[i+1] == '\\' || strings.HasPrefix(s[i+1:], delim)):
			b.WriteString(`\\`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// reflectWithProperType does the opposite thing as setWithProperType.
func reflectWithProperType(t reflect.Type, key *Key, field reflect.Value, delim string, allowShadow bool) error {
	switch {
	case t.Kind() == reflect.Slice:
		return reflectSliceWithProperType(key, field, delim, allowShadow)
	case t.Kind() == reflect.Pointer:
		if !field.IsNil() {
			return reflectWithProperType(t.Elem(), key, field.Elem(), delim, allowShadow)
		}
	case isValueType(t):
		key.SetValue(formatValue(field))
	default:
		return fmt.Errorf("unsupported type %q", t)
	}
	return nil
}

// CR: copied from encoding/json/encode.go with modifications of time.Time support.
// TODO: add more test coverage.
func isEmptyValue(v reflect.Value) bool {
	if v.Type() == timeType {
		return v.Interface().(time.Time).IsZero()
	}
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

// StructReflector is the interface implemented by struct types that can extract themselves into INI objects.
type StructReflector interface {
	ReflectINIStruct(*File) error
}

// reflectFrom reflects the section from val. The path holds the struct types
// being reflected from, see tooDeep.
func (s *Section) reflectFrom(val reflect.Value, path []reflect.Type) error {
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	typ := val.Type()
	path = append(path, typ)

	for i := 0; i < typ.NumField(); i++ {
		if !val.Field(i).CanInterface() {
			continue
		}

		field := val.Field(i)
		tpField := typ.Field(i)

		tag := tpField.Tag.Get("ini")
		if tag == "-" {
			continue
		}

		rawName, omitEmpty, allowShadow, allowNonUnique, extends := parseTagOptions(tag)
		if omitEmpty && isEmptyValue(field) {
			continue
		}

		if r, ok := field.Interface().(StructReflector); ok {
			if err := r.ReflectINIStruct(s.f); err != nil {
				return err
			}
			continue
		}

		fieldName := s.parseFieldName(tpField.Name, rawName)
		if len(fieldName) == 0 || !field.CanSet() {
			continue
		}

		// A nil pointer has nothing to write.
		if field.Kind() == reflect.Pointer && field.IsNil() {
			continue
		}

		isStruct := isStructType(tpField.Type)
		isStructPtr := tpField.Type.Kind() == reflect.Pointer && isStructType(tpField.Type.Elem())
		structType := tpField.Type
		if isStructPtr {
			structType = tpField.Type.Elem()
		}

		if extends && tpField.Anonymous && (isStruct || isStructPtr) {
			if tooDeep(path, structType) {
				continue
			}
			if err := s.reflectFrom(field, path); err != nil {
				return fmt.Errorf("reflect from field %q: %v", fieldName, err)
			}
			continue
		}

		if isStruct || isStructPtr {
			if tooDeep(path, structType) {
				continue
			}
			// Note: The only error here is section doesn't exist.
			sec, err := s.f.GetSection(fieldName)
			if err != nil {
				// Note: fieldName can never be empty here, ignore error.
				sec, _ = s.f.NewSection(fieldName)
			}

			// Add comment from comment tag
			s.f.defaultComment(&sec.Comment, tpField.Tag.Get("comment"))

			if err = sec.reflectFrom(field, path); err != nil {
				return fmt.Errorf("reflect from field %q: %v", fieldName, err)
			}
			continue
		}

		if allowNonUnique && tpField.Type.Kind() == reflect.Slice {
			if field.Len() == 0 {
				continue
			}
			structType = tpField.Type.Elem()
			if structType.Kind() == reflect.Pointer {
				structType = structType.Elem()
			}
			if !isStructType(structType) {
				return fmt.Errorf("field %q is not a slice of pointer or struct", fieldName)
			}
			if tooDeep(path, structType) {
				continue
			}

			for i := 0; i < field.Len(); i++ {
				elem := field.Index(i)
				if elem.Kind() == reflect.Pointer && elem.IsNil() {
					continue
				}

				sec, err := s.f.NewSection(fieldName)
				if err != nil {
					return err
				}

				// Add comment from comment tag
				s.f.defaultComment(&sec.Comment, tpField.Tag.Get("comment"))

				if err := sec.reflectFrom(elem, path); err != nil {
					return fmt.Errorf("reflect from field %q: %v", fieldName, err)
				}
			}
			continue
		}

		// Note: Same reason as section.
		key, err := s.GetKey(fieldName)
		if err != nil {
			key, _ = s.NewKey(fieldName, "")
		}

		// Add comment from comment tag
		s.f.defaultComment(&key.Comment, tpField.Tag.Get("comment"))

		delim := parseDelim(tpField.Tag.Get("delim"))
		if err = reflectWithProperType(tpField.Type, key, field, delim, allowShadow); err != nil {
			return fmt.Errorf("reflect field %q: %v", fieldName, err)
		}

	}
	return nil
}

// ReflectFrom reflects section from given struct. It overwrites existing ones.
func (s *Section) ReflectFrom(v interface{}) error {
	typ := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if typ == nil || (typ.Kind() == reflect.Pointer && val.IsNil()) {
		return errors.New("not a pointer to a struct")
	}

	if !s.f.isDefaultSection(s.name) && s.f.options.AllowNonUniqueSections &&
		(typ.Kind() == reflect.Slice || typ.Kind() == reflect.Pointer) {
		if typ.Kind() == reflect.Pointer && !isStructType(typ.Elem()) {
			return errors.New("not a pointer to a struct")
		}
		if typ.Kind() == reflect.Slice && (typ.Elem().Kind() != reflect.Pointer || !isStructType(typ.Elem().Elem())) {
			return errors.New("not a slice of pointers to structs")
		}

		// Clear sections to make sure none exists before adding the new ones
		s.f.DeleteSection(s.name)

		if typ.Kind() == reflect.Pointer {
			sec, err := s.f.NewSection(s.name)
			if err != nil {
				return err
			}
			return sec.reflectFrom(val.Elem(), nil)
		}

		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if elem.IsNil() {
				continue
			}

			sec, err := s.f.NewSection(s.name)
			if err != nil {
				return err
			}

			err = sec.reflectFrom(elem, nil)
			if err != nil {
				return fmt.Errorf("reflect from %dth field: %v", i, err)
			}
		}

		return nil
	}

	if typ.Kind() != reflect.Pointer || typ.Elem().Kind() != reflect.Struct {
		return errors.New("not a pointer to a struct")
	}
	return s.reflectFrom(val.Elem(), nil)
}

// ReflectFrom reflects file from given struct.
func (f *File) ReflectFrom(v interface{}) error {
	return f.Section("").ReflectFrom(v)
}

// ReflectFromWithMapper reflects data sources from given struct with name mapper.
func ReflectFromWithMapper(cfg *File, v interface{}, mapper NameMapper) error {
	cfg.NameMapper = mapper
	return cfg.ReflectFrom(v)
}

// ReflectFrom reflects data sources from given struct.
func ReflectFrom(cfg *File, v interface{}) error {
	return ReflectFromWithMapper(cfg, v, nil)
}

// Unmarshal parses INI data with opts and maps it to the struct v points to,
// as File.MapTo does. It reads what Marshal writes: when v has allowshadow
// fields it loads with AllowShadows, AllowDuplicateShadowValues and
// KeepEmptyShadowValues, and when it has nonunique fields with
// AllowNonUniqueSections, as those tags need.
func Unmarshal(data []byte, v any, opts ...LoadOption) error {
	if v != nil {
		shadows, nonUnique := tagOptions(reflect.TypeOf(v), map[reflect.Type]bool{})
		if shadows {
			opts = append([]LoadOption{AllowShadows(), AllowDuplicateShadowValues(), KeepEmptyShadowValues()}, opts...)
		}
		if nonUnique {
			opts = append([]LoadOption{AllowNonUniqueSections()}, opts...)
		}
	}
	return MapTo(v, data, opts...)
}

// tagOptions reports whether t, or a struct type it holds, has allowshadow
// or nonunique fields.
func tagOptions(t reflect.Type, seen map[reflect.Type]bool) (shadows, nonUnique bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if !isStructType(t) || seen[t] {
		return false, false
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		_, _, allowShadow, allowNonUnique, _ := parseTagOptions(t.Field(i).Tag.Get("ini"))
		s, n := tagOptions(t.Field(i).Type, seen)
		shadows = shadows || allowShadow || s
		nonUnique = nonUnique || allowNonUnique || n
	}
	return shadows, nonUnique
}

// Marshal returns the INI encoding of v, a struct or a pointer to one, as
// File.ReflectFrom and File.Write produce it.
func Marshal(v any, opts ...WriteOption) ([]byte, error) {
	if val := reflect.ValueOf(v); val.Kind() == reflect.Struct {
		ptr := reflect.New(val.Type())
		ptr.Elem().Set(val)
		v = ptr.Interface()
	}
	// Shadows and non-unique sections let allowshadow and nonunique fields
	// write all their values.
	f := Empty(AllowShadows(), AllowDuplicateShadowValues(), AllowNonUniqueSections())
	if err := f.ReflectFrom(v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := f.Write(&buf, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
