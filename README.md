# INI

[![GoDoc](https://img.shields.io/badge/GoDoc-Reference-blue?style=for-the-badge&logo=go)](https://pkg.go.dev/unknwon.dev/ini/v2?tab=doc)

![](https://avatars0.githubusercontent.com/u/10216035?v=3&s=200)

Package ini reads and writes INI files in Go.

## Features

- **Fast loading**: a single-pass lexer, linear in the size of the input and up to 42× faster than v1.
- **Several sources**: files, `[]byte` and `io.Reader`, with later sources overriding earlier ones.
- **Rich INI syntax**: parent-child sections, auto-increment keys, repeated (shadow) keys, nested and multiline values, and `%(name)s` references.
- **Typed values**: numbers, booleans, durations and times, with generic lists and candidates of any of them, and ranges of all but booleans.
- **Struct mapping**: fill a struct from a file, or a file from a struct, guided by `ini` struct tags. Types can format and parse their own values with `MarshalINI` and `UnmarshalINI`.
- **Safe writes**: comments and order are kept, and what is written loads back the same, or `Write` returns an error.
- **Precise errors**: parse errors give the line and column.
- **Concurrency**: safe for concurrent use, or lock-free with `Locking(false)` for files that never change.

## Installation

The minimum requirement of Go is **1.26**.

```sh
go get unknwon.dev/ini/v2@latest
```

## Getting started

```go
package main

import (
	"fmt"
	"log"
	"time"

	"unknwon.dev/ini/v2"
)

func main() {
	f, err := ini.Load("app.ini")
	if err != nil {
		log.Fatal(err) // e.g., "line 3, column 1: key-value delimiter not found: broken line"
	}

	server := f.Section("server")
	port := server.Key("port").MustInt(8080)
	timeout := server.Key("timeout").MustDuration(30 * time.Second)
	fmt.Println(port, timeout)

	server.Key("port").SetValue("9090")
	if err := f.SaveTo("app.ini"); err != nil {
		log.Fatal(err)
	}
}
```

Options change the syntax that is accepted, such as `ini.Load("app.ini", ini.AllowShadows(true), ini.IgnoreInlineComment(true))`. The [package documentation](https://pkg.go.dev/unknwon.dev/ini/v2) has examples of struct mapping, typed lists and write options.

## Performance

v2 loads files 3 to 42 times faster than v1, with a seventh to a two-hundredth of the allocations:

| Operation | v1.67.3 | v2 | Allocations, v1 → v2 |
|---|---|---|---|
| Load 10,000 keys in one section | 84 ms | 2.0 ms (42× faster) | 40.2k → 0.2k |
| Load 10,000 keys in 1,000 sections | 6.8 ms | 2.1 ms (3.2× faster) | 55.1k → 8.1k |
| Load 10,000 keys, mixed syntax | 6.7 ms | 2.3 ms (3.0× faster) | 54.8k → 8.4k |
| Write 10,000 keys | 1.7–2.7 ms | 3.2–4.0 ms (1.5–1.8× slower) | 17–8.9k → 10.0k–22.8k |
| `MapTo` a 13-field struct | 2.8 µs | 4.9 µs | 50 → 48 |
| `ReflectFrom` the same struct | 2.9 µs | 4.8 µs | 51 → 48 |
| Read one key | 45 ns | 56 ns, or 31 ns with `Locking(false)` | 0 → 0 |

- **Load** is linear in the number of keys. v1 compared each new key with every key before it in its section, so a large section took quadratic time.
- **Write** is slower because it checks that every line it writes reads back as the same value. v1 could write files that loaded differently.
- **Reads** take the file's read lock so that they are safe while other goroutines change the file. v1 read values without it, as in `Key.String`.

The mixed input is 100 sections of 100 keys with comments, inline comments, quoted values, colons, auto-increment keys and continuation lines. Measured with Go 1.27.4 on an Intel Xeon at 2.60 GHz with `GOMAXPROCS=4`, as the median of 8 runs.

## Migrating from v1

Without options, v2 parses input as v1 does, apart from a few fixes such as `#` inside quoted values. The API changed: the import path is `unknwon.dev/ini/v2`, options are functions, package-level settings became options, and typed list methods became generic functions.

The [migration guide](docs/migrate-v1-to-v2.md) lists every change.

## Getting help

- [Package documentation and examples](https://pkg.go.dev/unknwon.dev/ini/v2)

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.
