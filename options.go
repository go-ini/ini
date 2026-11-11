package ini

import (
	"os"
	"runtime"
	"strings"
)

// LoadOption configures how data sources are loaded and parsed. Without
// options, parsing behaves exactly as gopkg.in/ini.v1's Load. When two options set the
// same thing, the last one wins.
type LoadOption func(*loadOptions)

// loadOptions holds the settings that LoadOption values apply. A File keeps them,
// so Reload and Append parse the same way as the first load.
type loadOptions struct {
	Loose                       bool
	Insensitive                 bool
	InsensitiveSections         bool
	InsensitiveKeys             bool
	IgnoreContinuation          bool
	IgnoreInlineComment         bool
	SkipUnrecognizableLines     bool
	ShortCircuit                bool
	AllowBooleanKeys            bool
	AllowShadows                bool
	AllowNestedValues           bool
	AllowPythonMultilineValues  bool
	SpaceBeforeInlineComment    bool
	UnescapeValueDoubleQuotes   bool
	UnescapeValueCommentSymbols bool
	UnparseableSections         []string
	KeyValueDelimiters          string
	ChildSectionDelimiter       string
	PreserveSurroundingQuotes   bool
	DebugFunc                   DebugFunc
	AllowNonUniqueSections      bool
	AllowDuplicateShadowValues  bool
	DefaultSection              string
	CommentSymbols              string
	PreserveEmptyShadowValues   bool
	Locking                     bool
	NameMapper                  NameMapper
	ValueMapper                 ValueMapper
}

func newLoadOptions(opts []LoadOption) loadOptions {
	o := loadOptions{
		Locking:               true,
		KeyValueDelimiters:    "=:",
		ChildSectionDelimiter: ".",
		DefaultSection:        DefaultSection,
		CommentSymbols:        "#;",
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Locking sets whether a File takes a lock so it can be read from many
// goroutines while others change it. The default is true. Turn it off for a
// File only one goroutine uses, to save the cost of locking.
func Locking(v bool) LoadOption { return func(o *loadOptions) { o.Locking = v } }

// NameMapping sets the NameMapper that maps struct field names to section
// and key names, for MapTo, StrictMapTo and ReflectFrom.
func NameMapping(mapper NameMapper) LoadOption {
	return func(o *loadOptions) { o.NameMapper = mapper }
}

// ValueMapping sets the ValueMapper applied to raw values, such as
// os.ExpandEnv, by Key.String and the accessors built on it.
func ValueMapping(mapper ValueMapper) LoadOption {
	return func(o *loadOptions) { o.ValueMapper = mapper }
}

// DebugFunc is the type of function called to log parse events.
type DebugFunc func(message string)

// IgnoreMissingFiles skips data sources that are files which don't exist,
// instead of returning an error.
func IgnoreMissingFiles(v bool) LoadOption { return func(o *loadOptions) { o.Loose = v } }

// CaseInsensitive lowercases all section and key names.
func CaseInsensitive(v bool) LoadOption { return func(o *loadOptions) { o.Insensitive = v } }

// CaseInsensitiveSections lowercases all section names.
func CaseInsensitiveSections(v bool) LoadOption {
	return func(o *loadOptions) { o.InsensitiveSections = v }
}

// CaseInsensitiveKeys lowercases all key names.
func CaseInsensitiveKeys(v bool) LoadOption { return func(o *loadOptions) { o.InsensitiveKeys = v } }

// IgnoreContinuation treats a backslash at the end of a line as part of the
// value instead of joining the next line.
func IgnoreContinuation(v bool) LoadOption { return func(o *loadOptions) { o.IgnoreContinuation = v } }

// IgnoreInlineComment keeps comment symbols after a value as part of it.
// Comments on lines of their own still work.
func IgnoreInlineComment(v bool) LoadOption {
	return func(o *loadOptions) { o.IgnoreInlineComment = v }
}

// SkipUnrecognizableLines skips lines that are not sections, keys or
// comments instead of returning an error.
func SkipUnrecognizableLines(v bool) LoadOption {
	return func(o *loadOptions) { o.SkipUnrecognizableLines = v }
}

// FirstSourceOnly stops loading after the first data source that loads.
func FirstSourceOnly(v bool) LoadOption { return func(o *loadOptions) { o.ShortCircuit = v } }

// AllowBooleanKeys reads a line without a delimiter as a key whose value is
// "true", as in my.cnf.
func AllowBooleanKeys(v bool) LoadOption { return func(o *loadOptions) { o.AllowBooleanKeys = v } }

// AllowShadows keeps every value of a key that repeats in a section,
// instead of only the last one.
func AllowShadows(v bool) LoadOption { return func(o *loadOptions) { o.AllowShadows = v } }

// AllowNestedValues reads indented lines under a key with an empty value as
// AWS-style nested values.
// Docs: http://docs.aws.amazon.com/cli/latest/topic/config-vars.html#nested-values
func AllowNestedValues(v bool) LoadOption { return func(o *loadOptions) { o.AllowNestedValues = v } }

// AllowPythonMultilineValues continues a value on the following lines that
// are indented.
// Docs: https://docs.python.org/3/library/configparser.html#supported-ini-file-structure
func AllowPythonMultilineValues(v bool) LoadOption {
	return func(o *loadOptions) { o.AllowPythonMultilineValues = v }
}

// SpaceBeforeInlineComment starts an inline comment only at a comment symbol
// that follows a space, so values can contain comment symbols.
func SpaceBeforeInlineComment(v bool) LoadOption {
	return func(o *loadOptions) { o.SpaceBeforeInlineComment = v }
}

// UnescapeValueDoubleQuotes unescapes \" inside values surrounded by double
// quotes, e.g., key="a \"value\"" is `a "value"`.
func UnescapeValueDoubleQuotes(v bool) LoadOption {
	return func(o *loadOptions) { o.UnescapeValueDoubleQuotes = v }
}

// UnescapeValueCommentSymbols unescapes \# and \; in values that are not
// surrounded by quotes.
func UnescapeValueCommentSymbols(v bool) LoadOption {
	return func(o *loadOptions) { o.UnescapeValueCommentSymbols = v }
}

// RawSections keeps the bodies of the named sections as raw text,
// available from Section.Body.
func RawSections(names ...string) LoadOption {
	return func(o *loadOptions) { o.UnparseableSections = names }
}

// KeyValueDelimiters sets the characters that separate a key from its value.
// The default is "=:", and an empty string keeps the default.
func KeyValueDelimiters(delimiters string) LoadOption {
	return func(o *loadOptions) {
		if delimiters != "" {
			o.KeyValueDelimiters = delimiters
		}
	}
}

// ChildSectionDelimiter sets the separator between parent and child section
// names. The default is ".", and an empty string keeps the default.
func ChildSectionDelimiter(delimiter string) LoadOption {
	return func(o *loadOptions) {
		if delimiter != "" {
			o.ChildSectionDelimiter = delimiter
		}
	}
}

// PreserveSurroundingQuotes keeps single and double quotes around values.
func PreserveSurroundingQuotes(v bool) LoadOption {
	return func(o *loadOptions) { o.PreserveSurroundingQuotes = v }
}

// Debug calls fn with debug information while parsing, currently about
// Python-style multiline values.
func Debug(fn DebugFunc) LoadOption { return func(o *loadOptions) { o.DebugFunc = fn } }

// AllowNonUniqueSections keeps sections that repeat a name as separate
// sections, instead of merging them.
func AllowNonUniqueSections(v bool) LoadOption {
	return func(o *loadOptions) { o.AllowNonUniqueSections = v }
}

// AllowDuplicateShadowValues keeps shadow values that repeat an earlier
// value of the key. By default they are dropped.
func AllowDuplicateShadowValues(v bool) LoadOption {
	return func(o *loadOptions) { o.AllowDuplicateShadowValues = v }
}

// PreserveEmptyShadowValues keeps empty values in Key.Values and when
// writing shadow keys. An empty value can be meaningful: in systemd units it
// resets the values before it.
func PreserveEmptyShadowValues(v bool) LoadOption {
	return func(o *loadOptions) { o.PreserveEmptyShadowValues = v }
}

// DefaultSectionName names the section that holds keys before the first
// section header, such as "default" for AWS config files. The default is
// DefaultSection, and an empty string keeps the default.
func DefaultSectionName(name string) LoadOption {
	return func(o *loadOptions) {
		if name != "" {
			o.DefaultSection = name
		}
	}
}

// CommentSymbols sets the characters that start a comment. The default is
// "#;", and an empty string means no line is a comment.
func CommentSymbols(symbols string) LoadOption {
	return func(o *loadOptions) { o.CommentSymbols = symbols }
}

// WriteOption configures how a File is written.
type WriteOption func(*writeOptions)

type writeOptions struct {
	indent            string
	lineBreak         string
	keyValueDelimiter string
	defaultHeader     bool
	blankLines        bool
	alignDelimiters   bool
	formatLeft        string
	formatRight       string
}

// inTest keeps "\n" line breaks in this package's tests on Windows.
var inTest = len(os.Args) > 0 && strings.HasSuffix(strings.TrimSuffix(os.Args[0], ".exe"), ".test")

func newWriteOptions(opts []WriteOption) writeOptions {
	o := writeOptions{
		lineBreak:         "\n",
		keyValueDelimiter: "=",
		blankLines:        true,
		alignDelimiters:   true,
	}
	if runtime.GOOS == "windows" && !inTest {
		o.lineBreak = "\r\n"
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Indent puts indent before the keys and comments of every section except
// the default one. Write returns an error when an indented line would read
// back as part of the value before it: after any value with
// AllowPythonMultilineValues, and after an empty value with
// AllowNestedValues.
func Indent(indent string) WriteOption { return func(o *writeOptions) { o.indent = indent } }

// LineBreak sets the line break to write. The default is "\r\n" on Windows
// and "\n" elsewhere.
func LineBreak(lineBreak string) WriteOption {
	return func(o *writeOptions) { o.lineBreak = lineBreak }
}

// WriteDelimiter sets the delimiter written between keys and
// values. The default is "=".
func WriteDelimiter(delimiter string) WriteOption {
	return func(o *writeOptions) { o.keyValueDelimiter = delimiter }
}

// DefaultHeader writes the header of the default section.
func DefaultHeader(v bool) WriteOption { return func(o *writeOptions) { o.defaultHeader = v } }

// BlankLineBetweenSections sets whether to put an empty line between
// sections. The default is true.
func BlankLineBetweenSections(blank bool) WriteOption {
	return func(o *writeOptions) { o.blankLines = blank }
}

// AlignDelimiters sets whether to pad the keys of a section with spaces so
// that their delimiters line up, with one space on each side. The default
// is true.
func AlignDelimiters(align bool) WriteOption {
	return func(o *writeOptions) { o.alignDelimiters = align }
}

// DelimiterSpaces puts left and right around the delimiter when
// AlignDelimiters is false. DelimiterSpaces(" ", " ") writes "k = v"
// without aligning.
func DelimiterSpaces(left, right string) WriteOption {
	return func(o *writeOptions) { o.formatLeft, o.formatRight = left, right }
}
