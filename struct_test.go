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
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type testNested struct {
	Cities []string `delim:"|"`
	Visits []time.Time
	Note   string
	Unused int `ini:"-"`
}

type testEmbeded struct {
	GPA float64
}

type testStruct struct {
	Name         string `ini:"NAME"`
	Age          int
	Male         bool
	Money        float64
	Born         time.Time
	Time         time.Duration `ini:"Duration"`
	Others       testNested
	*testEmbeded `ini:"grade"`
	Unused       int `ini:"-"`
	Unsigned     uint
}

type testStructPassingFlags struct {
	Name		string `ini:"NAME" iniFlags:"mustExist"`
	Age			int `iniFlags:"strictParse"`
	Male		bool `iniFlags:"mustExist;strictParse"`
}

type testStructFailingMustExistFlag struct {
	Job			string `iniFlags:"mustExist"`
}

type testStructFailingStrictParseFlag struct {
	Name		int `ini:"NAME" iniFlags:"strictParse"`
}

const _CONF_DATA_STRUCT = `
NAME = Unknown
Age = 21
Male = true
Money = 1.25
Born = 1993-10-07T20:17:05Z
Duration = 2h45m
Unsigned = 3

[Others]
Cities = HangZhou|Boston
Visits = 1993-10-07T20:17:05Z, 1993-10-07T20:17:05Z
Note = Hello world!

[grade]
GPA = 2.8

[foo.bar]
Here = there
When = then
`

type unsupport struct {
	Byte byte
}

type unsupport2 struct {
	Others struct {
		Cities byte
	}
}

type unsupport3 struct {
	Cities byte
}

type unsupport4 struct {
	*unsupport3 `ini:"Others"`
}

type defaultValue struct {
	Name   string
	Age    int
	Male   bool
	Money  float64
	Born   time.Time
	Cities []string
}

type fooBar struct {
	Here, When string
}

const _INVALID_DATA_CONF_STRUCT = `
Name = 
Age = age
Male = 123
Money = money
Born = nil
Cities = 
`

type zeroValueDataStruct struct {
	ZeroAge			int
	ZeroMoney		float64
	ZeroDuration	time.Duration
	Age				int
	Money			float64
}

const _ZERO_VALUE_DATA_CONF_STRUCT = `
ZeroAge = 0
ZeroMoney = 0.0
ZeroDuration = 0
Age =
`

func Test_Struct(t *testing.T) {
	Convey("Map to struct", t, func() {
		Convey("Map file to struct", func() {
			ts := new(testStruct)
			So(MapTo(ts, []byte(_CONF_DATA_STRUCT)), ShouldBeNil)

			So(ts.Name, ShouldEqual, "Unknown")
			So(ts.Age, ShouldEqual, 21)
			So(ts.Male, ShouldBeTrue)
			So(ts.Money, ShouldEqual, 1.25)
			So(ts.Unsigned, ShouldEqual, 3)

			t, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
			So(err, ShouldBeNil)
			So(ts.Born.String(), ShouldEqual, t.String())

			dur, err := time.ParseDuration("2h45m")
			So(err, ShouldBeNil)
			So(ts.Time.Seconds(), ShouldEqual, dur.Seconds())

			So(strings.Join(ts.Others.Cities, ","), ShouldEqual, "HangZhou,Boston")
			So(ts.Others.Visits[0].String(), ShouldEqual, t.String())
			So(ts.Others.Note, ShouldEqual, "Hello world!")
			So(ts.testEmbeded.GPA, ShouldEqual, 2.8)
		})

		Convey("Map section to struct", func() {
			foobar := new(fooBar)
			f, err := Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)

			So(f.Section("foo.bar").MapTo(foobar), ShouldBeNil)
			So(foobar.Here, ShouldEqual, "there")
			So(foobar.When, ShouldEqual, "then")
		})

		Convey("Map to non-pointer struct", func() {
			cfg, err := Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			So(cfg, ShouldNotBeNil)

			So(cfg.MapTo(testStruct{}), ShouldNotBeNil)
		})

		Convey("Map to unsupported type", func() {
			cfg, err := Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			So(cfg, ShouldNotBeNil)

			cfg.NameMapper = func(raw string) string {
				if raw == "Byte" {
					return "NAME"
				}
				return raw
			}
			So(cfg.MapTo(&unsupport{}), ShouldNotBeNil)
			So(cfg.MapTo(&unsupport2{}), ShouldNotBeNil)
			So(cfg.MapTo(&unsupport4{}), ShouldNotBeNil)
		})

		Convey("Map from invalid data source", func() {
			So(MapTo(&testStruct{}, "hi"), ShouldNotBeNil)
		})

		Convey("Map to wrong types and gain default values", func() {
			cfg, err := Load([]byte(_INVALID_DATA_CONF_STRUCT))
			So(err, ShouldBeNil)

			t, err := time.Parse(time.RFC3339, "1993-10-07T20:17:05Z")
			So(err, ShouldBeNil)
			dv := &defaultValue{"Joe", 10, true, 1.25, t, []string{"HangZhou", "Boston"}}
			So(cfg.MapTo(dv), ShouldBeNil)
			So(dv.Name, ShouldEqual, "Joe")
			So(dv.Age, ShouldEqual, 10)
			So(dv.Male, ShouldBeTrue)
			So(dv.Money, ShouldEqual, 1.25)
			So(dv.Born.String(), ShouldEqual, t.String())
			So(strings.Join(dv.Cities, ","), ShouldEqual, "HangZhou,Boston")
		})
		Convey("Map to struct with passing iniFlags", func() {
			cfg, err := Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			testStruct := new(testStructPassingFlags)
			So(cfg.MapTo(testStruct), ShouldBeNil)
			So(testStruct.Name, ShouldEqual, "Unknown")
			So(testStruct.Age, ShouldEqual, 21)
			So(testStruct.Male, ShouldBeTrue)
		})
		Convey("Map to struct with failing strictParse", func() {
			cfg, err := Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			So(cfg.MapTo(&testStructFailingStrictParseFlag{}), ShouldNotBeNil)
		})
		Convey("Map to struct with failing mustExist", func() {
			cfg, err := Load([]byte(_CONF_DATA_STRUCT))
			So(err, ShouldBeNil)
			So(cfg.MapTo(&testStructFailingMustExistFlag{}), ShouldNotBeNil)
		})
		Convey("Map to struct with defaults overridden by 0 vals in ini", func(){
			cfg, err := Load([]byte(_ZERO_VALUE_DATA_CONF_STRUCT))
			So(err, ShouldBeNil)
			testStruct := &zeroValueDataStruct{ZeroAge: 3,
											   ZeroMoney: 2.2,
											   ZeroDuration: 25,
											   Age: 3,
											   Money: 2.2}
			So(cfg.MapTo(testStruct), ShouldBeNil)
			So(testStruct.ZeroAge, ShouldEqual, 0)
			So(testStruct.ZeroMoney, ShouldEqual, 0)
			So(testStruct.ZeroDuration, ShouldEqual, 0)
			So(testStruct.Age, ShouldEqual, 3)
			So(testStruct.Money, ShouldEqual, 2.2)
		})
	})

	Convey("Reflect from struct", t, func() {
		type Embeded struct {
			Dates  []time.Time `delim:"|"`
			Places []string
			None   []int
		}
		type Author struct {
			Name      string `ini:"NAME"`
			Male      bool
			Age       int
			GPA       float64
			NeverMind string `ini:"-"`
			*Embeded  `ini:"infos"`
		}
		a := &Author{"Unknown", true, 21, 2.8, "",
			&Embeded{
				[]time.Time{time.Now(), time.Now()},
				[]string{"HangZhou", "Boston"},
				[]int{},
			}}
		cfg := Empty()
		So(ReflectFrom(cfg, a), ShouldBeNil)
		cfg.SaveTo("testdata/conf_reflect.ini")

		Convey("Reflect from non-point struct", func() {
			So(ReflectFrom(cfg, Author{}), ShouldNotBeNil)
		})
	})
}

type testMapper struct {
	PackageName string
}

func Test_NameGetter(t *testing.T) {
	Convey("Test name mappers", t, func() {
		So(MapToWithMapper(&testMapper{}, TitleUnderscore, []byte("packag_name=ini")), ShouldBeNil)

		cfg, err := Load([]byte("PACKAGE_NAME=ini"))
		So(err, ShouldBeNil)
		So(cfg, ShouldNotBeNil)

		cfg.NameMapper = AllCapsUnderscore
		tg := new(testMapper)
		So(cfg.MapTo(tg), ShouldBeNil)
		So(tg.PackageName, ShouldEqual, "ini")
	})
}
