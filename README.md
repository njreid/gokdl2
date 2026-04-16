# gokdl2

`gokdl2` is a Go library for the [KDL Document Language](https://kdl.dev/).

This repository is an actively maintained fork of [`sblinch/kdl-go`](https://github.com/sblinch/kdl-go). The original project remains the historical upstream reference, while ongoing development for this fork lives here in `njreid/gokdl2`.

## What It Does

- KDL v2 parsing
- KDL v1 parsing
- automatic parsing mode that tries v2 first and falls back to v1
- generating, marshaling, and unmarshaling KDL documents

## Why This Fork

- active KDL v2 support alongside KDL v1 compatibility
- automatic version selection through `ParseVersionAuto`, with explicit `ParseVersionV1` and `ParseVersionV2` controls
- backtick-delimited expression literals preserved as `document.Expression` values for CEL or similar runtimes
- friendlier parse errors with line, column, and source excerpts
- parser allocation work focused on reducing parse-time allocation churn on larger documents

## Features

- passes the vendored official KDL v1 and KDL v2 parser compliance suites
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

## More Usage

For fuller examples covering parsing modes, expressions, generating, marshaling, unmarshaling, and relaxed nginx-style syntax, see [USAGE.md](USAGE.md).

Detailed marshaling and unmarshaling behavior is documented in [docs/marshal.md](docs/marshal.md) and [docs/unmarshal.md](docs/unmarshal.md).

## Verifying spec compliance

The repository vendors the official parser compliance suites for both KDL versions.

It also supports the broader [`kdl-org/kdl-test`](https://github.com/kdl-org/kdl-test) compatibility suite through `TestKDLTestSuite` when the suite is checked out locally at `../kdl-test` or exposed via `KDL_TEST_SUITE_DIR`.

Run the normal test suite:

```bash
go test ./...
```

Run the vendored official v2 compliance suite explicitly:

```bash
KDL_RUN_V2_CASES=1 go test ./internal/parser -run TestKDLOrgTestCasesV2 -count=1
```

## Development status

`gokdl2` is actively maintained and tracks the newer work happening in this fork.

Issue reports and pull requests are welcome.

## License

`gokdl2` is released under the MIT license. See [LICENSE](LICENSE) for details.
