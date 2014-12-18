ini
===

Package ini provides INI file read and write functionality in Go.

## Feature

- Load multiple data sources([]byte or file) with overwrites.
- Read with parent-child sections.
- Read with recursion values.
- Read with auto-increment key names.
- Read with multiple-line values.
- Read with tons of helper methods.
- Read and convert values to Go types.
- Read and **WRITE** comments of sections and keys.
- Manipulate sections, keys and comments with ease.
- Keep sections and keys in order as you parse and save.

## Installation

	go get gopkg.in/ini.v0

## Getting Started

### Loading from data sources

A `Data Source` is either raw data in type `[]byte` or a file name with type `string` and you can load **as many as** data sources you want. Passing other types will simply returns error.

```go
cfg, err := ini.Load([]byte("raw data"), "filename")
```

When you cannot decide how many data sources to load at the beginning, you still able to **Append()** them later.

```go
err := cfg.Append("other file", []byte("other raw data"))
```

### Working with sections

To get a section, you would need to:

```go
section, err := cfg.GetSection("section name")
```

For a shortcut for default section, just give an empty string as name:

```go
section, err := cfg.GetSection("")
```

When you're pretty sure the section exists, following code could make your life easier:

```go
section := cfg.Section("")
```

What happens when the section somehow does not exists? Won't panic, it returns an empty section object.

To create a new section:

```go
err := cfg.NewSection("new section")
```

To get a list of sections or section names:

```go
sections := cfg.Sections()
names := cfg.SectionStrings()
```

### Working with keys

To get a key under a section:

```go
key, err := cfg.Section("").GetKey("key name")
```

Same rule applies to key operations:

```go
key := cfg.Section("").Key("key name")
```

To create a new key:

```go
err := cfg.Section("").NewKey("name", "value")
```

To get a list of keys or key names:

```go
keys := cfg.Section().Keys()
names := cfg.Section().KeyStrings()
```

### Working with values

For all value of keys, there is a special syntax `%(<name>)s`, where `<name>` is the key name in same section or default section, and `%(<name>)s` will be replaced by corresponding value(empty string if key not found). You can use this syntax at most 99 level of recursions.

To get a string value:

```go
val := cfg.Section("").Key("key name").String()
```

To get value with types:

```go
v, err = cfg.Section("").Key("key name").Bool()
v, err = cfg.Section("").Key("key name").Float64()
v, err = cfg.Section("").Key("key name").Int()
v, err = cfg.Section("").Key("key name").Int64()

v = cfg.Section("").Key("key name").MustBool()
v = cfg.Section("").Key("key name").MustFloat64()
v = cfg.Section("").Key("key name").MustInt()
v = cfg.Section("").Key("key name").MustInt64()

// Methods start with Must also accept one argument for default value
// when key not found or fail to parse value to given type.

v = cfg.Section("").Key("key name").MustBool(true)
v = cfg.Section("").Key("key name").MustFloat64(1.25)
v = cfg.Section("").Key("key name").MustInt(10)
v = cfg.Section("").Key("key name").MustInt64(99)
```

### Example

Please see [ini_test.go](ini_test.go) for complete and most advanced usages.

## Getting Help

- [API Documentation](https://gowalker.org/gopkg.in/ini.v0)
- [File An Issue](https://github.com/go-ini/ini/issues/new)

## FAQs

### What does `BlockMode` field do?

By default, library lets you read and write values so we need a locker to make sure your data is safe. But in cases that you are very sure about only reading data through the library, you can set `cfg.BlockMode = false` to speed up read operations about **50%** faster.

### Why another INI library?

Many people are using my another INI library [goconfig](https://github.com/Unknwon/goconfig), so the reason for this one is I would like to make more Go style code. Also when you set `cfg.BlockMode = false`, this one is about **10%** faster.

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.
