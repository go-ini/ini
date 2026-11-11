package ini

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The Mx types cover each kind that struct mapping treats differently,
// without methods, with only MarshalINI, with only UnmarshalINI, and with
// both. A method of a type that must not use it returns errMxCalled. The
// methods of the other types read what they write, in a format that differs
// from the kind's own where both are implemented, so that the test sees which
// one ran.
var errMxCalled = errors.New("method must not be called")

type (
	MxIntNone int
	MxIntM    int
	MxIntU    int
	MxIntMU   int
)

func (v MxIntM) MarshalINI() ([]byte, error) { return []byte(strconv.Itoa(int(v))), nil }

func (v *MxIntU) UnmarshalINI(b []byte) error {
	n, err := strconv.Atoi(string(b))
	*v = MxIntU(n)
	return err
}

func (v MxIntMU) MarshalINI() ([]byte, error) { return []byte("#" + strconv.Itoa(int(v))), nil }

func (v *MxIntMU) UnmarshalINI(b []byte) error {
	s, ok := strings.CutPrefix(string(b), "#")
	if !ok {
		return fmt.Errorf("missing # in %q", b)
	}
	n, err := strconv.Atoi(s)
	*v = MxIntMU(n)
	return err
}

type (
	MxStructNone struct{ A string }
	MxStructM    struct{ A string }
	MxStructU    struct{ A string }
	MxStructMU   struct{ A string }
)

func (MxStructM) MarshalINI() ([]byte, error) { return nil, errMxCalled }

func (*MxStructU) UnmarshalINI([]byte) error { return errMxCalled }

// MarshalINI has a pointer receiver, unlike the other types.
func (v *MxStructMU) MarshalINI() ([]byte, error) { return []byte("<" + v.A + ">"), nil }

func (v *MxStructMU) UnmarshalINI(b []byte) error {
	s, ok := strings.CutPrefix(string(b), "<")
	if s, ok2 := strings.CutSuffix(s, ">"); ok && ok2 {
		v.A = s
		return nil
	}
	return fmt.Errorf("missing <> in %q", b)
}

type (
	MxSliceNone []string
	MxSliceM    []string
	MxSliceU    []string
	MxSliceMU   []string
)

func (MxSliceM) MarshalINI() ([]byte, error) { return nil, errMxCalled }

func (*MxSliceU) UnmarshalINI([]byte) error { return errMxCalled }

func (v MxSliceMU) MarshalINI() ([]byte, error) { return []byte(strings.Join(v, "+")), nil }

func (v *MxSliceMU) UnmarshalINI(b []byte) error {
	*v = nil
	if len(b) > 0 {
		*v = strings.Split(string(b), "+")
	}
	return nil
}

type (
	MxMapNone map[string]string
	MxMapM    map[string]string
	MxMapU    map[string]string
	MxMapMU   map[string]string
)

func (MxMapM) MarshalINI() ([]byte, error) { return nil, errMxCalled }

func (*MxMapU) UnmarshalINI([]byte) error { return errMxCalled }

func (v MxMapMU) MarshalINI() ([]byte, error) {
	var pairs []string
	for k, val := range v {
		pairs = append(pairs, k+":"+val)
	}
	sort.Strings(pairs)
	return []byte(strings.Join(pairs, "&")), nil
}

func (v *MxMapMU) UnmarshalINI(b []byte) error {
	*v = nil
	if len(b) == 0 {
		return nil
	}
	*v = MxMapMU{}
	for _, pair := range strings.Split(string(b), "&") {
		k, val, ok := strings.Cut(pair, ":")
		if !ok {
			return fmt.Errorf("missing : in %q", pair)
		}
		(*v)[k] = val
	}
	return nil
}

type (
	MxArrayNone [2]string
	MxArrayM    [2]string
	MxArrayU    [2]string
	MxArrayMU   [2]string
)

func (MxArrayM) MarshalINI() ([]byte, error) { return nil, errMxCalled }

func (*MxArrayU) UnmarshalINI([]byte) error { return errMxCalled }

func (v MxArrayMU) MarshalINI() ([]byte, error) { return []byte(v[0] + "~" + v[1]), nil }

func (v *MxArrayMU) UnmarshalINI(b []byte) error {
	a, c, ok := strings.Cut(string(b), "~")
	if !ok {
		return fmt.Errorf("missing ~ in %q", b)
	}
	*v = MxArrayMU{a, c}
	return nil
}

// mxSamples holds a value of each Mx type that is not the zero value.
var mxSamples = []any{
	MxIntNone(5), MxIntM(5), MxIntU(5), MxIntMU(5),
	MxStructNone{"a"}, MxStructM{"a"}, MxStructU{"a"}, MxStructMU{"a"},
	MxSliceNone{"a", "b"}, MxSliceM{"a", "b"}, MxSliceU{"a", "b"}, MxSliceMU{"a", "b"},
	MxMapNone{"k": "v"}, MxMapM{"k": "v"}, MxMapU{"k": "v"}, MxMapMU{"k": "v"},
	MxArrayNone{"a", "b"}, MxArrayM{"a", "b"}, MxArrayU{"a", "b"}, MxArrayMU{"a", "b"},
}

// mxInputs are inputs that read a value into a field F, or into F in a
// section S, in each of the forms struct mapping reads.
var mxInputs = []string{
	"F = 5\n", "F = #5\n", "F = a,b\n", "F = a+b\n", "F = <a>\n", "F = k:v\n",
	"F = <a>,<b>\n", "F = #5,#6\n", "F = a~b\n",
	"[F]\nA = a\n", "[F]\nA = a\n[F]\nA = b\n",
	"[S]\nF = 5\n", "[S]\nF = #5\n", "[S]\nF = a,b\n", "[S]\nF = a+b\n", "[S]\nF = <a>\n", "[S]\nF = k:v\n",
	"[F]\nA = a\n[S]\nB = b\n", "[S]\nA = a\nB = b\n",
}

// mxShape is a way to hold a value of a type in a struct.
type mxShape struct {
	name string
	// typ returns the struct type that holds t, or nil when t can't be
	// held that way.
	typ func(t reflect.Type) reflect.Type
	// set stores the sample value v of type t in the struct s.
	set func(s, v reflect.Value)
	// opts are the options the file is loaded and written with.
	opts []LoadOption
}

var mxShapes = []mxShape{
	{
		name: "allowshadow field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.SliceOf(t), Tag: `ini:"F,,allowshadow"`}})
		},
		set: func(s, v reflect.Value) {
			s.Field(0).Set(reflect.Append(reflect.MakeSlice(s.Field(0).Type(), 0, 2), v, v))
		},
		opts: []LoadOption{AllowShadows(true), AllowDuplicateShadowValues(true)},
	},
	{
		name: "delim field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.SliceOf(t), Tag: `delim:"|"`}})
		},
		set: func(s, v reflect.Value) {
			s.Field(0).Set(reflect.Append(reflect.MakeSlice(s.Field(0).Type(), 0, 2), v, v))
		},
	},
	{
		name: "field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: t}})
		},
		set: func(s, v reflect.Value) { s.Field(0).Set(v) },
	},
	{
		name: "pointer field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.PointerTo(t)}})
		},
		set: func(s, v reflect.Value) {
			p := reflect.New(v.Type())
			p.Elem().Set(v)
			s.Field(0).Set(p)
		},
	},
	{
		name: "slice field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.SliceOf(t)}})
		},
		set: func(s, v reflect.Value) {
			s.Field(0).Set(reflect.Append(reflect.MakeSlice(s.Field(0).Type(), 0, 2), v, v))
		},
	},
	{
		name: "double pointer field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.PointerTo(reflect.PointerTo(t))}})
		},
		set: func(s, v reflect.Value) {
			p := reflect.New(v.Type())
			p.Elem().Set(v)
			pp := reflect.New(p.Type())
			pp.Elem().Set(p)
			s.Field(0).Set(pp)
		},
	},
	{
		name: "array field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.ArrayOf(2, t)}})
		},
		set: func(s, v reflect.Value) {
			s.Field(0).Index(0).Set(v)
			s.Field(0).Index(1).Set(v)
		},
	},
	{
		name: "interface field",
		typ: func(reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.TypeFor[any]()}})
		},
		set: func(s, v reflect.Value) { s.Field(0).Set(v) },
	},
	{
		name: "field in a section",
		typ: func(t reflect.Type) reflect.Type {
			inner := reflect.StructOf([]reflect.StructField{{Name: "F", Type: t}})
			return reflect.StructOf([]reflect.StructField{{Name: "S", Type: inner}})
		},
		set: func(s, v reflect.Value) { s.Field(0).Field(0).Set(v) },
	},
	{
		name: "nonunique field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.SliceOf(t), Tag: `ini:"F,,,nonunique"`}})
		},
		set: func(s, v reflect.Value) {
			s.Field(0).Set(reflect.Append(reflect.MakeSlice(s.Field(0).Type(), 0, 2), v, v))
		},
		opts: []LoadOption{AllowNonUniqueSections(true)},
	},
	{
		name: "nonunique pointer field",
		typ: func(t reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "F", Type: reflect.SliceOf(reflect.PointerTo(t)), Tag: `ini:"F,,,nonunique"`}})
		},
		set: func(s, v reflect.Value) {
			p := reflect.New(v.Type())
			p.Elem().Set(v)
			s.Field(0).Set(reflect.Append(reflect.MakeSlice(s.Field(0).Type(), 0, 2), p, p))
		},
		opts: []LoadOption{AllowNonUniqueSections(true)},
	},
}

// mxResult is what happened to one value: "ok" when it was written and read
// back the same, or the step that failed.
func mxRoundTrip(in reflect.Value, opts []LoadOption) string {
	f := Empty(opts...)
	if err := f.ReflectFrom(in.Addr().Interface()); err != nil {
		return "write error"
	}
	var buf bytes.Buffer
	if _, err := f.Write(&buf); err != nil {
		return "write error"
	}
	out := reflect.New(in.Type())
	if err := StrictMapTo(out.Interface(), buf.Bytes(), opts...); err != nil {
		return fmt.Sprintf("written %q but StrictMapTo failed: %v", buf.String(), err)
	}
	if !reflect.DeepEqual(out.Elem().Interface(), in.Interface()) {
		return fmt.Sprintf("written %q but read back %+v", buf.String(), out.Elem().Interface())
	}
	lax := reflect.New(in.Type())
	if err := MapTo(lax.Interface(), buf.Bytes(), opts...); err != nil || !reflect.DeepEqual(lax.Elem().Interface(), in.Interface()) {
		return fmt.Sprintf("written %q but MapTo read back %+v, %v", buf.String(), lax.Elem().Interface(), err)
	}
	return "ok"
}

// mxReads returns the first of mxInputs that StrictMapTo reads into a value
// of type t that is not the zero value, or "".
func mxReads(t reflect.Type, opts []LoadOption) string {
	for _, in := range mxInputs {
		out := reflect.New(t)
		if err := StrictMapTo(out.Interface(), []byte(in), opts...); err == nil && !out.Elem().IsZero() {
			return in
		}
	}
	return ""
}

// TestStructMappingMatrix checks every Mx type in every shape: a value is
// either written and read back the same by both MapTo and StrictMapTo, or
// can't be written, and then no input reads a value of its type either.
func TestStructMappingMatrix(t *testing.T) {
	for _, shape := range mxShapes {
		for _, sample := range mxSamples {
			v := reflect.ValueOf(sample)
			name := shape.name + "/" + v.Type().Name()
			st := shape.typ(v.Type())
			opts := shape.opts

			in := reflect.New(st).Elem()
			shape.set(in, v)
			res := mxRoundTrip(in, opts)
			if res == "write error" {
				if read := mxReads(st, opts); read != "" {
					res = fmt.Sprintf("can't be written, but %q reads", read)
				}
			}
			t.Logf("%s: %s", name, res)
			if res != "ok" && res != "write error" {
				t.Errorf("%s: %s", name, res)
			}
		}
	}
}

// The MxEmbed and MxExtends types embed each Mx struct type, plainly and
// with the extends tag.
type (
	MxEmbedNone struct {
		MxStructNone
		B string
	}
	MxEmbedM struct {
		MxStructM
		B string
	}
	MxEmbedU struct {
		MxStructU
		B string
	}
	MxEmbedMU struct {
		MxStructMU
		B string
	}
	MxExtendsNone struct {
		MxStructNone `ini:",,,,extends"`
		B            string
	}
	MxExtendsM struct {
		MxStructM `ini:",,,,extends"`
		B         string
	}
	MxExtendsU struct {
		MxStructU `ini:",,,,extends"`
		B         string
	}
	MxExtendsMU struct {
		MxStructMU `ini:",,,,extends"`
		B          string
	}
)

// TestStructMappingMatrixEmbedded checks the same for embedded struct types,
// held directly and in a section.
func TestStructMappingMatrixEmbedded(t *testing.T) {
	for _, v := range []any{
		MxEmbedNone{MxStructNone{"a"}, "b"}, MxEmbedM{MxStructM{"a"}, "b"},
		MxEmbedU{MxStructU{"a"}, "b"}, MxEmbedMU{MxStructMU{"a"}, "b"},
		MxExtendsNone{MxStructNone{"a"}, "b"}, MxExtendsM{MxStructM{"a"}, "b"},
		MxExtendsU{MxStructU{"a"}, "b"}, MxExtendsMU{MxStructMU{"a"}, "b"},
	} {
		for _, shape := range mxShapes {
			if shape.name != "field" {
				continue
			}
			rv := reflect.ValueOf(v)
			for _, held := range []struct {
				name string
				typ  reflect.Type
				set  func(s reflect.Value)
			}{
				{"top level", rv.Type(), func(s reflect.Value) { s.Set(rv) }},
				{shape.name, shape.typ(rv.Type()), func(s reflect.Value) { shape.set(s, rv) }},
			} {
				name := held.name + "/" + rv.Type().Name()
				in := reflect.New(held.typ).Elem()
				held.set(in)
				if held.name != "top level" && strings.HasSuffix(name, "MU") {
					// The embedded type's methods are promoted, so the field
					// maps to one key, as Unmarshaler says, and B isn't kept.
					f := Empty()
					if err := f.ReflectFrom(in.Addr().Interface()); err != nil {
						t.Errorf("%s: %v", name, err)
					}
					var buf bytes.Buffer
					_, _ = f.Write(&buf)
					if buf.String() != "F = <a>\n" {
						t.Errorf("%s: written %q, want one key", name, buf.String())
					}
					continue
				}
				res := mxRoundTrip(in, nil)
				if res == "write error" {
					if read := mxReads(held.typ, nil); read != "" {
						res = fmt.Sprintf("can't be written, but %q reads", read)
					}
				}
				t.Logf("%s: %s", name, res)
				if res != "ok" && res != "write error" {
					t.Errorf("%s: %s", name, res)
				}
			}
		}
	}
}

// TestStructMappingMatrixSections checks the same for values passed for a
// section, as one struct and, with AllowNonUniqueSections, as several.
func TestStructMappingMatrixSections(t *testing.T) {
	for _, sample := range mxSamples[4:8] {
		v := reflect.ValueOf(sample)
		for _, nonUnique := range []bool{false, true} {
			var opts []LoadOption
			if nonUnique {
				opts = []LoadOption{AllowNonUniqueSections(true)}
			}
			cases := []struct {
				name string
				in   reflect.Value
			}{{"struct", v}}
			if nonUnique {
				p := reflect.New(v.Type())
				p.Elem().Set(v)
				cases = append(cases, struct {
					name string
					in   reflect.Value
				}{"slice", reflect.Append(reflect.MakeSlice(reflect.SliceOf(p.Type()), 0, 2), p, p)})
			}
			for _, c := range cases {
				name := fmt.Sprintf("%s/%s/nonunique=%v", c.name, v.Type().Name(), nonUnique)
				res := mxSectionRoundTrip(c.in, opts)
				t.Logf("%s: %s", name, res)
				if res != "ok" && res != "write and read error" {
					t.Errorf("%s: %s", name, res)
				}
			}
		}
	}
}

// mxSectionRoundTrip writes in, a struct or a slice of pointers to structs,
// to section S with Section.ReflectFrom and reads it back with
// Section.StrictMapTo.
func mxSectionRoundTrip(in reflect.Value, opts []LoadOption) string {
	arg := in.Interface()
	if in.Kind() == reflect.Struct {
		p := reflect.New(in.Type())
		p.Elem().Set(in)
		arg = p.Interface()
	}
	f := Empty(opts...)
	werr := f.Section("S").ReflectFrom(arg)
	var buf bytes.Buffer
	if werr == nil {
		_, werr = f.Write(&buf)
	}

	read := func(data string) (reflect.Value, error) {
		g, err := Load([]byte(data), opts...)
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(in.Type())
		return out.Elem(), g.Section("S").StrictMapTo(out.Interface())
	}
	if werr != nil {
		if out, err := read("[S]\nA = a\n[S]\nA = b\n"); err == nil && !out.IsZero() {
			return fmt.Sprintf("can't be written (%v), but reads %+v", werr, out.Interface())
		}
		return "write and read error"
	}
	out, err := read(buf.String())
	if err != nil {
		return fmt.Sprintf("written %q but StrictMapTo failed: %v", buf.String(), err)
	}
	if !reflect.DeepEqual(out.Interface(), in.Interface()) {
		return fmt.Sprintf("written %q but read back %+v", buf.String(), out.Interface())
	}
	return "ok"
}
