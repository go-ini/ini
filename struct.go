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
	"time"
	"unicode"
	"strings"
)

// NameMapper represents a ini tag name mapper.
type NameMapper func(string) string

// Built-in name getters.
var (
	// AllCapsUnderscore converts to format ALL_CAPS_UNDERSCORE.
	AllCapsUnderscore NameMapper = func(raw string) string {
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
				chr -= ('A' - 'a')
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

var reflectTime = reflect.TypeOf(time.Now()).Kind()

// setWithProperType sets proper value to field based on its type,
// but it does not return error for failing parsing,
// because we want to use default value that is already assigned to strcut.
func setWithProperType(t reflect.Type, key *Key, field reflect.Value, delim string) error {
	return setWithProperTypeWithFlags(t, key, field, delim, make(map[string]bool))
}

func handleParseError(key *Key, flags map[string]bool, targetTypeName string) error {
	if flags["strictParse"] {
		var val string
		if targetTypeName == "string" {
			val = "<string parse failure>"
		} else {
			val = key.String()
			if len(val) == 0 {
				val = "<empty or could not parse as string>"
			}
		}
		return fmt.Errorf("strictParse set on field, but '%s' could not parse as %s.", val, targetTypeName)
	}
	return nil
}

func setIntLike(key *Key, field reflect.Value, retrievedValue reflect.Value, retrievalError error, flags map[string]bool, fieldTypeName string) error {
	// force parse failure errors so upstream can handle
	strictFlags := make(map[string]bool)
	for flag, value := range flags {
		strictFlags[flag] = value
	}
	strictFlags["strictParse"] = true
	if retrievalError != nil {
		return handleParseError(key, strictFlags, fieldTypeName)
	} else if retrievedValue.Int() != 0 {
		field.SetInt(retrievedValue.Int())
		return nil
	// Parse duration returns error on empty string
	} else if fieldTypeName == "duration" && retrievedValue.Int() == 0 {
		field.SetInt(retrievedValue.Int())
		return nil
	} else {
		str := key.String()
		if str != "0" && str != "-0" {
			return handleParseError(key, strictFlags, fieldTypeName)
		} else {
			field.SetInt(retrievedValue.Int())
			return nil
		}
	}
}

func setUintLike(key *Key, field reflect.Value, retrievedValue reflect.Value, retrievalError error, flags map[string]bool, fieldTypeName string) error {
	// force parse failure errors so upstream can handle
	strictFlags := make(map[string]bool)
	for flag, value := range flags {
		strictFlags[flag] = value
	}
	strictFlags["strictParse"] = true
	if retrievalError != nil {
		return handleParseError(key, strictFlags, fieldTypeName)
	} else if retrievedValue.Uint() != 0 {
		field.SetUint(retrievedValue.Uint())
		return nil
	// Parse duration returns error on empty string
	} else if fieldTypeName == "duration" && retrievedValue.Int() == 0 {
		field.SetUint(retrievedValue.Uint())
		return nil
	} else {
		str := key.String()
		if str != "0" && str != "-0" {
			return handleParseError(key, strictFlags, fieldTypeName)
		} else {
			field.SetUint(retrievedValue.Uint())
			return nil
		}
	}
}


// setWithProperTypeWithFlags sets proper value to field based on its type,
// returning errors during parsing based on flags provided.
func setWithProperTypeWithFlags(t reflect.Type, key *Key, field reflect.Value, delim string, flags map[string]bool) error {
	switch t.Kind() {
	case reflect.String:
		if len(key.String()) == 0 {
			return handleParseError(key, flags, "string")
		}
		field.SetString(key.String())
	case reflect.Bool:
		boolVal, err := key.Bool()
		if err != nil {
			return handleParseError(key, flags, "bool")
		}
		field.SetBool(boolVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := key.Int64()
		intErr := setIntLike(key, field, reflect.ValueOf(intVal), err, flags, "int")
		if intErr != nil {
			durVal, err := key.Duration()
			durErr := setIntLike(key, field, reflect.ValueOf(durVal), err, flags, "duration")
			if durErr != nil {
				if flags["strictParse"] {
					return intErr
				} else {
					return nil
				}
			}
		}
	//	byte is an alias for uint8, so supporting uint8 breaks support for byte
	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := key.Uint64()
		intErr := setUintLike(key, field, reflect.ValueOf(uintVal), err, flags, "uint")
		if intErr != nil {
			durVal, err := key.Duration()
			durErr := setUintLike(key, field, reflect.ValueOf(durVal), err, flags, "duration")
			if durErr != nil {
				if flags["strictParse"] {
					return intErr
				} else {
					return nil
				}
			}
		}

	case reflect.Float64:
		floatVal, err := key.Float64()
		if err != nil {
			return handleParseError(key, flags, "float")
		}
		field.SetFloat(floatVal)
	case reflectTime:
		timeVal, err := key.Time()
		if err != nil {
			return handleParseError(key, flags, "time")
		}
		field.Set(reflect.ValueOf(timeVal))
	case reflect.Slice:
		vals := key.Strings(delim)
		numVals := len(vals)
		if numVals == 0 {
			return handleParseError(key, flags, "slice")
		}

		sliceOf := field.Type().Elem().Kind()

		var times []time.Time
		if sliceOf == reflectTime {
			times = key.Times(delim)
		}

		slice := reflect.MakeSlice(field.Type(), numVals, numVals)
		for i := 0; i < numVals; i++ {
			switch sliceOf {
			case reflectTime:
				slice.Index(i).Set(reflect.ValueOf(times[i]))
			default:
				slice.Index(i).Set(reflect.ValueOf(vals[i]))
			}
		}
		field.Set(slice)
	default:
		return fmt.Errorf("unsupported type '%s'", t)
	}
	return nil
}

func (s *Section) mapTo(val reflect.Value) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := val.Field(i)
		tpField := typ.Field(i)

		tag := tpField.Tag.Get("ini")
		flags := make(map[string]bool)
		for _, flag := range strings.Split(tpField.Tag.Get("iniFlags"), ";") {
			flags[flag] = true
		}
		if tag == "-" {
			continue
		}

		fieldName := s.parseFieldName(tpField.Name, tag)
		if len(fieldName) == 0 || !field.CanSet() {
			continue
		}

		isAnonymous := tpField.Type.Kind() == reflect.Ptr && tpField.Anonymous
		isStruct := tpField.Type.Kind() == reflect.Struct
		if isAnonymous {
			field.Set(reflect.New(tpField.Type.Elem()))
		}

		if isAnonymous || isStruct {
			if sec, err := s.f.GetSection(fieldName); err == nil {
				if err = sec.mapTo(field); err != nil {
					return fmt.Errorf("error mapping field(%s): %v", fieldName, err)
				}
				continue
			} else if flags["mustExist"] {
				return fmt.Errorf("%s defined with mustExist but field(%s) not found in loaded data: %v", tpField.Name, fieldName, err)
			}
		}

		if key, err := s.GetKey(fieldName); err == nil {
			if err = setWithProperTypeWithFlags(tpField.Type, key, field, parseDelim(tpField.Tag.Get("delim")), flags); err != nil {
				return fmt.Errorf("error mapping field(%s): %v", fieldName, err)
			}
		} else if flags["mustExist"] {
			return fmt.Errorf("%s defined with mustExist but field(%s) not found in loaded data: %v", tpField.Name, fieldName, err)
		}
	}
	return nil
}

// MapTo maps section to given struct.
func (s *Section) MapTo(v interface{}) error {
	typ := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	} else {
		return errors.New("cannot map to non-pointer struct")
	}

	return s.mapTo(val)
}

// MapTo maps file to given struct.
func (f *File) MapTo(v interface{}) error {
	return f.Section("").MapTo(v)
}

// MapTo maps data sources to given struct with name mapper.
func MapToWithMapper(v interface{}, mapper NameMapper, source interface{}, others ...interface{}) error {
	cfg, err := Load(source, others...)
	if err != nil {
		return err
	}
	cfg.NameMapper = mapper
	return cfg.MapTo(v)
}

// MapTo maps data sources to given struct.
func MapTo(v, source interface{}, others ...interface{}) error {
	return MapToWithMapper(v, nil, source, others...)
}

// reflectWithProperType does the opposite thing with setWithProperType.
func reflectWithProperType(t reflect.Type, key *Key, field reflect.Value, delim string) error {
	switch t.Kind() {
	case reflect.String:
		key.SetValue(field.String())
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float64,
		reflectTime:
		key.SetValue(fmt.Sprint(field))
	case reflect.Slice:
		vals := field.Slice(0, field.Len())
		if field.Len() == 0 {
			return nil
		}

		var buf bytes.Buffer
		isTime := fmt.Sprint(field.Type()) == "[]time.Time"
		for i := 0; i < field.Len(); i++ {
			if isTime {
				buf.WriteString(vals.Index(i).Interface().(time.Time).Format(time.RFC3339))
			} else {
				buf.WriteString(fmt.Sprint(vals.Index(i)))
			}
			buf.WriteString(delim)
		}
		key.SetValue(buf.String()[:buf.Len()-1])
	default:
		return fmt.Errorf("unsupported type '%s'", t)
	}
	return nil
}

func (s *Section) reflectFrom(val reflect.Value) error {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := val.Field(i)
		tpField := typ.Field(i)

		tag := tpField.Tag.Get("ini")
		if tag == "-" {
			continue
		}

		fieldName := s.parseFieldName(tpField.Name, tag)
		if len(fieldName) == 0 || !field.CanSet() {
			continue
		}

		if (tpField.Type.Kind() == reflect.Ptr && tpField.Anonymous) ||
			(tpField.Type.Kind() == reflect.Struct) {
			// Note: The only error here is section doesn't exist.
			sec, err := s.f.GetSection(fieldName)
			if err != nil {
				// Note: fieldName can never be empty here, ignore error.
				sec, _ = s.f.NewSection(fieldName)
			}
			if err = sec.reflectFrom(field); err != nil {
				return fmt.Errorf("error reflecting field(%s): %v", fieldName, err)
			}
			continue
		}

		// Note: Same reason as secion.
		key, err := s.GetKey(fieldName)
		if err != nil {
			key, _ = s.NewKey(fieldName, "")
		}
		if err = reflectWithProperType(tpField.Type, key, field, parseDelim(tpField.Tag.Get("delim"))); err != nil {
			return fmt.Errorf("error reflecting field(%s): %v", fieldName, err)
		}

	}
	return nil
}

// ReflectFrom reflects secion from given struct.
func (s *Section) ReflectFrom(v interface{}) error {
	typ := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	} else {
		return errors.New("cannot reflect from non-pointer struct")
	}

	return s.reflectFrom(val)
}

// ReflectFrom reflects file from given struct.
func (f *File) ReflectFrom(v interface{}) error {
	return f.Section("").ReflectFrom(v)
}

// ReflectFrom reflects data sources from given struct with name mapper.
func ReflectFromWithMapper(cfg *File, v interface{}, mapper NameMapper) error {
	cfg.NameMapper = mapper
	return cfg.ReflectFrom(v)
}

// ReflectFrom reflects data sources from given struct.
func ReflectFrom(cfg *File, v interface{}) error {
	return ReflectFromWithMapper(cfg, v, nil)
}
