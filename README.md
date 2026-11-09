# INI

[![GoDoc](https://img.shields.io/badge/GoDoc-Reference-blue?style=for-the-badge&logo=go)](https://pkg.go.dev/unknwon.dev/ini/v2?tab=doc)

![](https://avatars0.githubusercontent.com/u/10216035?v=3&s=200)

Package ini provides INI file read and write functionality in Go.

## Features

- Load from multiple data sources(file, `[]byte`, `io.Reader` and `io.ReadCloser`) with overwrites.
- Read with recursion values.
- Read with parent-child sections.
- Read with auto-increment key names.
- Read with multiple-line values.
- Read with tons of helper methods.
- Read and convert values to Go types.
- Read and **WRITE** comments of sections and keys.
- Manipulate sections, keys and comments with ease.
- Keep sections and keys in order as you parse and save.

## Installation

The minimum requirement of Go is **1.26**.

```sh
$ go get unknwon.dev/ini/v2@latest
```

## Migrating from v1

Without options, v2 parses every input exactly as v1's `Load` does. The parity tests check this against the released `gopkg.in/ini.v1`. The exception: a Python-style multiline value longer than 4 KiB is read in full instead of failing. The API changes are:

| v1 | v2 |
|---|---|
| `import "gopkg.in/ini.v1"` | `import "unknwon.dev/ini/v2"` (package name is still `ini`) |
| `Load(a, b)` | `LoadSources([]any{a, b})`; `Load(a)` is unchanged |
| `LoadSources(LoadOptions{AllowShadows: true}, a)` | `Load(a, ini.AllowShadows())` |
| `LooseLoad`, `InsensitiveLoad`, `ShadowLoad` | `ini.IgnoreMissingFiles()`, `ini.CaseInsensitive()`, `ini.AllowShadows()` |
| `LoadOptions.DebugFunc` | `ini.Debug(fn)` |
| `LoadOptions.ReaderBufferSize` | removed; values are never cut at a buffer size |
| `ini.DefaultSection = "default"` | `ini.DefaultSectionName("default")` |
| `ini.LineBreak`, `PrettyFormat`, `PrettySection`, `PrettyEqual`, `DefaultHeader`, `DefaultFormatLeft/Right` | `WriteOption`s of `File.Write` and `File.SaveTo` |
| `LoadOptions.KeyValueDelimiterOnWrite` | `ini.WriteDelimiter(d)` write option |
| `WriteToIndent(w, i)`, `SaveToIndent(f, i)` | `Write(w, ini.Indent(i))`, `SaveTo(f, ini.Indent(i))` |
| `MapTo(v, a, b)` | `MapTo(v, a, opts...)`; one source |
| `DEFAULT_SECTION`, `AllCapsUnderscore`, `SnackCase` | `DefaultSection`, `SnakeCase` |
| `LoadOptions.Insensitive`, `InsensitiveSections`, `InsensitiveKeys` | `ini.CaseInsensitive()`, `ini.CaseInsensitiveSections()`, `ini.CaseInsensitiveKeys()` |
| `LoadOptions.Loose`, `ShortCircuit`, `UnparseableSections` | `ini.IgnoreMissingFiles()`, `ini.FirstSourceOnly()`, `ini.RawSections(names...)` |
| `File.BlockMode = false` | `ini.NoLocking()` |
| `MapToWithMapper(v, m, src)`, `StrictMapToWithMapper` | `MapTo(v, src, ini.NameMapping(m))`, `StrictMapTo(v, src, ini.NameMapping(m))` |
| `ReflectFrom(f, v)`, `ReflectFromWithMapper(f, v, m)` | `f.ReflectFrom(v)`; set `f.NameMapper = m` or load with `ini.NameMapping(m)` |
| `IsErrDelimiterNotFound(err)`, `IsErrEmptyKeyName(err)` | `errors.Is(err, ini.ErrDelimiterNotFound)`, `errors.Is(err, ini.ErrEmptyKeyName)` |
| `File.SectionStrings`, `Section.KeyStrings`, `Section.KeysHash` | `SectionNames`, `KeyNames`, `KeyValues` |
| `Key.ValueWithShadows`, `Key.IsBooleanType`, `Section.Haskey` | `Values`, `IsBoolean`, `HasKey` |
| `k.Ints(",")`, `k.StrictInts(",")`, `k.ValidInts(",")` and the same for the other types | `ini.List[int](k, ",")`, which returns an error for an invalid element; any `Scalar` type works, such as `int8`, `time.Duration` or a named string type |
| `k.InInt(def, cands)`, `k.In(def, cands)` and the others | `ini.In(k, def, cands)` |
| `k.RangeInt(def, lo, hi)` and the others | `ini.Range(k, def, lo, hi)`; type the default for non-int types, as in `ini.Range(k, 0.5, 0, 1)`. Times compare with `Key.Time` |
| `TimesFormat`, `InTimeFormat`, `RangeTimeFormat` | `Strings` with `time.Parse`; `TimeFormat` and `MustTimeFormat` remain |
| `Reload` merged sources into the current contents; `Append` re-read every source | `Reload` replaces the contents with the sources (kept `*Section`/`*Key` pointers see new values; on error nothing changes); `Append` reads only the new sources; readers are buffered so they can be reloaded |

New in v2:

- Parse errors are `*ini.ParseError` with 1-based `Line` and `Column`.
- `ini.CommentSymbols(s)` sets the comment characters.
- `ini.KeepEmptyShadowValues()` keeps empty values of shadow keys.
- `Key.IsBoolean()`, and `ini.Marshal`/`ini.Unmarshal`.
- `File.Write` writes files that load back to the same sections, keys, values and comments: it picks a quoting the parser reads back, writes empty shadow values, and returns an error instead of writing a file that loads differently.
- `MapTo`, `StrictMapTo`, `Unmarshal`, `ReflectFrom` and `Marshal` return errors instead of panicking on unsupported, named, pointer, nil and recursive types. Differences from v1 that valid input can show:
  - Out-of-range numbers (int8 `300`) are an error in `StrictMapTo` and leave the field unchanged in `MapTo`; v1 wrapped them.
  - `StrictMapTo` reports invalid slice elements.
  - Durations of `0` and negative durations are applied.
  - uint8 fields map.
  - Times keep sub-second precision when written.
  - Slice elements containing the delimiter, or ending in a backslash, are escaped so they read back as one element.
  - Nil pointer fields aren't written.
  - Fields after an empty `nonunique` slice or a `StructReflector` field are written.
  - A `nonunique` field with no sections is left empty instead of failing.
- Unless loaded with `NoLocking()`, a `File` can be read from many goroutines while others call `Reload`, `Append`, `SetValue`, `NewKey`, `NewSection`, the delete methods, `ReflectFrom` or `Write`; v1 locked only some writes. `Comment` fields are plain fields: set them before sharing the `File`.
- `Unmarshal` reads what `Marshal` writes, including `allowshadow` slices with repeated, empty or delimiter-containing elements and `nonunique` sections.
- Loading is O(n) in the number of keys; a 10,000-key section loads in about 6 ms instead of about 150 ms with v1.67.3.

## Getting Help

- [Getting Started](https://ini.unknwon.io/docs/intro/getting_started)

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.
