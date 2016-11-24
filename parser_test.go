package ini

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCleanComment(t *testing.T) {
	Convey("Sentences with comments", t, func() {
		bytes, hasComment := cleanComment([]byte("key = value ;comment"))
		So(hasComment, ShouldBeTrue)
		So(string(bytes), ShouldEqual, ";comment")

		bytes, hasComment = cleanComment([]byte("key = value #comment"))
		So(hasComment, ShouldBeTrue)
		So(string(bytes), ShouldEqual, "#comment")
	})

	Convey("Sentences with escaped comments", t, func() {
		bytes, hasComment := cleanComment([]byte("key = value \\;comment"))
		So(hasComment, ShouldBeFalse)
		So(len(bytes), ShouldEqual, 0)

		bytes, hasComment = cleanComment([]byte("key = value \\#comment"))
		So(hasComment, ShouldBeFalse)
		So(len(bytes), ShouldEqual, 0)
	})
}
