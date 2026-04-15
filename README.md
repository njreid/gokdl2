# gokdl2

`gokdl2` is a Go library for the [KDL Document Language](https://kdl.dev/).

This repository is an actively maintained fork of [`sblinch/kdl-go`](https://github.com/njreid/gokdl2). The original project remains the historical upstream reference, while ongoing development for this fork lives here in `njreid/gokdl2`.

It now supports:

- KDL v2 parsing
- KDL v1 parsing
- automatic parsing mode that tries v2 first and falls back to v1
- generating, marshaling, and unmarshaling KDL documents

## Features

- passes the vendored official KDL v1 and KDL v2 parser compliance suites
- public parse version selection via `ParseVersionAuto`, `ParseVersionV1`, and `ParseVersionV2`
- deterministic output for generated documents and marshaled maps/properties
- familiar API and tag syntax, similar to `encoding/json`
- marshaling and unmarshaling for Go structs, maps, and custom `(Un)Marshal` interfaces
- support for `encoding/json/v2`-style `format` options for `time.Time`, `time.Duration`, `[]byte`, and `float32/64`
- contextual parse errors with line/column information and source excerpts
- backtick-delimited expression literals, preserved as `document.Expression` values for CEL or similar runtimes

## Import

```go
import "github.com/njreid/gokdl2"
```

## Parsing

`Parse()` uses automatic version selection: it tries KDL v2 first, then falls back to KDL v1.

```go
data := "node .child\n"

doc, err := kdl.Parse(strings.NewReader(data))
if err != nil {
	panic(err)
}

fmt.Println(doc.Version)
fmt.Println(doc.Nodes[0].Name.NodeNameStringV2())
```

```text
2
node
```

Use `ParseWithOptions()` to force v1-only or v2-only parsing:

```go
v1doc, err := kdl.ParseWithOptions(strings.NewReader("node r\"raw\"\n"), kdl.ParseOptions{
	Version: kdl.ParseVersionV1,
})
if err != nil {
	panic(err)
}

v2doc, err := kdl.ParseWithOptions(strings.NewReader("node .child\n"), kdl.ParseOptions{
	Version: kdl.ParseVersionV2,
})
if err != nil {
	panic(err)
}

fmt.Println(v1doc.Version, v2doc.Version)
```

```text
1 2
```

## Expressions

Backtick-delimited expressions are parsed as `document.Expression` instead of ordinary strings, so downstream code can distinguish them from plain text.

```go
data := "rule `request.auth.claims.sub` filter=```\nrequest.auth != nil\n```\n"

doc, err := kdl.Parse(strings.NewReader(data))
if err != nil {
	panic(err)
}

arg := doc.Nodes[0].Arguments[0].ResolvedValue().(document.Expression)
fmt.Println(arg)
```

## Generating

`Generate()` writes a `*document.Document` back out as KDL:

```go
data := "name \"Bob\"\nage 76\nactive true\n"

doc, err := kdl.Parse(strings.NewReader(data))
if err != nil {
	panic(err)
}
if err := kdl.Generate(doc, os.Stdout); err != nil {
	panic(err)
}
```

```kdl
name "Bob"
age 76
active true
```

## Unmarshaling

### via `Unmarshal`

```go
type Person struct {
	Name   string `kdl:"name"`
	Age    int    `kdl:"age"`
	Active bool   `kdl:"active"`
}

data := []byte("name \"Bob\"\nage 76\nactive true\n")

var person Person
if err := kdl.Unmarshal(data, &person); err != nil {
	panic(err)
}

fmt.Printf("%+v\n", person)
```

```text
{Name:Bob Age:76 Active:true}
```

Detailed behavior is described in [docs/unmarshal.md](docs/unmarshal.md).

### via `Decoder`

```go
type Person struct {
	Name   string `kdl:"name"`
	Age    int    `kdl:"age"`
	Active bool   `kdl:"active"`
}

data := "name \"Bob\"\nage 76\nactive true\ngeriatric true\n"

var person Person
dec := kdl.NewDecoder(strings.NewReader(data))
dec.Options.AllowUnhandledNodes = true

if err := dec.Decode(&person); err != nil {
	panic(err)
}

fmt.Printf("%+v\n", person)
```

## Marshaling

### via `Marshal`

```go
type Person struct {
	Name   string `kdl:"name"`
	Age    int    `kdl:"age"`
	Active bool   `kdl:"active"`
}

person := Person{Name: "Bob", Age: 32, Active: true}

data, err := kdl.Marshal(person)
if err != nil {
	panic(err)
}

fmt.Println(string(data))
```

```kdl
name "Bob"
age 32
active true
```

Detailed behavior is described in [docs/marshal.md](docs/marshal.md).

### via `Encoder`

```go
type Person struct {
	Name   string `kdl:"name"`
	Age    int    `kdl:"age"`
	Active bool   `kdl:"active"`
}

enc := kdl.NewEncoder(os.Stdout)
if err := enc.Encode(Person{Name: "Bob Jones", Age: 32, Active: true}); err != nil {
	panic(err)
}
```

```kdl
name "Bob Jones"
age 32
active true
```

## Relaxed nginx-style syntax

`kdl-go` can also parse nginx-style configuration files using `relaxed.NGINXSyntax`:

```go
data := `
    # web root
    location / {
        root /var/www/html;
    }

    # a missing location
    location /missing {
        return 404;
    }
`

type Location struct {
	Root   string `kdl:"root,omitempty,child"`
	Return int    `kdl:"return,omitempty,child"`
}

type NginxServer struct {
	Locations map[string]Location `kdl:"location,multiple"`
}

var ngx NginxServer
dec := kdl.NewDecoder(strings.NewReader(data))
dec.Options.RelaxedNonCompliant |= relaxed.NGINXSyntax

if err := dec.Decode(&ngx); err != nil {
	panic(err)
}
```

See [docs/unmarshal.md](docs/unmarshal.md) for more detail.

## Verifying spec compliance

The repository vendors the official parser compliance suites for both KDL versions.

Run the normal test suite:

```bash
go test ./...
```

Run the vendored official v2 compliance suite explicitly:

```bash
KDL_RUN_V2_CASES=1 go test ./internal/parser -run TestKDLOrgTestCasesV2 -count=1
```

## Development status

`kdl-go` is actively maintained and has been used in production applications for several years.

Issue reports and pull requests are welcome.

## License

`kdl-go` is released under the MIT license. See [LICENSE](LICENSE) for details.
