# KDLv2 Support Design for kdl-go

> Status: Draft
> KDL v2 spec: https://kdl.dev/spec/ (released 2024-12-21, v2.0.0)
> Reference implementation: https://github.com/kdl-org/kdl-rs

---

## Table of Contents

1. [Overview and Goals](#1-overview-and-goals)
2. [KDLv1 → KDLv2 Breaking Changes](#2-kdlv1--kdlv2-breaking-changes)
3. [Compatibility Strategy](#3-compatibility-strategy)
4. [Architecture Changes](#4-architecture-changes)
5. [Tokenizer Changes](#5-tokenizer-changes)
6. [Parser Changes](#6-parser-changes)
7. [Generator Changes](#7-generator-changes)
8. [Document Model Changes](#8-document-model-changes)
9. [Public API Changes](#9-public-api-changes)
10. [Test Plan](#10-test-plan)
11. [Edge Cases and Gotchas](#11-edge-cases-and-gotchas)
12. [Implementation Phases](#12-implementation-phases)

---

## 1. Overview and Goals

### What We're Adding

KDLv2 (released December 2024) introduced meaningful syntax changes over v1. This document describes how to add v2 support to kdl-go while keeping full backward compatibility with v1.

### Goals

- Parse KDLv2 documents correctly
- Generate KDLv2 documents (opt-in)
- Auto-detect the version from the version marker when present
- Fall back gracefully: try v2, then v1 (or vice versa) when no marker is present
- Keep the existing v1 public API working without changes
- Maintain the single `document.Document` / `document.Node` / `document.Value` model — the data model is version-agnostic

### Non-Goals

- Translating v1 documents to v2 or vice versa (though the generator can target either)
- Supporting every possible edge of the compatibility guarantee as anything other than "best effort"

---

## 2. KDLv1 → KDLv2 Breaking Changes

This section is the authoritative diff between the two versions as it applies to kdl-go.

### 2.1 Keywords: `#true`, `#false`, `#null`, `#inf`, `#-inf`, `#nan`

**The single largest breaking change.**

| v1 | v2 |
|----|----|
| `true` | `#true` |
| `false` | `#false` |
| `null` | `#null` |
| `inf` | `#inf` |
| `-inf` | `#-inf` |
| `nan` | `#nan` |

In v1, `true`/`false`/`null` were reserved words that were not valid identifiers. In v2, they have the `#` prefix and are a distinct token class (`keyword`). The bare words `true`, `false`, `null`, `inf`, `nan` are **disallowed as identifiers** in v2 (`disallowed-keyword-identifiers`).

**Implication for parser compatibility**: a document containing bare `true` will be accepted as a boolean by a v1 parser but rejected by a v2 parser, and vice versa for `#true`. This is the primary mechanism that makes version detection unambiguous.

**Grammar (v2)**:
```
keyword        := boolean | '#null'
keyword-number := '#inf' | '#-inf' | '#nan'
boolean        := '#true' | '#false'
```

### 2.2 Raw String Syntax Change

**v1**: `r"..."` and `r#"..."#`, `r##"..."##`, etc. (Rust-inspired, `r` prefix)
**v2**: `#"..."#`, `##"..."##`, etc. (no `r`, just leading `#` characters)

The number of `#` on both sides must match. The string ends at the first `"` followed by a matching count of `#`.

```
// v1
r"no escapes \n here"
r##"can contain "# inside"##

// v2
#"no escapes \n here"#
##"can contain "# inside"##
```

The `r` identifier prefix is no longer special in v2 — `r` followed by `"` would just be an identifier `r` followed by a string (parse error in node context, since there's no `=`).

### 2.3 Multi-line Strings (New in v2)

v2 introduces triple-quoted strings that span multiple lines and automatically dedent based on the indentation of the closing `"""`.

```kdl
// v2 only
node """
    line one
    line two
    """
```

The value is `"line one\nline two"` — the indentation of the closing `"""` sets the base indent that is stripped from all lines.

Rules:
- Opening `"""` must be followed immediately by a newline
- Each content line has the base indent stripped (determined by the closing `"""` line's indentation)
- Content lines may have less indentation than the base — they keep whatever indentation they have relative to base
- Literal newlines in the content are normalized to `\n` (LF)
- Escaped whitespace (`\` + whitespace) still collapses to nothing
- Multi-line raw strings: `#"""..."""#`

**Indentation stripping algorithm**:
1. The closing `"""` line's leading whitespace is the "base indent"
2. Each content line: if it starts with the base indent, strip it; if it has less, keep what's there; if it's empty/all-whitespace, it becomes an empty line

### 2.4 New String Escape: `\s`

v2 adds `\s` as an escape for a literal space character (U+0020). This is useful in multi-line strings to preserve trailing spaces.

| Escape | v1 | v2 |
|--------|----|----|
| `\s` | invalid | U+0020 (space) |

Full v2 escape table (additions bolded):
| Escape | Value |
|--------|-------|
| `\n` | LF |
| `\r` | CR |
| `\t` | Tab |
| `\\` | Backslash |
| `\"` | Double quote |
| `\b` | Backspace |
| `\f` | Form feed |
| **`\s`** | **Space (U+0020)** |
| `\u{HHHHHH}` | Unicode scalar value |

### 2.5 Escaped Whitespace in Strings (New in v2)

In v2, a `\` followed by one or more literal whitespace characters (including newlines) discards the backslash and all the whitespace. This allows line continuation inside strings without including the newline in the value.

```kdl
// v2: these are identical
node "Hello World"
node "Hello \    World"
node "Hello \
      World"
```

This is distinct from `escline` (backslash outside a string). In v2, this works **inside** quoted strings too.

**Tokenizer impact**: when scanning a quoted string body in v2 mode, a `\` followed by whitespace (space, tab, any newline) must consume and discard all following whitespace.

### 2.6 Quoted Strings Cannot Contain Literal Newlines

In v2, literal newlines are **not** allowed inside single-line quoted strings. In v1, they were also not allowed (this is the same), but v2 enforces this more explicitly: use `\n` escape or switch to multi-line `"""` syntax.

### 2.7 Identifier String Changes

The `#` character is now **non-identifier** in v2 (it was allowed in v1). This is necessary because `#true`, `#false`, etc. need `#` to be a non-identifier character.

```
// v1: identifier-char excludes: \()[]{}/;"#= and whitespace
// v2: identifier-char excludes: \\/(){}[];\"#= and whitespace
```

Note v2 also adds `[` and `]` to the exclusion list.

The `disallowed-keyword-identifiers` list in v2:
```
'true' | 'false' | 'null' | 'inf' | '-inf' | 'nan'
```
These strings cannot be used as bare identifiers even if they otherwise pass the character rules.

### 2.8 Version Marker

v2 introduces an optional version marker that must appear at the very start of the document (after optional BOM):

```
/- kdl-version 2
/- kdl-version 1
```

Grammar:
```
version := '/-' unicode-space* 'kdl-version' unicode-space+ ('1' | '2') unicode-space* newline
```

Note this **looks like** a slashdash comment from a v1 parser's perspective. A v1 parser would see `/- kdl-version 2` as a slashdash-commented node named `kdl-version` with argument `2`, and skip it. This is the key to backward compatibility.

### 2.9 Type Annotation on Node Name (v2 clarification)

The grammar makes explicit that type annotations can appear before the node name itself:

```kdl
(mytype)node-name arg1 prop=val
```

This was permitted in v1 too, but the v2 grammar is explicit. No tokenizer change required, but the parser should ensure this path is exercised.

### 2.10 `[` and `]` Are Non-Identifier in v2

The `[` and `]` characters were allowed in identifiers in v1 but are forbidden in v2. This is a minor change — they were unusual to use but technically valid.

---

## 3. Compatibility Strategy

### 3.1 The Spec Guarantee

The v2 spec guarantees: for any document, a v1 and v2 parser will either **both fail** or **produce identical results**. This means there is no ambiguity — a document is unambiguously v1, v2, or both.

The key distinguishing features:
- `#true` / `#false` / `#null` → v2 only (v1 parser sees `#` as unknown char and may error)
- `true` / `false` / `null` (bare) → valid in v1, **rejected** in v2
- `r"..."` raw strings → v1 only
- `#"..."#` raw strings → v2 only
- `"""..."""` multi-line → v2 only
- Version marker → hint (optional)

### 3.2 Recommended Parse Strategy

```
1. Scan for version marker at start of document
2. If version marker found:
   - Use that version's parser exclusively
   - Error if document uses syntax from the other version
3. If no version marker:
   a. Try v2 parser first (stricter, rejects more)
   b. If v2 parse fails, try v1 parser
   c. If both fail, return the v2 error (or make it configurable)
```

Alternative: expose `ParseOptions.Version` as `V1`, `V2`, or `Auto` (default `Auto`).

### 3.3 "Auto" Mode Heuristics

The fallback strategy is safe because of the spec guarantee. In `Auto` mode:
- Detect version marker → use that version
- No marker → attempt v2 first (stricter subset accepts more valid docs), fallback to v1

The `Auto` fallback should be cheap: the first parse attempt that fails on a keyword token mismatch (e.g., bare `true` in v2 mode) is an O(1) detection that we need v1 mode.

---

## 4. Architecture Changes

### 4.1 Version Type

Add a `Version` type:

```go
// internal/tokenizer/version.go (or document/version.go)
type Version int

const (
    VersionAuto Version = 0  // detect from marker, fallback
    VersionV1   Version = 1
    VersionV2   Version = 2
)
```

### 4.2 Scanner Version Field

Add `Version Version` to `tokenizer.Scanner`. The scanner uses this to:
- Switch raw-string token recognition (`r"..."` vs `#"..."#`)
- Switch keyword recognition (`true` vs `#true`)
- Allow/disallow `#` in identifiers
- Allow/disallow `[`, `]` in identifiers
- Handle `\s` and escaped-whitespace in string scanning
- Handle `"""` multi-line strings

### 4.3 Parser Version Field

Add `Version Version` to `parser.ParseContextOptions`. The parser:
- Passes version to scanner
- In `Auto` mode: tries v2, catches version-specific errors, retries with v1

### 4.4 Generator Version Field

Add `Version Version` to `generator.Options`. The generator uses this to:
- Emit `#true`/`#false`/`#null` (v2) vs `true`/`false`/`null` (v1)
- Emit `#"..."#` (v2) vs `r"..."` (v1) for raw strings
- Emit `"""..."""` for long/multiline strings (v2 only)
- Optionally emit the version marker

### 4.5 No Document Model Changes Needed

`document.Document`, `document.Node`, and `document.Value` require **no changes**. The version difference is syntactic; the semantic model is identical. The `Version` field on `Document` is optional metadata.

Add an optional `Version` field to `document.Document` to record which version was detected:

```go
type Document struct {
    Nodes    []*Node
    Version  int  // 0=unknown, 1=v1, 2=v2
}
```

---

## 5. Tokenizer Changes

The tokenizer (`internal/tokenizer/scanner.go`) is the primary change surface.

### 5.1 Version-Switched Token Scanning

#### 5.1.1 Keywords

Current v1 code recognizes `true`, `false`, `null` as `TypeBool`/`TypeNull`. In v2 mode, these become `TypeIdentifier` candidates that then fail validation (since they're in `disallowed-keyword-identifiers`).

```go
// pseudocode in readNext() after reading an identifier-like token:
if s.Version == VersionV2 {
    if tok == "true" || tok == "false" || tok == "null" ||
       tok == "inf" || tok == "nan" {
        return s.errorf("bare %q is not valid in KDL v2; use #%s", tok, tok)
    }
}
```

In v2 mode, `#true`, `#false`, `#null`, `#inf`, `#-inf`, `#nan` must be recognized as their respective token types when `#` is the current character:

```go
case '#':
    if s.Version == VersionV2 {
        return s.readHashKeyword()
    }
    // v1: # is valid in identifiers, fall through to identifier reading
```

`readHashKeyword()` peeks at what follows `#` and dispatches to bool/null/inf/nan token types, or errors if unrecognized.

#### 5.1.2 Raw Strings

```go
// v1 mode: r" starts a raw string
case 'r':
    if s.Version != VersionV2 {
        next, _ := s.peekAt(1)
        if next == '"' || next == '#' {
            return s.readRawString()  // r"..." or r#"..."#
        }
    }
    // fall through to identifier

// v2 mode: # starts a raw string (when followed by " or #)
case '#':
    if s.Version == VersionV2 {
        next, _ := s.peekAt(1)
        if next == '"' || next == '#' {
            return s.readRawStringV2()  // #"..."# or ##"..."##
        }
        return s.readHashKeyword()
    }
```

`readRawStringV2()`: count leading `#` characters, expect `"`, read until `"` followed by same count of `#`.

#### 5.1.3 Multi-line Strings

When scanning a `"` in v2 mode, peek ahead to detect `"""`:

```go
case '"':
    if s.Version == VersionV2 {
        p1, _ := s.peekAt(1)
        p2, _ := s.peekAt(2)
        if p1 == '"' && p2 == '"' {
            return s.readMultiLineString()
        }
    }
    return s.readQuotedString()
```

`readMultiLineString()`:
1. Consume `"""`
2. Expect a newline immediately after
3. Accumulate lines until a line whose non-whitespace content is just `"""`
4. The leading whitespace of the closing `"""` line = base indent
5. Strip base indent from each content line
6. Normalize `\r\n` and `\r` to `\n`
7. Apply escape sequences (except in raw variant)
8. Return as `TypeString`

#### 5.1.4 `\s` Escape and Escaped Whitespace in Strings

In `readStringEscape()`, add:

```go
case 's':
    if s.Version == VersionV2 {
        return ' ', nil
    }
    return 0, s.errorf(`unknown escape \s in KDL v1`)

// Escaped whitespace (v2 only): \ followed by spaces/tabs/newlines
// Consume all following whitespace and discard
```

The escaped-whitespace rule is tricky because the `\` is followed not by a single character but by run of whitespace. In `readStringBody()`, after reading a `\`:

```go
if s.Version == VersionV2 {
    if isWhitespace(nextChar) || isNewline(nextChar) {
        // consume all following whitespace
        for isWhitespace(peek()) || isNewline(peek()) {
            s.skip()
        }
        continue // discard, produce no character
    }
}
```

### 5.2 Identifier Character Set Changes

In `ctype.go`, the `isIdentChar()` function needs version awareness (or a separate v2 variant):

```go
// v1 excludes: \()[]{}/;"= and whitespace
// v2 also excludes: # and [, ]
// (v1 already excluded [] in practice but grammar wasn't explicit)

func isIdentCharV2(r rune) bool {
    switch r {
    case '\\', '/', '(', ')', '{', '}', '[', ']', ';', '"', '#', '=':
        return false
    }
    return !isWhitespace(r) && !isNewline(r) && !isDisallowedCodePoint(r)
}
```

The scanner should call the appropriate function based on `s.Version`.

### 5.3 Version Marker Detection

Before the main scan loop, add a peek for the version marker:

```go
func (s *Scanner) detectVersion() {
    // peek for: /- kdl-version N\n
    // If found, set s.Version and consume the marker
    // If not found, leave s.Version as VersionAuto
}
```

The version marker grammar: `'/-' unicode-space* 'kdl-version' unicode-space+ ('1'|'2') unicode-space* newline`

This is called once at scanner init (or on first `Scan()`). Since a v1 parser would treat this as a slashdash comment on a node named `kdl-version`, and we call `detectVersion()` before the parser starts, this is transparent to the rest of the scanner/parser pipeline.

### 5.4 Token Type Additions

Add new token types to `token.go`:

```go
const (
    // existing...
    TypeBoolTrue   // existing, used for true/false
    TypeBoolFalse  // or consolidate into TypeBool
    TypeNull        // existing

    // new for v2:
    TypeHashBoolTrue   // #true
    TypeHashBoolFalse  // #false
    TypeHashNull       // #null
    TypeHashInf        // #inf
    TypeHashNegInf     // #-inf
    TypeHashNaN        // #nan
)
```

Alternatively, reuse existing token types and handle the `#` disambiguation at the scanner level, emitting the same `TypeBool`, `TypeNull`, etc. tokens regardless of version. This is cleaner since the document model doesn't change. **Recommended**: scanner emits the same token types for both versions; only the scanner's _input recognition_ differs.

---

## 6. Parser Changes

The parser (`internal/parser/`) is relatively insulated from version changes since the token types are the same. The main changes:

### 6.1 Version Propagation

`ParseContextOptions` gets a `Version` field. The parser passes this to the tokenizer. In `Auto` mode, the parser implements the try-v2/fallback-v1 logic.

```go
type ParseContextOptions struct {
    RelaxedNonCompliant relaxed.Flags
    Flags               ParseFlags
    Version             Version  // NEW: V1, V2, or Auto (default)
}
```

### 6.2 Auto-Detection Logic

In `kdl.go`'s `parse()` function (or a new `parseAuto()`):

```go
func parseWithAutoVersion(s *tokenizer.Scanner) (*document.Document, error) {
    // Step 1: check for version marker
    if v := s.DetectedVersion(); v != VersionAuto {
        s.Version = v
        return parse(s)
    }

    // Step 2: try v2 first
    s.Version = VersionV2
    doc, err := parse(s)
    if err == nil {
        return doc, nil
    }
    if !isVersionMismatchError(err) {
        return nil, err  // real error, not a v1/v2 syntax mismatch
    }

    // Step 3: fallback to v1
    s.Reset()
    s.Version = VersionV1
    return parse(s)
}
```

`isVersionMismatchError()` checks whether the error is the kind that could indicate a v1 document being parsed as v2 (e.g., bare `true`/`false`/`null` encountered, or `r"..."` raw string syntax).

**Important**: The scanner must be resettable — it needs to replay from the beginning. This already exists via `tokenizer.NewSlice()` for byte slices. For `io.Reader` inputs in auto mode, the input needs buffering (read all bytes first, then NewSlice). This is already what most callers do.

### 6.3 Version Mismatch Errors

Define a sentinel error type:

```go
type VersionMismatchError struct {
    Detected Version  // the version the input appears to be
    Parsed   Version  // the version we tried to parse as
    Wrapped  error
}
```

The tokenizer emits `VersionMismatchError` when it sees version-specific syntax in the wrong mode (e.g., `#true` in v1 mode, bare `true` as a value in v2 mode).

---

## 7. Generator Changes

The generator (`internal/generator/generator.go`) needs version-aware output.

### 7.1 Generator Options

```go
type Options struct {
    // existing...
    Indent         string

    // new:
    Version        Version  // V1 (default) or V2
    EmitVersionMarker bool  // emit /- kdl-version N at top
}
```

### 7.2 Boolean/Null/Inf/NaN Generation

```go
func (g *Generator) generateValue(v *document.Value) {
    switch v.Type {
    case document.TypeBool:
        if g.opts.Version == VersionV2 {
            if v.Value.(bool) {
                g.write("#true")
            } else {
                g.write("#false")
            }
        } else {
            // v1
            g.write(strconv.FormatBool(v.Value.(bool)))
        }
    case document.TypeNull:
        if g.opts.Version == VersionV2 {
            g.write("#null")
        } else {
            g.write("null")
        }
    // handle #inf, #-inf, #nan for v2
    }
}
```

### 7.3 Raw String Generation

```go
func (g *Generator) generateRawString(s string) {
    // count # needed: find max run of " in s, need that many +1 #
    hashes := strings.Repeat("#", requiredHashes(s))
    if g.opts.Version == VersionV2 {
        fmt.Fprintf(g.w, `%s"%s"%s`, hashes, s, hashes)
    } else {
        fmt.Fprintf(g.w, `r%s"%s"%s`, hashes, s, hashes)
    }
}
```

### 7.4 Multi-line String Generation (v2)

For strings containing newlines, the generator can optionally emit `"""..."""` in v2 mode:

```go
func (g *Generator) generateString(s string) {
    if g.opts.Version == VersionV2 && strings.Contains(s, "\n") {
        g.generateMultiLineString(s)
        return
    }
    // ... existing quoted string logic
}
```

`generateMultiLineString()` emits the string with the appropriate indentation. The closing `"""` is placed at the current indentation level, and all content lines are indented one level deeper.

### 7.5 Version Marker Emission

```go
func (g *Generator) Generate(doc *document.Document) error {
    if g.opts.EmitVersionMarker {
        fmt.Fprintf(g.w, "/- kdl-version %d\n", g.opts.Version)
    }
    // ... rest of generation
}
```

---

## 8. Document Model Changes

### 8.1 `document.Document`

Add a `Version` field:

```go
type Document struct {
    Nodes   []*Node
    Version int  // 0=unknown/unset, 1=v1, 2=v2
}
```

The parser sets this when a version marker is found or when auto-detection resolves to a specific version.

### 8.2 No Changes to Node or Value

`document.Node` and `document.Value` are version-agnostic. The syntax differences (bare `true` vs `#true`) are fully resolved at tokenize/parse time into the same in-memory representation.

---

## 9. Public API Changes

### 9.1 Parse Options

```go
// In kdl.go (or document/version.go, exported):
type Version int

const (
    VersionAuto Version = 0
    VersionV1   Version = 1
    VersionV2   Version = 2
)

// ParseOptions gains Version:
type ParseOptions = parser.ParseContextOptions
// parser.ParseContextOptions gains:
//   Version Version
```

The zero value of `ParseOptions` (all fields zero/false) continues to work exactly as today (v1 behavior for existing callers, or `Auto` once we decide the default).

**Default for existing callers**: `VersionAuto` could be defined as `0` but behave as v1-only to avoid surprising existing users. We can introduce `VersionAuto` as an explicit opt-in.

**Recommended**: keep existing `Parse()` as v1-only. Add new `ParseV2()` and `ParseAuto()` convenience functions. The `ParseOptions.Version` field enables fine-grained control.

```go
// New convenience functions:
func ParseV2(r io.Reader) (*document.Document, error)
func ParseAutoVersion(r io.Reader) (*document.Document, error)
```

### 9.2 Generate Options

```go
type GenerateOptions = generator.Options
// generator.Options gains:
//   Version          Version
//   EmitVersionMarker bool

// New convenience:
func GenerateV2(doc *document.Document, w io.Writer) error
```

### 9.3 Marshal/Unmarshal Options

The marshal/unmarshal layer operates on `document.Document` objects and is therefore version-agnostic. No changes needed. The version only affects the tokenizer/parser (input) and generator (output) layers.

---

## 10. Test Plan

### 10.1 Official KDL Test Suite (v2)

The KDL org provides test cases at `https://github.com/kdl-org/kdl/tree/main/tests/test_cases`. The existing test infrastructure already clones the `release/v1` branch; we need to add the v2 test cases:

```bash
# In internal/parser/ alongside the existing test:
git clone --branch release/v2 https://github.com/kdl-org/kdl kdl-org-v2
```

Add `TestKDLOrgTestCasesV2` mirroring `TestKDLOrgTestCases` but using the v2 test cases and `Version: VersionV2`.

### 10.2 Unit Tests: Tokenizer

File: `internal/tokenizer/scanner_v2_test.go`

#### Keywords
```go
// PASS: v2 keywords
{"#true",  TypeBool, true},
{"#false", TypeBool, false},
{"#null",  TypeNull, nil},
{"#inf",   TypeFloat, math.Inf(1)},
{"#-inf",  TypeFloat, math.Inf(-1)},
{"#nan",   TypeFloat, math.NaN()},

// FAIL in v2: bare keywords
{"true",   error},  // disallowed-keyword-identifier in v2
{"false",  error},
{"null",   error},

// PASS in v2: bare keywords used as identifiers → still error as values
// but valid as node names? No — disallowed-keyword-identifiers means
// they can't appear anywhere as identifiers.
```

#### Raw Strings (v2)
```go
// PASS v2 raw strings
{`#"hello"#`,            "hello"},
{`##"hello "# world"##`, `hello "# world`},
{`#"\n is literal"#`,    `\n is literal`},  // no escape processing

// FAIL v2: v1-style raw strings
{`r"hello"`,  error},  // 'r' is just an identifier in v2

// PASS v1 raw strings (v1 mode)
{`r"hello"`,           "hello"},
{`r#"he said "hi""#`,  `he said "hi"`},

// FAIL v1: v2-style raw strings
{`#"hello"#`,  error},  // # not valid in v1 identifiers... actually need to check
```

#### Multi-line Strings (v2 only)
```go
// basic multi-line
{`"""
    hello
    """`, "hello"},

// multi-line with indentation stripping
{`"""
        foo
    bar
    """`, "    foo\nbar"},

// multi-line with escapes
{`"""
    hello\nworld
    """`, "hello\nworld"},

// raw multi-line
{`#"""
    no \n escapes
    """#`, `no \n escapes`},

// FAIL: opening """ not followed by newline
{`"""hello"""`, error},

// FAIL: content line less indented than closing
// (this is actually valid — it just keeps its whitespace)
```

#### `\s` Escape
```go
// v2
{`"hello\sworld"`, "hello world"},  // \s → space

// v1: \s is invalid
{`"hello\sworld"`, error},  // in v1 mode
```

#### Escaped Whitespace in Strings
```go
// v2: \ + whitespace → discard all
{`"hello \   world"`,  "hello world"},
{`"hello \` + "\n" + `   world"`, "hello world"},  // \ + newline + spaces → nothing
{`"a\` + "\t\t\t" + `b"`, "ab"},

// v1: \ + space is not valid (not a recognized escape)
```

### 10.3 Unit Tests: Parser

File: `internal/parser/parser_v2_test.go`

```go
// Version marker parsing
{`/- kdl-version 2
node #true`, doc(node("node", arg(true)))},

{`/- kdl-version 1
node true`, doc(node("node", arg(true)))},

// Version marker with BOM
{"\uFEFF/- kdl-version 2\nnode #true", ...},

// Properties with v2 keywords
{`node key=#true`,   doc(node("node", prop("key", true)))},
{`node key=#false`,  doc(node("node", prop("key", false)))},
{`node key=#null`,   doc(node("node", prop("key", nil)))},

// Type annotation on node name
{`(mytype)node "val"`, doc(node("node").withType("mytype").withArg("val"))},

// Slashdash still works in v2
{`/- node #true \nother #false`, doc(node("other", arg(false)))},

// Multi-line string as arg
{`node """` + "\n" + `    hello` + "\n" + `    """`,
    doc(node("node", arg("hello")))},

// Error cases in v2 mode
{`node true`,   error},  // bare true not valid in v2
{`node false`,  error},
{`node null`,   error},
{`node r"str"`, error},  // v1 raw string not valid in v2
```

### 10.4 Unit Tests: Generator

File: `internal/generator/generator_v2_test.go`

```go
// v2 boolean output
genV2(node("n", arg(true)))   → `n #true\n`
genV2(node("n", arg(false)))  → `n #false\n`
genV2(node("n", arg(nil)))    → `n #null\n`

// v1 boolean output (unchanged)
genV1(node("n", arg(true)))   → `n true\n`

// v2 raw string output
genV2(node("n", rawArg(`he said "hi"`)))  → `n #"he said \"hi\""#\n`
// actually raw string: n ##"he said "hi""##

// v2 multi-line string
genV2(node("n", arg("line1\nline2")))  →
    `n """\n    line1\n    line2\n    """\n`

// version marker emission
genV2WithMarker(doc) → `/- kdl-version 2\n...`
```

### 10.5 Unit Tests: Auto-Detection

File: `kdl_v2_test.go`

```go
// Auto: v2 document (has #true) → parsed as v2
{mode: Auto, input: `node #true`, expect: boolArg(true)},

// Auto: v1 document (has bare true) → parsed as v1
{mode: Auto, input: `node true`, expect: boolArg(true)},

// Auto: version marker forces v2
{mode: Auto, input: "/- kdl-version 2\nnode #true", expect: boolArg(true)},

// Auto: version marker forces v1
{mode: Auto, input: "/- kdl-version 1\nnode true", expect: boolArg(true)},

// Error: version marker says v1 but content is v2
{mode: Auto, input: "/- kdl-version 1\nnode #true", expect: error},

// Error: version marker says v2 but content is v1
{mode: Auto, input: "/- kdl-version 2\nnode true", expect: error},

// Explicit v2 mode rejects v1 syntax
{mode: V2, input: `node true`, expect: error},

// Explicit v1 mode rejects v2 syntax
{mode: V1, input: `node #true`, expect: error},
```

### 10.6 Round-Trip Tests

```go
// Parse v2 → generate v2 → parse v2 → same document
// Parse v1 → generate v1 → parse v1 → same document
// Parse v2 → generate v1 → parse v1 → same document (cross-version)
```

---

## 11. Edge Cases and Gotchas

### 11.1 `#` in v1 Identifiers

In v1, `#` is allowed in identifiers (e.g., `C#` is a valid node name). In v2, `#` is non-identifier. This is a meaningful breaking change for any v1 document using `#` in names.

**Action**: In v2 mode, any bare `#` not followed by a keyword prefix must be a parse error. The version fallback in Auto mode handles this: such a document will fail v2 parsing and succeed v1 parsing.

Test:
```go
// v1 only: C# as node name
{mode: V1, input: `C# "lang"`, expect: node("C#", arg("lang"))},
{mode: V2, input: `C# "lang"`, expect: error},
{mode: Auto, input: `C# "lang"`, expect: node("C#", arg("lang"))},  // falls back to v1
```

### 11.2 `inf`, `-inf`, `nan` as Identifiers in v1

In v1, `inf`, `-inf`, `nan` were already special (keyword numbers). Check whether kdl-go v1 currently allows them as identifiers (node names). The v2 explicitly bans them. This should not be a breaking change for kdl-go since they were already numeric keywords.

### 11.3 Multi-line String Closing Delimiter Indentation

The closing `"""` can use spaces **or** tabs for its indentation, but it must be consistent with the content lines. Mixing tabs and spaces in indentation is technically valid but creates ambiguity. The spec does not define tab width, so mixing should probably be rejected.

**Action**: If the content lines use spaces and the closing `"""` uses a tab (or vice versa), emit a parse error: "inconsistent indentation in multi-line string".

### 11.4 Empty Multi-line Strings

```kdl
node """
    """
```

The content between `"""` and the closing `"""` is empty (just a newline). This should produce an empty string `""`.

### 11.5 Multi-line String with Zero-Width Base Indent

```kdl
node """
content here
"""
```

The closing `"""` has zero indentation → base indent is empty → no stripping occurs. The value is `"content here"`.

### 11.6 Escaped Whitespace Spanning a Newline

```go
"hello \
    world"
```

The `\` is followed by a space, then a newline, then spaces. All of it (` \n    `) is discarded. Result: `"helloworld"`. This is surprising but spec-compliant.

### 11.7 `\s` in Multi-line Raw Strings

In `#"""..."""#`, no escape processing occurs. `\s` is literal. This is consistent with other raw strings.

### 11.8 Version Marker Must Be Exact

The version marker grammar is strict:
```
'/-' unicode-space* 'kdl-version' unicode-space+ ('1'|'2') unicode-space* newline
```

`/- kdl-version 3` is not a valid version marker (version 3 doesn't exist) and must be treated as a regular slashdash comment (not a parse error). Similarly, `/- kdl-version 2 extra stuff` is invalid as a marker.

### 11.9 BOM Handling with Version Marker

The BOM (`U+FEFF`) may precede the version marker. The scanner must handle:
```
\uFEFF/- kdl-version 2\n...
```
The BOM should be consumed silently before version marker detection.

### 11.10 `#` Followed by Non-keyword in v2

What happens in v2 mode when `#` is followed by something that's not a keyword?

```kdl
node #foo  // in v2 mode
```

`#` is not an identifier character, so `#foo` is not an identifier. `#f` is the start of `#false` — if `#foo` is present, the scanner reads `#f` and then expects `alse` but gets `oo`. This should be a clear error: "unknown keyword #foo".

### 11.11 `r` as an Identifier in v2

In v2, `r"..."` is no longer a raw string. The `r` is parsed as an identifier. Since the next token is `"..."`, the parser sees two adjacent tokens where one argument was expected, which is a parse error in a different way.

```kdl
// v2 parse of: node r"hello"
// Tokenizes as: node(ident), r(ident), "hello"(string)
// Parser: node name=node, then sees 'r' as an argument (bare string)
// then sees "hello" as another argument. Both are valid arguments.
// So this would parse as: node with args ["r", "hello"] — NOT an error!
```

This is the surprising case. In v2, `r"hello"` parses as two arguments: the identifier `r` and the string `"hello"`. This is spec-compliant but may surprise users migrating from v1. The Auto detection handles this: if the document also uses `#true` etc., v2 is used; if it uses bare `true`, v1 is used. A document that uses `r"..."` and bare `true` is a v1 document.

**Test**:
```go
{mode: V2, input: `node r"hello"`, expect: node("node", arg("r"), arg("hello"))},
{mode: V1, input: `node r"hello"`, expect: node("node", rawArg("hello"))},
```

### 11.12 Multi-line String Indentation with Mixed Escapes

```kdl
node """
    line1\
    continued
    """
```

The `\` at end of `line1` followed by whitespace (newline + indent) is the escaped-whitespace rule: `\` + `\n    ` all discarded. Result: `"line1continued"`.

This interacts with the dedentation rule: the newline consumed by the escape is NOT a "line" for dedentation purposes. Only actual unescaped newlines create new lines.

### 11.13 Property Key as String in v2

In v2, property keys are `string` (not just identifier-string):
```
prop := string node-space* '=' node-space* value
```

This means quoted strings are valid property keys:
```kdl
node "hello world"="value"
```

In v1, property keys were also strings (same grammar), so this should already work. Verify that the existing parser handles this.

---

## 12. Implementation Phases

### Phase 1: Infrastructure (No Behavior Change)

- Add `Version` type to `document` or `internal/tokenizer`
- Add `Version` field to `ParseContextOptions`, `generator.Options`
- Add `VersionV1`, `VersionV2`, `VersionAuto` constants
- Wire `Version` through tokenizer → scanner (but scanner behavior unchanged)
- All existing tests continue to pass

### Phase 2: v2 Tokenizer

- Implement `readHashKeyword()` for `#true`, `#false`, `#null`, `#inf`, `#-inf`, `#nan`
- Implement `readRawStringV2()` for `#"..."#`
- Implement `readMultiLineString()` for `"""..."""` and `#"""..."""#`
- Add `\s` escape to `readStringEscape()`
- Add escaped-whitespace handling to string scanning
- Switch identifier char set based on version
- Implement `detectVersion()` for version marker
- Add tokenizer unit tests (§10.2)

### Phase 3: v2 Parser

- Propagate version from `ParseContextOptions` to scanner
- Add `VersionMismatchError` type
- Implement auto-detect fallback logic in `parse()`
- Add parser unit tests (§10.3)
- Pass all official KDLv2 test cases

### Phase 4: v2 Generator

- Version-aware keyword output (`#true` vs `true`)
- Version-aware raw string output (`#"..."#` vs `r"..."`)
- Multi-line string output for v2
- Version marker emission
- Add generator unit tests (§10.4)
- Round-trip tests (§10.6)

### Phase 5: Public API

- Add `ParseV2()`, `ParseAutoVersion()` convenience functions
- Add `GenerateV2()` convenience function
- Update `Encoder`/`Decoder` `Options` to expose `Version`
- Document migration guide
- Add auto-detection tests (§10.5)

---

## Appendix A: KDLv2 Full Grammar Reference

```ebnf
document := bom? version? nodes

nodes := (line-space* node)* line-space*

base-node := slashdash? type? node-space* string
    (node-space+ slashdash? node-prop-or-arg)*
    (node-space+ slashdash node-children)*
    (node-space+ node-children)?
    (node-space+ slashdash node-children)*
    node-space*
node := base-node node-terminator
final-node := base-node node-terminator?

node-prop-or-arg := prop | value
node-children := '{' nodes final-node? '}'
node-terminator := single-line-comment | newline | ';' | eof

prop := string node-space* '=' node-space* value
value := type? node-space* (string | number | keyword)
type := '(' node-space* string node-space* ')'

string := identifier-string | quoted-string | raw-string

identifier-string := unambiguous-ident | signed-ident | dotted-ident
unambiguous-ident :=
    ((identifier-char - digit - sign - '.') identifier-char*)
    - disallowed-keyword-strings
signed-ident :=
    sign ((identifier-char - digit - '.') identifier-char*)?
dotted-ident :=
    sign? '.' ((identifier-char - digit) identifier-char*)?
identifier-char :=
    unicode - unicode-space - newline - [\\/(){}[];\"#=]
    - disallowed-literal-code-points
disallowed-keyword-identifiers :=
    'true' | 'false' | 'null' | 'inf' | '-inf' | 'nan'

quoted-string :=
    '"' single-line-string-body '"' |
    '"""' newline (multi-line-string-body newline)? (unicode-space | ws-escape)* '"""'
single-line-string-body := (string-character - newline)*
string-character :=
    '\\' escape |
    unicode - '\\' - '"' - disallowed-literal-code-points
escape :=
    ["\\/bfnrts] | 'u{' hex-digit{1,6} '}' | ws-escape
ws-escape := (unicode-space | newline)+

raw-string := '#'* '"' single-line-raw-string-body? '"' '#'*
            | '#'* '"""' newline multi-line-raw-string-body? newline
              (unicode-space)* '"""' '#'*

number := keyword-number | hex | octal | binary | decimal
decimal := sign? integer ('.' integer)? exponent?
exponent := ('e' | 'E') sign? integer
integer := digit (digit | '_')*
hex := sign? '0x' hex-digit (hex-digit | '_')*
octal := sign? '0o' [0-7] [0-7_]*
binary := sign? '0b' ('0'|'1') ('0'|'1'|'_')*
sign := '+' | '-'

keyword := boolean | '#null'
keyword-number := '#inf' | '#-inf' | '#nan'
boolean := '#true' | '#false'

ws := unicode-space | multi-line-comment
escline := '\\' ws* (single-line-comment | newline | eof)
newline := CR LF | CR | LF | NEL | FF | LS | PS
line-space := node-space | newline | single-line-comment
node-space := ws* escline ws* | ws+

single-line-comment := '//' (unicode - newline)* (newline | eof)
multi-line-comment := '/*' (multi-line-comment | '*'+ [^/] | [^*])* '*'+ '/'
slashdash := '/-'

version := '/-' unicode-space* 'kdl-version' unicode-space+ ('1'|'2')
           unicode-space* newline
bom := '\u{FEFF}'
```

## Appendix B: v1 vs v2 Syntax Cheat Sheet

| Feature | KDLv1 | KDLv2 |
|---------|-------|-------|
| Boolean true | `true` | `#true` |
| Boolean false | `false` | `#false` |
| Null | `null` | `#null` |
| Infinity | `inf` | `#inf` |
| Negative infinity | `-inf` | `#-inf` |
| NaN | `nan` | `#nan` |
| Raw string | `r"..."` | `#"..."#` |
| Raw string w/ hashes | `r#"..."#` | `##"..."##` |
| Multi-line string | N/A | `"""..."""` |
| Space escape | N/A | `\s` |
| String whitespace escape | N/A | `\` + whitespace |
| `#` in identifiers | Allowed | **Not allowed** |
| `[`, `]` in identifiers | Allowed | **Not allowed** |
| `true`/`false`/`null` as identifiers | Not allowed (keywords) | Not allowed (disallowed-keyword-identifiers) |
| Version marker | N/A | `/- kdl-version N` |
