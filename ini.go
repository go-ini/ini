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
// Later sources add to earlier ones and overwrite keys they repeat, or add
// shadow values to them with AllowShadows.
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
