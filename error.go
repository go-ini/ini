package ini

import (
	"errors"
	"fmt"
)

// ErrDelimiterNotFound and ErrEmptyKeyName are the parse errors that
// errors.Is can identify. Other parse errors, such as an unclosed section,
// have no sentinel.
var (
	// ErrDelimiterNotFound: a line that should be a key has no key-value
	// delimiter.
	ErrDelimiterNotFound = errors.New("key-value delimiter not found")
	// ErrEmptyKeyName: a line starts with its key-value delimiter.
	ErrEmptyKeyName = errors.New("empty key name")
)

// ParseError reports where in a data source parsing failed. Err describes
// the problem and wraps ErrDelimiterNotFound or ErrEmptyKeyName when one
// applies.
type ParseError struct {
	Line   int // 1-based line number
	Column int // 1-based column, in characters, where the line's content starts
	Err    error
}

func (err *ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %v", err.Line, err.Column, err.Err)
}

func (err *ParseError) Unwrap() error {
	return err.Err
}
