package ini

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNested struct {
	Cities      []string `delim:"|"`
	Visits      []time.Time
	Years       []int
	Numbers     []int64
	Ages        []uint
	Populations []uint64
	Coordinates []float64
	Flags       []bool
	Note        string
	Unused      int `ini:"-"`
}

type TestEmbeded struct {
	GPA float64
}

type testStruct struct {
	Name           string `ini:"NAME"`
	Age            int
	Male           bool
	Money          float64
	Born           time.Time
	Time           time.Duration `ini:"Duration"`
	OldVersionTime time.Duration
	Others         testNested
	OthersPtr      *testNested
	NilPtr         *testNested
	*TestEmbeded   `ini:"grade"`
	Unused         int `ini:"-"`
	Unsigned       uint
	Omitted        bool     `ini:"omitthis,omitempty"`
	Shadows        []string `ini:",allowshadow"`
	ShadowInts     []int    `ini:"Shadows,allowshadow"`
	BoolPtr        *bool
	BoolPtrNil     *bool
	FloatPtr       *float64
	FloatPtrNil    *float64
	IntPtr         *int
	IntPtrNil      *int
	UintPtr        *uint
	UintPtrNil     *uint
	StringPtr      *string
	StringPtrNil   *string
	TimePtr        *time.Time
	TimePtrNil     *time.Time
	DurationPtr    *time.Duration
	DurationPtrNil *time.Duration
}

type testInterface struct {
	Address    string
	ListenPort int
	PrivateKey string
}

type testPeer struct {
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string `delim:","`
}

type testNonUniqueSectionsStruct struct {
	Interface testInterface
	Peer      []testPeer `ini:",nonunique"`
}

type BaseStruct struct {
	Base bool
}

type testExtend struct {
	BaseStruct `ini:",extends"`
	Extend     bool
}

const confDataStruct = `
NAME = Unknwon
Age = 21
Male = true
Money = 1.25
Born = 1993-10-07T20:17:05Z
Duration = 2h45m
OldVersionTime = 30
Unsigned = 3
omitthis = true
Shadows = 1, 2
Shadows = 3, 4
BoolPtr = false
FloatPtr = 0
IntPtr = 0
UintPtr = 0
StringPtr = ""
TimePtr = 0001-01-01T00:00:00Z
DurationPtr = 0s

[Others]
Cities = HangZhou|Boston
Visits = 1993-10-07T20:17:05Z, 1993-10-07T20:17:05Z
Years = 1993,1994
Numbers = 10010,10086
Ages = 18,19
Populations = 12345678,98765432
Coordinates = 192.168,10.11
Flags       = true,false
Note = Hello world!

[OthersPtr]
Cities = HangZhou|Boston
Visits = 1993-10-07T20:17:05Z, 1993-10-07T20:17:05Z
Years = 1993,1994
Numbers = 10010,10086
Ages = 18,19
Populations = 12345678,98765432
Coordinates = 192.168,10.11
Flags       = true,false
Note = Hello world!

[grade]
GPA = 2.8

[foo.bar]
Here = there
When = then

[extended]
Base = true
Extend = true
`

const confNonUniqueSectionDataStruct = `[Interface]
Address    = 10.2.0.1/24
ListenPort = 34777
PrivateKey = privServerKey

[Peer]
PublicKey    = pubClientKey
PresharedKey = psKey
AllowedIPs   = 10.2.0.2/32,fd00:2::2/128

[Peer]
PublicKey    = pubClientKey2
PresharedKey = psKey2
AllowedIPs   = 10.2.0.3/32,fd00:2::3/128
`

// complex128 is supported neither as a key value nor as a section.
type unsupport struct {
	Byte complex128
}

type unsupport2 struct {
	Others struct {
		Cities complex128
	}
}

type Unsupport3 struct {
	Cities complex128
}

type unsupport4 struct {
	*Unsupport3 `ini:"Others"`
}

type defaultValue struct {
	Name     string
	Age      int
	Male     bool
	Optional *bool
	Money    float64
	Born     time.Time
	Cities   []string
}

type fooBar struct {
	Here, When string
}

const invalidDataConfStruct = `
Name = 
Age = age
Male = 123
Money = money
Born = nil
Cities = 
`

func Test_MapToStruct(t *testing.T) {
	t.Run("map to struct", func(t *testing.T) {
		t.Run("map file to struct", func(t *testing.T) {
			ts := new(testStruct)
			assert.NoError(t, MapTo(ts, []byte(confDataStruct)))

			assert.Equal(t, "Unknwon", ts.Name)
			assert.Equal(t, 21, ts.Age)
			assert.True(t, ts.Male)
			assert.Equal(t, 1.25, ts.Money)
			assert.Equal(t, uint(3), ts.Unsigned)

			ti, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
			require.NoError(t, err)
			assert.Equal(t, ti.String(), ts.Born.String())

			dur, err := time.ParseDuration("2h45m")
			require.NoError(t, err)
			assert.Equal(t, dur.Seconds(), ts.Time.Seconds())

			assert.Equal(t, 30*time.Second, ts.OldVersionTime*time.Second)

			assert.Equal(t, "HangZhou,Boston", strings.Join(ts.Others.Cities, ","))
			assert.Equal(t, ti.String(), ts.Others.Visits[0].String())
			assert.Equal(t, "[1993 1994]", fmt.Sprint(ts.Others.Years))
			assert.Equal(t, "[10010 10086]", fmt.Sprint(ts.Others.Numbers))
			assert.Equal(t, "[18 19]", fmt.Sprint(ts.Others.Ages))
			assert.Equal(t, "[12345678 98765432]", fmt.Sprint(ts.Others.Populations))
			assert.Equal(t, "[192.168 10.11]", fmt.Sprint(ts.Others.Coordinates))
			assert.Equal(t, "[true false]", fmt.Sprint(ts.Others.Flags))
			assert.Equal(t, "Hello world!", ts.Others.Note)
			assert.Equal(t, 2.8, ts.GPA)

			assert.Equal(t, "HangZhou,Boston", strings.Join(ts.OthersPtr.Cities, ","))
			assert.Equal(t, ti.String(), ts.OthersPtr.Visits[0].String())
			assert.Equal(t, "[1993 1994]", fmt.Sprint(ts.OthersPtr.Years))
			assert.Equal(t, "[10010 10086]", fmt.Sprint(ts.OthersPtr.Numbers))
			assert.Equal(t, "[18 19]", fmt.Sprint(ts.OthersPtr.Ages))
			assert.Equal(t, "[12345678 98765432]", fmt.Sprint(ts.OthersPtr.Populations))
			assert.Equal(t, "[192.168 10.11]", fmt.Sprint(ts.OthersPtr.Coordinates))
			assert.Equal(t, "[true false]", fmt.Sprint(ts.OthersPtr.Flags))
			assert.Equal(t, "Hello world!", ts.OthersPtr.Note)

			assert.Nil(t, ts.NilPtr)

			assert.Equal(t, false, *ts.BoolPtr)
			assert.Nil(t, ts.BoolPtrNil)
			assert.Equal(t, float64(0), *ts.FloatPtr)
			assert.Nil(t, ts.FloatPtrNil)
			assert.Equal(t, 0, *ts.IntPtr)
			assert.Nil(t, ts.IntPtrNil)
			assert.Equal(t, uint(0), *ts.UintPtr)
			assert.Nil(t, ts.UintPtrNil)
			assert.Equal(t, "", *ts.StringPtr)
			assert.Nil(t, ts.StringPtrNil)
			assert.NotNil(t, *ts.TimePtr)
			assert.Nil(t, ts.TimePtrNil)
			assert.Equal(t, time.Duration(0), *ts.DurationPtr)
			assert.Nil(t, ts.DurationPtrNil)
		})

		t.Run("map section to struct", func(t *testing.T) {
			foobar := new(fooBar)
			f, err := Load([]byte(confDataStruct))
			require.NoError(t, err)

			assert.NoError(t, f.Section("foo.bar").MapTo(foobar))
			assert.Equal(t, "there", foobar.Here)
			assert.Equal(t, "then", foobar.When)
		})

		t.Run("map to non-pointer struct", func(t *testing.T) {
			f, err := Load([]byte(confDataStruct))
			require.NoError(t, err)
			require.NotNil(t, f)

			assert.Error(t, f.MapTo(testStruct{}))
		})

		t.Run("map to unsupported type", func(t *testing.T) {
			f, err := Load([]byte(confDataStruct), NameMapping(func(raw string) string {
				if raw == "Byte" {
					return "NAME"
				}
				return raw
			}))
			require.NoError(t, err)
			require.NotNil(t, f)
			assert.Error(t, f.MapTo(&unsupport{}))
			assert.Error(t, f.MapTo(&unsupport2{}))
			assert.Error(t, f.MapTo(&unsupport4{}))
		})

		t.Run("map to omitempty field", func(t *testing.T) {
			ts := new(testStruct)
			assert.NoError(t, MapTo(ts, []byte(confDataStruct)))

			assert.Equal(t, true, ts.Omitted)
		})

		t.Run("map with shadows", func(t *testing.T) {
			f, err := Load([]byte(confDataStruct), AllowShadows(true))
			require.NoError(t, err)
			ts := new(testStruct)
			assert.NoError(t, f.MapTo(ts))

			assert.Equal(t, "1 2 3 4", strings.Join(ts.Shadows, " "))
			assert.Equal(t, "[1 2 3 4]", fmt.Sprintf("%v", ts.ShadowInts))
		})

		t.Run("map from invalid data source", func(t *testing.T) {
			assert.Error(t, MapTo(&testStruct{}, "hi"))
		})

		t.Run("map to wrong types and gain default values", func(t *testing.T) {
			f, err := Load([]byte(invalidDataConfStruct))
			require.NoError(t, err)

			ti, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
			require.NoError(t, err)
			dv := &defaultValue{"Joe", 10, true, nil, 1.25, ti, []string{"HangZhou", "Boston"}}
			assert.NoError(t, f.MapTo(dv))
			assert.Equal(t, "Joe", dv.Name)
			assert.Equal(t, 10, dv.Age)
			assert.True(t, dv.Male)
			assert.Equal(t, 1.25, dv.Money)
			assert.Equal(t, ti.String(), dv.Born.String())
			assert.Equal(t, "HangZhou,Boston", strings.Join(dv.Cities, ","))
		})

		t.Run("map to extended base", func(t *testing.T) {
			f, err := Load([]byte(confDataStruct))
			require.NoError(t, err)
			require.NotNil(t, f)
			te := testExtend{}
			assert.NoError(t, f.Section("extended").MapTo(&te))
			assert.True(t, te.Base)
			assert.True(t, te.Extend)
		})
	})

	t.Run("map to struct in strict mode", func(t *testing.T) {
		f, err := Load([]byte(`
name=bruce
age=a30`))
		require.NoError(t, err)

		type Strict struct {
			Name string `ini:"name"`
			Age  int    `ini:"age"`
		}
		s := new(Strict)

		assert.Error(t, f.Section("").StrictMapTo(s))
	})

	t.Run("map slice in strict mode", func(t *testing.T) {
		f, err := Load([]byte(`
names=alice, bruce`))
		require.NoError(t, err)

		type Strict struct {
			Names []string `ini:"names"`
		}
		s := new(Strict)

		assert.NoError(t, f.Section("").StrictMapTo(s))
		assert.Equal(t, "[alice bruce]", fmt.Sprint(s.Names))
	})
}

func Test_MapToStructNonUniqueSections(t *testing.T) {
	t.Run("map to struct non unique", func(t *testing.T) {
		t.Run("map file to struct non unique", func(t *testing.T) {
			f, err := Load([]byte(confNonUniqueSectionDataStruct), AllowNonUniqueSections(true))
			require.NoError(t, err)
			ts := new(testNonUniqueSectionsStruct)

			assert.NoError(t, f.MapTo(ts))

			assert.Equal(t, "10.2.0.1/24", ts.Interface.Address)
			assert.Equal(t, 34777, ts.Interface.ListenPort)
			assert.Equal(t, "privServerKey", ts.Interface.PrivateKey)

			assert.Equal(t, "pubClientKey", ts.Peer[0].PublicKey)
			assert.Equal(t, "psKey", ts.Peer[0].PresharedKey)
			assert.Equal(t, "10.2.0.2/32", ts.Peer[0].AllowedIPs[0])
			assert.Equal(t, "fd00:2::2/128", ts.Peer[0].AllowedIPs[1])

			assert.Equal(t, "pubClientKey2", ts.Peer[1].PublicKey)
			assert.Equal(t, "psKey2", ts.Peer[1].PresharedKey)
			assert.Equal(t, "10.2.0.3/32", ts.Peer[1].AllowedIPs[0])
			assert.Equal(t, "fd00:2::3/128", ts.Peer[1].AllowedIPs[1])
		})

		t.Run("map non unique section to struct", func(t *testing.T) {
			newPeer := new(testPeer)
			newPeerSlice := make([]testPeer, 0)

			f, err := Load([]byte(confNonUniqueSectionDataStruct), AllowNonUniqueSections(true))
			require.NoError(t, err)

			// try only first one
			assert.NoError(t, f.Section("Peer").MapTo(newPeer))
			assert.Equal(t, "pubClientKey", newPeer.PublicKey)
			assert.Equal(t, "psKey", newPeer.PresharedKey)
			assert.Equal(t, "10.2.0.2/32", newPeer.AllowedIPs[0])
			assert.Equal(t, "fd00:2::2/128", newPeer.AllowedIPs[1])

			// try all
			assert.NoError(t, f.Section("Peer").MapTo(&newPeerSlice))
			assert.Equal(t, "pubClientKey", newPeerSlice[0].PublicKey)
			assert.Equal(t, "psKey", newPeerSlice[0].PresharedKey)
			assert.Equal(t, "10.2.0.2/32", newPeerSlice[0].AllowedIPs[0])
			assert.Equal(t, "fd00:2::2/128", newPeerSlice[0].AllowedIPs[1])

			assert.Equal(t, "pubClientKey2", newPeerSlice[1].PublicKey)
			assert.Equal(t, "psKey2", newPeerSlice[1].PresharedKey)
			assert.Equal(t, "10.2.0.3/32", newPeerSlice[1].AllowedIPs[0])
			assert.Equal(t, "fd00:2::3/128", newPeerSlice[1].AllowedIPs[1])
		})

		t.Run("map non unique sections with subsections to struct", func(t *testing.T) {
			iniFile, err := Load(strings.NewReader(`
[Section]
FieldInSubSection = 1
FieldInSubSection2 = 2
FieldInSection = 3

[Section]
FieldInSubSection = 4
FieldInSubSection2 = 5
FieldInSection = 6
`), AllowNonUniqueSections(true))
			require.NoError(t, err)

			type SubSection struct {
				FieldInSubSection string `ini:"FieldInSubSection"`
			}
			type SubSection2 struct {
				FieldInSubSection2 string `ini:"FieldInSubSection2"`
			}

			type Section struct {
				SubSection     `ini:"Section"`
				SubSection2    `ini:"Section"`
				FieldInSection string `ini:"FieldInSection"`
			}

			type File struct {
				Sections []Section `ini:"Section,nonunique"`
			}

			f := new(File)
			err = iniFile.MapTo(f)
			require.NoError(t, err)

			assert.Equal(t, "1", f.Sections[0].FieldInSubSection)
			assert.Equal(t, "2", f.Sections[0].FieldInSubSection2)
			assert.Equal(t, "3", f.Sections[0].FieldInSection)

			assert.Equal(t, "4", f.Sections[1].FieldInSubSection)
			assert.Equal(t, "5", f.Sections[1].FieldInSubSection2)
			assert.Equal(t, "6", f.Sections[1].FieldInSection)
		})
	})
}

func Test_ReflectFromStruct(t *testing.T) {
	t.Run("reflect from struct", func(t *testing.T) {
		type Embeded struct {
			Dates       []time.Time `delim:"|" comment:"Time data"`
			Places      []string
			Years       []int
			Numbers     []int64
			Ages        []uint
			Populations []uint64
			Coordinates []float64
			Flags       []bool
			None        []int
		}
		type Author struct {
			Name      string `ini:"NAME"`
			Male      bool
			Optional  *bool
			Age       int `comment:"Author's age"`
			Height    uint
			GPA       float64
			Date      time.Time
			NeverMind string `ini:"-"`
			ignored   string
			*Embeded  `ini:"infos" comment:"Embeded section"`
		}

		ti, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
		require.NoError(t, err)
		a := &Author{"Unknwon", true, nil, 21, 100, 2.8, ti, "", "ignored",
			&Embeded{
				[]time.Time{ti, ti},
				[]string{"HangZhou", "Boston"},
				[]int{1993, 1994},
				[]int64{10010, 10086},
				[]uint{18, 19},
				[]uint64{12345678, 98765432},
				[]float64{192.168, 10.11},
				[]bool{true, false},
				[]int{},
			}}
		cfg := Empty()
		assert.NoError(t, cfg.ReflectFrom(a))

		var buf bytes.Buffer
		_, err = cfg.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, `NAME   = Unknwon
Male   = true
; Author's age
Age    = 21
Height = 100
GPA    = 2.8
Date   = 1993-10-07T20:17:05Z

; Embeded section
[infos]
; Time data
Dates       = 1993-10-07T20:17:05Z|1993-10-07T20:17:05Z
Places      = HangZhou,Boston
Years       = 1993,1994
Numbers     = 10010,10086
Ages        = 18,19
Populations = 12345678,98765432
Coordinates = 192.168,10.11
Flags       = true,false
None        = 
`,
			buf.String(),
		)

		t.Run("reflect from non-point struct", func(t *testing.T) {
			assert.Error(t, cfg.ReflectFrom(Author{}))
		})

		t.Run("reflect from struct with omitempty", func(t *testing.T) {
			cfg := Empty()
			type SpecialStruct struct {
				FirstName  string    `ini:"first_name"`
				LastName   string    `ini:"last_name,omitempty"`
				JustOmitMe string    `ini:"omitempty"`
				LastLogin  time.Time `ini:"last_login,omitempty"`
				LastLogin2 time.Time `ini:",omitempty"`
				NotEmpty   int       `ini:"omitempty"`
				Number     int64     `ini:",omitempty"`
				Ages       uint      `ini:",omitempty"`
				Population uint64    `ini:",omitempty"`
				Coordinate float64   `ini:",omitempty"`
				Flag       bool      `ini:",omitempty"`
				Note       *string   `ini:",omitempty"`
			}
			special := &SpecialStruct{
				FirstName: "John",
				LastName:  "Doe",
				NotEmpty:  9,
			}

			assert.NoError(t, cfg.ReflectFrom(special))

			var buf bytes.Buffer
			_, err = cfg.WriteTo(&buf)
			assert.Equal(t, `first_name = John
last_name  = Doe
omitempty  = 9
`,
				buf.String(),
			)
		})

		t.Run("reflect from struct with non-anonymous structure pointer", func(t *testing.T) {
			cfg := Empty()
			type Rpc struct {
				Enable  bool   `ini:"enable"`
				Type    string `ini:"type"`
				Address string `ini:"addr"`
				Name    string `ini:"name"`
			}
			type Cfg struct {
				Rpc *Rpc `ini:"rpc"`
			}

			config := &Cfg{
				Rpc: &Rpc{
					Enable:  true,
					Type:    "type",
					Address: "address",
					Name:    "name",
				},
			}
			assert.NoError(t, cfg.ReflectFrom(config))

			var buf bytes.Buffer
			_, err = cfg.WriteTo(&buf)
			assert.Equal(t, `[rpc]
enable = true
type   = type
addr   = address
name   = name
`,
				buf.String(),
			)
		})
	})
}

func Test_ReflectFromStructNonUniqueSections(t *testing.T) {
	t.Run("reflect from struct with non unique sections", func(t *testing.T) {
		nonUnique := &testNonUniqueSectionsStruct{
			Interface: testInterface{
				Address:    "10.2.0.1/24",
				ListenPort: 34777,
				PrivateKey: "privServerKey",
			},
			Peer: []testPeer{
				{
					PublicKey:    "pubClientKey",
					PresharedKey: "psKey",
					AllowedIPs:   []string{"10.2.0.2/32", "fd00:2::2/128"},
				},
				{
					PublicKey:    "pubClientKey2",
					PresharedKey: "psKey2",
					AllowedIPs:   []string{"10.2.0.3/32", "fd00:2::3/128"},
				},
			},
		}

		cfg := Empty(AllowNonUniqueSections(true))

		assert.NoError(t, cfg.ReflectFrom(nonUnique))

		var buf bytes.Buffer
		_, err := cfg.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, confNonUniqueSectionDataStruct, buf.String())

		// note: using ReflectFrom from should overwrite the existing sections
		err = cfg.Section("Peer").ReflectFrom([]*testPeer{
			{
				PublicKey:    "pubClientKey3",
				PresharedKey: "psKey3",
				AllowedIPs:   []string{"10.2.0.4/32,fd00:2::4/128"},
			},
			{
				PublicKey:    "pubClientKey4",
				PresharedKey: "psKey4",
				AllowedIPs:   []string{"10.2.0.5/32,fd00:2::5/128"},
			},
		})

		require.NoError(t, err)

		buf = bytes.Buffer{}
		_, err = cfg.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, `[Interface]
Address    = 10.2.0.1/24
ListenPort = 34777
PrivateKey = privServerKey

[Peer]
PublicKey    = pubClientKey3
PresharedKey = psKey3
AllowedIPs   = 10.2.0.4/32\,fd00:2::4/128

[Peer]
PublicKey    = pubClientKey4
PresharedKey = psKey4
AllowedIPs   = 10.2.0.5/32\,fd00:2::5/128
`,
			buf.String(),
		)

		// note: using ReflectFrom from should overwrite the existing sections
		err = cfg.Section("Peer").ReflectFrom(&testPeer{
			PublicKey:    "pubClientKey5",
			PresharedKey: "psKey5",
			AllowedIPs:   []string{"10.2.0.6/32,fd00:2::6/128"},
		})

		require.NoError(t, err)

		buf = bytes.Buffer{}
		_, err = cfg.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, `[Interface]
Address    = 10.2.0.1/24
ListenPort = 34777
PrivateKey = privServerKey

[Peer]
PublicKey    = pubClientKey5
PresharedKey = psKey5
AllowedIPs   = 10.2.0.6/32\,fd00:2::6/128
`,
			buf.String(),
		)
	})
}

// Inspired by https://github.com/go-ini/ini/issues/196
func TestMapToAndReflectFromStructWithShadows(t *testing.T) {
	t.Run("map to struct and then reflect with shadows should generate original config content", func(t *testing.T) {
		type include struct {
			Paths []string `ini:"path,omitempty,allowshadow"`
		}

		cfg, err := Load([]byte(`
[include]
path = /tmp/gpm-profiles/test5.profile
path = /tmp/gpm-profiles/test1.profile`), AllowShadows(true))
		require.NoError(t, err)

		sec := cfg.Section("include")
		inc := new(include)
		err = sec.MapTo(inc)
		require.NoError(t, err)

		err = sec.ReflectFrom(inc)
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = cfg.WriteTo(&buf)
		require.NoError(t, err)
		assert.Equal(t, `[include]
path = /tmp/gpm-profiles/test5.profile
path = /tmp/gpm-profiles/test1.profile
`,
			buf.String(),
		)

		t.Run("reflect from struct with shadows", func(t *testing.T) {
			cfg := Empty(AllowShadows(true))
			type ShadowStruct struct {
				StringArray      []string    `ini:"sa,allowshadow"`
				EmptyStringArrat []string    `ini:"empty,omitempty,allowshadow"`
				Allowshadow      []string    `ini:"allowshadow,allowshadow"`
				Dates            []time.Time `ini:",allowshadow"`
				Places           []string    `ini:",allowshadow"`
				Years            []int       `ini:",allowshadow"`
				Numbers          []int64     `ini:",allowshadow"`
				Ages             []uint      `ini:",allowshadow"`
				Populations      []uint64    `ini:",allowshadow"`
				Coordinates      []float64   `ini:",allowshadow"`
				Flags            []bool      `ini:",allowshadow"`
				None             []int       `ini:",allowshadow"`
			}

			shadow := &ShadowStruct{
				StringArray: []string{"s1", "s2"},
				Allowshadow: []string{"s3", "s4"},
				Dates: []time.Time{time.Date(2020, 9, 12, 00, 00, 00, 651387237, time.UTC),
					time.Date(2020, 9, 12, 00, 00, 00, 651387237, time.UTC)},
				Places:      []string{"HangZhou", "Boston"},
				Years:       []int{1993, 1994},
				Numbers:     []int64{10010, 10086},
				Ages:        []uint{18, 19},
				Populations: []uint64{12345678, 98765432},
				Coordinates: []float64{192.168, 10.11},
				Flags:       []bool{true, false},
				None:        []int{},
			}

			assert.NoError(t, cfg.ReflectFrom(shadow))

			var buf bytes.Buffer
			_, err := cfg.WriteTo(&buf)
			require.NoError(t, err)
			assert.Equal(t, `sa          = s1
sa          = s2
allowshadow = s3
allowshadow = s4
Dates       = 2020-09-12T00:00:00.651387237Z
Places      = HangZhou
Places      = Boston
Years       = 1993
Years       = 1994
Numbers     = 10010
Numbers     = 10086
Ages        = 18
Ages        = 19
Populations = 12345678
Populations = 98765432
Coordinates = 192.168
Coordinates = 10.11
Flags       = true
Flags       = false
None        = 
`,
				buf.String(),
			)
		})
	})
}

type testMapper struct {
	PackageName string
}

func Test_NameGetter(t *testing.T) {
	t.Run("test name mappers", func(t *testing.T) {
		assert.NoError(t, MapTo(&testMapper{}, []byte("packag_name=ini"), NameMapping(TitleUnderscore)))

		cfg, err := Load([]byte("PACKAGE_NAME=ini"), NameMapping(SnakeCase))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		tg := new(testMapper)
		assert.NoError(t, cfg.MapTo(tg))
		assert.Equal(t, "ini", tg.PackageName)
	})
}

type testDurationStruct struct {
	Duration time.Duration `ini:"Duration"`
}

func Test_Duration(t *testing.T) {
	t.Run("duration less than 16m50s", func(t *testing.T) {
		ds := new(testDurationStruct)
		assert.NoError(t, MapTo(ds, []byte("Duration=16m49s")))

		dur, err := time.ParseDuration("16m49s")
		require.NoError(t, err)
		assert.Equal(t, dur.Seconds(), ds.Duration.Seconds())
	})
}

type Employer struct {
	Name  string
	Title string
}

type Employers []*Employer

func (es Employers) ReflectINIStruct(f *File) error {
	for _, e := range es {
		f.Section(e.Name).Key("Title").SetValue(e.Title)
	}
	return nil
}

// Inspired by https://github.com/go-ini/ini/issues/199
func Test_StructReflector(t *testing.T) {
	t.Run("reflect with StructReflector interface", func(t *testing.T) {
		p := &struct {
			FirstName string
			Employer  Employers
		}{
			FirstName: "Andrew",
			Employer: []*Employer{
				{
					Name:  `Employer "VMware"`,
					Title: "Staff II Engineer",
				},
				{
					Name:  `Employer "EMC"`,
					Title: "Consultant Engineer",
				},
			},
		}

		f := Empty()
		assert.NoError(t, f.ReflectFrom(p))

		var buf bytes.Buffer
		_, err := f.WriteTo(&buf)
		require.NoError(t, err)

		assert.Equal(t, `FirstName = Andrew

[Employer "VMware"]
Title = Staff II Engineer

[Employer "EMC"]
Title = Consultant Engineer
`,
			buf.String(),
		)
	})
}

// marshal writes v, a struct or a pointer to one, with File.ReflectFrom and
// File.Write. Shadows and non-unique sections let allowshadow and nonunique
// fields write all their values.
func marshal(v any) ([]byte, error) {
	if val := reflect.ValueOf(v); val.Kind() == reflect.Struct {
		ptr := reflect.New(val.Type())
		ptr.Elem().Set(val)
		v = ptr.Interface()
	}
	f := Empty(AllowShadows(true), AllowDuplicateShadowValues(true), AllowNonUniqueSections(true))
	if err := f.ReflectFrom(v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// unmarshal maps data to v with MapTo, reading what marshal writes: when v
// has allowshadow fields it loads with AllowShadows,
// AllowDuplicateShadowValues and PreserveEmptyShadowValues, and when it has
// nonunique fields with AllowNonUniqueSections. These come after opts.
func unmarshal(data []byte, v any, opts ...LoadOption) error {
	if v != nil {
		shadows, nonUnique := tagOptions(reflect.TypeOf(v), map[reflect.Type]bool{})
		opts = slices.Clone(opts)
		if shadows {
			opts = append(opts, AllowShadows(true), AllowDuplicateShadowValues(true), PreserveEmptyShadowValues(true))
		}
		if nonUnique {
			opts = append(opts, AllowNonUniqueSections(true))
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

func TestStructRoundTrip(t *testing.T) {
	type server struct {
		Host  string   `ini:"host"`
		Ports []int    `ini:"ports"`
		Tags  []string `ini:"tags" delim:"|"`
	}
	type config struct {
		Name   string `ini:"name"`
		Server server `ini:"server"`
	}
	in := config{Name: "app", Server: server{Host: "localhost", Ports: []int{80, 443}, Tags: []string{"a", "b"}}}

	data, err := marshal(in)
	require.NoError(t, err)
	assert.Equal(t, "name = app\n\n[server]\nhost  = localhost\nports = 80,443\ntags  = a|b\n", string(data))

	var out config
	require.NoError(t, unmarshal(data, &out))
	assert.Equal(t, in, out)

	err = unmarshal([]byte("name"), &out)
	assert.ErrorIs(t, err, ErrDelimiterNotFound)
}

func TestStructRoundTrip_ShadowsAndNonUniqueSections(t *testing.T) {
	type peer struct {
		Key string `ini:"key"`
	}
	type config struct {
		Paths []string `ini:"path,,allowshadow"`
		Peers []peer   `ini:"Peer,,,nonunique"`
	}
	in := config{Paths: []string{"a", "b", "c"}, Peers: []peer{{"x"}, {"y"}}}

	data, err := marshal(in)
	require.NoError(t, err)
	assert.Equal(t, "path = a\npath = b\npath = c\n\n[Peer]\nkey = x\n\n[Peer]\nkey = y\n", string(data))

	var out config
	require.NoError(t, unmarshal(data, &out, AllowShadows(true), AllowNonUniqueSections(true)))
	assert.Equal(t, in, out)
}

func TestStructRoundTrip_Nil(t *testing.T) {
	type peer struct {
		Key string `ini:"key"`
	}
	type config struct {
		Name string `ini:"name"`
		Peer *peer  `ini:"peer"`
	}

	_, err := marshal(nil)
	assert.Error(t, err)
	_, err = marshal((*config)(nil))
	assert.Error(t, err)
	n := 1
	_, err = marshal(&n)
	assert.Error(t, err)
	assert.Error(t, unmarshal([]byte("name = x"), nil))
	assert.Error(t, unmarshal([]byte("name = x"), (*config)(nil)))

	// A nil section pointer writes nothing for that section.
	data, err := marshal(config{Name: "n"})
	require.NoError(t, err)
	assert.Equal(t, "name = n\n", string(data))
}

func TestMapTo_UintFieldWithDuration(t *testing.T) {
	type cfg struct {
		U uint
		P *uint64
	}
	c := cfg{U: 7}
	assert.NoError(t, MapTo(&c, []byte("U = 1s\nP = 1s\n")))
	assert.Equal(t, uint(7), c.U)
	assert.Nil(t, c.P)
	assert.Error(t, StrictMapTo(&cfg{}, []byte("U = 1s\n")))

	assert.NoError(t, MapTo(&c, []byte("U = 3\nP = 4\n")))
	assert.Equal(t, uint(3), c.U)
	require.NotNil(t, c.P)
	assert.Equal(t, uint64(4), *c.P)
}

func TestMapTo_DurationPointerWithNumber(t *testing.T) {
	var c struct{ D *time.Duration }
	require.NoError(t, MapTo(&c, []byte("D = 5\n")))
	require.NotNil(t, c.D)
	assert.Equal(t, time.Duration(5), *c.D)
}

func TestMapTo_StructFieldsFromKeys(t *testing.T) {
	type sub struct{ A int }
	var c1 struct{ Sub sub }
	assert.Error(t, MapTo(&c1, []byte("Sub = 2006-01-02T15:04:05Z\n")))
	var c2 struct{ Sub *sub }
	assert.Error(t, MapTo(&c2, []byte("Sub = 2006-01-02T15:04:05Z\n")))
	var c3 struct{ Items []sub }
	assert.Error(t, MapTo(&c3, []byte("Items = x\n")))
}

type testTag string

type testBool bool

func TestMapTo_NamedTypes(t *testing.T) {
	var c struct {
		Tag   testTag
		Tags  []testTag
		TagP  *testTag
		BoolP *testBool
		Durs  []time.Duration
	}
	require.NoError(t, MapTo(&c, []byte("Tag = t\nTags = a,b\nTagP = p\nBoolP = true\nDurs = 1s,2s,3\n")))
	assert.Equal(t, testTag("t"), c.Tag)
	assert.Equal(t, []testTag{"a", "b"}, c.Tags)
	require.NotNil(t, c.TagP)
	assert.Equal(t, testTag("p"), *c.TagP)
	require.NotNil(t, c.BoolP)
	assert.Equal(t, testBool(true), *c.BoolP)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 3}, c.Durs)
}

func TestMapTo_PointerTargets(t *testing.T) {
	type cfg struct{ Name string }
	f, err := Load([]byte("Name = x\n"))
	require.NoError(t, err)

	var c *cfg
	require.NoError(t, f.MapTo(&c))
	require.NotNil(t, c)
	assert.Equal(t, "x", c.Name)

	var cc **cfg
	require.NoError(t, f.MapTo(&cc))
	require.NotNil(t, cc)
	assert.Equal(t, "x", (*cc).Name)

	var i int
	assert.Error(t, f.MapTo(&i))
	var ip *int
	assert.Error(t, f.MapTo(&ip))
	assert.Nil(t, ip)
}

func TestMapTo_NonUniqueSlices(t *testing.T) {
	type peer struct{ Key string }
	type cfg struct {
		Peers []*peer `ini:"Peer,nonunique"`
		Other []peer  `ini:"Other,nonunique"`
		After string
	}
	var c cfg
	require.NoError(t, MapTo(&c, []byte("After = a\n[Peer]\nKey = x\n[Peer]\nKey = y\n"), AllowNonUniqueSections(true)))
	require.Len(t, c.Peers, 2)
	assert.Equal(t, "x", c.Peers[0].Key)
	assert.Equal(t, "y", c.Peers[1].Key)
	assert.Empty(t, c.Other, "no [Other] sections leaves the slice empty")
	assert.Equal(t, "a", c.After, "fields after a nonunique field without sections are mapped")

	var s struct {
		Names []string `ini:"Name,nonunique"`
	}
	assert.Error(t, MapTo(&s, []byte("[Name]\nk = v\n"), AllowNonUniqueSections(true)))
}

type testNode struct {
	X     int
	Child *testNode
	Kids  []testNode `ini:"Kid,nonunique"`
}

func TestMapToReflectFrom_RecursiveType(t *testing.T) {
	var n testNode
	require.NoError(t, MapTo(&n, []byte("X = 1\n[Child]\nX = 2\n[Kid]\nX = 3\n[Kid]\nX = 4\n"), AllowNonUniqueSections(true)))
	assert.Equal(t, 1, n.X)
	require.NotNil(t, n.Child)
	assert.Equal(t, 2, n.Child.X)
	assert.Nil(t, n.Child.Child, "the same section is not mapped again one level deeper")
	require.Len(t, n.Kids, 2)
	assert.Equal(t, 3, n.Kids[0].X)
	assert.Empty(t, n.Kids[0].Kids)

	// A cyclic value is written one level deep.
	cyclic := &testNode{X: 1}
	cyclic.Child = cyclic
	data, err := marshal(cyclic)
	require.NoError(t, err)
	assert.Equal(t, "X = 1\n\n[Child]\nX = 1\n", string(data))
}

func TestReflectFrom_StructSlice(t *testing.T) {
	type sub struct{ A int }
	_, err := marshal(struct{ Items []sub }{Items: []sub{{1}}})
	assert.Error(t, err)
}

// A pointer to a struct pointer is accepted, as in ReflectFrom(&cfg) with cfg
// a package-level *Config.
func TestReflectFrom_PointerToPointer(t *testing.T) {
	type conf struct{ Lang string }
	c := &conf{Lang: "en"}
	for _, opts := range [][]LoadOption{nil, {AllowNonUniqueSections(true)}} {
		f := Empty(opts...)
		require.NoError(t, f.Section("i18n").ReflectFrom(&c))
		assert.Equal(t, "en", f.Section("i18n").Key("Lang").String())
	}

	var nilConf *conf
	assert.Error(t, Empty().Section("i18n").ReflectFrom(&nilConf))
}

func TestReflectFrom_NonUniqueNilAndNonStruct(t *testing.T) {
	type peer struct{ Key string }
	data, err := marshal(struct {
		Peers []*peer `ini:"Peer,nonunique"`
	}{Peers: []*peer{nil, {"x"}, nil}})
	require.NoError(t, err)
	assert.Equal(t, "[Peer]\nKey = x\n", string(data))

	n := 1
	_, err = marshal(struct {
		Ints []*int `ini:"Int,nonunique"`
	}{Ints: []*int{&n}})
	assert.Error(t, err)

	f := Empty(AllowNonUniqueSections(true))
	require.NoError(t, f.Section("Peer").ReflectFrom([]*peer{nil, {"y"}}))
	secs, err := f.SectionsByName("Peer")
	require.NoError(t, err)
	require.Len(t, secs, 1)
	assert.Equal(t, "y", secs[0].Key("Key").String())

	assert.Error(t, f.Section("Peer").ReflectFrom([]*int{&n}))
	assert.Error(t, f.Section("Peer").ReflectFrom(&n))
	assert.True(t, f.HasSection("Peer"), "a rejected value does not delete the sections")
}

func TestMapTo_Overflow(t *testing.T) {
	type cfg struct {
		I8  int8
		U16 uint16
		F32 float32
		Is  []int
	}
	src := []byte("I8 = 300\nU16 = 70000\nF32 = 1e300\n")
	c := cfg{I8: 1, U16: 2, F32: 3}
	require.NoError(t, MapTo(&c, src))
	assert.Equal(t, cfg{I8: 1, U16: 2, F32: 3}, c)

	for _, line := range []string{"I8 = 300", "I8 = -129", "U16 = 70000", "F32 = 1e300"} {
		assert.Error(t, StrictMapTo(&cfg{}, []byte(line)), line)
	}
	require.NoError(t, StrictMapTo(&c, []byte("I8 = -128\nU16 = 65535\nF32 = 1.5\n")))
	assert.Equal(t, cfg{I8: -128, U16: 65535, F32: 1.5}, c)
}

func TestStrictMapTo_InvalidSliceElement(t *testing.T) {
	var c struct{ Is []int }
	require.NoError(t, MapTo(&c, []byte("Is = 1,x,3\n")))
	assert.Equal(t, []int{1, 0, 3}, c.Is)
	assert.Error(t, StrictMapTo(&c, []byte("Is = 1,x,3\n")))
}

func TestMapTo_ZeroAndNegativeDuration(t *testing.T) {
	c := struct{ D, Z time.Duration }{D: time.Hour, Z: time.Hour}
	require.NoError(t, MapTo(&c, []byte("D = -5s\nZ = 0\n")))
	assert.Equal(t, -5*time.Second, c.D)
	assert.Equal(t, time.Duration(0), c.Z)
}

func TestStructRoundTrip_TimePointer(t *testing.T) {
	type cfg struct {
		T  *time.Time
		TV time.Time
	}
	ti := time.Date(2020, 9, 12, 1, 2, 3, 0, time.UTC)
	data, err := marshal(cfg{T: &ti, TV: ti})
	require.NoError(t, err)
	assert.Equal(t, "T  = 2020-09-12T01:02:03Z\nTV = 2020-09-12T01:02:03Z\n", string(data))

	var out cfg
	require.NoError(t, unmarshal(data, &out))
	require.NotNil(t, out.T)
	assert.True(t, ti.Equal(*out.T))
	assert.True(t, ti.Equal(out.TV))
}

func TestStructRoundTrip_SubSecondTime(t *testing.T) {
	type cfg struct {
		T  time.Time
		Ts []time.Time
	}
	ti := time.Date(2020, 9, 12, 0, 0, 0, 651387237, time.UTC)
	in := cfg{T: ti, Ts: []time.Time{ti, ti.Add(time.Second)}}
	data, err := marshal(in)
	require.NoError(t, err)
	assert.Equal(t, "T  = 2020-09-12T00:00:00.651387237Z\nTs = 2020-09-12T00:00:00.651387237Z,2020-09-12T00:00:01.651387237Z\n", string(data))

	var out cfg
	require.NoError(t, unmarshal(data, &out))
	assert.Equal(t, in, out)
}

func TestStructRoundTrip_EscapedSliceElements(t *testing.T) {
	type cfg struct {
		S []string
		P []string `delim:"|"`
	}
	in := cfg{S: []string{"a", "b, c", `d\e`, `f\`}, P: []string{"x|y", "z,w"}}
	data, err := marshal(in)
	require.NoError(t, err)
	assert.Equal(t, "S = `a,b\\, c,d\\e,f\\\\`\nP = x\\|y|z,w\n", string(data))

	var out cfg
	require.NoError(t, unmarshal(data, &out))
	assert.Equal(t, in, out)

	// Key.Strings drops an empty last element, so a slice ending in an empty
	// string cannot round trip. Empty elements before the last one do.
	data, err = marshal(cfg{S: []string{"", "a"}, P: []string{"", ""}})
	require.NoError(t, err)
	assert.Equal(t, "S = ,a\nP = |\n", string(data))
	out = cfg{}
	require.NoError(t, unmarshal(data, &out))
	assert.Equal(t, []string{"", "a"}, out.S)
}

func TestStructRoundTrip_NilPointerFields(t *testing.T) {
	type cfg struct {
		PS   *string
		PI   *int
		Name string
	}
	data, err := marshal(cfg{Name: "n"})
	require.NoError(t, err)
	assert.Equal(t, "Name = n\n", string(data))

	var out cfg
	require.NoError(t, unmarshal(data, &out))
	assert.Nil(t, out.PS)
	assert.Nil(t, out.PI)
}

func TestReflectFrom_FieldsAfterEmptyNonUniqueAndReflector(t *testing.T) {
	type peer struct{ Key string }
	data, err := marshal(struct {
		Peers []peer `ini:"Peer,nonunique"`
		After string
	}{After: "a"})
	require.NoError(t, err)
	assert.Equal(t, "After = a\n", string(data))

	data, err = marshal(struct {
		Employer Employers
		After    string
	}{Employer: Employers{{Name: "E", Title: "T"}}, After: "a"})
	require.NoError(t, err)
	assert.Equal(t, "After = a\n\n[E]\nTitle = T\n", string(data))
}

func TestReflectFrom_AllowShadow(t *testing.T) {
	type cfg struct {
		Paths []string `ini:"path,allowshadow" comment:"search paths"`
	}
	data, err := marshal(cfg{Paths: []string{"a", "b"}})
	require.NoError(t, err)
	// The comment of a key goes before its last line, as the lexer reads it.
	assert.Equal(t, "path = a\n; search paths\npath = b\n", string(data))

	assert.Error(t, Empty().ReflectFrom(&cfg{Paths: []string{"a", "b"}}), "values must not be dropped silently")
	f := Empty()
	require.NoError(t, f.ReflectFrom(&cfg{Paths: []string{"a"}}))
	assert.Equal(t, "a", f.Section("").Key("path").String())
}

func TestReflectFrom_InsensitiveDefaultSection(t *testing.T) {
	for _, opt := range []LoadOption{CaseInsensitive(true), CaseInsensitiveSections(true)} {
		f, err := Load([]byte("a=1\n[x]\nb=2\n"), opt, AllowNonUniqueSections(true))
		require.NoError(t, err)
		require.NoError(t, f.ReflectFrom(&struct{ C int }{3}))
		assert.Equal(t, "1", f.Section("").Key("a").String())
		assert.Equal(t, "3", f.Section("").Key("C").String())
	}
}

// https://github.com/go-ini/ini/issues/302
func TestStructRoundTrip_Uint8(t *testing.T) {
	type cfg struct {
		B  uint8
		BP *byte
	}
	var out cfg
	require.NoError(t, unmarshal([]byte("B = 200\nBP = 7\n"), &out))
	assert.Equal(t, uint8(200), out.B)
	require.NotNil(t, out.BP)
	assert.Equal(t, byte(7), *out.BP)

	data, err := marshal(out)
	require.NoError(t, err)
	assert.Equal(t, "B  = 200\nBP = 7\n", string(data))
}

// https://github.com/go-ini/ini/issues/274
func TestMapTo_EmbeddedPointerKeepsValues(t *testing.T) {
	type Base struct{ A, B string }
	type cfg struct {
		*Base `ini:"base"`
	}
	c := cfg{Base: &Base{A: "keep", B: "old"}}
	require.NoError(t, MapTo(&c, []byte("[base]\nB = new\n")))
	assert.Equal(t, Base{A: "keep", B: "new"}, *c.Base)

	var nilBase cfg
	require.NoError(t, MapTo(&nilBase, []byte("X = 1\n")))
	assert.NotNil(t, nilBase.Base, "a nil embedded pointer is still allocated")
}

func TestStructRoundTrip_AllowShadowElements(t *testing.T) {
	type cfg struct {
		Sh []string `ini:"sh,,allowshadow"`
		N  []int    `ini:"n,,allowshadow"`
	}
	in := cfg{Sh: []string{"a", "a", `b\\c`, "", `d\`}, N: []int{1, 1, 2}}
	data, err := marshal(in)
	require.NoError(t, err)

	var out cfg
	require.NoError(t, unmarshal(data, &out))
	assert.Equal(t, in, out, "written: %q", data)

	// The tags need these options. Turning them off, as with a config
	// flag that defaults to false, must not drop elements.
	out = cfg{}
	require.NoError(t, unmarshal(data, &out, AllowShadows(false), AllowDuplicateShadowValues(false), PreserveEmptyShadowValues(false)))
	assert.Equal(t, in, out, "written: %q", data)

	// Each value is split at the delimiter when read, so an element
	// containing it can't be written.
	_, err = marshal(cfg{Sh: []string{"b,c"}})
	assert.Error(t, err)
}

type fzName string

type fzSub struct {
	A int
	B string
	T time.Time
}

type fzNode struct {
	X     int
	Child *fzNode
}

type fzEmbed struct {
	E string
}

// fzCfg has a field of every kind struct mapping supports.
type fzCfg struct {
	S   string
	I   int
	I8  int8
	U   uint
	U8  uint8
	F32 float32
	F   float64
	B   bool
	D   time.Duration
	T   time.Time

	PS *string
	PI *int
	PT *time.Time
	PD *time.Duration

	SS []string
	SI []int
	SU []uint64
	SF []float64
	SB []bool
	ST []time.Time
	SP []string `delim:"|"`

	Named  fzName
	PNamed *fzName

	Perm  testPerm
	PPerm *testPerm
	Perms []testPerm

	Sub     fzSub
	PSub    *fzSub
	Shadow  []string `ini:"shadow,,allowshadow"`
	Peers   []fzSub  `ini:"Peer,,,nonunique"`
	PPeers  []*fzSub `ini:"PPeer,,,nonunique"`
	Node    *fzNode
	fzEmbed `ini:",extends"`
	Skip    string `ini:"-"`
}

// FuzzStructMapping loads arbitrary input under random options, reads it
// with every Key accessor and maps it into fzCfg. Nothing may panic or hang,
// and a value written by marshal must read back the same.
func FuzzStructMapping(f *testing.F) {
	inputs := append(fuzzInputs(f),
		"S = s\nI = -3\nI8 = 127\nU = 4\nU8 = 255\nF32 = 1.5\nF = 2.5\nB = true\nD = 1s\nT = 2020-01-02T03:04:05.5Z\n"+
			"PS = p\nPI = 7\nPT = 2020-01-02T03:04:05Z\nPD = 3s\nSS = a,b\\, c,d\nSI = 1,2\nSU = 3,4\nSF = 1.5,2\nSB = true,false\n"+
			"ST = 2020-01-02T03:04:05Z\nSP = x|y\nNamed = n\nPNamed = m\nshadow = a\nshadow = b\nE = e\n"+
			"Perm = write\nPPerm = read\nPerms = read, x, write\n"+
			"[Sub]\nA = 1\nB = b\n[PSub]\nA = 2\n[Peer]\nA = 3\n[Peer]\nA = 4\n[PPeer]\nB = q\n[Node]\nX = 1\n[Child]\nX = 2\n",
		"I8 = 300\nU8 = -1\nF32 = 1e300\nD = -5s\nSI = 1,x,3\nT = 2006-01-02T15:04:05.123456789+07:00\n",
		"a = %(b)s\nb = %(a)s\nS = %(S)s%(S)s\n",
	)
	for _, in := range inputs {
		for _, flags := range []uint32{0, 1 << 7, 1<<14 | 1<<7, 1<<0 | 1<<14, 1<<20 - 1} {
			f.Add([]byte(in), flags)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte, flags uint32) {
		opts := optionsFromBits(flags)
		file, err := Load(data, opts...)
		if err != nil {
			return
		}

		for _, sec := range file.Sections() {
			_ = sec.ChildSections()
			_ = sec.ParentKeys()
			_ = sec.KeyValues()
			for _, k := range sec.Keys() {
				_ = k.String()
				for _, delim := range []string{",", "", "|", "\\", "::"} {
					_ = k.Strings(delim)
				}
				_ = k.StringsWithShadows(",")
				_ = List[int](k, ",")
				_ = ValidList[uint64](k, ",")
				_ = List[float32](k, ",")
				_ = ValidList[bool](k, ",")
				_, _ = StrictList[time.Time](k, ",")
				_, _ = StrictList[time.Duration](k, ",")
				_ = k.TimesFormat("2006-01-02", ",")
				_ = k.RangeTimeFormat("2006-01-02", time.Time{}, time.Time{}, time.Now())
				_, _ = k.Duration()
				_, _ = k.Time()
				_ = k.MustInt(1)
				_ = In(k, "a", []string{"a", "b"})
				_ = In(k, int8(1), []int8{1, 2})
				_ = Range(k, 1.5, -1e9, 1e9)
				_ = Range(k, uint16(1), 0, 100)
				_ = Range(k, time.Time{}, time.Time{}, time.Now())
				_ = k.Values()
				_ = k.NestedValues()
			}
		}
		_ = file.SectionWithIndex("", 1)
		_ = file.SectionWithIndex("Peer", -1)

		var cfg fzCfg
		_ = file.MapTo(&cfg)
		var strict fzCfg
		_ = file.StrictMapTo(&strict)

		// What marshal writes, unmarshal reads back the same, except
		// for the limits of the format that are checked below.
		out, err := marshal(cfg)
		if err != nil || bytes.Contains(out, []byte("%(")) {
			// A %(name)s reference in a value is expanded when read.
			return
		}
		var back fzCfg
		require.NoError(t, unmarshal(out, &back), "marshaled: %q", out)
		normalizeFz(&cfg)
		normalizeFz(&back)
		require.Equal(t, cfg, back, "marshal/unmarshal changed the value\nmarshaled: %q", out)

		// ReflectFrom into the loaded file must not panic and must write.
		if err := file.ReflectFrom(&cfg); err == nil {
			var buf bytes.Buffer
			_, _ = file.Write(&buf)
		}
	})
}

// normalizeFz clears what the format can't carry: trailing empty strings in
// slices, empty slices and nil versus empty, monotonic clock readings and
// locations of times.
func normalizeFz(c *fzCfg) {
	v := reflect.ValueOf(c).Elem()
	normalizeFzValue(v)
}

func normalizeFzValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			normalizeFzValue(v.Elem())
		}
	case reflect.Struct:
		if v.Type() == timeType {
			if v.CanSet() {
				t := v.Interface().(time.Time)
				v.Set(reflect.ValueOf(time.Unix(t.Unix(), int64(t.Nanosecond())).UTC()))
			}
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				normalizeFzValue(v.Field(i))
			}
		}
	case reflect.Slice:
		// Key.Strings drops an empty last element, so trailing empty
		// strings can't round trip.
		if v.Type().Elem().Kind() == reflect.String {
			n := v.Len()
			for n > 0 && v.Index(n-1).String() == "" {
				n--
			}
			v.Set(v.Slice(0, n))
		}
		if v.Len() == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		for i := 0; i < v.Len(); i++ {
			normalizeFzValue(v.Index(i))
		}
	}
}

// testPerm is an enum that reads and writes itself by name.
type testPerm int

var testPermNames = []string{"none", "read", "write"}

func (p *testPerm) UnmarshalINI(b []byte) error {
	for i, name := range testPermNames {
		if string(b) == name {
			*p = testPerm(i)
			return nil
		}
	}
	return fmt.Errorf("unknown permission %q", b)
}

func (p testPerm) MarshalINI() ([]byte, error) {
	if int(p) >= len(testPermNames) {
		return nil, fmt.Errorf("invalid permission %d", int(p))
	}
	return []byte(testPermNames[p]), nil
}

// testHostPort is a struct that maps to one key, written as host:port.
type testHostPort struct {
	Host string
	Port int
}

func (h *testHostPort) UnmarshalINI(b []byte) error {
	host, port, ok := strings.Cut(string(b), ":")
	if !ok {
		return fmt.Errorf("missing port in %q", b)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	h.Host, h.Port = host, n
	return nil
}

func (h *testHostPort) MarshalINI() ([]byte, error) {
	return []byte(h.Host + ":" + strconv.Itoa(h.Port)), nil
}

func TestMarshalerUnmarshaler(t *testing.T) {
	type config struct {
		Perm  testPerm
		Perms []testPerm
		PPerm *testPerm
		Addr  testHostPort
		Addrs []testHostPort `delim:"|"`
	}

	t.Run("read", func(t *testing.T) {
		data := []byte("Perm = write\nPerms = read, none\nPPerm = read\nAddr = example.com:80\nAddrs = a:1|b:2\n" +
			"[Addr]\nHost = from a section\n")
		var c config
		require.NoError(t, StrictMapTo(&c, data))
		read := testPerm(1)
		assert.Equal(t, config{
			Perm:  2,
			Perms: []testPerm{1, 0},
			PPerm: &read,
			// A struct that parses itself maps to its key, not to the
			// section of the same name.
			Addr:  testHostPort{"example.com", 80},
			Addrs: []testHostPort{{"a", 1}, {"b", 2}},
		}, c)
	})

	t.Run("write and read back", func(t *testing.T) {
		read := testPerm(1)
		in := config{Perm: 2, Perms: []testPerm{1, 0}, PPerm: &read, Addr: testHostPort{"h", 8}, Addrs: []testHostPort{{"a", 1}}}
		data, err := marshal(in)
		require.NoError(t, err)
		assert.Equal(t, "Perm  = write\nPerms = read,none\nPPerm = read\nAddr  = h:8\nAddrs = a:1\n", string(data))

		var out config
		require.NoError(t, unmarshal(data, &out))
		assert.Equal(t, in, out)
	})

	t.Run("errors", func(t *testing.T) {
		c := config{Perm: 1}
		require.NoError(t, MapTo(&c, []byte("Perm = admin\n")))
		assert.Equal(t, testPerm(1), c.Perm, "an invalid value leaves the field unchanged")

		err := StrictMapTo(&c, []byte("Perm = admin\n"))
		assert.ErrorContains(t, err, `unknown permission "admin"`)
		assert.ErrorContains(t, StrictMapTo(&c, []byte("Perms = read, admin\n")), "admin")

		_, err = marshal(config{Perm: 9})
		assert.ErrorContains(t, err, "invalid permission 9")
		_, err = marshal(config{Perms: []testPerm{1, 9}})
		assert.ErrorContains(t, err, "invalid permission 9")
	})

	t.Run("generic accessors", func(t *testing.T) {
		f, err := Load([]byte("perms = write, x, read\nperm = write\n"))
		require.NoError(t, err)
		sec := f.Section("")
		assert.Equal(t, []testPerm{2, 0, 1}, List[testPerm](sec.Key("perms"), ","))
		assert.Equal(t, []testPerm{2, 1}, ValidList[testPerm](sec.Key("perms"), ","))
		_, err = StrictList[testPerm](sec.Key("perms"), ",")
		assert.Error(t, err)
		assert.Equal(t, testPerm(2), In(sec.Key("perm"), 0, []testPerm{1, 2}))
	})

	t.Run("one method on a slice or struct type", func(t *testing.T) {
		// Without both methods, a slice type is still a list and a struct
		// type a section, in both directions.
		type oneWay struct {
			Hosts testMarshalOnlyList
			Tags  testUnmarshalOnlyList
			Peer  testMarshalOnlyStruct
		}
		var c oneWay
		require.NoError(t, StrictMapTo(&c, []byte("Hosts = a,b\nTags = x,y\n[Peer]\nName = p\n")))
		assert.Equal(t, oneWay{Hosts: testMarshalOnlyList{"a", "b"}, Tags: testUnmarshalOnlyList{"x", "y"}, Peer: testMarshalOnlyStruct{Name: "p"}}, c)

		data, err := marshal(c)
		require.NoError(t, err)
		assert.Equal(t, "Hosts = a,b\nTags  = x,y\n\n[Peer]\nName = p\n", string(data))
	})
}

// testMarshalOnlyList, testUnmarshalOnlyList and testMarshalOnlyStruct
// implement one of Marshaler and Unmarshaler, with methods that must not be
// called.
type (
	testMarshalOnlyList   []string
	testUnmarshalOnlyList []string
	testMarshalOnlyStruct struct{ Name string }
)

func (testMarshalOnlyList) MarshalINI() ([]byte, error) { panic("called") }

func (*testUnmarshalOnlyList) UnmarshalINI([]byte) error { panic("called") }

func (testMarshalOnlyStruct) MarshalINI() ([]byte, error) { panic("called") }
