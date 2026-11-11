package ini_test

import (
	"errors"
	"fmt"
	"os"
	"time"

	"unknwon.dev/ini/v2"
)

const exampleConfig = `; Application settings
app_name = demo

[server]
port    = 8080
timeout = 30s
hosts   = a.example.com, b.example.com
root    = /srv/demo
logs    = %(root)s/logs
`

func Example() {
	f, err := ini.Load([]byte(exampleConfig))
	if err != nil {
		panic(err)
	}

	// Keys before the first header are in the default section, "DEFAULT".
	for _, sec := range f.Sections() {
		fmt.Println(sec.Name(), sec.KeyNames())
	}
	fmt.Println(f.Section("").Key("app_name").String())

	server := f.Section("server")
	port, _ := server.Key("port").Int()
	timeout := server.Key("timeout").MustDuration(10 * time.Second)
	fmt.Println(port, timeout)
	fmt.Println(server.Key("hosts").Strings(","))
	fmt.Println(server.Key("logs").String())
	fmt.Println(server.Key("missing").MustString("fallback"))
	// Output:
	// DEFAULT [app_name]
	// server [port timeout hosts root logs]
	// demo
	// 8080 30s
	// [a.example.com b.example.com]
	// /srv/demo/logs
	// fallback
}

// Programs often ship defaults and let an optional file override them.
func ExampleLoadSources() {
	defaults := []byte("[server]\nport = 8080\nhost = localhost\n")
	custom := []byte("[server]\nport = 9090\n")

	f, err := ini.LoadSources([]any{defaults, custom, "missing.ini"}, ini.IgnoreMissingFiles(true))
	if err != nil {
		panic(err)
	}
	fmt.Println(f.Section("server").KeyValues())
	// Output:
	// map[host:localhost port:9090]
}

func ExampleLoad_options() {
	data := []byte(`
[unit]
ExecStart = /bin/a
ExecStart = /bin/b
password = abc#123
`)
	f, err := ini.Load(data, ini.AllowShadows(true), ini.IgnoreInlineComment(true))
	if err != nil {
		panic(err)
	}
	sec := f.Section("unit")
	fmt.Println(sec.Key("ExecStart").Values())
	fmt.Println(sec.Key("password").String())
	// Output:
	// [/bin/a /bin/b]
	// abc#123
}

func ExampleList() {
	f, err := ini.Load([]byte("ports = 80, 443, x\nratio = 1.5\n"))
	if err != nil {
		panic(err)
	}
	k := f.Section("").Key("ports")

	// List gives the zero value for an element that doesn't parse,
	// ValidList leaves it out, and StrictList returns an error.
	fmt.Println(ini.List[int](k, ","))
	fmt.Println(ini.ValidList[int](k, ","))
	_, err = ini.StrictList[int](k, ",")
	fmt.Println(err != nil)

	// 1.5 is outside [0, 1], so Range returns the default.
	ratio := f.Section("").Key("ratio")
	fmt.Println(ini.Range(ratio, 1.0, 0, 1))
	// Output:
	// [80 443 0]
	// [80 443]
	// true
	// 1
}

type Config struct {
	AppName string `ini:"app_name"`
	Server  struct {
		Port    int           `comment:"Port to listen on"`
		Timeout time.Duration `ini:",omitempty"`
		Hosts   []string
	} `ini:"server"`
}

// Fields keep their values when the section has no key for them, so a
// struct can be filled with defaults first.
func ExampleSection_MapTo() {
	f, err := ini.Load([]byte("[server]\nPort = 9090\n"))
	if err != nil {
		panic(err)
	}

	var cfg Config
	cfg.Server.Port = 8080
	cfg.Server.Timeout = 30 * time.Second
	if err := f.Section("server").MapTo(&cfg.Server); err != nil {
		panic(err)
	}
	fmt.Println(cfg.Server.Port, cfg.Server.Timeout)
	// Output:
	// 9090 30s
}

func ExampleFile_Write() {
	f := ini.Empty()
	f.Section("").Key("name").SetValue("demo")
	sec, _ := f.NewSection("server")
	_, _ = sec.NewKey("port", "8080")
	_, _ = sec.NewKey("bind_address", "0.0.0.0")
	sec.SetComment("Network settings")

	_, _ = f.Write(os.Stdout, ini.AlignDelimiters(false), ini.DelimiterSpaces("", ""))
	// Output:
	// name=demo
	//
	// ; Network settings
	// [server]
	// port=8080
	// bind_address=0.0.0.0
}

func ExampleParseError() {
	_, err := ini.Load([]byte("[server]\nport = 8080\nbroken line\n"))

	var perr *ini.ParseError
	if errors.As(err, &perr) {
		fmt.Println(perr.Line, perr.Column)
	}
	fmt.Println(errors.Is(err, ini.ErrDelimiterNotFound))
	fmt.Println(err)
	// Output:
	// 3 1
	// true
	// line 3, column 1: key-value delimiter not found: broken line
}
