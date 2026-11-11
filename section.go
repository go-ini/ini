package ini

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Section represents a config section.
type Section struct {
	f       *File
	name    string
	comment string
	// keys are in the order they were added, and byName indexes them by name.
	keys   []*Key
	byName map[string]*Key

	isRawSection bool
	rawBody      string

	// missing is set on a section File.Section or File.SectionWithIndex
	// returned for a name, or an index of it, the file doesn't have. The
	// section isn't in the file until a change adds it. index is the index
	// of the name it stands for. added is set once a change adds it, so a
	// missing section for another index doesn't take it for its own.
	missing bool
	added   bool
	index   int
}

func newSection(f *File, name string) *Section {
	return &Section{
		f:      f,
		name:   name,
		keys:   make([]*Key, 0, 10),
		byName: make(map[string]*Key),
	}
}

// current returns the section s reads and changes. That is s, unless s is
// missing and its file has since gained a section of its name and index,
// other than one added for another index. The caller holds the lock.
func (s *Section) current() *Section {
	if s.missing {
		if secs := s.f.byName[s.name]; s.index >= 0 && s.index < len(secs) {
			if c := secs[s.index]; !c.added || c.index == s.index {
				return c
			}
		}
	}
	return s
}

// attach adds s to its file if it's missing and the file still has no
// section of its name and index, and returns the section to change. The
// caller holds the write lock.
func (s *Section) attach() *Section {
	c := s.current()
	if c == s && s.missing {
		s.missing, s.added = false, true
		s.f.sections = append(s.f.sections, s)
		s.f.byName[s.name] = append(s.f.byName[s.name], s)
	}
	return c
}

// Name returns name of Section.
func (s *Section) Name() string {
	return s.name
}

// Comment returns the comment lines before the section header, with their
// comment symbols.
func (s *Section) Comment() string {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	return s.current().comment
}

// SetComment sets the comment lines written before the section header.
// Lines without a comment symbol get one.
func (s *Section) SetComment(comment string) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	s.attach().comment = comment
}

// Body returns rawBody of Section if the section was marked as unparseable.
// It still follows the other rules of the INI format surrounding leading/trailing whitespace.
func (s *Section) Body() string {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	return strings.TrimSpace(s.current().rawBody)
}

// SetBody updates body content only if section is raw.
func (s *Section) SetBody(body string) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	if s = s.current(); !s.isRawSection {
		return
	}
	s.rawBody = body
}

// NewKey creates a new key to given section.
func (s *Section) NewKey(name, val string) (*Key, error) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	key, err := s.current().addKey(name, val)
	if err == nil {
		s.attach()
	}
	return key, err
}

// addKey is NewKey for a caller that holds the lock.
func (s *Section) addKey(name, val string) (*Key, error) {
	if len(name) == 0 {
		return nil, errors.New("error creating new key: empty key name")
	} else if s.f.options.Insensitive || s.f.options.InsensitiveKeys {
		name = strings.ToLower(name)
	}

	if key := s.byName[name]; key != nil {
		if s.f.options.AllowShadows {
			if err := key.addShadow(val); err != nil {
				return nil, err
			}
		} else {
			key.value = val
		}
		return key, nil
	}

	key := newKey(s, name, val)
	s.keys = append(s.keys, key)
	s.byName[name] = key
	return key, nil
}

// NewBooleanKey creates a new boolean type key to given section.
func (s *Section) NewBooleanKey(name string) (*Key, error) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	key, err := s.current().addBooleanKey(name)
	if err == nil {
		s.attach()
	}
	return key, err
}

// addBooleanKey is NewBooleanKey for a caller that holds the lock.
func (s *Section) addBooleanKey(name string) (*Key, error) {
	key, err := s.addKey(name, "true")
	if err != nil {
		return nil, err
	}

	key.isBooleanType = true
	return key, nil
}

// GetKey returns key in section by given name.
func (s *Section) GetKey(name string) (*Key, error) {
	if s.f.locking {
		s.f.mu.RLock()
	}
	if s.f.options.Insensitive || s.f.options.InsensitiveKeys {
		name = strings.ToLower(name)
	}
	key := s.current().byName[name]
	if s.f.locking {
		s.f.mu.RUnlock()
	}

	if key == nil {
		// Check if it is a child-section.
		sname := s.name
		for {
			if i := strings.LastIndex(sname, s.f.options.ChildSectionDelimiter); i > -1 {
				sname = sname[:i]
				sec, err := s.f.GetSection(sname)
				if err != nil {
					continue
				}
				return sec.GetKey(name)
			}
			break
		}
		return nil, fmt.Errorf("error when getting key of section %q: key %q not exists", s.name, name)
	}
	return key, nil
}

// HasKey returns true if section contains a key with given name.
func (s *Section) HasKey(name string) bool {
	key, _ := s.GetKey(name)
	return key != nil
}

// HasValue returns true if section contains given raw value.
func (s *Section) HasValue(value string) bool {
	if s.f.locking {
		s.f.mu.RLock()
		defer s.f.mu.RUnlock()
	}

	for _, k := range s.current().keys {
		if value == k.value {
			return true
		}
	}
	return false
}

// Key returns the key with the given name, looking in parent sections too
// as GetKey does. When there is none, it returns a key that reads as empty
// without adding it to the section. SetValue or SetComment adds it, and so
// do AddShadow and AddNestedValue when the file allows shadows or nested
// values.
func (s *Section) Key(name string) *Key {
	if key, err := s.GetKey(name); err == nil {
		return key
	}
	key := newKey(s, name, "")
	if name == "" {
		// An empty name can't be added.
		return key
	}
	if s.f.options.Insensitive || s.f.options.InsensitiveKeys {
		key.name = strings.ToLower(name)
	}
	key.missing = true
	return key
}

// Keys returns list of keys of section.
func (s *Section) Keys() []*Key {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	return slices.Clone(s.current().keys)
}

// ParentKeys returns list of keys of parent section.
func (s *Section) ParentKeys() []*Key {
	var parentKeys []*Key
	sname := s.name
	for {
		if i := strings.LastIndex(sname, s.f.options.ChildSectionDelimiter); i > -1 {
			sname = sname[:i]
			sec, err := s.f.GetSection(sname)
			if err != nil {
				continue
			}
			parentKeys = append(parentKeys, sec.Keys()...)
		} else {
			break
		}

	}
	return parentKeys
}

// KeyNames returns list of key names of section.
func (s *Section) KeyNames() []string {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	s = s.current()
	names := make([]string, len(s.keys))
	for i, k := range s.keys {
		names[i] = k.name
	}
	return names
}

// KeyValues returns the names and raw values of the section's keys.
func (s *Section) KeyValues() map[string]string {
	if s.f.locking {
		s.f.mu.RLock()
		defer s.f.mu.RUnlock()
	}

	s = s.current()
	hash := make(map[string]string, len(s.keys))
	for _, k := range s.keys {
		hash[k.name] = k.value
	}
	return hash
}

// DeleteKey deletes a key from section.
func (s *Section) DeleteKey(name string) {
	if s.f.options.Insensitive || s.f.options.InsensitiveKeys {
		name = strings.ToLower(name)
	}
	if s.f.locking {
		s.f.mu.Lock()
		defer s.f.mu.Unlock()
	}

	s = s.current()
	if key := s.byName[name]; key != nil {
		delete(s.byName, name)
		s.keys = slices.DeleteFunc(s.keys, func(k *Key) bool { return k == key })
	}
}

// ChildSections returns a list of child sections of current section.
// For example, "[parent.child1]" and "[parent.child12]" are child sections
// of section "[parent]".
func (s *Section) ChildSections() []*Section {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	prefix := s.name + s.f.options.ChildSectionDelimiter
	children := make([]*Section, 0, 3)
	for _, sec := range s.f.sections {
		if strings.HasPrefix(sec.name, prefix) {
			children = append(children, sec)
		}
	}
	return children
}
