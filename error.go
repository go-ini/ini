// Copyright 2016 Unknwon
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
