package ini

import (
	"fmt"
	"io"
	"os"
)

var (
	_ dataSource = (*sourceFile)(nil)
	_ dataSource = (*sourceData)(nil)
	_ dataSource = (*sourceReadCloser)(nil)
)

// dataSource is a source of INI input.
type dataSource interface {
	// read returns the whole input.
	read() (string, error)
}

// sourceFile represents an object that contains content on the local file system.
type sourceFile struct {
	name string
}

func (s sourceFile) read() (string, error) {
	data, err := os.ReadFile(s.name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// sourceData represents an object that contains content in memory.
type sourceData struct {
	data []byte
}

func (s *sourceData) read() (string, error) {
	// Copy, as the caller may change the slice later.
	return string(s.data), nil
}

// sourceReadCloser represents an input stream with Close method. A stream
// can be read only once, so its data is kept for reloading.
type sourceReadCloser struct {
	reader io.ReadCloser
	data   string
	err    error
}

func (s *sourceReadCloser) read() (string, error) {
	if s.reader != nil {
		data, err := io.ReadAll(s.reader)
		s.data, s.err = string(data), err
		if err := s.reader.Close(); s.err == nil {
			s.err = err
		}
		s.reader = nil
	}
	return s.data, s.err
}

func newDataSource(source any) (dataSource, error) {
	switch s := source.(type) {
	case string:
		return sourceFile{s}, nil
	case []byte:
		return &sourceData{s}, nil
	case io.ReadCloser:
		return &sourceReadCloser{reader: s}, nil
	case io.Reader:
		return &sourceReadCloser{reader: io.NopCloser(s)}, nil
	default:
		return nil, fmt.Errorf("error parsing data source: unknown type %q", s)
	}
}
