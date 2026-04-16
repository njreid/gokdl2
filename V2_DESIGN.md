# KDLv2 Support Design for kdl-go

This document outlines the design and implementation plan for adding full KDLv2 support to `kdl-go`, while maintaining backward compatibility with KDLv1.

## 1. Overview

KDLv2 introduces several breaking syntax changes and new features (triple-quoted strings, `#` prefixed keywords, etc.). The goal is to support both versions seamlessly, with automatic detection and manual overrides.

## 2. Compatibility Strategy

- **VersionAuto**: The default mode.
  1. Detect version marker (`/- kdl-version N`) at the start of the document.
  2. If no marker, try parsing as KDLv2 (stricter).
  3. If KDLv2 parsing fails with a version-specific error (e.g., bare `true`), fall back to KDLv1.
- **VersionV1**: Strict KDLv1 mode.
- **VersionV2**: Strict KDLv2 mode.

## 3. Architecture Changes

### 3.1 Tokenizer
- Added `Version` to `Scanner`.
- Updated `isBareIdentifierChar` and `isBareIdentifierStartChar` to be version-aware (rejecting `#` in V2).
- Added `readHashToken` to handle `#true`, `#false`, `#null`, `#inf`, `#-inf`, `#nan` and raw strings `#"..."#`.
- Added `readMultiLineString` for triple-quoted strings `"""..."""`.
- Support for `\s` (space) escape and escaped whitespace (`\` + newline) in strings for V2.

### 3.2 Parser
- `ParseContextOptions` now includes `Version`.
- `kdl.go` implements the `VersionAuto` fallback logic.
- `document.Document` stores the detected `Version`.

### 3.3 Generator
- `generator.Options` includes `Version` and `EmitVersionMarker`.
- `Value` and `Node` methods updated to support version-specific output (e.g., `#true` in V2).

## 4. Refactoring Opportunities

The current implementation in the working directory has some areas for improvement:

### 4.1 Unify Versioned Methods
Instead of adding `MethodV2()` for every string-returning method (e.g., `StringV2`, `NodeNameStringV2`, `UnformattedStringV2` in `Value` and `Properties`), we should prefer methods that take a `Version` parameter or a configuration object.

**Proposed Change:**
- Keep existing `String()`, `FormattedString()`, etc., for backward compatibility (defaulting to V1).
- Add internal `string(version tokenizer.Version, ...)` methods.
- Consider if we want to expose a generic `StringWithVersion(v Version)` instead of multiple `V2` specific methods.

### 4.2 Consolidate Identifier Logic
Current `ctype.go` has `isBareIdentifierChar` and `isBareIdentifierCharVersion`. We should eventually consolidate these or make the versioned one the primary one, with the non-versioned one being a wrapper.

### 4.3 Triple-Quoted String Dedentation
The dedentation logic in `document/strings.go` should be carefully verified against the KDLv2 spec, especially regarding the interaction between escapes and indentation.

## 5. Test Plan

### 5.1 Official KDL Test Suite
We must integrate the official KDLv2 test cases from `https://github.com/kdl-org/kdl/tree/main/tests/test_cases`.
- Clone `kdl-org` repo at a specific tag/commit for stability.
- Add `TestKDLOrgTestCasesV2` to `internal/parser/parser_test.go`.

### 5.2 Comparative Coverage
The test coverage should meet or exceed `kdl-rs` by including:
- All official v2 test cases (success and failure).
- Edge cases for triple-quoted strings (varying indents, mixed tabs/spaces).
- Version marker detection in various positions (with/without BOM).
- Fallback logic verification (ensuring V1 docs still parse in `Auto` mode).

## 6. Implementation Status (Current Working Tree)

- [x] Version infrastructure (`Version` type, fields in `Scanner`, `Document`, `Options`).
- [x] V2 Tokenizer (Keywords, Raw strings, Multi-line strings, Escapes).
- [x] V2 Parser (Basic support, Version marker detection).
- [x] V2 Generator (Basic support, version-aware values).
- [ ] Refactored unified methods (Ongoing).
- [ ] Comprehensive V2 test suite (Pending official test cases integration).

## 7. Next Steps

1. Review and refine the `UnquoteMultiLineString` implementation.
2. Integrate official KDLv2 test cases.
3. Refactor repetitive `V2` methods if possible without breaking API.
4. Finalize `VersionAuto` fallback heuristics.
