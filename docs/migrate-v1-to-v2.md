# Migrating from v1 to v2

v2 lives at a new import path, and the package name is still `ini`:

```go
import "unknwon.dev/ini/v2" // was "gopkg.in/ini.v1"
```

It needs Go 1.26 or later. Both versions can be used in one program, so a large codebase can migrate one package at a time.

**Parsing doesn't change.** Without options, v2 parses every input exactly as v1's `Load` does: the same sections, keys, values, comments and errors. The one exception is a Python-style multiline value longer than 4 KiB, which v1 cut at its read buffer and then failed on; v2 reads it in full.

Most API changes are renames and stop the build, so the compiler finds them for you; the [API changes](#api-changes) tables list the replacements. Start with the next section, though: these changes compile fine and only show up at run time.

## Changes the compiler won't catch

### `Must*` methods no longer store the default

In v1, `k.MustInt(3)` returned 3 for a missing or invalid value **and wrote "3" into the key**. Later reads, `MapTo` and `Write` then saw the default, so saving a config wrote out every default the program had asked for. In v2, `Must*` methods only read. If your code reads a default back later, or relies on it being saved, store it yourself:

```go
// v1
port := sec.Key("PORT").MustInt(3000) // also set PORT = 3000

// v2
port := sec.Key("PORT").MustInt(3000)
sec.Key("PORT").SetValue(strconv.Itoa(port)) // only if you need it stored
```

### `Reload` and `Append`

- `Reload` replaces the contents with what the sources hold now. In v1 it merged them into the current contents, so keys removed from a file stayed and shadow values piled up. `*Section` and `*Key` values you hold see the new values, and if loading fails, nothing changes.
- `Append` reads only the sources it is given, so changes you made in memory survive it. In v1 it read every source again.
- `io.Reader` sources are read once and kept, so `Reload` works with them; in v1 it failed on the exhausted reader.

### Lists

`ini.List` (v1's `Ints`, `Float64s` and friends) gives 0 for an element that is out of range for the type, as it does for any element that doesn't parse. v1 kept the clamped value that `strconv` returns, such as `math.MaxInt64` or `+Inf`.

### Struct mapping

`MapTo`, `StrictMapTo` and `ReflectFrom` return errors instead of panicking on unsupported, named, pointer, nil and recursive types. Valid input can see these differences:

- An out-of-range number, such as `300` for an `int8` field, is an error in `StrictMapTo` and leaves the field unchanged in `MapTo`. v1 wrapped it to 44.
- `StrictMapTo` reports invalid slice elements; v1 set them to zero.
- Durations of `0` and negative durations are applied; v1 skipped them.
- `uint8` fields map; v1 returned an error.
- An embedded struct pointer that is already set keeps its values; v1 replaced it.
- A `nonunique` field with no matching sections is left empty; v1 failed.
- When writing:
  - times keep sub-second precision;
  - slice elements that contain the delimiter or end in a backslash are escaped, so they read back as one element;
  - nil pointer fields aren't written;
  - fields after an empty `nonunique` slice or a `StructReflector` field are written;
  - an `allowshadow` element that contains the delimiter is an error, because it would read back as two elements.

### Writing

`Write` output is byte-identical to v1's for ordinary files. Where v1 wrote a file that loaded back differently, v2 writes one that loads back the same, or returns an error:

- values ending in `\`, containing `"""`, or starting with a delimiter are quoted;
- empty shadow values are written;
- comments of keys written on several lines are kept;
- with `CaseInsensitive`, an explicit `[DEFAULT]` section is written with its own keys;
- a raw section body without a final newline no longer runs into the next header.

### Smaller fixes you might notice

- A `%(name)s` reference cycle is left unexpanded. In v1 it overflowed the stack and crashed the program.
- With `CaseInsensitive`, `DeleteKey("Name")` deletes the key; v1 ignored the case-folded name.
- `ChildSections` lists each child once with `AllowNonUniqueSections`.
- `Key.Strings("")`, a negative section index and `Section.Key("")` no longer hang or panic.
- Reading a key takes the file's read lock, so reads are safe while other goroutines change the file. That costs about 10 ns per read; load with `ini.Locking(false)` if the file never changes after loading.

## API changes

### Loading

| v1 | v2 |
|---|---|
| `Load(a, b)` | `LoadSources([]any{a, b})`; `Load(a)` is unchanged |
| `LoadSources(LoadOptions{AllowShadows: true}, a)` | `Load(a, ini.AllowShadows(true))` |
| `LooseLoad`, `InsensitiveLoad`, `ShadowLoad` | `Load` with `ini.IgnoreMissingFiles(true)`, `ini.CaseInsensitive(true)`, `ini.AllowShadows(true)` |
| `MapTo(v, a, b)` | `MapTo(v, a, opts...)`, with one source |
| `MapToWithMapper(v, m, a)`, `StrictMapToWithMapper` | `MapTo(v, a, ini.NameMapping(m))`, `StrictMapTo(v, a, ini.NameMapping(m))` |
| `f.NameMapper = m`, `f.ValueMapper = fn` | load with `ini.NameMapping(m)`, `ini.ValueMapping(fn)` |
| `f.BlockMode = false` | load with `ini.Locking(false)` |
| `ini.DefaultSection = "default"` | load with `ini.DefaultSectionName("default")`; `ini.DefaultSection` is now a constant |

Every `LoadOptions` field is now an option function of the same name that takes its value, such as `ini.SkipUnrecognizableLines(true)` or `ini.KeyValueDelimiters("=")`. A later option overrides an earlier one. These were renamed:

| `LoadOptions` field | v2 option |
|---|---|
| `Loose` | `IgnoreMissingFiles` |
| `Insensitive`, `InsensitiveSections`, `InsensitiveKeys` | `CaseInsensitive`, `CaseInsensitiveSections`, `CaseInsensitiveKeys` |
| `ShortCircuit` | `FirstSourceOnly` |
| `UnparseableSections` | `RawSections(names...)` |
| `PreserveSurroundedQuote` | `PreserveSurroundingQuotes` |
| `DebugFunc` | `Debug(fn)` |
| `KeyValueDelimiterOnWrite` | `WriteDelimiter(d)`, an option of `Write` |
| `ReaderBufferSize` | removed; nothing is cut at a buffer size |

New options: `CommentSymbols`, `PreserveEmptyShadowValues` and `DefaultSectionName`.

### Writing

The package variables that changed every file's output are options of `File.Write` and `File.SaveTo`, so one caller can't change another's output. The defaults write what v1 wrote.

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

`WriteTo` stays, so a `*File` is still an `io.WriterTo`.

### Sections and keys

| v1 | v2 |
|---|---|
| `s.Comment`, `k.Comment` fields | `Comment()` and `SetComment(c)` methods |
| `f.SectionStrings()`, `s.KeyStrings()`, `s.KeysHash()` | `SectionNames()`, `KeyNames()`, `KeyValues()` |
| `k.ValueWithShadows()` | `k.Values()` |
| `s.Haskey(name)` | `s.HasKey(name)` |
| `DEFAULT_SECTION`, `AllCapsUnderscore`, `SnackCase` | `DefaultSection`, `SnakeCase` |

### Typed values

The typed list, candidate and range methods are generic functions. They behave as before, and work for any `Scalar` type, such as `int8`, `time.Duration` or a named string type.

| v1 | v2 |
|---|---|
| `k.Ints(",")`, `k.Float64s(",")`, `k.Bools(",")`, `k.Times(",")`… | `ini.List[int](k, ",")`, `ini.List[float64](k, ",")`… |
| `k.ValidInts(",")`… | `ini.ValidList[int](k, ",")`… |
| `k.StrictInts(",")`… | `ini.StrictList[int](k, ",")`… |
| `k.In(def, cands)`, `k.InInt(def, cands)`, `k.InTime(def, cands)`… | `ini.In(k, def, cands)` |
| `k.RangeInt(def, lo, hi)`, `k.RangeTime(def, lo, hi)`… | `ini.Range(k, def, lo, hi)`; type the default for other types, as in `ini.Range(k, 0.5, 0, 1)` |

`String`, `Int`, `Bool`, `Duration`, `Strings`, the `Must*` methods, and the custom-layout time methods (`TimesFormat`, `InTimeFormat`, `RangeTimeFormat` and their `Valid` and `Strict` forms) stay on `Key`.

### Errors

| v1 | v2 |
|---|---|
| `ini.IsErrDelimiterNotFound(err)` | `errors.Is(err, ini.ErrDelimiterNotFound)` |
| `ini.IsErrEmptyKeyName(err)` | `errors.Is(err, ini.ErrEmptyKeyName)` |

Parse errors are `*ini.ParseError` values with the 1-based `Line` and `Column` of the problem; the message under the position is the same as v1's.
