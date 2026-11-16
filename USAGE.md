# Usage

`gokdl2` supports parsing, generation, marshaling, unmarshaling, version selection, and [expression strings](#expression-strings).

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

## Expression strings

`gokdl2` extends KDL with backtick-delimited **expression strings** for embedding code, such as CEL, JMESPath, SQL fragments or templates, without having to escape every quote. They parse to `document.Expression`, a string type distinct from ordinary KDL strings, so your code can tell an expression from plain text and pass it to an evaluator.

> **Non-standard.** Expression strings are a `gokdl2` extension, not part of the KDL specification. Other KDL parsers reject documents that use them. `gokdl2` accepts them in v1, v2 and automatic mode, with no option required.

### Syntax

A **single-line** expression is wrapped in single backticks. Double quotes need no escaping:

```kdl
rule `request.auth.claims.role == "admin"`
```

A **multi-line** expression uses triple backticks. The opening ```` ``` ```` must be followed by a newline. The indentation of the closing ```` ``` ```` line is removed from every line, as with KDL v2 `"""` strings:

````kdl
policy ```
    request.path.startsWith("/admin")
      && !request.auth.mfa
    ```
````

That value is `request.path.startsWith("/admin")\n  && !request.auth.mfa`.

Escapes work as in KDL quoted strings, in both forms:

| Escape | Meaning |
|---|---|
| `` \` `` | a literal backtick |
| `\\` | a backslash |
| `\n` `\r` `\t` `\b` `\f` `\s` | newline, carriage return, tab, backspace, form feed, space |
| `\u{1F600}` | a Unicode code point (1–6 hex digits) |
| `\` followed by whitespace or a newline | removes that whitespace, so a long line can be wrapped |

Any other escape is an error, including `\"`, because double quotes are literal. Comment markers (`//`, `/* */`) and `#` inside an expression are just text. There is no raw form (``#```…```#``), so a backslash always starts an escape.

### Where expressions can appear

Expressions are **values**:

- arguments: `` rule `a > 1` ``
- property values: `` rule when=`a > 1` ``
- with a type annotation, for example to name the language: `` rule (cel)`a > 1` ``

They cannot be node names or property keys. In KDL v2 an expression must be separated from neighbouring values by whitespace, like any other value, so ``` rule `a``b` ``` is an error and `` rule `a` `b` `` has two arguments. A `;`, `}`, comment or `/-` may follow directly.

### Reading expressions

```go
data := "rule (cel)`request.auth.claims.role == \"admin\"` deny=```\n" +
	"    request.path.startsWith(\"/admin\")\n" +
	"      && !request.auth.mfa\n" +
	"    ```\n"

doc, err := kdl.Parse(strings.NewReader(data))
if err != nil {
	panic(err)
}

arg := doc.Nodes[0].Arguments[0]
expr := arg.Value.(document.Expression)
fmt.Printf("%s %q\n", arg.Type, expr)

deny, _ := doc.Nodes[0].Properties.Get("deny")
fmt.Printf("%q\n", deny.Value.(document.Expression))
```

```text
cel "request.auth.claims.role == \"admin\""
"request.path.startsWith(\"/admin\")\n  && !request.auth.mfa"
```

`Value.ResolvedValue()` also returns the `document.Expression`, and `Value.ValueString()` returns its text. Parsed values carry `document.FlagExpression`, plus `document.FlagMultiLine` for the triple-backtick form.

### Writing expressions

Any `document.Expression` value is generated with backticks. A value containing a newline is written in triple-backtick form, and backticks inside the value are escaped.

```go
node := document.NewNode()
node.SetName("rule")
node.AddArgument(document.Expression("size(items) > 0"), "")
node.AddProperty("when", document.Expression("a\n&& b"), "")

fmt.Println(node.String())
```

````text
rule `size(items) > 0` when=```
a
&& b
```
````

### Marshaling and unmarshaling

Struct fields of type `document.Expression` marshal as expressions. An expression unmarshals into a `document.Expression` field, into a `string` field (as its text), or into an `interface{}` field, where it stays a `document.Expression`. Ordinary quoted strings also unmarshal into `document.Expression` fields.

```go
type Policy struct {
	Allow document.Expression `kdl:"allow"`
	Note  string              `kdl:"note"`
	Any   interface{}         `kdl:"any,omitempty"`
}

out, err := kdl.Marshal(Policy{Allow: "user.admin", Note: "plain text"})
if err != nil {
	panic(err)
}
fmt.Print(string(out))

var p Policy
if err := kdl.Unmarshal([]byte("allow `user.admin`\nnote `as text`\nany `kept`\n"), &p); err != nil {
	panic(err)
}
fmt.Printf("%q %q %T\n", p.Allow, p.Note, p.Any)
```

```text
allow `user.admin`
note "plain text"
"user.admin" "as text" document.Expression
```

These examples are checked by `example_expression_test.go`.

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

### Comments

Comments are dropped unless `ParseOptions.ParseComments` is set. With it, each comment is kept on the node or value it belongs to, and `Generate` writes it back in the same place:

```go
doc, err := kdl.ParseWithOptions(strings.NewReader(data), kdl.ParseOptions{ParseComments: true})
```

| Where the comment is | Where it is kept |
| --- | --- |
| lines before a node | `Node.Comment.Before` |
| on the node's first line, after its last entry (or after `{` if it has a child block) | `Node.Comment.Trailing` |
| between the last entry and `{`, such as a slash-dashed argument | `Node.Comment.BeforeBlock` |
| on the line of the closing `}` of the node's child block | `Node.Comment.Closing` |
| between entries | `Value.Comment` of the entry after it |
| at the end of a child block, before `}` | `Node.Comment.Inside` of the node that owns the block |
| after the last top-level node | `Node.Comment.After` of that node (`Document.Comment` if there are no nodes) |

Entries are never written after a `}`: a slash-dashed argument or property goes before the `{` (`a 1 /-2 { x }`), and the parser rejects one after the block in KDL v2. Slash-dashed child blocks may be before or after the real block. Slash-dashed nodes, arguments, properties and child blocks are kept verbatim, as text, in the place they would be if they were a comment, so `/-a 1` survives generation and can be parsed again. Generating is idempotent after the first pass: parsing the output and generating again gives the same output. A blank line before a node or between comments is kept, a run of blank lines becomes one, and the indentation is normalised. See the doc comments of `document.Comment` for the exact rules.

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
