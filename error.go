package ini

import (
	"errors"
	"fmt"
)

// Errors a ParseError wraps, for errors.Is.
var (
	// ErrDelimiterNotFound: a line that should be a key has no key-value
	// delimiter.
	ErrDelimiterNotFound = errors.New("key-value delimiter not found")
	// ErrEmptyKeyName: a line starts with its key-value delimiter.
	ErrEmptyKeyName = errors.New("empty key name")
)

// ParseError reports where in a data source parsing failed. Err is the
// error at that position, such as ErrDelimiterNotFound.
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
