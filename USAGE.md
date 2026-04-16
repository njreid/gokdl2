# Usage

`gokdl2` supports parsing, generation, marshaling, unmarshaling, version selection, and expression literals.

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

Detailed behavior is described in [docs/unmarshal.md](docs/unmarshal.md).

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

Detailed behavior is described in [docs/marshal.md](docs/marshal.md).

## Relaxed nginx-style syntax

`gokdl2` can also parse nginx-style configuration files using `relaxed.NGINXSyntax`:

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
