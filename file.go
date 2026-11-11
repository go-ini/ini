package ini

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
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

	// locking guards the contents with mu, unless loaded with Locking(false).
	locking bool
	mu      sync.RWMutex

	// sections are in the order they were added, and byName indexes
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
// section's name. Loading with CaseInsensitive or CaseInsensitiveSections
// stores it lower-cased.
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

// maybeLock, maybeUnlock, maybeRLock and maybeRUnlock call Lock, Unlock,
// RLock and RUnlock of mu when locking is set, and do nothing otherwise.
// They guard the file's sections, keys and values. Unexported methods that
// change them expect the caller to hold the lock, or the file not to be
// shared yet, as while parsing.
func (f *File) maybeRLock() {
	if f.locking {
		f.mu.RLock()
	}
}

func (f *File) maybeRUnlock() {
	if f.locking {
		f.mu.RUnlock()
	}
}

func (f *File) maybeLock() {
	if f.locking {
		f.mu.Lock()
	}
}

func (f *File) maybeUnlock() {
	if f.locking {
		f.mu.Unlock()
	}
}

// NewSection creates a new section.
func (f *File) NewSection(name string) (*Section, error) {
	f.maybeLock()
	defer f.maybeUnlock()
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
	f.maybeLock()
	defer f.maybeUnlock()
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
	f.maybeRLock()
	defer f.maybeRUnlock()
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
	f.maybeRLock()
	defer f.maybeRUnlock()
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

// Section returns the section with the given name, or the default section
// for "". When there is none, it returns a section that reads as empty
// without adding it to the file. Section.SetComment, Section.NewKey,
// Section.NewBooleanKey, Section.ReflectFrom or a change to one of its keys
// adds it.
func (f *File) Section(name string) *Section {
	if sec, err := f.GetSection(name); err == nil {
		return sec
	}
	return f.missingSection(name, 0)
}

// SectionWithIndex returns the index-th section with the given name. When
// there is none, it returns a section that reads as empty, as Section does,
// and a change adds it after the last one of its name. Without
// AllowNonUniqueSections, it returns the section of the name for any index.
func (f *File) SectionWithIndex(name string, index int) *Section {
	if sec, err := f.sectionWithIndex(name, index); err == nil {
		return sec
	}
	if !f.options.AllowNonUniqueSections {
		return f.Section(name)
	}
	return f.missingSection(name, index)
}

// missingSection returns a section for the index-th section with the given
// name that isn't in the file.
func (f *File) missingSection(name string, index int) *Section {
	if name == "" {
		name = f.defaultSectionName()
	} else if (f.options.Insensitive || f.options.InsensitiveSections) && name != f.options.DefaultSection {
		name = strings.ToLower(name)
	}
	sec := newSection(f, name)
	sec.missing, sec.index = true, index
	return sec
}

// Sections returns a list of Section stored in the current instance.
func (f *File) Sections() []*Section {
	f.maybeRLock()
	defer f.maybeRUnlock()
	return slices.Clone(f.sections)
}

// ChildSections returns a list of child sections of given section name.
func (f *File) ChildSections(name string) []*Section {
	return f.Section(name).ChildSections()
}

// Subsections returns the sections written in Git's config style as
// [name "sub"], with each one's sub. The sections are read each time the
// result is ranged over. The name matches in any case, and spaces and tabs
// between the two parts are allowed. In sub, \" and \\ read as " and \, and
// a backslash before another character is dropped, as in Git.
//
// Section names are read as written, so a few things differ from Git:
//   - Spaces and tabs before the name or after the closing quote are
//     allowed, where Git rejects the header.
//   - A section whose name doesn't split as above, such as a sub without
//     its closing quote, is skipped without an error.
//   - Headers that differ only in spacing, such as [forge "x"] and
//     [forge  "x"], are separate sections, and both are returned with sub
//     "x". So are headers that differ only in the case of the name, such as
//     [Forge "x"], unless CaseInsensitive or CaseInsensitiveSections is set.
//   - With CaseInsensitive or CaseInsensitiveSections, sub is lowercased,
//     as the whole section name is when it's read.
func (f *File) Subsections(name string) iter.Seq2[string, *Section] {
	return func(yield func(string, *Section) bool) {
		for _, sec := range f.Sections() {
			if n, sub, ok := splitSubsection(sec.name); ok && strings.EqualFold(n, name) {
				if !yield(sub, sec) {
					return
				}
			}
		}
	}
}

// Subsection returns the section written in Git's config style as
// [name "sub"], matched as Subsections does. When there are several, it
// returns the one written exactly as [name "sub"], or else the first. When
// there is none, it returns a section that reads as empty, as Section does,
// and a change adds it as [name "sub"], with " and \ in sub escaped. With
// CaseInsensitive or CaseInsensitiveSections, the added header is
// lowercased, as any section name is.
func (f *File) Subsection(name, sub string) *Section {
	quoted := name + ` "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(sub) + `"`
	if sec, err := f.GetSection(quoted); err == nil {
		return sec
	}
	if f.options.Insensitive || f.options.InsensitiveSections {
		sub = strings.ToLower(sub)
	}
	for s, sec := range f.Subsections(name) {
		if s == sub {
			return sec
		}
	}
	return f.Section(quoted)
}

// splitSubsection splits a section name of the form `name "sub"` into name
// and the unescaped sub.
func splitSubsection(s string) (name, sub string, ok bool) {
	s = strings.Trim(s, " \t")
	i := strings.IndexAny(s, " \t")
	if i < 1 {
		return "", "", false
	}
	name, rest := s[:i], strings.TrimLeft(s[i:], " \t")
	if len(rest) < 2 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return "", "", false
	}
	var b strings.Builder
	for i := 1; i < len(rest)-1; i++ {
		switch c := rest[i]; c {
		case '\\':
			i++
			if i == len(rest)-1 {
				return "", "", false // the closing quote is escaped
			}
			b.WriteByte(rest[i])
		case '"':
			return "", "", false
		default:
			b.WriteByte(c)
		}
	}
	return name, b.String(), true
}

// SectionNames returns list of section names.
func (f *File) SectionNames() []string {
	f.maybeRLock()
	defer f.maybeRUnlock()
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

	f.maybeLock()
	defer f.maybeUnlock()
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
// shadows don't pile up. Sections and keys that are still there keep their
// *Section and *Key values, so pointers to them see the new contents. On
// error the file is unchanged.
func (f *File) Reload() error {
	f.maybeRLock()
	sources := f.dataSources
	f.maybeRUnlock()
	next := &File{options: f.options, byName: make(map[string][]*Section)}
	if err := next.load(sources); err != nil {
		return err
	}

	f.maybeLock()
	defer f.maybeUnlock()
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
			o.isRawSection, o.rawBody, o.footer = sec.isRawSection, sec.rawBody, sec.footer
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
// sections and keys add to the current ones and overwrite keys they repeat,
// or add shadow values to them with AllowShadows. Changes made to the file
// in memory stay. With FirstSourceOnly, Append calls Reload instead, so the
// first source that loads wins and changes made in memory are lost.
func (f *File) Append(source any, others ...any) error {
	sources := make([]dataSource, 0, 1+len(others))
	for _, s := range append([]any{source}, others...) {
		ds, err := newDataSource(s)
		if err != nil {
			return err
		}
		sources = append(sources, ds)
	}
	f.maybeLock()
	f.dataSources = append(f.dataSources, sources...)
	if f.options.ShortCircuit {
		f.maybeUnlock()
		// Only the first source that loads counts.
		return f.Reload()
	}
	defer f.maybeUnlock()
	return f.load(sources)
}

func (f *File) writeToBuffer(opts []WriteOption) (*bytes.Buffer, error) {
	f.maybeRLock()
	defer f.maybeRUnlock()
	o := newWriteOptions(opts)
	indent, lineBreak := o.indent, o.lineBreak
	equalSign := o.formatLeft + o.keyValueDelimiter + o.formatRight
	if o.alignDelimiters {
		equalSign = fmt.Sprintf(" %s ", o.keyValueDelimiter)
	}

	// Comment lines that don't start with a comment symbol get one: ";" when
	// it is a comment symbol, otherwise the first one. Without comment
	// symbols, a comment can't be written so that it reads back.
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
	// The usual quoting comes first. When the lexer reads that back
	// differently, other quotings and a bare delimiter are tried, and when
	// none reads back, it's an error rather than a file that loads
	// differently. A file read with delimiters that don't include the
	// written one is meant for other readers, and is written unchecked.
	verify := strings.ContainsAny(o.keyValueDelimiter, f.options.KeyValueDelimiters)
	keyLine := func(key *Key, boolean bool, name, form, eq, val string, nested []string) (string, error) {
		var after string
		for _, nv := range nested {
			after += indent + "  " + nv + lineBreak
		}
		if !boolean && name == "-" && !key.isAutoIncrement {
			// Any key named "-" reads back as an auto-increment key.
			return "", fmt.Errorf("cannot write key %q in section %q: it would read back as an auto-increment key", key.name, key.s.name)
		}
		var line string
		switch {
		case boolean:
			line = form + lineBreak
			if !f.options.AllowBooleanKeys {
				return line, nil
			}
		case strings.Contains(val, "\n") || (val != "" && val[0] == '`'):
			line = form + eq + `"""` + val + `"""` + lineBreak
		case !f.options.IgnoreInlineComment && strings.ContainsAny(val, symbols):
			if strings.Contains(val, "`") {
				line = form + eq + `"""` + val + `"""` + lineBreak
			} else {
				line = form + eq + "`" + val + "`" + lineBreak
			}
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

	// writeComment writes the lines of a section's comment or footer. They
	// are split at "\n" too, so a comment set with "\n" and written with
	// another lineBreak has no line without a comment symbol.
	writeComment := func(comment string) error {
		if comment == "" {
			return nil
		}
		for _, line := range splitLines(comment, lineBreak) {
			line, err := formatComment(line, false)
			if err != nil {
				return err
			}
			buf.WriteString(line + lineBreak)
		}
		return nil
	}
	// Per-key buffers, reused for every key.
	var vals, lines []string
	var empty []bool
	lastSectionIdx := len(f.sections) - 1
	for i, sec := range f.sections {
		sname := sec.name
		if strings.Contains(sname, "\n") {
			return nil, fmt.Errorf("cannot write section %q: its name has a line break", sname)
		}
		if err := writeComment(sec.comment); err != nil {
			return nil, err
		}

		if sec.isRawSection && len(sec.keys) > 0 {
			// Only the body of a raw section is written.
			return nil, fmt.Errorf("cannot write section %q: it is raw and has keys", sname)
		}

		// The first section's header is left out when it is the default
		// section, unless DefaultHeader is set, it has a comment, which needs
		// the header, or it is raw, which only a header makes it. Without
		// keys, it is then not written at all. With keys, the header is
		// still written when the name upper-cased isn't the configured
		// default section name, so a default section renamed in lower case
		// keeps its header, as in v1, unless the header would make it raw.
		isDefault := i == 0 && f.isDefaultSection(sec.name)
		emptyDefault := isDefault && len(sec.keys) == 0
		writeHeader := !isDefault || o.defaultHeader || len(sec.comment) > 0 || sec.isRawSection ||
			(strings.ToUpper(sec.name) != f.options.DefaultSection && !emptyDefault && !f.readsAsRaw(sname))
		if writeHeader {
			if _, err := buf.WriteString("[" + sname + "]" + lineBreak); err != nil {
				return nil, err
			}
		} else if len(sec.keys) == 0 {
			if err := writeComment(sec.footer); err != nil {
				return nil, err
			}
			continue
		}
		if writeHeader && !sec.isRawSection && len(sec.keys) > 0 && f.readsAsRaw(sname) {
			// The keys would read back as the body of a raw section.
			return nil, fmt.Errorf("cannot write the keys of section %q: it is read as a raw section", sname)
		}

		isLastSection := i == lastSectionIdx
		if sec.isRawSection {
			if !f.rawBodyReadsBack(sname, sec.rawBody) {
				return nil, fmt.Errorf("cannot write the body of section %q so that it reads back: a line would start a section or be a comment", sname)
			}
			if _, err := buf.WriteString(sec.rawBody); err != nil {
				return nil, err
			}
			if (!isLastSection || sec.footer != "") && sec.rawBody != "" && !strings.HasSuffix(sec.rawBody, "\n") {
				// The next section header, or the footer, starts on its
				// own line.
				buf.WriteString(lineBreak)
			}
			if err := writeComment(sec.footer); err != nil {
				return nil, err
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
			// value "true" added, and a value set after the boolean line
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
				comments := splitLines(key.comment, lineBreak)
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
		if err := writeComment(sec.footer); err != nil {
			return nil, err
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
	return err == nil && tok.kind == tokEOF && tok.comment == ""
}

// readsAsRaw reports whether the lexer reads the section name, after its
// header, as a raw section.
func (f *File) readsAsRaw(name string) bool {
	l := lexer{src: "[" + name + "]\n", opts: &f.options}
	tok, err := l.next()
	return err == nil && tok.kind == tokSection && l.inRaw
}

// rawBodyReadsBack reports whether the lexer reads body, under the header of
// the raw section name, as that section's body and nothing else. Blank lines
// and the indentation of lines aren't kept, as when loading. A section that
// the file's options don't read as raw is written for other readers, and
// isn't checked.
func (f *File) rawBodyReadsBack(name, body string) bool {
	if !f.readsAsRaw(name) {
		return true
	}
	l := lexer{src: "[" + name + "]\n" + body, opts: &f.options}
	if _, err := l.next(); err != nil {
		return false
	}
	tok, err := l.next()
	if err == nil && tok.kind == tokRawBody {
		tok, err = l.next()
	}
	return err == nil && tok.kind == tokEOF && tok.comment == ""
}

// splitLines splits s at lineBreak, and then at "\n", so that no line has
// a "\n" in it.
func splitLines(s, lineBreak string) []string {
	var lines []string
	for _, line := range strings.Split(s, lineBreak) {
		lines = append(lines, strings.Split(line, "\n")...)
	}
	return lines
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
