package ini

import (
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
	// SnakeCase converts to format SNAKE_CASE.
	SnakeCase NameMapper = func(raw string) string {
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
	if s.f.options.NameMapper != nil {
		return s.f.options.NameMapper(raw)
	}
	return raw
}

func parseDelim(actual string) string {
	if len(actual) > 0 {
		return actual
	}
	return ","
}

// Marshaler is implemented by types that format themselves as a key value.
// ReflectFrom calls MarshalINI for fields, pointer fields and slice elements
// of such a type, whatever its kind. A type that isn't a string, boolean or
// number also needs Unmarshaler, as described there.
type Marshaler interface {
	MarshalINI() ([]byte, error)
}

// Unmarshaler is implemented by types that parse themselves from a key value.
// MapTo and StrictMapTo call UnmarshalINI for fields, pointer fields and slice
// elements of such a type, with the value as Key.String returns it, or with
// one element of a list, and so do List, ValidList, StrictList, In and Range
// for a Scalar type. An error is handled as for a value that doesn't parse:
// MapTo leaves a field unchanged and makes an element of a slice field the
// zero value, and StrictMapTo and StrictList return it.
//
// A type that isn't a string, boolean or number, such as a struct, slice or
// map type, maps to one key only when it implements both Marshaler and
// Unmarshaler, so that it maps the same way in both directions. Otherwise a
// struct type still maps to a section and a slice type to a list. A nonunique
// field, or a value passed for a section of a file with
// AllowNonUniqueSections, maps a struct type to sections either way. A struct that embeds such a type
// gets its methods, and so maps to one key too.
type Unmarshaler interface {
	UnmarshalINI([]byte) error
}

var (
	timeType        = reflect.TypeFor[time.Time]()
	marshalerType   = reflect.TypeFor[Marshaler]()
	unmarshalerType = reflect.TypeFor[Unmarshaler]()
)

// isMarshaler and isUnmarshaler report whether t, or a pointer to t,
// implements Marshaler or Unmarshaler.
func isMarshaler(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(marshalerType)
}

func isUnmarshaler(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(unmarshalerType)
}

// isCustomType reports whether t formats or parses itself, so that it maps
// to a key value whatever its kind. A string, boolean or number type needs
// one method, as its kind handles the other direction. Any other type needs
// both.
func isCustomType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return isMarshaler(t) || isUnmarshaler(t)
	}
	return isMarshaler(t) && isUnmarshaler(t)
}

// isStructType reports whether t is a struct type that maps to a section,
// which is any struct type but time.Time and custom types.
func isStructType(t reflect.Type) bool {
	return isSectionsType(t) && !isCustomType(t)
}

// isSectionsType reports whether t can map to sections where they are asked
// for explicitly, by a nonunique field or a value passed for a section of a
// file with AllowNonUniqueSections: any struct type but time.Time, custom
// types too.
func isSectionsType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t != timeType
}

// isValueType reports whether a key value can be parsed as or formatted from
// a value of type t.
func isValueType(t reflect.Type) bool {
	if t == timeType || isCustomType(t) {
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
	if t == timeType || isCustomType(t) {
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
	if isUnmarshaler(t) {
		err := v.Addr().Interface().(Unmarshaler).UnmarshalINI([]byte(str))
		return v, err
	}
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
func formatValue(v reflect.Value) (string, error) {
	t := v.Type()
	if isMarshaler(t) {
		var m Marshaler
		switch {
		case t.Implements(marshalerType):
			m = v.Interface().(Marshaler)
		case v.CanAddr():
			m = v.Addr().Interface().(Marshaler)
		default:
			// MarshalINI has a pointer receiver, so call it on a copy.
			p := reflect.New(t)
			p.Elem().Set(v)
			m = p.Interface().(Marshaler)
		}
		b, err := m.MarshalINI()
		return string(b), err
	}
	if t == timeType {
		return v.Interface().(time.Time).Format(time.RFC3339Nano), nil
	}
	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Bool:
		return fmt.Sprint(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprint(v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprint(v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return fmt.Sprint(v.Float()), nil
	}
	return "", fmt.Errorf("unsupported type %q", t)
}

// defaultComment sets *c to comment if it is empty, under the file's lock.
func (f *File) defaultComment(c *string, comment string) {
	f.maybeLock()
	defer f.maybeUnlock()
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

// decodeSlice sets proper values to slice based on its type.
func decodeSlice(key *Key, field reflect.Value, delim string, allowShadow, isStrict bool) error {
	elemType := field.Type().Elem()
	if !isSliceElemType(elemType) {
		return fmt.Errorf("unsupported type '[]%s'", elemType)
	}

	var strs []string
	if allowShadow {
		// Each value is split at delim with no escapes, and each element
		// trimmed and expanded. An empty value, kept with
		// PreserveEmptyShadowValues, is an empty element, unless it is the only
		// one: an empty slice is written as one empty value.
		vals := key.Values()
		if len(vals) == 1 && vals[0] == "" {
			vals = nil
		}
		for _, v := range vals {
			for _, s := range strings.Split(v, delim) {
				strs = append(strs, key.expandValue(strings.TrimSpace(s)))
			}
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

// decodeValue sets field from the value of key. A value that doesn't parse
// leaves the field as it is, so it keeps its default, and is an error only
// when isStrict. An unsupported type is always an error.
func decodeValue(t reflect.Type, key *Key, field reflect.Value, delim string, allowShadow, isStrict bool) error {
	if t.Kind() == reflect.Slice && !isCustomType(t) {
		return decodeSlice(key, field, delim, allowShadow, isStrict)
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
	// An empty string keeps the default value of a string field, unless
	// the type parses itself.
	if vt.Kind() == reflect.String && len(str) == 0 && !isPtr && !isUnmarshaler(vt) {
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

// decodeStruct maps the given value to the matching field of the given section.
// The sectionIndex is the index (if non unique sections are enabled) to which the value should be added.
// The path holds the struct types being mapped, see tooDeep.
func (s *Section) decodeStruct(val reflect.Value, isStrict bool, sectionIndex int, sectionName string, path []reflect.Type) error {
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
			if err := fieldSection.decodeStruct(field, isStrict, sectionIndex, sectionName, path); err != nil {
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
				if err = secs[sectionIndex].decodeStruct(field, isStrict, sectionIndex, fieldName, path); err != nil {
					return fmt.Errorf("map to field %q: %v", fieldName, err)
				}
				continue
			}
		}

		// Map non-unique sections
		if allowNonUnique && tpField.Type.Kind() == reflect.Slice {
			newField, err := s.decodeSections(fieldName, field, isStrict, path)
			if err != nil {
				return fmt.Errorf("map to slice %q: %v", fieldName, err)
			}

			field.Set(newField)
			continue
		}

		if key, err := s.GetKey(fieldName); err == nil {
			delim := parseDelim(tpField.Tag.Get("delim"))
			if err = decodeValue(tpField.Type, key, field, delim, allowShadow, isStrict); err != nil {
				return fmt.Errorf("set field %q: %v", fieldName, err)
			}
		}
	}
	return nil
}

// decodeSections maps all sections with the same name and returns the new value.
// The type of the Value must be a slice of structs or of pointers to structs.
func (s *Section) decodeSections(secName string, val reflect.Value, isStrict bool, path []reflect.Type) (reflect.Value, error) {
	elemType := val.Type().Elem()
	isPtr := elemType.Kind() == reflect.Pointer
	structType := elemType
	if isPtr {
		structType = elemType.Elem()
	}
	if !isSectionsType(structType) {
		return reflect.Value{}, fmt.Errorf("unsupported type %q for non-unique sections", val.Type())
	}

	secs, err := s.f.SectionsByName(secName)
	if err != nil || tooDeep(path, structType) {
		// The slice is left as it is when there are no sections.
		return val, nil
	}

	for i, sec := range secs {
		elem := reflect.New(structType)
		if err = sec.decodeStruct(elem, isStrict, i, sec.name, path); err != nil {
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
func (s *Section) mapTo(v any, isStrict bool) error {
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
		newField, err := s.decodeSections(s.name, val, isStrict, nil)
		if err != nil {
			return err
		}

		val.Set(newField)
		return nil
	}

	return s.decodeStruct(val, isStrict, 0, s.name, nil)
}

// MapTo maps section to given struct.
func (s *Section) MapTo(v any) error {
	return s.mapTo(v, false)
}

// StrictMapTo maps section to given struct in strict mode,
// which returns all possible error including value parsing error.
func (s *Section) StrictMapTo(v any) error {
	return s.mapTo(v, true)
}

// MapTo maps file to given struct.
func (f *File) MapTo(v any) error {
	return f.Section("").MapTo(v)
}

// StrictMapTo maps file to given struct in strict mode,
// which returns all possible error including value parsing error.
func (f *File) StrictMapTo(v any) error {
	return f.Section("").StrictMapTo(v)
}

// MapTo loads a data source with opts and maps it to the given struct.
// NameMapping sets how field names map to section and key names.
func MapTo(v, source any, opts ...LoadOption) error {
	cfg, err := Load(source, opts...)
	if err != nil {
		return err
	}
	return cfg.MapTo(v)
}

// StrictMapTo is MapTo that also returns an error for a value that doesn't
// parse.
func StrictMapTo(v, source any, opts ...LoadOption) error {
	cfg, err := Load(source, opts...)
	if err != nil {
		return err
	}
	return cfg.StrictMapTo(v)
}

// encodeSlice does the opposite thing as decodeSlice.
func encodeSlice(key *Key, field reflect.Value, delim string, allowShadow bool) error {
	if field.Len() == 0 {
		return nil
	}
	elemType := field.Type().Elem()
	if !isSliceElemType(elemType) {
		return fmt.Errorf("unsupported type '[]%s'", elemType)
	}

	vals := make([]string, field.Len())
	for i := range vals {
		var err error
		if vals[i], err = formatValue(field.Index(i)); err != nil {
			return err
		}
	}

	if allowShadow {
		// Each element is a value of its own, and the reader splits values
		// at delim without escapes.
		for _, v := range vals {
			if strings.Contains(v, delim) {
				return fmt.Errorf("element %q contains the delimiter %q, which an allowshadow slice can't hold", v, delim)
			}
		}
		if len(vals) > 1 && !key.s.f.options.AllowShadows {
			return errors.New("shadow key is not allowed")
		}
		// Shadows deduplicated as the file's options say, on a key that
		// isn't boolean or auto-increment.
		tmp := newKey(key.s, key.name, vals[0])
		for _, val := range vals[1:] {
			if err := tmp.addShadow(val); err != nil {
				return err
			}
		}
		key.s.f.maybeLock()
		key.value, key.shadows = tmp.value, tmp.shadows
		key.s.f.maybeUnlock()
		return nil
	}
	for i := range vals {
		vals[i] = escapeSliceElem(vals[i], delim)
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

// encodeValue does the opposite thing as decodeValue.
func encodeValue(t reflect.Type, key *Key, field reflect.Value, delim string, allowShadow bool) error {
	switch {
	case t.Kind() == reflect.Slice && !isCustomType(t):
		return encodeSlice(key, field, delim, allowShadow)
	case t.Kind() == reflect.Pointer:
		if !field.IsNil() {
			return encodeValue(t.Elem(), key, field.Elem(), delim, allowShadow)
		}
	case isValueType(t):
		v, err := formatValue(field)
		if err != nil {
			return err
		}
		key.SetValue(v)
	default:
		return fmt.Errorf("unsupported type %q", t)
	}
	return nil
}

// isEmptyValue is encoding/json's, with a zero time.Time empty too.
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

// encodeStruct reflects the section from val. The path holds the struct types
// being reflected from, see tooDeep.
func (s *Section) encodeStruct(val reflect.Value, path []reflect.Type) error {
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
			if err := s.encodeStruct(field, path); err != nil {
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
			s.f.defaultComment(&sec.comment, tpField.Tag.Get("comment"))

			if err = sec.encodeStruct(field, path); err != nil {
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
			if !isSectionsType(structType) {
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
				s.f.defaultComment(&sec.comment, tpField.Tag.Get("comment"))

				if err := sec.encodeStruct(elem, path); err != nil {
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
		s.f.defaultComment(&key.comment, tpField.Tag.Get("comment"))

		delim := parseDelim(tpField.Tag.Get("delim"))
		if err = encodeValue(tpField.Type, key, field, delim, allowShadow); err != nil {
			return fmt.Errorf("reflect field %q: %v", fieldName, err)
		}

	}
	return nil
}

// ReflectFrom reflects section from given struct. It overwrites existing ones.
func (s *Section) ReflectFrom(v any) error {
	typ := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if typ == nil || (typ.Kind() == reflect.Pointer && val.IsNil()) {
		return errors.New("not a pointer to a struct")
	}
	// A pointer to a pointer to a struct is accepted too, as in
	// ReflectFrom(&cfg) with cfg a *Config.
	for typ.Kind() == reflect.Pointer && typ.Elem().Kind() == reflect.Pointer {
		if val = val.Elem(); val.IsNil() {
			return errors.New("not a pointer to a struct")
		}
		typ = typ.Elem()
	}

	if !s.f.isDefaultSection(s.name) && s.f.options.AllowNonUniqueSections &&
		(typ.Kind() == reflect.Slice || typ.Kind() == reflect.Pointer) {
		if typ.Kind() == reflect.Pointer && !isSectionsType(typ.Elem()) {
			return errors.New("not a pointer to a struct")
		}
		if typ.Kind() == reflect.Slice && (typ.Elem().Kind() != reflect.Pointer || !isSectionsType(typ.Elem().Elem())) {
			return errors.New("not a slice of pointers to structs")
		}

		// Clear sections to make sure none exists before adding the new ones
		s.f.DeleteSection(s.name)

		if typ.Kind() == reflect.Pointer {
			sec, err := s.f.NewSection(s.name)
			if err != nil {
				return err
			}
			return sec.encodeStruct(val.Elem(), nil)
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

			err = sec.encodeStruct(elem, nil)
			if err != nil {
				return fmt.Errorf("reflect from %dth field: %v", i, err)
			}
		}

		return nil
	}

	if typ.Kind() != reflect.Pointer || typ.Elem().Kind() != reflect.Struct {
		return errors.New("not a pointer to a struct")
	}
	s.f.maybeLock()
	s = s.attach()
	s.f.maybeUnlock()
	return s.encodeStruct(val.Elem(), nil)
}

// ReflectFrom reflects file from given struct.
func (f *File) ReflectFrom(v any) error {
	return f.Section("").ReflectFrom(v)
}
