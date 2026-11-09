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
| `LooseLoad`, `InsensitiveLoad`, `ShadowLoad` | `ini.Loose()`, `ini.Insensitive()`, `ini.AllowShadows()` |
| `LoadOptions.DebugFunc` | `ini.Debug(fn)` |
| `LoadOptions.ReaderBufferSize` | removed; values are never cut at a buffer size |
| `ini.DefaultSection = "default"` | `ini.DefaultSectionName("default")` |
| `ini.LineBreak`, `PrettyFormat`, `PrettySection`, `PrettyEqual`, `DefaultHeader`, `DefaultFormatLeft/Right` | `WriteOption`s of `File.Write` and `File.SaveTo` |
| `LoadOptions.KeyValueDelimiterOnWrite` | `ini.KeyValueDelimiterOnWrite(d)` write option |
| `WriteToIndent(w, i)`, `SaveToIndent(f, i)` | `Write(w, ini.Indent(i))`, `SaveTo(f, ini.Indent(i))` |
| `MapTo(v, a, b)` | `MapTo(v, a, opts...)`; one source |
| `DEFAULT_SECTION`, `AllCapsUnderscore` | `DefaultSection`, `SnackCase` |

New in v2:

- Parse errors are `*ini.ParseError` with 1-based `Line` and `Column`.
- `ini.CommentSymbols(s)` sets the comment characters.
- `ini.KeepEmptyShadowValues()` keeps empty values of shadow keys.
- `Key.IsBooleanType()`, and `ini.Marshal`/`ini.Unmarshal`.
- Loading is O(n) in the number of keys; a 10,000-key section loads in about 6 ms instead of about 150 ms with v1.67.3.

## Getting Help

- [Getting Started](https://ini.unknwon.io/docs/intro/getting_started)

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.
