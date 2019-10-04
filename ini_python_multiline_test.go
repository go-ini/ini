package ini_test

import (
	"path/filepath"
	"testing"

	"gopkg.in/ini.v1"
	. "github.com/smartystreets/goconvey/convey"
)

type testData struct {
	Value1 string `ini:"value1"`
	Value2 string `ini:"value2"`
}

func TestMultiline(t *testing.T) {
	Convey("Parse Python-style multiline values", t, func() {

		// enable Python-style multiline values
		opts := ini.LoadOptions{
			AllowPythonMultilineValues: true,
		}

		// load test data
		path := filepath.Join("testdata", "multiline.ini")
		f, err := ini.LoadSources(opts, path)

		// Should have no error
		So(err, ShouldBeNil)

		// Should have parsed data
		So(f, ShouldNotBeNil)

		// Should have only the default section
		So(len(f.Sections()), ShouldEqual, 1)

		// Should have default section
		defaultSection := f.Section("")
		So(f.Section(""), ShouldNotBeNil)

		// Default section should map to test data struct
		var testData testData
		e = defaultSection.MapTo(&testData)
		So(e, ShouldBeNil)

		// Parsed values should match expected values
		So(testData.Value1, ShouldEqual, "some text here\nsome more text here\n\nthere is an empty line above and below\n")
		So(testData.Value2, ShouldEqual, "there is an empty line above\nthat is not indented so it should not be part\nof the value")
	})
}