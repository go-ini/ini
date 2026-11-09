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

// Package ini provides INI file read and write functionality in Go.
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

	// Maximum allowed depth when recursively substituing variable names.
	depthValues = 99
	// Limits of one %(name)s expansion; see Key.expand.
	maxSubstitutions = 10000
	maxExpandedLen   = 1 << 20
)

// Variable regexp pattern: %(variable)s
var varPattern = regexp.MustCompile(`%\(([^)]+)\)s`)

// Load loads and parses an INI data source. A source is a file name
// (string), raw data ([]byte), or an io.Reader; an io.ReadCloser is closed
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
