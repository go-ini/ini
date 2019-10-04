package ini_test

import (
	"gopkg.in/ini.v1"
	. "github.com/smartystreets/goconvey/convey"
	"path/filepath"
	"testing"
)

type testData struct {
	Value1 string `ini:"value1"`
	Value2 string `ini:"value2"`
}

func TestMultiline(t *testing.T) {
	Value1 := "some text here\n"+
		"some more text here\n"+
		"\n"+
		"there is an empty line above and below\n"

	Value2 := "there is an empty line above\n"+
		"that is not indented so it should not be part\n"+
		"of the value"

	Convey("Parse Python-style multiline values", t, func() {

		// enable Python-style multiline values
		opts := ini.LoadOptions{
			AllowPythonMultilineValues: true,
		}

		// load test data
		path := filepath.Join("testdata", "multiline.ini")
		data, e := ini.LoadSources(opts, path)

		// Should have no error
		So(e, ShouldBeNil)

		// Should have parsed data
		So(data, ShouldNotBeNil)

		// Should have only the default section
		So(len(data.Sections()), ShouldEqual, 1)

		// Should have default section
		defaultSection := data.Section("")
		So(defaultSection, ShouldNotBeNil)

		// Default section should map to test data struct
		var testData testData
		e = defaultSection.MapTo(&testData)
		So(e, ShouldBeNil)

		// 'value1' should match expected value
		So(testData.Value1, ShouldEqual, Value1)

		// 'value2' should match expected value
		So(testData.Value2, ShouldEqual, Value2)
	})
}