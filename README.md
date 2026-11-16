# gokdl2

`gokdl2` is a Go library for the [KDL Document Language](https://kdl.dev/).

This repository is an actively maintained fork of [`sblinch/kdl-go`](https://github.com/sblinch/kdl-go). The original project remains the historical upstream reference, while ongoing development for this fork lives here in `njreid/gokdl2`.

## What's New

### v0.7.2

Comments survive a round trip, and the parser is much harder to trip up.

- **Comments ([#43](https://github.com/njreid/gokdl2/issues/43)):** with `ParseComments`, every comment is kept in place: trailing, end-of-block and end-of-document comments, comments between arguments and after `}`, multi-line `/* */` comments with their indentation, and slash-dashed nodes, entries and blocks. Output is stable on a second pass. See [USAGE.md](USAGE.md#comments).
- **Hang fixed ([#45](https://github.com/njreid/gokdl2/issues/45)):** a number with a huge exponent, such as `1e-10210200`, made parsing or generating run for minutes. It now takes well under a millisecond and round-trips exactly.
- **Parsing edge cases ([#44](https://github.com/njreid/gokdl2/issues/44)):** values at the very end of the input are no longer dropped. Unclosed blocks, a dangling `/-`, duplicate child blocks, entries after a block, and text after a `\` continuation are rejected. `/* */` works as a separator everywhere.

### v0.7.1

Every name gokdl2 generates now parses back, in both KDL versions.

- **Generation quotes names that would be misread:** v2 keywords and number-like names such as `true`, `inf`, `+1` and `.5` ([#5](https://github.com/njreid/gokdl2/pull/5)); v1 `true`, `false`, `null`, sign-plus-digit names and names starting with `r#` ([#39](https://github.com/njreid/gokdl2/issues/39)); and, in both versions, names starting with a backtick ([#40](https://github.com/njreid/gokdl2/issues/40)) or containing U+FEFF.
- **Unmarshal:** repeated keys in `,multiple` map nodes no longer panic ([#3](https://github.com/njreid/gokdl2/issues/3), [#4](https://github.com/njreid/gokdl2/pull/4)).
- **Parsing:** KDL v2 identifiers may start with a comma ([#41](https://github.com/njreid/gokdl2/issues/41)), and v2 rejects `-.5x`-style identifiers ([#5](https://github.com/njreid/gokdl2/pull/5)).
- **Testing:** a fuzz target checks that any string round-trips as a node name, property key and argument.

### v0.7.0

Faster, stricter, and much closer to the spec. Full details are in [PR #29](https://github.com/njreid/gokdl2/pull/29) and the [v0.7.0 release](https://github.com/njreid/gokdl2/releases/tag/v0.7.0).

#### Performance

Parsing is about 2× faster, generation about 3× faster, and all of them allocate far less. These are medians of 5 alternating runs of v0.6.1 against v0.7.0, on a 1 MB document, using `bench_test.go`:

| Benchmark | Time | Memory per op | Allocations |
|---|---|---|---|
| Parse v2 (auto) | 76 → 39 ms (−49%) | 21.8 → 11.8 MB | 273k → 181k |
| Parse v1 (auto) | 66 → 35 ms (−46%) | 30.4 → 11.8 MB | 286k → 181k |
| Parse, 668 B document | 78 → 28 µs (−64%) | 80 → 12 KB | 206 → 139 |
| Generate | 14.3 → 5.2 ms (−64%) | 4.2 → 0.07 MB | 295k → 835 |
| Unmarshal | 99 → 71 ms (−28%) | 29.3 → 19.4 MB | 514k → 421k |
| Marshal | 43 → 33 ms (−23%) | 18.2 → 12.9 MB | 601k → 309k |

Where the gains come from:

- **Tokenizer:** ASCII fast paths for whitespace, identifiers and quoted strings ([#24](https://github.com/njreid/gokdl2/issues/24)), and no allocations per token ([#20](https://github.com/njreid/gokdl2/issues/20)).
- **Parser:** a flattened `[state][token]` dispatch table instead of two map lookups per token ([#25](https://github.com/njreid/gokdl2/issues/25)).
- **Input:** read once instead of buffered and copied twice ([#21](https://github.com/njreid/gokdl2/issues/21)).
- **Memory:** nodes and values come from small on-demand chunks instead of a large up-front arena ([#22](https://github.com/njreid/gokdl2/issues/22)), and node properties are a slice instead of a map plus a slice ([#23](https://github.com/njreid/gokdl2/issues/23)).
- **Generator:** builds each node in one buffer and writes it once ([#26](https://github.com/njreid/gokdl2/issues/26)).
- **Marshaler:** caches struct type information across calls ([#27](https://github.com/njreid/gokdl2/issues/27)).

#### Correctness

**Parsing now accepts valid KDL it used to reject:**
- numbers directly followed by `}`, `{`, `;`, a comment, `/-` or `\`. For example, `n {x 1}` and `n 0xff;` failed before ([#32](https://github.com/njreid/gokdl2/issues/32));
- in v2, a type-annotated argument after another argument, as in `node 1 (u8)2` ([#28](https://github.com/njreid/gokdl2/issues/28));
- signed hexadecimal, octal and binary numbers such as `-0xff` ([#14](https://github.com/njreid/gokdl2/issues/14));
- a literal U+FFFD character ([#34](https://github.com/njreid/gokdl2/issues/34));
- in v1, a comment directly after an argument ([#32](https://github.com/njreid/gokdl2/issues/32)).

**…and rejects invalid KDL it used to accept:**
- a property left without a value, as in `node { a b= }` ([#14](https://github.com/njreid/gokdl2/issues/14));
- `\u{zz}` and other non-hex Unicode escapes ([#14](https://github.com/njreid/gokdl2/issues/14));
- a bare identifier as a v1 property value, as in `node x=bad` ([#33](https://github.com/njreid/gokdl2/issues/33));
- in v2, values not separated by whitespace, such as `1(u8)2` and adjacent expression strings ([#28](https://github.com/njreid/gokdl2/issues/28)).

**Parsing behaviour:**
- A failing `io.Reader` is reported instead of silently producing an empty document ([#6](https://github.com/njreid/gokdl2/issues/6)).
- A `/- kdl-version` marker is authoritative, and when a document parses as neither version the v2 error is reported ([#11](https://github.com/njreid/gokdl2/issues/11)).
- Quoted type annotations are decoded, so `("u8")` and `(u8)` are equal ([#17](https://github.com/njreid/gokdl2/issues/17)).

**Error messages:**
- They point at the start of the offending token, as kdl-rs does; an unclosed block is reported at its `{` ([#35](https://github.com/njreid/gokdl2/issues/35)).
- Line numbers are correct in files with CRLF line endings ([#30](https://github.com/njreid/gokdl2/issues/30)), and an off-by-one in columns after the first line is fixed.
- Strings report "unexpected newline" and "unterminated string" precisely ([#15](https://github.com/njreid/gokdl2/issues/15)).

**Generation:**
- Generated KDL always parses back to the same document, including multi-line and raw strings, `\u{…}` escapes and negative hex ([#12](https://github.com/njreid/gokdl2/issues/12), [#31](https://github.com/njreid/gokdl2/issues/31)).
- Hex values keep their type annotation ([#13](https://github.com/njreid/gokdl2/issues/13)).
- Write errors are no longer dropped ([#18](https://github.com/njreid/gokdl2/issues/18)).

**Marshaling and unmarshaling:**
- `UnmarshalWithOptions` honours all of its options ([#7](https://github.com/njreid/gokdl2/issues/7)).
- `encoding.TextMarshaler` output is no longer truncated to its first word ([#8](https://github.com/njreid/gokdl2/issues/8)).
- Types with the same name no longer share field tables ([#9](https://github.com/njreid/gokdl2/issues/9)).
- Structs stored in `interface{}` no longer panic ([#10](https://github.com/njreid/gokdl2/issues/10)).
- Concurrent marshals no longer share scanner state ([#19](https://github.com/njreid/gokdl2/issues/19)).

#### Testing and tooling

- All 563 official kdl-org v1 and v2 test cases run on every `go test ./...`; the v2 suite used to be opt-in ([#36](https://github.com/njreid/gokdl2/issues/36)).
- 118 cases ported from [kdl-rs](https://github.com/kdl-org/kdl-rs), the official Rust implementation, including expected error locations.
- Fuzz tests check that the tokenizer's fast paths match its general path, and that strings survive a generate-and-parse round trip.
- CI runs `go vet`, the tests and the race detector, and pushing a `v*` tag creates a GitHub release ([#16](https://github.com/njreid/gokdl2/issues/16)).

#### Upgrading from v0.6

- **Stricter parsing:** documents that relied on the invalid forms listed above now fail to parse.
- **Error positions:** error lines and columns differ, and an unclosed block's message starts with `unclosed child block:`.
- **Go version:** Go 1.22 or later is required.
- **New API:**
  - `document.Node.EmptyType` and `document.Value.EmptyType`, which record an explicit `("")` annotation
  - `tokenizer.DetectVersion`
  - `tokenizer.ScanOneN`
- **`Document.Retain`** is now a no-op.

## What It Does

- KDL v2 parsing
- KDL v1 parsing
- automatic parsing mode that tries v2 first and falls back to v1
- generating, marshaling, and unmarshaling KDL documents

## Why This Fork

- active KDL v2 support alongside KDL v1 compatibility
- automatic version selection through `ParseVersionAuto`, with explicit `ParseVersionV1` and `ParseVersionV2` controls
- [backtick expression strings](#expression-strings), preserved as `document.Expression` values for CEL or similar runtimes
- friendlier parse errors with line, column, and source excerpts
- fast: about twice the parse speed of v0.6 ([details](#performance))

## Features

- passes all 563 cases of the official KDL v1 and v2 compliance suites, plus 118 cases ported from kdl-rs
- deterministic output for generated documents and marshaled maps/properties
- familiar API and tag syntax, similar to `encoding/json`
- marshaling and unmarshaling for Go structs, maps, and custom `(Un)Marshal` interfaces
- support for `encoding/json/v2`-style `format` options for `time.Time`, `time.Duration`, `[]byte`, and `float32/64`

## Import

```go
import "github.com/njreid/gokdl2"
```

## Quick Start

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

## Expression strings

`gokdl2` adds backtick-delimited expression strings to KDL for embedding code, such as CEL, JMESPath or SQL, without escaping every quote. They parse to `document.Expression`, a string type distinct from plain strings:

````kdl
rule (cel)`request.auth.claims.role == "admin"`
deny ```
    request.path.startsWith("/admin")
      && !request.auth.mfa
    ```
````

```go
expr := doc.Nodes[0].Arguments[0].Value.(document.Expression)
```

- Single backticks hold a one-line expression, and triple backticks a multi-line one, dedented like KDL v2 `"""` strings.
- Escapes are those of KDL quoted strings, with `` \` `` for a backtick.
- They work as arguments and property values, with optional type annotations, in v1, v2 and auto mode.
- They round-trip through generation and marshaling, and unmarshal into `document.Expression`, `string` or `interface{}`.

This is an extension, not part of the KDL spec, so other parsers will reject documents that use it. The full reference is in [USAGE.md](USAGE.md#expression-strings).

## More Usage

For fuller examples covering parsing modes, expressions, generating, marshaling, unmarshaling, and relaxed nginx-style syntax, see [USAGE.md](USAGE.md).

Detailed marshaling and unmarshaling behavior is documented in [docs/marshal.md](docs/marshal.md) and [docs/unmarshal.md](docs/unmarshal.md).

## Verifying spec compliance

The repository vendors the official compliance suites for both KDL versions (`kdl-org-v1/`, and `kdl-org-v2/` at the commit recorded in [`kdl-org-v2/VENDORED.md`](kdl-org-v2/VENDORED.md)). Both run as part of the normal test suite, together with the cases ported from kdl-rs in `upstream_cases_test.go`:

```bash
go test ./...
```

The broader [`kdl-org/kdl-test`](https://github.com/kdl-org/kdl-test) suite runs through `TestKDLTestSuite` when it is checked out at `../kdl-test`, or at the path in `KDL_TEST_SUITE_DIR`.

To fuzz the tokenizer and the string round trip:

```bash
go test -run '^$' -fuzz FuzzFastPathMatchesRunePath ./internal/tokenizer
go test -run '^$' -fuzz FuzzStringRoundTrip .
```

## Development status

`gokdl2` is actively maintained and tracks the newer work happening in this fork.

Issue reports and pull requests are welcome.

## License

`gokdl2` is released under the MIT license. See [LICENSE](LICENSE) for details.
