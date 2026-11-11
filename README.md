# INI

[![GoDoc](https://img.shields.io/badge/GoDoc-Reference-blue?style=for-the-badge&logo=go)](https://pkg.go.dev/unknwon.dev/ini/v2?tab=doc)

![](https://avatars0.githubusercontent.com/u/10216035?v=3&s=200)

Package ini reads and writes INI files in Go.

## Features

- **Fast loading**: a single-pass lexer, linear in the size of the input and up to 31× faster than v1.
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

v2 loads files up to 31 times faster than v1, with a third to a fifth of the allocations:

| Operation | v1.67.3 | v2 | Allocations, v1 → v2 |
|---|---|---|---|
| Load 10,000 keys in one section | 137 ms | 4.4 ms (31× faster) | 50.2k → 10.1k |
| Load 10,000 keys in 1,000 sections | 8.2 ms | 3.2 ms (2.5× faster) | 58.1k → 18.1k |
| Load 10,000 keys, mixed syntax | 101 ms | 3.9 ms (26× faster) | 57.6k → 18.8k |
| Write 10,000 keys | 2.2–4.3 ms | 4.3–7.4 ms (1.7–2.0× slower) | 3.0k–20.0k → 13.0k–30.0k |
| `MapTo` a 13-field struct | 3.8 µs | 4.5 µs | 44 → 48 |
| `ReflectFrom` the same struct | 13.9 µs | 11.8 µs | 132 → 118 |
| Read one key | 60 ns | 71 ns, or 36 ns with `Locking(false)` | 0 → 0 |

- **Load** is linear in the number of keys. v1 compared each new key with every key before it in its section, so a large section took quadratic time.
- **Write** is slower because it checks that every line it writes reads back as the same value. v1 could write files that loaded differently.
- **Reads** take the file's read lock so that they are safe while other goroutines change the file. v1 read values without it, as in `Key.String`.

Measured with Go 1.26.10 on a 4-core Intel Xeon at 2.60 GHz, as the median of 8 runs.

## Migrating from v1

Without options, v2 parses every input exactly as v1 does. The API changed: the import path is `unknwon.dev/ini/v2`, options are functions, package-level settings became options, and typed list methods became generic functions.

The [migration guide](docs/migrate-v1-to-v2.md) lists every change.

## Getting help

- [Package documentation and examples](https://pkg.go.dev/unknwon.dev/ini/v2)

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.
