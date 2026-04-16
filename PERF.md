# KDL Parser Performance Improvements

Prioritized by estimated impact × implementation feasibility. Updated after full re-review of current codebase.

## Progress Update

- Implemented parser-side pooled allocation for `Node` and `Value` via `document.Allocator`, retained on `Document` so arena-backed objects stay alive with the parsed document.
- Implemented `parseNumber()` cleanup and reduced parse-only `[]byte` to `string` allocations in `document/value.go`.
- Benchmarked replacing parser transition map dispatch with direct `switch` dispatch and reverted it: on this codebase it regressed parse throughput enough that it is not currently worth pursuing.
- Current arena sizing uses real input length when available at the API boundary, and falls back to heap allocation if the pool estimate is exhausted.

Measured against clean `HEAD` on the same machine with `go test -run=^$ -bench='Parse(Tiny|Flat|Properties|Nested|Large)$' -benchmem -count=3 ./perf/compare`:

- `ParseTiny`: roughly flat, allocs `22 -> 20`
- `ParseFlat`: faster, allocs `401 -> 249`
- `ParseProperties`: roughly flat to slightly better, allocs `292 -> 213`
- `ParseNested`: flat to modestly better, allocs `295 -> 199`
- `ParseLarge`: faster, allocs `17950 -> 12591`

Tradeoff observed so far:

- `ParseLarge` still uses more `B/op` because the allocator intentionally front-loads retained storage for parsed objects. That is acceptable for now, but allocator sizing is the next obvious tuning target.

---

## P1 — High Impact, Relatively Straightforward

### 1. Replace map-of-maps state dispatch with nested switch statements

**File:** `internal/parser/transitions.go`

The parser uses `map[parserState]map[tokenizer.TokenID]stateTransitionFunc` for state transitions. A `// TODO: benchmark this; it's likely faster to do this using switch statements` comment already acknowledges this. Each token dispatch costs two map lookups (state → inner map, then tokenID → func), plus a third lookup through the fallback `token.ID.Classes()` path when the tokenID isn't found directly. A nested `switch` over `parserState` then `tokenizer.TokenID` eliminates all map overhead and is compiler-inlineable.

Status: benchmarked and rejected for now. The direct `switch` rewrite made parse benchmarks worse on this codebase, so the existing map-based dispatch remains in place until profiling shows a different shape.

### 2. Arena allocation for Node and Value objects

**Files:** `document/document.go`, `document/value.go`, `internal/parser/context.go`

Currently every `Node` (~112 bytes) and `Value` (~49 bytes) is an independent heap allocation. For a 1 MB KDL file this is roughly 40K–100K separate allocations, each with its own GC tracking overhead. An arena is an excellent fit here:

- **Lifetime match**: all nodes and values are created in a burst during parsing and live exactly as long as the returned `*Document` — there is no incremental or mixed-lifetime allocation pattern.
- **Sizing heuristic**: use input byte count (or line count if available) to pre-size. Empirically, KDL has roughly one `Node` per 20–40 bytes and 1–2 `Value` objects per node. A reasonable formula:
  ```
  estimatedNodes  = inputBytes / 25
  nodeArenaBytes  = estimatedNodes * 120   // sizeof(Node) + headroom
  valueArenaBytes = estimatedNodes * 2 * 52 // sizeof(Value) * avg per node
  ```
  Overestimate by 2–3× is fine — the arena is freed when the document is freed.
- **Implementation sketch**:
  ```go
  type parseArena struct {
      nodes    []Node
      values   []Value
      ni, vi   int
  }
  func (a *parseArena) newNode() *Node  { n := &a.nodes[a.ni]; a.ni++; return n }
  func (a *parseArena) newValue() *Value { v := &a.values[a.vi]; a.vi++; return v }
  ```
  `ParseContext` receives the arena; overflow falls back to `new()`. Add an `Arena *parseArena` field to `Document` so it stays alive.
- **Expected win**: 15–30% reduction in parse time, significant drop in GC pressure on large documents.

**What the arena does NOT need to hold**: token `Data` byte slices (already managed by scanner buffer), `Arguments`/`Children` slice backing arrays (small, use standard append), property map internals.

Status: implemented in a parser-only form. `ParseContext` now allocates `Node` and `Value` structs from a pooled allocator, and `Document` retains that allocator to preserve object lifetime. The fallback path still uses ordinary heap allocation if the estimate is too small.

### 3. Eliminate redundant passes in `parseNumber()`

**File:** `document/value.go`

`parseNumber()` does multiple passes over the same bytes:
- `bytes.ReplaceAll(b, []byte{'_'}, []byte{})` always allocates and copies even when no `_` is present. Gate it behind `bytes.IndexByte(b, '_') >= 0`.
- `bytes.IndexByte` is called separately for `.`, `e`, `E`, and `_` — four linear scans. A single classification pass sets all four flags at once.
- `string(b)` is called twice (once for `ParseFloat`, once for `ParseInt`). Use `unsafe.String(&b[0], len(b))` or keep a reusable scratch string to avoid the allocation.

Status: implemented. `parseNumber()` now classifies numeric input in one pass, only strips underscores when present, and reuses a single byte-to-string conversion.

### 4. Avoid `string([]byte)` boxing in number and identifier parsing

**Files:** `document/value.go`, `document/document.go`

`strconv.ParseFloat(string(b), 64)` and `strconv.ParseInt(string(b), base, 64)` both allocate a temporary string. `strconv` reads the string byte-by-byte internally — the string conversion exists only to satisfy the type signature. Use `strconv.ParseFloat` via the `unsafe` string trick (`unsafe.String`) or write thin wrappers that accept `[]byte`. Same pattern applies to bare identifier conversion (`string(t.Data)`) in the value-from-token path.

Status: partially implemented. The parse-heavy paths in `document/value.go` now use a zero-copy byte-to-string conversion for number parsing, keyword parsing, quoted token parsing, and bare identifiers. There are still other conversion sites elsewhere in the codebase if profiling points back at them.

---

## P2 — High Impact, More Invasive

### 5. Replace `interface{}` value storage with a tagged union

**File:** `document/value.go`

`Value.Value` holds `interface{}` for one of: `int64`, `float64`, `*big.Int`, `*big.Float`, `string`, `bool`, `nil`. Every use requires a type assertion and causes scalar types (`int64`, `float64`, `bool`) to escape to the heap when boxed. Replace with a struct tag + union layout:

```go
type valueKind uint8
const (kindNil valueKind = iota; kindBool; kindInt64; kindFloat64; kindBigInt; kindBigFloat; kindString)

type Value struct {
    Type     TypeAnnotation
    kind     valueKind
    Flag     ValueFlag
    RawHashes int
    ival     int64        // also holds bool (0/1)
    fval     float64
    sval     string       // also used for big.Int/big.Float text repr
    bigval   interface{}  // only non-nil for big.Int / big.Float
}
```

Eliminates all type assertions in the hot rendering/access path and prevents scalar heap escapes.

### 6. Pre-size Properties map at construction

**File:** `document/properties_production.go`

`Properties.Alloc()` calls `make(Properties)` with no capacity hint, so the map starts at minimum size and grows through rehashing. Most KDL nodes have 0–8 properties. Pre-sizing to `make(Properties, 8)` would eliminate the first one or two rehashes. For deterministic mode, the `order []string` slice also benefits from a capacity hint.

This is now a stronger next candidate than parser dispatch, especially for property-heavy documents.

### 7. Pre-compute peeked size instead of summing in a loop

**File:** `internal/tokenizer/scanner.go`

`peekThreeSize()` iterates the `peeked []struct{c rune; size int}` slice summing sizes on every call. A `peekedSize int` field maintained by `peek()` and `consume()` makes this O(1).

---

## P3 — Moderate Impact, Low Risk

### 8. Avoid closure allocation per `readWhile()` call for character classification

**Files:** `internal/tokenizer/scanner.go`, `internal/tokenizer/ctype.go`

`readWhile(validRune func(rune) bool, ...)` is called with a freshly-created closure at each call site. For version-sensitive classification (e.g. `isBareIdentifierCharVersion`), that closure heap-escapes. Pre-computing one version-specific named function and storing it on the `Scanner` struct at construction time would reduce this to a single field read per `readWhile` call.

### 9. Reduce multiline string processing passes

**File:** `document/strings.go`

`dedentMultilineContent` calls `strings.Split()` (allocates string slice) then iterates twice — once to find minimum indentation, once to trim. Replacing with a single `bytes.IndexByte`-driven scan over the original `[]byte` (no split, no intermediate string slice) would eliminate two allocations and a full string copy.

### 10. ASCII fast-path in character classification

**File:** `internal/tokenizer/ctype.go`

`isWhiteSpace()` and `isNewline()` check Unicode ranges. For KDL documents that are ASCII or near-ASCII, adding `if c < 0x80 { return c == ' ' || c == '\t' }` before the Unicode range checks would short-circuit the common case.

### 11. Intern/pool common bare identifier strings

**File:** `document/value.go`

Every bare identifier (`string(t.Data)`) allocates a new string even for repeated values like `type`, `name`, `id`, `true`, `false`, `null`. A small bounded intern table (e.g. a `[64]struct{k, v string}` direct-mapped cache keyed by `len ^ first_byte ^ last_byte`) would eliminate these allocations with near-zero overhead.

---

## P4 — Speculative / Needs Profiling First

### 12. Bulk string scanning for quoted strings

**File:** `internal/tokenizer/scanner.go`

Quoted string reading advances one rune at a time scanning for `\` and `"`. For long strings without escapes, `bytes.IndexByte` to the next `\` or `"` could skip many characters at once. Only relevant for documents with long string values.

### 13. Fixed-capacity mark stack on Scanner struct

**File:** `internal/tokenizer/scanner.go`

`pushMark()` appends to `marks []int`. Replacing with a `[8]int` array + length counter on the struct would eliminate all slice growth allocations on the backtracking path. Overflow (rare) can still fall back to a slice.

### 14. Buffered writer for serialization output

**File:** `document/strings.go`

Serialization builds output via recursive `WriteTo([]byte)` calls using `append`. For large documents written to an `io.Writer`, wrapping with `bufio.Writer` would reduce system-call overhead. Low priority since serialization is typically not the bottleneck.

---

## Arena Allocation: Detailed Assessment

**Short answer: yes, this is one of the highest-value changes available.**

KDL parsing has textbook arena characteristics:
- All objects created in a single burst, freed together.
- Input size is known (or estimable from line count) before allocation starts.
- The returned `*Document` is the only long-lived owner.

**Sizing heuristic based on line count:**

If line count is available (e.g. from a `bufio.Scanner` pre-pass or estimated as `inputBytes/30`):
```
nodes  ≈ lines * 0.8      (most lines are nodes; comment/blank lines reduce this)
values ≈ lines * 1.5      (arguments + properties per node)
```
Alternatively, use raw byte count with `inputBytes / 25` for nodes.

**Accuracy matters less than you'd think**: if the arena is exhausted mid-parse, fall back to `new()`. A 2× overestimate wastes memory but doesn't affect correctness; a 2× underestimate still saves ~50% of allocations.

**What goes in the arena**: `Node` structs, `Value` structs. Slice backing arrays for `Arguments` and `Children` are small and fine on the regular heap. Strings from token data are already owned by the scanner buffer.

**Integration point**: `ParseContext` in `internal/parser/context.go` is the right place to add `arena *parseArena`. Pass input size from `Parse()`/`ParseBytes()` in `kdl.go` down to context construction.

---

## Benchmarking Notes

```
go test -bench=. -benchmem ./...
go tool pprof cpu.pprof   # existing profile in repo root
go tool pprof mem.pprof   # existing allocation profile in repo root
```

Examine the existing `cpu.pprof` and `mem.pprof` before writing new benchmarks — they may already show the dominant hotspots. Start with items 1–3 (switch dispatch, arena, parseNumber) as they are independent and measurable in isolation.

Current recommended order:

1. Tune arena sizing to reduce `B/op` while preserving the alloc/time win.
2. Pre-size property maps and deterministic property order slices.
3. Pre-compute scanner peeked size instead of summing on every deeper peek.
