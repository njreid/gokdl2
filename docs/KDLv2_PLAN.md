# KDLv2 Implementation Plan for kdl-go

> Companion to: `docs/KDLv2_DESIGN.md`
> Working directory: `/home/njr/code/kdl-go`

This is the step-by-step implementation plan. Each step lists the exact files and functions to change, what to change, and why. Steps within a phase are ordered by dependency; phases must be executed in order.

---

## Phase 1 — Version Infrastructure (zero behavior change)

Goal: add the `Version` type and wire it through without changing any parsing behavior. All existing tests must still pass after this phase.

### Step 1.1 — Add `Version` type to the tokenizer package

**File**: `internal/tokenizer/token.go` (append after constants)

```go
// Version identifies the KDL spec version to use for parsing/generating.
type Version int

const (
    VersionAuto Version = 0 // detect from marker; fall back v2→v1
    VersionV1   Version = 1
    VersionV2   Version = 2
)
```

Rationale: lives in `tokenizer` so both the scanner and the parser context can import it without a cycle. The public API aliases it.

### Step 1.2 — Add `Version` field to `Scanner`

**File**: `internal/tokenizer/scanner.go`

Add `Version Version` to the `Scanner` struct, after `ParseComments bool`:

```go
type Scanner struct {
    // ...existing fields...
    ParseComments       bool
    Version             Version   // NEW
    r                   io.Reader
}
```

No behavior change yet — field is read later in Phase 2.

### Step 1.3 — Add `Version` field to `ParseContextOptions`

**File**: `internal/parser/context.go`

```go
type ParseContextOptions struct {
    RelaxedNonCompliant relaxed.Flags
    Flags               ParseFlags
    Version             tokenizer.Version  // NEW
}
```

Wire it into `ParseContext.opts` (already stored) so it can be passed to the scanner.

**File**: `internal/parser/parser.go` (or wherever `parse()` creates the scanner)

In `kdl.go`, the `parse(s *tokenizer.Scanner)` function already receives a pre-configured scanner. The caller (`ParseWithOptions`) sets scanner fields from `ParseOptions`. Add:

```go
s.Version = opts.Version
```

### Step 1.4 — Add `Version` to public `ParseOptions` / `GenerateOptions`

**File**: `kdl.go`

`ParseOptions` is already a type alias for `parser.ParseContextOptions`, so the `Version` field is automatically exposed. No change needed here beyond verifying it compiles.

**File**: `internal/generator/generator.go`

Add `Version tokenizer.Version` to `Options`:

```go
type Options struct {
    Indent        string
    IgnoreFlags   bool
    AddSemicolons bool
    AddEquals     bool
    AddColons     bool
    Version       tokenizer.Version  // NEW: V1 (default=0 treated as V1) or V2
    EmitVersionMarker bool           // NEW: emit /- kdl-version N header
}
```

### Step 1.5 — Add `Version` field to `document.Document`

**File**: `document/document.go`

```go
type Document struct {
    Nodes   []*Node
    Version int  // 0=unknown, 1=v1, 2=v2; set by parser when detected
}
```

### Step 1.6 — Verify: run existing tests

```bash
cd /home/njr/code/kdl-go && go test ./...
```

All tests must pass. If anything breaks, fix before Phase 2.

---

## Phase 2 — v2 Tokenizer

Goal: the scanner correctly tokenizes KDLv2 syntax when `s.Version == VersionV2`, and correctly rejects v2 syntax in v1 mode and vice versa.

### Step 2.1 — Version-aware `isBareIdentifierChar` / `IsBareIdentifier`

**File**: `internal/tokenizer/ctype.go`

The current `isBareIdentifierChar` excludes `{`, `}`, `<`, `>`, `;`, `[`, `]`, `=`, `,` and (conditionally) `(`, `)`, `/`, `\`, `"`.

In KDLv2, `#` is additionally forbidden. The `relaxed.Flags` pattern already exists for conditional exclusions, but a `Version` parameter fits better here. Since `ctype.go` already imports `relaxed`, add a second parameter:

```go
// isBareIdentifierChar indicates whether c is a valid character for a bare identifier
// in the given KDL version.
func isBareIdentifierChar(c rune, r relaxed.Flags, v Version) bool {
    if isLineSpace(c) {
        return false
    }
    if c <= 0x20 || c > 0x10FFFF {
        return false
    }
    switch c {
    case '{', '}', '<', '>', ';', '[', ']', '=', ',':
        return false
    case '#':
        return v != VersionV2  // # forbidden in v2 identifiers
    case '(', ')', '/', '\\', '"':
        return r.Permit(relaxed.NGINXSyntax)
    case ':':
        return !r.Permit(relaxed.YAMLTOMLAssignments)
    default:
        return true
    }
}
```

Update all callers: `isBareIdentifierStartChar`, `IsBareIdentifier`, and the closures in `readBareIdentifier` and `readDecimal` to pass `s.Version`.

Also update `isBareIdentifierStartChar`:

```go
func isBareIdentifierStartChar(c rune, r relaxed.Flags, v Version) bool {
    if !isBareIdentifierChar(c, r, v) {
        return false
    }
    if isDigit(c) {
        return false
    }
    return true
}
```

Update the public `IsBareIdentifier(s string, rf relaxed.Flags)` — add `v Version` parameter (this is an exported function; adding a parameter is a **breaking API change**). Options:
- Add `IsBareIdentifierV2(s string, rf relaxed.Flags) bool` as a companion
- Or add a `Version` parameter and accept the break (this function is low-level; it's unlikely callers use it)

**Decision**: add `IsBareIdentifierVersion(s string, rf relaxed.Flags, v Version) bool` and keep `IsBareIdentifier` calling it with `VersionV1` for backward compat.

### Step 2.2 — Version-aware `readBareIdentifier`

**File**: `internal/tokenizer/readtype.go`

In `readBareIdentifier()`, the closure `isBareIdentifierCharClosure` needs to pass `s.Version`:

```go
isBareIdentifierCharClosure := func(c rune) bool {
    return isBareIdentifierChar(c, s.RelaxedNonCompliant, s.Version)
}
```

After reading the literal, add v2 keyword checks:

```go
if s.Version == VersionV2 {
    switch string(literal) {
    case "true", "false", "null", "inf", "nan":
        return Unknown, nil, fmt.Errorf(
            "%q is a reserved keyword in KDL v2; use #%s", string(literal), string(literal))
    case "-inf":
        return Unknown, nil, fmt.Errorf(
            "%q is a reserved keyword in KDL v2; use #-inf", string(literal))
    }
}
```

The existing `"true"/"false" → Boolean`, `"null" → Null` mapping stays for v1 mode (guarded by `s.Version != VersionV2`).

### Step 2.3 — Add `readHashKeyword()` for v2 `#true`, `#false`, `#null`, `#inf`, `#-inf`, `#nan`

**File**: `internal/tokenizer/readtype.go`

New function:

```go
// readHashKeyword reads a KDL v2 hash-prefixed keyword (#true, #false, #null,
// #inf, #-inf, #nan) from the current position. The '#' has already been peeked
// but not consumed.
func (s *Scanner) readHashKeyword() (TokenID, []byte, error) {
    s.pushMark()
    defer s.popMark()

    // consume '#'
    if _, err := s.get(); err != nil {
        return Unknown, nil, err
    }

    // read the rest as an identifier-like sequence
    kw, err := s.readWhile(func(c rune) bool {
        return c == '-' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
    }, 0)
    if err != nil && err != io.EOF {
        return Unknown, nil, err
    }

    lit := s.copyFromMark()
    switch string(kw) {
    case "true":
        return Boolean, lit, nil
    case "false":
        return Boolean, lit, nil
    case "null":
        return Null, lit, nil
    case "inf":
        return Decimal, lit, nil   // positive infinity
    case "-inf":
        return Decimal, lit, nil   // negative infinity
    case "nan":
        return Decimal, lit, nil   // NaN
    default:
        return Unknown, nil, fmt.Errorf("unknown KDL v2 keyword #%s", string(kw))
    }
}
```

Note: the full literal including `#` is captured via `s.copyFromMark()`. The token data for `#inf`/`#-inf`/`#nan` is the raw `[]byte` like `#inf`. Existing parse infrastructure in `document/value.go` and `document/strings.go` must be updated to recognize these literals.

**Token type consideration**: reuse `Boolean` for `#true`/`#false` and `Null` for `#null`. The parser transitions already handle `Boolean` and `Null` token IDs; as long as the literal bytes are passed correctly through to `document.Value`, the document layer can interpret them.

For `#inf`/`#-inf`/`#nan`: use a new `Keyword` token ID (add to `token.go`):

```go
Keyword  // KDL v2: #inf, #-inf, #nan
```

Add `Keyword` to `tokenClasses`:
```go
Keyword: {ClassValue, ClassNonStringValue},
```

### Step 2.4 — Add v2 raw string reading (`readRawStringV2`)

**File**: `internal/tokenizer/readtype.go`

The existing `readRawString()` expects `r` as first char. Add a new function for the v2 `#"..."#` syntax (where `#` has already been peeked):

```go
// readRawStringV2 reads a KDL v2 raw string (#"..."# or ##"..."##, etc.)
// The leading '#' characters have already been peeked but not consumed.
func (s *Scanner) readRawStringV2() ([]byte, error) {
    s.pushMark()
    defer s.popMark()

    // count leading '#' chars
    startHashes := 0
    for {
        c, err := s.peek()
        if err != nil {
            return nil, io.ErrUnexpectedEOF
        }
        if c == '#' {
            s.skip()
            startHashes++
        } else {
            break
        }
    }

    // expect opening '"' or '"""'
    c, err := s.get()
    if err != nil || c != '"' {
        return nil, fmt.Errorf("expected '\"' after '#' in raw string, got %c", c)
    }

    // check for multi-line raw string (""")
    p1, _ := s.peekAt(0)
    p2, _ := s.peekAt(1)
    if p1 == '"' && p2 == '"' {
        // raw multi-line: consume remaining "" and delegate
        s.skip(); s.skip()
        return s.readRawMultiLineStringBody(startHashes)
    }

    // single-line raw string: read until '"' + startHashes '#'
    endHashes := 0
    foundQuote := false
    for {
        c, err := s.get()
        if err != nil {
            return nil, io.ErrUnexpectedEOF
        }
        if isNewline(c) {
            return nil, fmt.Errorf("newline not allowed in single-line raw string")
        }
        if foundQuote {
            if c == '#' {
                endHashes++
                if endHashes == startHashes {
                    return s.copyFromMark(), nil
                }
            } else if c == '"' {
                endHashes = 0
                // foundQuote stays true
            } else {
                foundQuote = false
                endHashes = 0
            }
        } else if c == '"' {
            if startHashes == 0 {
                return s.copyFromMark(), nil
            }
            foundQuote = true
        }
    }
}
```

`readRawMultiLineStringBody(startHashes int)` reads until `\n` + optional whitespace + `"""` + `startHashes` `#`, applies dedentation, and returns the literal bytes.

### Step 2.5 — Version marker detection

**File**: `internal/tokenizer/scanner.go`

Add method `DetectVersionMarker() Version` that peeks at the start of input for the pattern:
```
/- <ws>* kdl-version <ws>+ (1|2) <ws>* <newline>
```

Called once from `New()` / `NewSlice()` after the scanner is initialized. If found, consumes the marker bytes and sets `s.Version`. If `s.Version` was already set explicitly (non-Auto), validates the marker matches.

```go
// detectVersionMarker checks for a KDL version marker at the start of input.
// If found, consumes it and sets s.Version (if VersionAuto) or validates it
// (if already set). Returns the detected version or VersionAuto if not found.
func (s *Scanner) detectVersionMarker() Version {
    // Must start with /-
    // peek ahead without consuming using s.raw
    // ...
}
```

Implementation note: peek using the raw buffer without advancing scanner state, then only consume if a complete valid marker is found.

### Step 2.6 — Update `readNext()` dispatch for v2

**File**: `internal/tokenizer/scanner.go`

In the big `switch c` in `readNext()`:

**`case '#'`** (currently handled only for NGINXSyntax):
```go
case '#':
    if s.Version == VersionV2 {
        // check for raw string (#"..."# or ##"..."##)
        _, c2, _ := s.peekTwo()
        if c2 == '"' || c2 == '#' {
            token.ID = RawString
            token.Data, err = s.readRawStringV2()
        } else {
            // must be a keyword: #true, #false, #null, #inf, #-inf, #nan
            token.ID, token.Data, err = s.readHashKeyword()
        }
    } else if s.RelaxedNonCompliant.Permit(relaxed.NGINXSyntax) {
        token.ID = SingleLineComment
        token.Data, err = s.readSingleLineComment()
    } else {
        // In v1 mode (non-NGINX), # falls through to identifier
        ignore = false
    }
```

**`case '"'`** — add multi-line detection in v2:
```go
case '"':
    if s.Version == VersionV2 {
        _, p1, _ := s.peekTwo()
        // peek one more: need 3-char lookahead
        p2 := s.peekAt(2) // need to add this helper or use marks
        if p1 == '"' && p2 == '"' {
            token.ID = QuotedString
            token.Data, err = s.readMultiLineString()
            break
        }
    }
    token.ID = QuotedString
    token.Data, err = s.readQuotedString()
```

**`case 'r'`** — skip raw string dispatch in v2:
In `readIdentifier()` (called from default path):
```go
case 'r':
    if s.Version != VersionV2 {
        _, c2, err := s.peekTwo()
        if err == nil && (c2 == '#' || c2 == '"') {
            literal, err := s.readRawString()
            return RawString, literal, err
        }
    }
    // fall through to bare identifier
    tokenType, literal, err := s.readBareIdentifier()
    return tokenType, literal, err
```

### Step 2.7 — Add `readMultiLineString()` for v2

**File**: `internal/tokenizer/readtype.go`

The opening `"""` has already been peeked (and we know it's v2). This function:
1. Consumes `"""`
2. Expects an immediate newline (error if not)
3. Reads lines until a line whose trimmed content is just `"""`
4. Returns raw bytes (the interpreter in `document/strings.go` will do escape processing + dedent)

**Alternatively**: do the full dedentation in the tokenizer and return cleaned bytes. This is simpler for the parser layer.

**Decision**: Return raw bytes including the delimiters. The `document/strings.go` `UnquoteString()` function already processes escape sequences for quoted strings; add a `UnquoteMultiLineString()` function there.

The token data for a multi-line string will look like:
```
"""\n    content line\n    """
```
— the full raw bytes. A `FlagMultiLine` value flag is set so the generator knows to use triple-quote syntax.

Add `FlagMultiLine ValueFlag` to `document/value.go`.

### Step 2.8 — Add `\s` escape and escaped-whitespace to `document/strings.go`

**File**: `document/strings.go`

Find the escape-processing function (likely `UnquoteString` or similar). Add:

```go
case 's':
    if version == 2 {
        b = append(b, ' ')
    } else {
        return nil, fmt.Errorf(`unknown escape \s`)
    }
```

For escaped whitespace (`\` followed by spaces/tabs/newlines), after reading `\`, if the next char is whitespace (not a letter/digit/`u`/{), consume all following whitespace chars and continue without adding anything to the output. This applies only in v2 mode.

The `UnquoteString` function currently takes `[]byte` (the raw token literal). Add a `version int` parameter, or pass a struct. Since this is an internal function, a `version int` parameter is fine.

### Step 2.9 — Add `UnquoteMultiLineString(data []byte, version int)` to `document/strings.go`

Algorithm:
1. Strip opening `"""` and mandatory newline
2. Find closing `"""` line; its leading whitespace is the base indent
3. For each content line: strip the base indent prefix, collect
4. Join with `\n`
5. Process escape sequences (same as `UnquoteString` but v2 mode)
6. Normalize `\r\n` and `\r` to `\n` for literal newlines

This function is called from `Value.SetToken()` or wherever quoted string tokens are turned into Go values.

### Step 2.10 — Update `Value` parsing to handle v2 keywords

**File**: `document/value.go` (or wherever `AddArgumentToken`/`SetNameToken` calls back to string parsing)

The `Boolean` token with literal `#true` (v2) must produce `true` (bool). The `Null` token with literal `#null` must produce `nil`. The `Keyword` token with `#inf`/`#-inf`/`#nan` must produce `math.Inf(1)`, `math.Inf(-1)`, `math.NaN()`.

Find where `tokenizer.Boolean` is handled (likely in `document/node.go`'s `AddArgumentToken` or `document/value.go`'s `SetFromToken`). Add v2 literal recognition:

```go
case tokenizer.Boolean:
    if bytes.Equal(data, []byte("true")) || bytes.Equal(data, []byte("#true")) {
        v.Value = true
    } else {
        v.Value = false
    }
case tokenizer.Null:
    v.Value = nil  // same for "null" and "#null"
case tokenizer.Keyword:
    switch string(data) {
    case "#inf":
        v.Value = math.Inf(1)
    case "#-inf":
        v.Value = math.Inf(-1)
    case "#nan":
        v.Value = math.NaN()
    }
```

### Step 2.11 — Run tokenizer tests

```bash
cd /home/njr/code/kdl-go && go test ./internal/tokenizer/... -v
```

Then run all tests:
```bash
go test ./...
```

---

## Phase 3 — Parser + Auto-detect

### Step 3.1 — Wire `Version` from `ParseContextOptions` into scanner

**File**: `kdl.go` — `ParseWithOptions`:

```go
func ParseWithOptions(r io.Reader, opts ParseOptions) (*document.Document, error) {
    s := tokenizer.New(r)
    s.RelaxedNonCompliant = opts.RelaxedNonCompliant
    s.ParseComments = opts.Flags.Has(parser.ParseComments)
    s.Version = opts.Version   // NEW
    return parse(s)
}
```

Similarly for `UnmarshalWithOptions` etc.

### Step 3.2 — Version marker detection in scanner initialization

In `tokenizer.New()` and `tokenizer.NewSlice()`, after creating the scanner, call `s.detectVersionMarker()` if `s.Version == VersionAuto`.

When auto-detecting: if the marker says version 2, set `s.Version = VersionV2`. The parser then sets `doc.Version = 2`.

When `Version` was explicitly set by caller: if a marker is found and disagrees, return an error.

### Step 3.3 — Auto-detect fallback in `parse()`

**File**: `kdl.go`

```go
func parse(s *tokenizer.Scanner) (*document.Document, error) {
    if s.Version != tokenizer.VersionAuto {
        return parseOnce(s)
    }
    return parseAuto(s)
}

func parseOnce(s *tokenizer.Scanner) (*document.Document, error) {
    defer s.Close()
    p := parser.New()
    opts := parser.ParseContextOptions{
        RelaxedNonCompliant: s.RelaxedNonCompliant,
        Version: s.Version,
    }
    if s.ParseComments {
        opts.Flags |= parser.ParseComments
    }
    c := p.NewContextOptions(opts)
    for s.Scan() {
        if err := p.Parse(c, s.Token()); err != nil {
            return nil, err
        }
    }
    if s.Err() != nil {
        return nil, s.Err()
    }
    return c.Document(), nil
}

func parseAuto(s *tokenizer.Scanner) (*document.Document, error) {
    // Version marker already consumed by scanner init; if it set a version, use it
    if s.Version != tokenizer.VersionAuto {
        return parseOnce(s)
    }

    // Try v2 first (stricter)
    raw := s.Raw() // get the raw bytes for retry
    s2 := tokenizer.NewSlice(raw)
    s2.RelaxedNonCompliant = s.RelaxedNonCompliant
    s2.ParseComments = s.ParseComments
    s2.Version = tokenizer.VersionV2

    doc, err := parseOnce(s2)
    if err == nil {
        doc.Version = 2
        return doc, nil
    }
    if !isVersionAmbiguousError(err) {
        return nil, err
    }

    // Fallback to v1
    s1 := tokenizer.NewSlice(raw)
    s1.RelaxedNonCompliant = s.RelaxedNonCompliant
    s1.ParseComments = s.ParseComments
    s1.Version = tokenizer.VersionV1
    doc, err = parseOnce(s1)
    if err != nil {
        return nil, err
    }
    doc.Version = 1
    return doc, nil
}
```

`isVersionAmbiguousError(err error) bool` returns true for errors that could indicate a v1 document being parsed as v2 (bare keyword, `r"..."` syntax, etc.). This is implemented by wrapping tokenizer errors in a typed error:

```go
type VersionSyntaxError struct {
    // Encountered is the version whose syntax was detected
    Encountered tokenizer.Version
    Err         error
}
```

The tokenizer emits this when it sees a version-specific token in the wrong mode.

**Expose `Raw()`** on `Scanner`:
```go
func (s *Scanner) Raw() []byte { return s.raw }
```

(The `raw` field already exists and holds the full input buffer.)

### Step 3.4 — Set `doc.Version` in the parser

After successful parse, the document's `Version` field is set. The `ParseContext.Document()` method in `context.go` can be modified to set `Version` from `opts.Version`.

### Step 3.5 — Run parser tests

```bash
go test ./internal/parser/... -v
go test ./...
```

---

## Phase 4 — Generator

### Step 4.1 — Version-aware keyword output

**File**: `internal/generator/generator.go`

Find where `Value.Value` is rendered. Currently, `nil` → `"null"`, `bool` → `"true"`/`"false"`.

```go
func (g *Generator) renderValue(v *document.Value) string {
    if g.options.Version == tokenizer.VersionV2 {
        return g.renderValueV2(v)
    }
    return g.renderValueV1(v)
}
```

`renderValueV2`:
- `nil` → `#null`
- `true` → `#true`
- `false` → `#false`
- `math.IsInf(f, 1)` → `#inf`
- `math.IsInf(f, -1)` → `#-inf`
- `math.IsNaN(f)` → `#nan`
- strings: see Step 4.2

### Step 4.2 — Version-aware raw string output

**File**: `internal/generator/generator.go`

When outputting a value with `FlagRaw`:
- v1: `r"..."` or `r#"..."#`
- v2: `#"..."#` or `##"..."##`

```go
func (g *Generator) renderRawString(s string) string {
    hashes := strings.Repeat("#", requiredHashCount(s))
    if g.options.Version == tokenizer.VersionV2 {
        return hashes + `"` + s + `"` + hashes
    }
    return "r" + hashes + `"` + s + `"` + hashes
}
```

`requiredHashCount(s string) int` returns the minimum number of `#` needed: count the maximum run of `#` chars that appear after `"` in `s`, add 1.

### Step 4.3 — Multi-line string output (v2 only)

**File**: `internal/generator/generator.go`

When `v.Flag == FlagMultiLine` (v2 only) or when the generator decides a string with embedded newlines should use multi-line syntax:

```go
func (g *Generator) renderMultiLineString(s string, indent string) string {
    var b strings.Builder
    b.WriteString(`"""`)
    b.WriteString("\n")
    for _, line := range strings.Split(s, "\n") {
        b.WriteString(indent)
        b.WriteString(line)
        b.WriteString("\n")
    }
    b.WriteString(indent)
    b.WriteString(`"""`)
    return b.String()
}
```

The `indent` is the current node's indentation level.

### Step 4.4 — Version marker emission

**File**: `internal/generator/generator.go`

```go
func (g *Generator) Generate(doc *document.Document) error {
    if g.options.EmitVersionMarker && g.options.Version != tokenizer.VersionAuto {
        v := 1
        if g.options.Version == tokenizer.VersionV2 {
            v = 2
        }
        if _, err := fmt.Fprintf(g.w, "/- kdl-version %d\n", v); err != nil {
            return err
        }
    }
    // ... existing node generation
}
```

### Step 4.5 — Run generator tests

```bash
go test ./internal/generator/... -v
go test ./...
```

---

## Phase 5 — Public API & Tests

### Step 5.1 — Convenience functions in `kdl.go`

```go
// ParseV2 parses a KDL v2 document.
func ParseV2(r io.Reader) (*document.Document, error) {
    return ParseWithOptions(r, ParseOptions{Version: tokenizer.VersionV2})
}

// ParseAutoVersion parses a KDL document, auto-detecting v1 vs v2.
func ParseAutoVersion(r io.Reader) (*document.Document, error) {
    return ParseWithOptions(r, ParseOptions{Version: tokenizer.VersionAuto})
}

// GenerateV2 writes a KDL v2 document.
func GenerateV2(doc *document.Document, w io.Writer) error {
    return GenerateWithOptions(doc, w, GenerateOptions{
        Indent:   "\t",
        Version:  tokenizer.VersionV2,
    })
}
```

Also export the `Version` type from the top-level package:

```go
// Re-export version constants for callers who don't import the tokenizer package.
const (
    VersionAuto = tokenizer.VersionAuto
    VersionV1   = tokenizer.VersionV1
    VersionV2   = tokenizer.VersionV2
)
```

### Step 5.2 — Official KDL v2 test suite

**File**: `internal/parser/parser_test.go` (or a new `parser_v2_test.go`)

Mirror the existing `TestKDLOrgTestCases` which clones `release/v1`. Add:

```go
// TestKDLOrgTestCasesV2 runs all official KDL v2 test cases.
// To run: git clone --branch release/v2 https://github.com/kdl-org/kdl internal/parser/kdl-org-v2
func TestKDLOrgTestCasesV2(t *testing.T) {
    // same structure as TestKDLOrgTestCases, but with Version: VersionV2
    // test cases directory: internal/parser/kdl-org-v2/tests/test_cases
}
```

Run after cloning:
```bash
cd /home/njr/code/kdl-go
git clone --branch release/v2 https://github.com/kdl-org/kdl internal/parser/kdl-org-v2
go test ./internal/parser/... -v -run TestKDLOrgTestCasesV2
```

### Step 5.3 — Unit tests: tokenizer v2

**File**: `internal/tokenizer/scanner_v2_test.go`

Table-driven tests (see `docs/KDLv2_DESIGN.md §10.2` for full case list).

Key groups:
- `#true`/`#false`/`#null`/`#inf`/`#-inf`/`#nan` in v2 mode → correct token IDs
- bare `true`/`false`/`null` in v2 mode → error
- `r"..."` in v2 mode → parses as two tokens (identifier + string)
- `#"..."#` in v2 mode → RawString token
- `"""..."""` in v2 mode → QuotedString (multi-line) token
- `\s` escape in v2 → space; in v1 → error
- escaped whitespace `\ ` in v2 → discard; in v1 → error

### Step 5.4 — Unit tests: parser v2

**File**: `internal/parser/parser_v2_test.go`

Key groups:
- Version marker recognized and sets `doc.Version`
- `#true`/`#false`/`#null` as arguments and property values
- Multi-line strings as arguments
- Type annotations on node names
- `/-` slashdash still works
- Error cases: bare `true` in v2 mode, `r"..."` in v2 mode, `#true` in v1 mode

### Step 5.5 — Unit tests: generator v2

**File**: `internal/generator/generator_v2_test.go` (or in existing test file)

Key groups:
- Bool/null → `#true`/`#false`/`#null` in v2 mode
- Bool/null → `true`/`false`/`null` in v1 mode (unchanged)
- Raw string → `#"..."#` in v2
- Raw string → `r"..."` in v1
- Multi-line string in v2
- Version marker emission

### Step 5.6 — Unit tests: auto-detection

**File**: `kdl_v2_test.go`

Key groups: (see `docs/KDLv2_DESIGN.md §10.5`)

### Step 5.7 — Round-trip tests

**File**: `kdl_v2_test.go`

```go
func TestRoundTripV2(t *testing.T) {
    inputs := []string{
        `node #true`,
        `node #false key=#null`,
        `node #"raw string"#`,
        `(mytype)node 42`,
    }
    for _, input := range inputs {
        doc, err := kdl.ParseV2(strings.NewReader(input))
        require.NoError(t, err)
        var buf bytes.Buffer
        require.NoError(t, kdl.GenerateV2(doc, &buf))
        doc2, err := kdl.ParseV2(strings.NewReader(buf.String()))
        require.NoError(t, err)
        // compare doc and doc2 structurally
    }
}
```

---

## Dependency Graph

```
Phase 1 (infrastructure)
  └── Phase 2 (tokenizer)
        ├── Step 2.1 ctype.go     — isBareIdentifierChar version-aware
        ├── Step 2.2 readtype.go  — readBareIdentifier v2 checks
        ├── Step 2.3 readtype.go  — readHashKeyword
        ├── Step 2.4 readtype.go  — readRawStringV2
        ├── Step 2.5 scanner.go   — detectVersionMarker
        ├── Step 2.6 scanner.go   — readNext dispatch
        ├── Step 2.7 readtype.go  — readMultiLineString
        ├── Step 2.8 strings.go   — \s escape, escaped-whitespace
        ├── Step 2.9 strings.go   — UnquoteMultiLineString
        └── Step 2.10 value.go    — #true/#false/#null/#inf token handling
              └── Phase 3 (parser + auto-detect)
                    ├── Step 3.1  — wire Version into scanner
                    ├── Step 3.2  — version marker in scanner init
                    ├── Step 3.3  — parseAuto fallback
                    └── Step 3.4  — doc.Version set
                          └── Phase 4 (generator)
                                ├── Step 4.1 — keyword output
                                ├── Step 4.2 — raw string output
                                ├── Step 4.3 — multi-line output
                                └── Step 4.4 — version marker emission
                                      └── Phase 5 (API + tests)
```

---

## File Change Summary

| File | Phase | Change |
|------|-------|--------|
| `internal/tokenizer/token.go` | 1 | Add `Version` type + constants, add `Keyword` TokenID |
| `internal/tokenizer/scanner.go` | 1, 2, 3 | Add `Version` field; `detectVersionMarker`; `readNext` dispatch; `Raw()` |
| `internal/tokenizer/ctype.go` | 2 | Add `Version` param to `isBareIdentifierChar`/`isBareIdentifierStartChar`/`IsBareIdentifier` |
| `internal/tokenizer/readtype.go` | 2 | `readBareIdentifier` v2 checks; add `readHashKeyword`, `readRawStringV2`, `readMultiLineString`, `readRawMultiLineStringBody` |
| `document/strings.go` | 2 | Add `\s` escape; escaped-whitespace; `UnquoteMultiLineString` |
| `document/value.go` | 2 | Add `FlagMultiLine`; handle `#true`/`#false`/`#null`/`Keyword` tokens |
| `document/document.go` | 1 | Add `Version int` field |
| `internal/parser/context.go` | 1, 3 | Add `Version` to `ParseContextOptions`; set `doc.Version` |
| `internal/parser/transitions.go` | 3 | (minimal) Ensure `Keyword` token in `ClassValue` is handled |
| `internal/generator/generator.go` | 1, 4 | Add `Version`/`EmitVersionMarker` to `Options`; v2 output functions |
| `kdl.go` | 1, 3, 5 | Wire `Version` into `ParseWithOptions`; add `ParseV2`/`ParseAutoVersion`/`GenerateV2`; export version constants |
| `internal/tokenizer/scanner_v2_test.go` | 5 | New: tokenizer unit tests |
| `internal/parser/parser_v2_test.go` | 5 | New: parser unit tests |
| `internal/generator/generator_v2_test.go` | 5 | New: generator unit tests |
| `kdl_v2_test.go` | 5 | New: auto-detection + round-trip tests |

---

## Key Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| `isBareIdentifierChar` signature change breaks callers | Keep old signature calling new one with `VersionV1`; add `IsBareIdentifierVersion` |
| Auto-detect try-v2-first is slow for large v1 documents | Both parses use `NewSlice` on same `[]byte`; second parse is rare in practice |
| `document/strings.go` escape processor needs version context | Pass `version int` parameter; callers already know the version |
| `readQuotedStringQ` uses raw closure, no escape processing | Escape processing is deferred to `document/strings.go` — correct approach |
| Multi-line string dedent + escape interaction (spec §3.12.3) | Process whitespace escapes first, then dedent |
| `#` in v1 identifiers (`C#` node name) breaks in auto-detect | v2 parse fails, fallback to v1 handles it |
