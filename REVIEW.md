# gokdl2 — code analysis and performance review

Reviewed at `87befa8` with Go 1.24.7 on linux/amd64. I read the tokenizer, parser, `document` package,
generator and public API in full, and the marshaler selectively (type index, entry points, hot paths).
Every correctness finding below marked **confirmed** was reproduced with a throwaway test; those tests and the
trial performance edits have been removed, so the tree is unchanged apart from this file.

## Baseline

| Check | Result |
|---|---|
| `go build ./...` | passes |
| `go test ./...` | 1 failure: `TestUnmarshalMapTimes` (`unmarshal_test.go:1191`) |
| `go test -race ./...` | same single failure, no races reported |
| `go vet ./...` | fails: unreachable code at `internal/parser/transitions.go:391` and `example_test.go:1034` |
| Streaming vs. slice scanning | identical tokens for all 561 corpus files at buffer sizes 16–64 bytes, including one-byte reads |

`TestUnmarshalMapTimes` fails because it compares `time.Local` against a UTC location with `DeepEqual`; it
fails on any machine whose local zone is UTC (most CI runners).

The last row is good news: the buffer-refill fix in `d9e89bb` holds up under a differential test.

---

## Correctness findings

Ordered by how much damage each can do.

### 1. Read errors are swallowed in auto-version mode — confirmed, high

`kdl.Parse` with the default options returns an **empty document and a nil error** when the reader fails.

- `tokenizer.NewBuffer` (`internal/tokenizer/scanner.go:943`) records the error in `s.err` and discards the
  bytes already read.
- `kdl.go:37` then calls `io.ReadAll(s)`. `Scanner.Read` never looks at `s.err`, and on a failed refill it falls
  through to `return 0, io.EOF` (`scanner.go:58`), so `ReadAll` reports success.

A reader that returns 14 valid bytes and then `disk on fire` produces a zero-node document with `err == nil`.
With an explicit version the error is reported correctly. Fix: have `Read` return `s.err`/`s.lastErr` before
EOF, or better, read the input directly in `parse` rather than through a scanner (see P4).

### 2. `UnmarshalWithOptions` ignores its options — confirmed, high

`unmarshal.go:68` calls `marshaler.Unmarshal(doc, v)` instead of `marshaler.UnmarshalWithOptions(doc, v, opts)`.
`AllowUnhandledNodes`, `AllowUnhandledArgs/Props/Children` and `CaseSensitive` are silently dropped; only the two
parser flags take effect. `Decoder.Decode` does it correctly, so the same options behave differently depending
on the entry point.

### 3. `encoding.TextMarshaler` output is truncated or retyped — confirmed, high

`marshalTextValue` (`internal/marshaler/marshal.go:469`) scans only the **first token** of the returned text and
uses that as the value:

| `MarshalText` returns | Marshaled as |
|---|---|
| `hello world` | `"hello"` |
| `10 Downing Street` | `10` |
| `true story` | `true` |

That is silent data loss. The token should only be trusted when it spans the whole input; otherwise the text
should be emitted as a string.

### 4. Type index is keyed by `reflect.Type.String()` — confirmed, high

`typeIndexer.index` (`internal/marshaler/typeindex.go:256`) uses the type's printed name as its key. Two distinct
types that print the same (same package name under different import paths, or a function-local type shadowing
a package-level one) share one entry. Reproduced: a struct with fields `x`, `y`, `beta` was marshaled as
`l alpha="x"` using the other type's field table, and unmarshaling it failed with "unexpected properties".
Key the map by `reflect.Type`.

### 5. Marshaling a struct held in an `interface{}` panics — confirmed, high

Only statically reachable types are indexed, so a struct stored in an `any` field or a
`map[string]any` value has no entry and the marshaler dereferences nil. `Marshal(map[string]any{"k": someStruct})`
crashes the process. Index dynamic types lazily on first sight.

### 6. Auto mode reports the wrong error and ignores the version marker — confirmed, medium

In `kdl.go:53` the guard `s2.Version == VersionV2 && s2.Offset() > 0` is meant to stop the v1 fallback when a
`/- kdl-version 2` marker is present. It can never be true: `s2.Version` was set to V2 by hand, and `parseOne`
has already called `s.Close()`, which nils the buffers, so `Offset()` is 0.

Consequences:
- Any syntax error in a v2 document is reported as a **v1** parse error. A v2 file with an unclosed brace on
  line 2 reports "expected '=' after the property name" on line 1.
- `/- kdl-version 1` followed by v2 syntax is accepted as v2.
- A failing v2 document is always parsed twice.

Detect the marker up front, and when both attempts fail, return the v2 error (or both).

### 7. Generator emits KDL that does not parse back — confirmed, medium

- **Multi-line strings** (`document/strings.go:701`, `:723`): the closing `"""` is written on the same line as
  the last content line. `"""\n  a\n   b\n  """` round-trips to `"""\na\n b"""`, which the parser rejects with
  "non-whitespace content on closing line".
- **Control characters** (`strings.go:109`): escaped as `\u1`, `\u7f` — no braces. The parser requires
  `\u{…}`, and `\u1b` is ambiguous. Invalid UTF-8 is written as the literal text `�` (`:77`, `:189`).
- **Negative hex/octal/binary** (`document/value.go`): `-255` with `FlagHexadecimal` prints `0x-ff`.

### 8. Type annotation lost on hex values — confirmed, medium

`document/value.go:115` does `b = make([]byte, 0, 18)` unconditionally for hex, discarding the `(type)` prefix
already in `b`. `node (u8)0xff` generates as `node 0xff`. The binary and octal branches guard with `b == nil`;
hex needs the same.

### 9. Parser accepts malformed input — confirmed, medium

- `node { a b= }` parses and silently drops `b=`. The `BraceClose` handlers in `stateProperty` and
  `statePropertyValue` (`internal/parser/transitions.go:453`, `:634`) pop the node without checking that a
  pending key was completed, and leave `c.ident` set.
- `\u{zz}` is accepted as U+0000: `hexTable` returns 0 for non-hex bytes (`strings.go:268`).
- Signed prefixed numbers (`-0xff`, `+0b1`) are rejected, though the spec allows a sign.

### 10. Unterminated strings give a misleading error — confirmed, low

In `readQuotedStringQ` the closure assigns `err` (`readtype.go:250`, `:271`) but the function returns
`readWhile`'s result, so that error is dead. The string is caught later by the unquoter, so nothing invalid is
accepted, but the message is "invalid quoted string" pointing at the wrong column instead of "unexpected
newline in string".

### 11. Smaller items

- **`go.mod` says `go 1.19`** but the code uses `slices`, `atomic.Bool` and `reflect.TypeFor`, which need 1.22.
- **Quoted type annotations keep their quotes**: `("my type")node` gives `Type == "\"my type\""`, so
  `("u8")` and `(u8)` compare unequal.
- **Write errors in `Node.WriteToOptions`**: `err = e` (`node.go:264`) lets a later successful write clear an
  earlier error, and the child loop shadows `err` (`node.go:369`), so a child's failure is dropped. In practice
  a failing writer keeps failing so the error still surfaces; a writer with transient errors could lose one.
- **`ScanOne`** (`scanner.go:404`) shares one scanner under a global mutex and never resets `Version` or
  `versionMarkerDetected`, so state leaks between calls and concurrent marshals serialize on it.
- **Tests write `cpu.pprof`/`mem.pprof`** into the source tree (`unmarshal_test.go:1293`,
  `internal/tokenizer/scanner_test.go:581`) and they are not in `.gitignore`.
- **`PERF.md` references `./perf/compare`**, which is not in the repository; there are no parse benchmarks
  checked in.
- **`bytesToString`** (`value.go:438`) makes document strings alias the token buffer. I checked every public
  entry point and none exposes caller-owned memory today, but only because auto mode happens to copy the input
  first; this is worth a comment or a test, since removing that copy (P4) would otherwise introduce aliasing.

---

## Performance

Benchmarks used a synthetic 1.0 MB v2 document (3,000 server blocks with properties, nested children, strings,
numbers and comments) on a 4-core Xeon VM. Numbers are single runs unless noted and the machine is noisy; treat
differences under about 10% as indistinguishable.

| Operation | Throughput | Bytes allocated | Allocations |
|---|---|---|---|
| Parse, auto version | 12.6 MB/s | 21.8 MB | 273k |
| Parse, explicit v2 via `bytes.Reader` | 11.7 MB/s | 17.8 MB | 522k |
| Parse, explicit v2, plain `io.Reader` | 9.9 MB/s | 14.8 MB | 606k |
| Parse, 668-byte document | 81 µs | 79.8 KB | 206 |
| Generate | 59 MB/s | 4.4 MB | 296k |
| Unmarshal into structs | 8.8 MB/s | 29.3 MB | 514k |
| Marshal from structs | 12 MB/s | 19.1 MB | 615k |

Parsing is the bottleneck: it is about 70% of `Unmarshal`. CPU time splits roughly 39% tokenizer, 43% parser and
value construction, and the rest allocation and GC.

### Measured: three small edits give about 20%

I trialled these together, ran the tokenizer and parser tests (pass), and compared medians of six runs:

| | Before | After |
|---|---|---|
| Parse 1 MB, auto | 93.1 ms, 273k allocs | 71.3 ms, 195k allocs |
| Parse 668 B | 86.8 µs, 206 allocs | 71.6 µs, 154 allocs |

**P1. Unguarded debug log** — `internal/tokenizer/readtype.go:690`. `s.log("checking…", "c", c, "c2", c2)`
builds and heap-allocates its variadic `[]interface{}` on every identifier even though `Logger` is nil. This
one line is 16% of all allocations in a parse. Wrap it in `if s.Logger != nil`; the same applies to the calls
with arguments at `scanner.go:674`, `:676` and `:702`.

**P2. `sLit := string(literal)`** — `readtype.go:333`. Allocates a string per bare identifier only to compare
it against six keywords; 12% of allocations. `switch string(literal)` compiles without allocating.

**P3. `recent.Get()` per token** — `internal/parser/parser.go:134`. Every v2 token copies up to five 64-byte
tokens into a temporary slice just to read the previous one. Index the ring buffer directly; about 5% of CPU.

### Not measured, ranked by expected payoff

**P4. Input is buffered and copied twice in auto mode.** `tokenizer.New` allocates a 64 KB buffer and fills it
(`scanner.go:962`), then `kdl.go:37` drains the scanner through `io.ReadAll` into a second, growing buffer
(24% of bytes allocated), and `Unmarshal(data)` copies a slice the caller already had. This is why a 668-byte
parse allocates 80 KB. Read the `io.Reader` once in `parse`, and take the `[]byte` path directly for
`Unmarshal`. That also removes finding 1. If `Unmarshal` stops copying, the strings must be copied instead
(see the `bytesToString` note).

**P5. The node/value arena is oversized and pins memory.** `NewAllocator(inputBytes/25)` (`parser.go:76`)
allocates and zeroes about 8.3 bytes per input byte up front — 38% of all bytes allocated — and each slot is
zeroed a second time when handed out (`allocator.go`). Two side effects beyond speed: a document that is mostly
comments or long strings still pays the full amount, and a caller who keeps one `*Node` from a large document
keeps the whole slab alive. Allocate in fixed chunks (say 256 nodes) on demand; `Document.retain` is also
unnecessary, since interior pointers already keep the slabs alive.

**P6. Properties cost a map and a slice per node.** `Properties.Alloc` (`document/properties.go:29`) makes an
8-entry map plus an 8-string slice for every node with at least one property — about 11% of bytes in parse and
13% in marshal. Most nodes have a handful of properties; a single slice of `{key, *Value}` with linear lookup
is smaller and faster at that size, and a map can be added lazily past, say, 16 entries.

**P7. The scanner works one rune and one closure call at a time.** Each character goes through `peek` →
`peekSize` → predicate closure → `skip` → `get`, with a refill check, a peek-buffer shift and a newline test
per rune, and `readQuotedStringQ` heap-allocates its closure state per string. ASCII fast paths that scan bytes
directly for whitespace, bare identifiers and unescaped string bodies (`bytes.IndexByte` for `"` and `\`) are
where the largest remaining tokenizer gain is. This is the biggest change on the list.

**P8. Streaming copies every token.** While the reader is still open, `copyInput`/`copyFromMark` allocate a
fresh slice per token, including whitespace and newlines the parser throws away. That is the difference between
273k and 606k allocations above. Skip the copy for trivia tokens unless `ParseComments` is set.

**P9. Dispatch through two maps.** `stateTransitions[state][token.ID]` plus the class fallback loop over
`tokenClasses` (itself a map) costs about 6% of parse CPU in hashing. `PERF.md` records that a `switch` rewrite
regressed; a middle path is to flatten the existing handlers into a `[numStates][numTokenIDs]` array built once
in `init`, with classes pre-resolved. Same handlers, no hashing.

**P10. Generation allocates per value.** `WriteToOptions` formats each argument and property into a new
`string`, converts it to `[]byte`, and issues a separate `Write`; `bytes.Repeat` builds the indent for every
node (`node.go:269`). Appending into one reusable buffer per node would remove most of the 296k allocations.
Low priority at 59 MB/s.

**P11. The type index is rebuilt on every call.** `newTypeIndexer` runs in each `Marshal`/`Unmarshal`, then
lookups go through `Type().String()`, which is not free. Caching per `reflect.Type` in a `sync.Map` fixes this
and finding 4 together, and matters most for many small documents.

### On `PERF.md`

Items 3 and 4 are done and effective. Item 7 (peeked size) and item 13 (mark stack) do not show up in the
profile and are not worth doing. Item 5 (tagged-union `Value`) would help but is an API break; P1–P6 are
cheaper and larger.

---

## Suggested order

1. Findings 1, 2, 3, 8 and P1–P3: each is a few lines and independently testable.
2. Findings 4 and 5 with P11: one change to the type index.
3. Finding 6 with P4: restructure `parse` so the input is read once and the marker decides the fallback.
4. Finding 7 (generator round-trip), ideally with a parse → generate → parse test over the vendored corpus.
5. P5 and P6, then P7 if more parse speed is needed.
6. Fix `go vet`, the timezone-dependent test and `go.mod`, and add CI so they stay fixed.
