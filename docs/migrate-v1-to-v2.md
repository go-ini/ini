# Migrating from v1 to v2

```go
import "unknwon.dev/ini/v2" // was "gopkg.in/ini.v1"; the package is still ini
```

v2 needs Go 1.26 or later. v1 and v2 can be used in the same program, so you can migrate one package at a time.

**Parsing is almost unchanged.** Without options, v2 parses input as v1 does: same sections, keys, values, comments and errors. These are the exceptions, each a fix:

- A value in single or double quotes keeps the `#` and `;` inside them, so `k = "a#b"` is `a#b`. v1 read `"a`. Text other than a comment after the closing quote still cuts the value as before.
- Comment lines after the last key of a file are kept and written back after it. v1 dropped them.
- A second byte order mark at the start of a file is skipped. v1 failed on it.
- A Python-style multiline value longer than 4 KiB is read in full. v1 cut it at its read buffer and failed.

With options:

- `AllowPythonMultilineValues`: empty lines followed by an indented line stay in the value, as in Python's configparser. v1 ended the value at the empty line.
- `SpaceBeforeInlineComment`: the first ` #` or ` ;` starts the comment. v1 looked for ` #` first.
- `RawSections`: a raw section with an empty body is raw, so `SetBody` works on it.

Most API changes are renames that the compiler finds, and the tables under [API changes](#api-changes) list the replacements. The changes in the next section still compile, so read it first.

## Changes the compiler won't catch

### `Must*` no longer stores the default

In v1, `k.MustInt(3)` returned 3 for a missing or invalid value and also wrote `3` into the key, so later reads, `MapTo` and `Write` saw it. In v2, `Must*` only reads. If you need the default stored, set it:

```go
port := sec.Key("PORT").MustInt(3000)
sec.Key("PORT").SetValue(strconv.Itoa(port))
```

### `File.Section` and `Section.Key` no longer add what's missing

In v1, `f.Section("server").Key("PORT")` added an empty `[server]` section and `PORT` key when they didn't exist, so `HasSection`, `HasKey`, `Keys` and `Write` saw them after a mere read. In v2 they return a section and key that read as empty. Only a change adds them:

- a key: `SetValue` or `SetComment`, or `AddShadow` and `AddNestedValue` when the file allows shadows or nested values
- a section: a change to one of its keys, `NewKey`, `NewBooleanKey`, `SetComment` or `ReflectFrom`

`f.Section(name).Key(key).SetValue(v)` works as before, except that when only a parent section has the key, it changes the parent's value without adding an empty child section. `SectionWithIndex` follows the same rule, and a change adds the section after the last one of its name.

### `Reload` and `Append`

- `Reload` replaces the contents with what the sources hold now. v1 merged them in, so removed keys stayed and shadow values piled up. `*Section` and `*Key` values you hold see the new values. On error, nothing changes.
- `Append` reads only the new sources, so changes made in memory survive. v1 read every source again. With `FirstSourceOnly`, `Append` reloads instead, so the first source that loads wins and changes made in memory are lost.
- `io.Reader` sources are read once and kept, so `Reload` works with them. In v1 it failed.

### Lists

`ini.List` gives 0 for an out-of-range element, as for any element that doesn't parse. v1's `Ints` and friends kept the clamped value, such as `math.MaxInt64` or `+Inf`.

### Struct mapping

`MapTo`, `StrictMapTo` and `ReflectFrom` return errors instead of panicking on unsupported types and nil targets. Named types and pointer fields map, and a recursive type is followed one level deep. Valid input can also behave differently:

- An out-of-range number, such as `300` for an `int8`, leaves the field unchanged in `MapTo` and is an error in `StrictMapTo`. v1 wrapped it to 44.
- `StrictMapTo` reports invalid slice elements. v1 set them to zero.
- Zero and negative durations are applied. v1 skipped them.
- `uint8` fields map. v1 returned an error.
- An embedded struct pointer that is already set keeps its values. v1 replaced it.
- A `nonunique` field with no matching sections is left unchanged. v1 failed.
- When writing:
  - Times keep sub-second precision.
  - Slice elements containing the delimiter or ending in a backslash are escaped.
  - Nil pointer fields are skipped.
  - Fields after an empty `nonunique` slice or a `StructReflector` field are written.
  - An `allowshadow` element containing the delimiter is an error, since it would read back as two elements.

### Writing

For ordinary files, `Write` output is byte-identical to v1's. Where v1 wrote a file that loaded back differently, v2 writes one that loads back the same, or returns an error. Comments load back with the same text, though the spacing after a comment symbol can change. Output meant for other readers isn't checked: keys written with a `WriteDelimiter` the file wasn't loaded with, boolean keys without `AllowBooleanKeys`, and raw sections without `RawSections`. For example:

- Values ending in `\`, containing `"""`, or starting with a delimiter are quoted.
- Empty shadow values and the comments of multi-line keys are kept.
- With `CaseInsensitive`, an explicit `[DEFAULT]` section is written with its own keys.
- A raw section body without a final newline no longer runs into the next header.
- A raw section body with a line that would start a section or be a comment is an error.
- A raw default section is written with its header, so it stays raw. v1 left out the whole section.
- Keys in a section that `RawSections` reads as raw, or in a raw section, are an error. v1 dropped the keys of a raw section.
- Multi-line comments containing `\n` are written correctly with `LineBreak("\r\n")`.
- A key named `-`, which would read back as an auto-increment key, is an error.
- A value with a backtick inside, but not at its start, is no longer wrapped in `"""` unless it also has a comment symbol.

### Other fixes

- A `%(name)s` reference cycle is left unexpanded. v1 overflowed the stack and crashed.
- With `CaseInsensitive`, `DeleteKey` matches names in any case.
- With `AllowNonUniqueSections`, `ChildSections` lists each child once.
- `Key.Strings("")`, a negative section index and `Section.Key("")` no longer hang or panic.
- Reads take the file's read lock, so they are safe while other goroutines change the file. v1 read values without it, as in `Key.String`. If the file never changes after loading, load it with `ini.Locking(false)`, which halves the time of a read.

## API changes

### Loading

| v1 | v2 |
|---|---|
| `Load(a, b)` | `LoadSources([]any{a, b})`, while `Load(a)` is unchanged |
| `LoadSources(LoadOptions{AllowShadows: true}, a)` | `Load(a, ini.AllowShadows(true))` |
| `LooseLoad`, `InsensitiveLoad`, `ShadowLoad` | `Load` with `ini.IgnoreMissingFiles(true)`, `ini.CaseInsensitive(true)`, `ini.AllowShadows(true)` |
| `MapTo(v, a, b)` | `MapTo(v, a, opts...)`, with one source |
| `MapToWithMapper(v, m, a)`, `StrictMapToWithMapper` | `MapTo(v, a, ini.NameMapping(m))`, `StrictMapTo(v, a, ini.NameMapping(m))` |
| `f.NameMapper = m`, `f.ValueMapper = fn` | load with `ini.NameMapping(m)`, `ini.ValueMapping(fn)` |
| `f.BlockMode = false` | load with `ini.Locking(false)` |
| `ini.DefaultSection = "default"` | load with `ini.DefaultSectionName("default")`. `DefaultSection` is now a constant. |

Each `LoadOptions` field becomes an option function of the same name, such as `ini.SkipUnrecognizableLines(true)` or `ini.KeyValueDelimiters("=")`. Later options override earlier ones. These were renamed:

| `LoadOptions` field | v2 option |
|---|---|
| `Loose` | `IgnoreMissingFiles` |
| `Insensitive`, `InsensitiveSections`, `InsensitiveKeys` | `CaseInsensitive`, `CaseInsensitiveSections`, `CaseInsensitiveKeys` |
| `ShortCircuit` | `FirstSourceOnly` |
| `UnparseableSections` | `RawSections(names...)` |
| `PreserveSurroundedQuote` | `PreserveSurroundingQuotes` |
| `DebugFunc` | removed, since it only logged Python-style multiline values |
| `KeyValueDelimiterOnWrite` | `WriteDelimiter(d)`, a write option |
| `ReaderBufferSize` | removed, since nothing is cut at a buffer size |

New options: `CommentSymbols`, `PreserveEmptyShadowValues` and `DefaultSectionName`.

### Writing

The package variables that changed every file's output are now options of `File.Write` and `File.SaveTo`. Their defaults write what v1 wrote.

| v1 | v2 |
|---|---|
| `ini.LineBreak = "\r\n"` | `ini.LineBreak("\r\n")` |
| `ini.DefaultHeader = true` | `ini.DefaultHeader(true)` |
| `ini.DefaultFormatLeft`, `DefaultFormatRight` | `ini.DelimiterSpaces(left, right)` |
| `ini.PrettyFormat = false` | `ini.AlignDelimiters(false)` |
| `ini.PrettySection = false` | `ini.BlankLineBetweenSections(false)` |
| `ini.PrettyEqual = true` | `ini.DelimiterSpaces(" ", " ")` |
| `f.WriteToIndent(w, "\t")`, `f.SaveToIndent(name, "\t")` | `f.Write(w, ini.Indent("\t"))`, `f.SaveTo(name, ini.Indent("\t"))` |
| `ReflectFrom(f, v)`, `ReflectFromWithMapper(f, v, m)` | `f.ReflectFrom(v)`, with `f` loaded with `ini.NameMapping(m)` |

`WriteTo` stays, so `*File` is still an `io.WriterTo`.

### Sections and keys

| v1 | v2 |
|---|---|
| `s.Comment`, `k.Comment` fields | `Comment()` and `SetComment(c)` |
| `f.SectionStrings()`, `s.KeyStrings()`, `s.KeysHash()` | `SectionNames()`, `KeyNames()`, `KeyValues()` |
| `k.ValueWithShadows()` | `k.Values()` |
| `s.Haskey(name)` | `s.HasKey(name)` |
| `DEFAULT_SECTION`, `AllCapsUnderscore`, `SnackCase` | `DefaultSection`, `SnakeCase` |

### Typed values

The typed list, candidate and range methods are now generic functions. They accept any `Scalar` type, such as `int8`, `time.Duration` or a named string type, and `Range` any of them but `bool`. `In` and `Range` return the default for a value that doesn't parse, where v1's `InInt`, `RangeInt` and friends read it as 0, or as the clamped value of an out-of-range number, and returned that when it was a candidate or in the range.

| v1 | v2 |
|---|---|
| `k.Ints(",")`, `k.Float64s(",")`, `k.Bools(",")`, `k.Times(",")`… | `ini.List[int](k, ",")`, `ini.List[float64](k, ",")`… |
| `k.ValidInts(",")`… | `ini.ValidList[int](k, ",")`… |
| `k.StrictInts(",")`… | `ini.StrictList[int](k, ",")`… |
| `k.In(def, cands)`, `k.InInt(def, cands)`, `k.InTime(def, cands)`… | `ini.In(k, def, cands)` |
| `k.RangeInt(def, lo, hi)`, `k.RangeTime(def, lo, hi)`… | `ini.Range(k, def, lo, hi)`. Type the default for non-int types, as in `ini.Range(k, 0.5, 0, 1)`. |

`String`, `Int`, `Bool`, `Duration`, `Strings`, the `Must*` methods and the custom-layout time methods (`TimesFormat`, `InTimeFormat`, `RangeTimeFormat` and their `Valid` and `Strict` forms) stay on `Key`.

### Errors

| v1 | v2 |
|---|---|
| `ini.IsErrDelimiterNotFound(err)` | `errors.Is(err, ini.ErrDelimiterNotFound)` |
| `ini.IsErrEmptyKeyName(err)` | `errors.Is(err, ini.ErrEmptyKeyName)` |

Parse errors are `*ini.ParseError` values with a 1-based `Line` and `Column`. The message after the position is the same as v1's.
