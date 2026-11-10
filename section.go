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
}

func newSection(f *File, name string) *Section {
	return &Section{
		f:      f,
		name:   name,
		keys:   make([]*Key, 0, 10),
		byName: make(map[string]*Key),
	}
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
	return s.comment
}

// SetComment sets the comment lines written before the section header.
// Lines without a comment symbol get one.
func (s *Section) SetComment(comment string) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	s.comment = comment
}

// Body returns rawBody of Section if the section was marked as unparseable.
// It still follows the other rules of the INI format surrounding leading/trailing whitespace.
func (s *Section) Body() string {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	return strings.TrimSpace(s.rawBody)
}

// SetBody updates body content only if section is raw.
func (s *Section) SetBody(body string) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	if !s.isRawSection {
		return
	}
	s.rawBody = body
}

// NewKey creates a new key to given section.
func (s *Section) NewKey(name, val string) (*Key, error) {
	s.f.maybeLock()
	defer s.f.maybeUnlock()
	return s.addKey(name, val)
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
	return s.addBooleanKey(name)
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
	key := s.byName[name]
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

	for _, k := range s.keys {
		if value == k.value {
			return true
		}
	}
	return false
}

// Key assumes named Key exists in section and returns a zero-value when not.
func (s *Section) Key(name string) *Key {
	key, err := s.GetKey(name)
	if err != nil {
		if key, err = s.NewKey(name, ""); err != nil {
			// An empty name can't be added, so return a key that isn't.
			return newKey(s, name, "")
		}
		return key
	}
	return key
}

// Keys returns list of keys of section.
func (s *Section) Keys() []*Key {
	s.f.maybeRLock()
	defer s.f.maybeRUnlock()
	return slices.Clone(s.keys)
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
