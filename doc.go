// Package ini reads and writes INI files.
//
// # Loading
//
// [Load] parses one source: a file name, a []byte or an [io.Reader].
// [LoadSources] merges several, later ones overriding earlier ones.
//
//	f, err := ini.Load("app.ini")
//	if err != nil {
//		return err
//	}
//	port := f.Section("server").Key("port").MustInt(8080)
//
// Without options, input parses as it did with gopkg.in/ini.v1, apart from
// a few fixes, such as comment symbols inside quoted values. [LoadOption]
// values such as [AllowShadows], [CaseInsensitive] and [IgnoreInlineComment]
// change the syntax that is accepted.
//
// # Sections and keys
//
// A [File] holds sections in order, and each [Section] holds keys in order.
// The default section, named [DefaultSection], holds the keys before the
// first section header. Pass "" to refer to it.
//
// [File.Section] and [Section.Key] return a section or key that reads as
// empty when there is none, and a change such as [Key.SetValue] adds it.
//
// A [Key] returns its value as a string or parses it, as in [Key.Int] or
// [Key.Duration]. The generic [List], [In] and [Range] parse lists and
// candidates of any [Scalar] type, and ranges of any [Ordered] one. A value
// can refer to another key of its section or of the default section as
// %(name)s.
//
// # Structs
//
// [MapTo] and [StrictMapTo] fill a struct from a data source or a [File],
// and [File.ReflectFrom] fills a File from a struct. Field names map to key
// and section names unchanged unless [NameMapping] sets a mapper, and the
// ini, delim and comment struct tags adjust each field. A type that
// implements [Marshaler] or [Unmarshaler] formats or parses its own values.
//
// # Writing
//
// [File.Write] and [File.SaveTo] write a File back with its comments and in
// its order, and [WriteOption] values set the formatting. What Write writes
// loads back to the same sections, keys, values and comment text, or Write
// returns an error. The spacing after a comment symbol can change.
//
// Output meant for other readers isn't checked:
//   - keys written with a [WriteDelimiter] the file wasn't loaded with
//   - boolean keys of a file loaded without [AllowBooleanKeys]
//   - raw sections of a file loaded without [RawSections] for them
//
// # Concurrency and errors
//
// A File is safe for concurrent use unless loaded with [Locking](false).
// Parse errors are [*ParseError] values that give the line and column.
package ini
