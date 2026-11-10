// Copyright 2017 Unknwon
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// File represents a combination of one or more INI files in memory.
type File struct {
	options     loadOptions
	dataSources []dataSource

	// locking guards the contents with lock, unless loaded with Locking(false).
	locking bool
	lock    sync.RWMutex

	// sections are in the order they were added; byName indexes
	// them, in the same order, by name.
	sections []*Section
	byName   map[string][]*Section
}

// newFile initializes File object with given data sources.
func newFile(dataSources []dataSource, opts []LoadOption) *File {
	o := newLoadOptions(opts)
	return &File{
		locking:     o.Locking,
		dataSources: dataSources,
		byName:      make(map[string][]*Section),
		options:     o,
	}
}

// Empty returns an empty file object.
func Empty(opts ...LoadOption) *File {
	// Ignore error here, we are sure our data is good.
	f, _ := Load([]byte(""), opts...)
	return f
}

// isDefaultSection reports whether name, as stored, is the default
// section's name. Loading with CaseInsensitive or CaseInsensitiveSections stores it
// lower-cased.
func (f *File) isDefaultSection(name string) bool {
	return name == f.options.DefaultSection || name == f.defaultSectionName()
}

// defaultSectionName returns the name the default section is stored under.
func (f *File) defaultSectionName() string {
	if f.options.Insensitive || f.options.InsensitiveSections {
		return strings.ToLower(f.options.DefaultSection)
	}
	return f.options.DefaultSection
}

// The lock helpers guard the file's sections, keys and values when
// locking is set. Unexported methods that change them expect the caller to
// hold the lock, or the file not to be shared yet, as while parsing.
func (f *File) rlock() {
	if f.locking {
		f.lock.RLock()
	}
}

func (f *File) runlock() {
	if f.locking {
		f.lock.RUnlock()
	}
}

func (f *File) wlock() {
	if f.locking {
		f.lock.Lock()
	}
}

func (f *File) wunlock() {
	if f.locking {
		f.lock.Unlock()
	}
}

// NewSection creates a new section.
func (f *File) NewSection(name string) (*Section, error) {
	f.wlock()
	defer f.wunlock()
	return f.addSection(name)
}

// addSection is NewSection for a caller that holds the lock.
func (f *File) addSection(name string) (*Section, error) {
	if len(name) == 0 {
		return nil, errors.New("empty section name")
	}

	if (f.options.Insensitive || f.options.InsensitiveSections) && name != f.options.DefaultSection {
		name = strings.ToLower(name)
	}

	if secs := f.byName[name]; len(secs) > 0 && !f.options.AllowNonUniqueSections {
		return secs[0], nil
	}

	sec := newSection(f, name)
	f.sections = append(f.sections, sec)
	f.byName[name] = append(f.byName[name], sec)
	return sec, nil
}

// NewRawSection creates a new section with an unparseable body.
func (f *File) NewRawSection(name, body string) (*Section, error) {
	f.wlock()
	defer f.wunlock()
	section, err := f.addSection(name)
	if err != nil {
		return nil, err
	}

	section.isRawSection = true
	section.rawBody = body
	return section, nil
}

// NewSections creates a list of sections.
func (f *File) NewSections(names ...string) (err error) {
	for _, name := range names {
		if _, err = f.NewSection(name); err != nil {
			return err
		}
	}
	return nil
}

// GetSection returns section by given name.
func (f *File) GetSection(name string) (*Section, error) {
	return f.sectionWithIndex(name, 0)
}

// sectionWithIndex returns the index-th section with the given name.
func (f *File) sectionWithIndex(name string, index int) (*Section, error) {
	f.rlock()
	defer f.runlock()
	secs, err := f.sectionsByName(name)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(secs) {
		return nil, fmt.Errorf("section %q has no index %d", name, index)
	}
	return secs[index], nil
}

// HasSection returns true if the file contains a section with given name.
func (f *File) HasSection(name string) bool {
	section, _ := f.GetSection(name)
	return section != nil
}

// SectionsByName returns all sections with given name.
func (f *File) SectionsByName(name string) ([]*Section, error) {
	f.rlock()
	defer f.runlock()
	secs, err := f.sectionsByName(name)
	// A copy, as deleting a section shifts the ones after it in place.
	return slices.Clone(secs), err
}

// sectionsByName is SectionsByName for a caller that holds the lock.
func (f *File) sectionsByName(name string) ([]*Section, error) {
	if len(name) == 0 {
		name = f.options.DefaultSection
	}
	if f.options.Insensitive || f.options.InsensitiveSections {
		name = strings.ToLower(name)
	}

	secs := f.byName[name]
	if len(secs) == 0 {
		return nil, fmt.Errorf("section %q does not exist", name)
	}
	return secs, nil
}

// Section assumes named section exists and returns a zero-value when not.
func (f *File) Section(name string) *Section {
	sec, err := f.GetSection(name)
	if err != nil {
		if name == "" {
			// The name the lookup used, so the next lookup finds it.
			name = f.defaultSectionName()
		}
		sec, _ = f.NewSection(name)
		return sec
	}
	return sec
}

// SectionWithIndex assumes named section exists and returns a new section when not.
func (f *File) SectionWithIndex(name string, index int) *Section {
	if name == "" {
		name = f.defaultSectionName()
	}
	sec, err := f.sectionWithIndex(name, index)
	if err != nil {
		newSec, _ := f.NewSection(name)
		return newSec
	}
	return sec
}

// Sections returns a list of Section stored in the current instance.
func (f *File) Sections() []*Section {
	f.rlock()
	defer f.runlock()
	return slices.Clone(f.sections)
}

// ChildSections returns a list of child sections of given section name.
func (f *File) ChildSections(name string) []*Section {
	return f.Section(name).ChildSections()
}

// SectionNames returns list of section names.
func (f *File) SectionNames() []string {
	f.rlock()
	defer f.runlock()
	names := make([]string, len(f.sections))
	for i, sec := range f.sections {
		names[i] = sec.name
	}
	return names
}

// DeleteSection deletes a section or all sections with given name.
func (f *File) DeleteSection(name string) {
	secs, err := f.SectionsByName(name)
	if err != nil {
		return
	}

	for i := 0; i < len(secs); i++ {
		// For non-unique sections, it is always needed to remove the first one so
		// in the next iteration, the subsequent section continue having index 0.
		// Ignoring the error as index 0 never returns an error.
		_ = f.DeleteSectionWithIndex(name, 0)
	}
}

// DeleteSectionWithIndex deletes a section with given name and index.
func (f *File) DeleteSectionWithIndex(name string, index int) error {
	if index < 0 {
		return fmt.Errorf("delete section with negative index %d", index)
	}
	if !f.options.AllowNonUniqueSections && index != 0 {
		return fmt.Errorf("delete section with non-zero index is only allowed when non-unique sections is enabled")
	}

	if len(name) == 0 {
		name = f.options.DefaultSection
	}
	if f.options.Insensitive || f.options.InsensitiveSections {
		name = strings.ToLower(name)
	}

	f.wlock()
	defer f.wunlock()
	secs := f.byName[name]
	if index >= len(secs) {
		return nil
	}
	sec := secs[index]
	if len(secs) == 1 {
		delete(f.byName, name)
	} else {
		f.byName[name] = slices.Delete(secs, index, index+1)
	}
	f.sections = slices.DeleteFunc(f.sections, func(s *Section) bool { return s == sec })
	return nil
}

// loadSource parses one data source into f.
func (f *File) loadSource(s dataSource) error {
	r, err := s.ReadCloser()
	if err != nil {
		return err
	}
	defer r.Close()

	return f.parse(r)
}

// Reload reads all data sources again and replaces the contents of the file
// with them: keys and sections no longer in the sources are gone, and
// shadows don't pile up. Sections and keys that are still there stay the
// same values, so pointers to them see the new contents. On error the file
// is unchanged.
func (f *File) Reload() error {
	f.rlock()
	sources := f.dataSources
	f.runlock()
	next := &File{options: f.options, byName: make(map[string][]*Section)}
	if err := next.load(sources); err != nil {
		return err
	}

	f.wlock()
	defer f.wunlock()
	for name, secs := range next.byName {
		old := f.byName[name]
		for i, sec := range secs {
			if i >= len(old) {
				sec.f = f
				continue
			}
			// Keep the section and keys callers may hold, with the new
			// contents. Their s and f stay, as readers use them to find
			// the lock.
			o := old[i]
			for j, k := range sec.keys {
				if ok := o.byName[k.name]; ok != nil {
					ok.comment, ok.value, ok.shadows = k.comment, k.value, k.shadows
					ok.isAutoIncrement, ok.isBooleanType = k.isAutoIncrement, k.isBooleanType
					ok.nestedValues = k.nestedValues
					sec.keys[j], sec.byName[k.name] = ok, ok
				} else {
					k.s = o
				}
			}
			o.comment, o.keys, o.byName = sec.comment, sec.keys, sec.byName
			o.isRawSection, o.rawBody = sec.isRawSection, sec.rawBody
			secs[i] = o
		}
	}
	// The n-th section of a name in next.sections is the n-th in
	// next.byName, which now holds the sections kept.
	seen := make(map[string]int)
	for i, sec := range next.sections {
		next.sections[i] = next.byName[sec.name][seen[sec.name]]
		seen[sec.name]++
	}
	f.sections, f.byName = next.sections, next.byName
	return nil
}

// load parses sources into f, in order.
func (f *File) load(sources []dataSource) error {
	for _, s := range sources {
		if err := f.loadSource(s); err != nil {
			// In loose mode, we create an empty default section for nonexistent files.
			if os.IsNotExist(err) && f.options.Loose {
				_ = f.parse(bytes.NewBuffer(nil))
				continue
			}
			return err
		}
		if f.options.ShortCircuit {
			return nil
		}
	}
	return nil
}

// Append adds one or more data sources and reads them into the file. Their
// sections and keys add to the current ones and overwrite keys they repeat;
// changes made to the file in memory stay.
func (f *File) Append(source any, others ...any) error {
	sources := make([]dataSource, 0, 1+len(others))
	for _, s := range append([]any{source}, others...) {
		ds, err := newDataSource(s)
		if err != nil {
			return err
		}
		sources = append(sources, ds)
	}
	f.wlock()
	f.dataSources = append(f.dataSources, sources...)
	if f.options.ShortCircuit {
		f.wunlock()
		// Only the first source that loads counts.
		return f.Reload()
	}
	defer f.wunlock()
	return f.load(sources)
}

func (f *File) writeToBuffer(opts []WriteOption) (*bytes.Buffer, error) {
	f.rlock()
	defer f.runlock()
	o := newWriteOptions(opts)
	indent, lineBreak := o.indent, o.lineBreak
	equalSign := o.formatLeft + o.keyValueDelimiter + o.formatRight
	if o.alignDelimiters {
		equalSign = fmt.Sprintf(" %s ", o.keyValueDelimiter)
	}

	// Comment lines that don't start with a comment symbol get one: ";" as in
	// v1 when it is a comment symbol, otherwise the first one. Without
	// comment symbols, a comment can't be written so that it reads back.
	symbols := f.options.CommentSymbols
	commentPrefix := ";"
	if !strings.Contains(symbols, ";") && symbols != "" {
		r, _ := utf8.DecodeRuneInString(symbols)
		commentPrefix = string(r)
	}
	// formatComment puts the comment symbol of line, or commentPrefix, and a
	// space before its text.
	formatComment := func(line string, trim bool) (string, error) {
		if symbols == "" {
			return "", errors.New("cannot write comment: no comment symbols set")
		}
		if trim {
			line = strings.TrimSpace(line)
		}
		if !startsWithAny(line, symbols) {
			return commentPrefix + " " + line, nil
		}
		_, n := utf8.DecodeRuneInString(line)
		return line[:n] + " " + strings.TrimSpace(line[n:]), nil
	}

	// keyLine returns the line that writes val as a value of key, written as
	// form, or key itself for a boolean key, followed by nested values.
	// v1's quoting comes first; when the lexer reads that back differently,
	// other quotings and a bare delimiter are tried, and when none reads
	// back, it's an error rather than a file that loads differently. A file
	// read with delimiters that don't include the written one is meant for
	// other readers, and is written as v1 did.
	verify := strings.ContainsAny(o.keyValueDelimiter, f.options.KeyValueDelimiters)
	keyLine := func(key *Key, boolean bool, name, form, eq, val string, nested []string) (string, error) {
		var after string
		for _, nv := range nested {
			after += indent + "  " + nv + lineBreak
		}
		var line string
		switch {
		case boolean:
			line = form + lineBreak
			if !f.options.AllowBooleanKeys {
				return line, nil
			}
		case strings.ContainsAny(val, "\n`"):
			line = form + eq + `"""` + val + `"""` + lineBreak
		case !f.options.IgnoreInlineComment && strings.ContainsAny(val, symbols):
			line = form + eq + "`" + val + "`" + lineBreak
		case len(strings.TrimSpace(val)) != len(val):
			line = form + eq + `"` + val + `"` + lineBreak
		default:
			line = form + eq + val + lineBreak
		}
		if !verify || f.readsBack(line+after, boolean, name, val, nested) {
			return line + after, nil
		}

		quotings := func(s string) []string {
			q := []string{s, "`" + s + "`", `"""` + s + `"""`, `"` + s + `"`}
			if f.options.UnescapeValueDoubleQuotes {
				q = append(q, `"`+strings.ReplaceAll(s, `"`, `\"`)+`"`)
			}
			return q
		}
		forms := []string{"-"}
		if !key.isAutoIncrement {
			forms = quotings(name)
		}
		if boolean {
			for _, k := range forms {
				if line = k + lineBreak; f.readsBack(line, boolean, name, "", nil) {
					return line, nil
				}
			}
		} else {
			for _, e := range []string{eq, o.keyValueDelimiter} {
				for _, k := range forms {
					for _, v := range quotings(val) {
						if line = k + e + v + lineBreak + after; f.readsBack(line, boolean, name, val, nested) {
							return line, nil
						}
					}
				}
			}
		}
		return "", fmt.Errorf("cannot write key %q with value %q in section %q so that it reads back", key.name, val, key.s.name)
	}

	// Use buffer to make sure target is safe until finish encoding.
	buf := bytes.NewBuffer(nil)
	// Per-key buffers, reused for every key.
	var vals, lines []string
	var empty []bool
	lastSectionIdx := len(f.sections) - 1
	for i, sec := range f.sections {
		sname := sec.name
		if strings.Contains(sname, "\n") {
			return nil, fmt.Errorf("cannot write section %q: its name has a line break", sname)
		}
		if len(sec.comment) > 0 {
			// Support multiline comments
			lines := strings.Split(sec.comment, lineBreak)
			for i := range lines {
				var err error
				if lines[i], err = formatComment(lines[i], false); err != nil {
					return nil, err
				}

				if _, err := buf.WriteString(lines[i] + lineBreak); err != nil {
					return nil, err
				}
			}
		}

		// v1 left out the header of the default section only when its name was
		// upper case, so a renamed default section keeps its header. It is
		// skipped like any default section when it has no keys.
		emptyDefault := i == 0 && len(sec.keys) == 0 && strings.EqualFold(sec.name, f.options.DefaultSection)
		// A comment needs the header to stay on the section.
		if i > 0 || o.defaultHeader || len(sec.comment) > 0 || (strings.ToUpper(sec.name) != f.options.DefaultSection && !emptyDefault) {
			if _, err := buf.WriteString("[" + sname + "]" + lineBreak); err != nil {
				return nil, err
			}
		} else {
			// Write nothing if default section is empty
			if len(sec.keys) == 0 {
				continue
			}
		}

		isLastSection := i == lastSectionIdx
		if sec.isRawSection {
			if _, err := buf.WriteString(sec.rawBody); err != nil {
				return nil, err
			}
			if !isLastSection && sec.rawBody != "" && !strings.HasSuffix(sec.rawBody, "\n") {
				// The next section header starts on its own line.
				buf.WriteString(lineBreak)
			}

			if o.blankLines && !isLastSection {
				// Put a line between sections
				if _, err := buf.WriteString(lineBreak); err != nil {
					return nil, err
				}
			}
			continue
		}

		// Count and generate alignment length and buffer spaces using the
		// longest key. Keys may be modified if they contain certain characters so
		// we need to take that into account in our calculation.
		alignLength := 0
		if o.alignDelimiters {
			for _, key := range sec.keys {
				kname := key.name
				keyLength := len(kname)
				// First case will surround key by ` and second by """
				if strings.Contains(kname, "\"") || strings.ContainsAny(kname, f.options.KeyValueDelimiters) {
					keyLength += 2
				} else if strings.Contains(kname, "`") {
					keyLength += 6
				}

				if keyLength > alignLength {
					alignLength = keyLength
				}
			}
		}
		alignSpaces := bytes.Repeat([]byte(" "), alignLength)

		keyIndent := ""
		if !f.isDefaultSection(sname) {
			keyIndent = indent
		}
		// swallows is set when the lexer would read an indented line as part
		// of the last value: always in Python multiline values, and after an
		// empty value with nested values.
		swallows := false
		writeIndented := func(line string) error {
			if keyIndent != "" && swallows {
				return fmt.Errorf("cannot indent line %q in section %q: it would be read as part of the value before it", line, sname)
			}
			buf.WriteString(keyIndent + line)
			return nil
		}

		for _, key := range sec.keys {
			kname := key.name
			name, form := kname, kname
			switch {
			case key.isAutoIncrement:
				name, form = "-", "-"
			case strings.Contains(kname, "\"") || strings.ContainsAny(kname, f.options.KeyValueDelimiters):
				form = "`" + kname + "`"
			case strings.Contains(kname, "`"):
				form = `"""` + kname + `"""`
			}

			pad := ""
			if o.alignDelimiters {
				pad = string(alignSpaces[:max(0, alignLength-len(form))])
			}
			// lines are the lines of the key, each with the nested values
			// that follow it, and empty tells whether its value is empty.
			lines, empty = lines[:0], empty[:0]
			addLine := func(boolean bool, val string, nested []string) error {
				eq := pad + equalSign
				if boolean {
					eq = ""
				}
				line, err := keyLine(key, boolean, name, form, eq, val, nested)
				lines = append(lines, line)
				empty = append(empty, !boolean && val == "")
				return err
			}

			// All values are written, empty ones too: an empty value resets
			// the values before it in systemd units, and nested values
			// follow an empty value.
			vals = append(vals[:0], key.value)
			vals = append(vals, key.shadows...)

			// regular are the values written on key lines. A boolean line
			// after a key of the same name makes it a boolean key with the
			// value "true" added; a value set after the boolean line
			// replaces "true".
			regular, boolFirst := vals, false
			if key.isBooleanType {
				switch {
				case !f.options.AllowBooleanKeys:
					regular = nil
				case len(key.shadows) > 0:
					if vals[len(vals)-1] == "true" {
						regular = vals[:len(vals)-1]
					}
				case key.value != "true":
					boolFirst = true
				default:
					regular = nil
				}
			}
			nested := key.nestedValues
			if key.isBooleanType && boolFirst {
				if len(nested) > 0 {
					// Nested values can't follow a boolean line, so they
					// go before it, after an empty value.
					if err := addLine(false, "", nested); err != nil {
						return nil, err
					}
					nested = nil
				}
				if err := addLine(true, "", nil); err != nil {
					return nil, err
				}
			}
			if len(nested) > 0 && !slices.Contains(regular, "") {
				// Nested values only follow an empty value. Without shadows
				// the value after it still wins.
				regular = append([]string{""}, regular...)
			}
			for _, val := range regular {
				if val != "" {
					if err := addLine(false, val, nil); err != nil {
						return nil, err
					}
					continue
				}
				if err := addLine(false, val, nested); err != nil {
					return nil, err
				}
				nested = nil
			}
			if key.isBooleanType && !boolFirst {
				if err := addLine(true, "", nil); err != nil {
					return nil, err
				}
			}

			// The lexer gives a key the comment before its last line, so a
			// key written on several lines has its comment there.
			last := len(lines) - 1
			for i, line := range lines[:last] {
				if err := writeIndented(line); err != nil {
					return nil, err
				}
				swallows = f.options.AllowPythonMultilineValues || (f.options.AllowNestedValues && empty[i])
			}
			if len(key.comment) > 0 {
				if err := writeIndented(""); err != nil {
					return nil, err
				}
				// Support multiline comments
				comments := strings.Split(key.comment, lineBreak)
				for i := range comments {
					var err error
					if comments[i], err = formatComment(comments[i], true); err != nil {
						return nil, err
					}
					buf.WriteString(comments[i] + lineBreak)
				}
			}
			if err := writeIndented(lines[last]); err != nil {
				return nil, err
			}
			swallows = f.options.AllowPythonMultilineValues || (f.options.AllowNestedValues && empty[last])
		}

		if o.blankLines && !isLastSection {
			// Put a line between sections
			if _, err := buf.WriteString(lineBreak); err != nil {
				return nil, err
			}
		}
	}

	return buf, nil
}

// readsBack reports whether the lexer reads text as one key, or boolean key,
// named name with value val and no comment, followed by nested values.
func (f *File) readsBack(text string, boolean bool, name, val string, nested []string) bool {
	// A byte order mark is skipped at the start of a file.
	l := lexer{src: skipBOM(text), opts: &f.options}
	tok, err := l.next()
	if err != nil {
		return false
	}
	kind := tokKey
	if boolean {
		kind = tokBooleanKey
	}
	if tok.kind != kind || tok.name != name || tok.value != val || tok.comment != "" {
		return false
	}
	for _, nv := range nested {
		if tok, err = l.next(); err != nil || tok.kind != tokNestedValue || tok.value != nv {
			return false
		}
	}
	tok, err = l.next()
	return err == nil && tok.kind == tokEOF && len(l.comment) == 0
}

// Write writes the file to w, formatted by opts.
func (f *File) Write(w io.Writer, opts ...WriteOption) (int64, error) {
	buf, err := f.writeToBuffer(opts)
	if err != nil {
		return 0, err
	}
	return buf.WriteTo(w)
}

// WriteTo writes the file to w with the default format.
func (f *File) WriteTo(w io.Writer) (int64, error) {
	return f.Write(w)
}

// SaveTo writes the file to the named file, formatted by opts.
func (f *File) SaveTo(filename string, opts ...WriteOption) error {
	buf, err := f.writeToBuffer(opts)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0666)
}
