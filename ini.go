// Package ini reads and writes INI files.
//
// Load parses one source, a file name, []byte or io.Reader, and LoadSources
// merges several, later ones overriding earlier ones. Without options, every
// input parses exactly as it did with gopkg.in/ini.v1. LoadOption values
// such as AllowShadows, CaseInsensitive or IgnoreInlineComment change the
// syntax that is accepted.
//
// A File holds Sections in order, and each Section holds Keys in order. The
// default section, named DefaultSection, holds the keys before the first
// section header. Pass "" to refer to it. Key returns values as strings or
// parses them, as in Key.Int or Key.Duration, and the generic List, In and
// Range parse lists, candidates and ranges of any Scalar type. A value can
// refer to another key of its section or of the default section as
// %(name)s.
//
// MapTo and StrictMapTo fill a struct from a File, and File.ReflectFrom
// fills a File from a struct. Unmarshal and Marshal do the same for INI
// text, like encoding/json. Field names map to key and section names
// unchanged unless NameMapping sets a mapper, and the ini, delim and
// comment struct tags adjust each field.
//
// File.Write and File.SaveTo write a File back with its comments and in its
// order, and WriteOption values set the formatting. What Write writes loads
// back to the same sections, keys, values and comments, or Write returns an
// error.
//
// A File is safe for concurrent use unless loaded with Locking(false).
// Parse errors are *ParseError values that give the line and column.
package ini

import (
	"regexp"
)

const (
	// DefaultSection is the default name of the section that holds keys
	// before the first section header. Pass "" to the section methods to
	// refer to it whatever its name, and change the name with
	// DefaultSectionName.
	DefaultSection = "DEFAULT"

	// Maximum depth of nested %(name)s references.
	depthValues = 99
	// Limits of one %(name)s expansion, described at Key.expand.
	maxSubstitutions = 10000
	maxExpandedLen   = 1 << 20
)

// Variable regexp pattern: %(variable)s
var varPattern = regexp.MustCompile(`%\(([^)]+)\)s`)

// Load loads and parses an INI data source. A source is a file name
// (string), raw data ([]byte), or an io.Reader. An io.ReadCloser is closed
// after reading.
func Load(source any, opts ...LoadOption) (*File, error) {
	return LoadSources([]any{source}, opts...)
}

// LoadSources loads and parses several INI data sources into one File.
// Later sources add to earlier ones and overwrite keys they repeat.
func LoadSources(sources []any, opts ...LoadOption) (*File, error) {
	dataSources := make([]dataSource, len(sources))
	for i := range sources {
		var err error
		if dataSources[i], err = newDataSource(sources[i]); err != nil {
			return nil, err
		}
	}
	f := newFile(dataSources, opts)
	if err := f.load(dataSources); err != nil {
		return nil, err
	}
	return f, nil
}
