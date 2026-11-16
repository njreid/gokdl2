package kdl

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

// Tests for issue #43: with ParseComments, parse -> Generate must keep every comment, in a stable place.

func commentParse(src string, pv ParseVersion, comments bool) (*document.Document, error) {
	return ParseWithOptions(strings.NewReader(src), ParseOptions{Version: pv, ParseComments: comments})
}

func commentGenerate(doc *document.Document, v tokenizer.Version) (string, error) {
	var b bytes.Buffer
	err := GenerateWithOptions(doc, &b, GenerateOptions{Indent: "\t", Version: v})
	return b.String(), err
}

// normalizeComment makes comment text comparable modulo indentation and trailing whitespace.
func normalizeComment(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Join(lines, "\n")
}

// scanComments returns the text of every comment token in src and the number of slash-dash tokens.
func scanComments(src string, v tokenizer.Version) (comments []string, slashes int, err error) {
	s := tokenizer.NewSlice([]byte(src))
	s.ParseComments = true
	s.Version = v
	for s.Scan() {
		t := s.Token()
		switch t.ID {
		case tokenizer.SingleLineComment, tokenizer.MultiLineComment:
			comments = append(comments, string(t.Data))
		case tokenizer.TokenComment:
			slashes++
		}
	}
	return comments, slashes, s.Err()
}

// danglingSlashDash reports whether src has a slash-dash that is followed by the end of a child block or of the
// document, as in "a 0 /-}". The parser tolerates that in some places, but there is nothing to slash-dash and no way
// to write it back.
func danglingSlashDash(src string, v tokenizer.Version) bool {
	s := tokenizer.NewSlice([]byte(src))
	s.Version = v
	slash := false
	for s.Scan() {
		switch t := s.Token(); t.ID {
		case tokenizer.TokenComment:
			slash = true
		case tokenizer.BraceClose:
			if slash {
				return true
			}
		case tokenizer.Whitespace, tokenizer.Newline, tokenizer.SingleLineComment, tokenizer.MultiLineComment, tokenizer.EOF:
		default:
			slash = false
		}
	}
	return slash
}

// checkCommentRoundTrip checks invariants 1 to 3 for src, parsed as the version pv, and returns the first generation
// (G1). The error says which invariant failed.
func checkCommentRoundTrip(src string, pv ParseVersion) (string, error) {
	doc, err := commentParse(src, pv, true)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	gv := tokenizer.Version(doc.Version)
	if danglingSlashDash(src, gv) {
		return "", nil
	}
	g1, err := commentGenerate(doc, gv)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	explicit := ParseVersion(gv)
	doc2, err := commentParse(g1, explicit, true)
	if err != nil {
		return g1, fmt.Errorf("G1 does not parse: %w\nG1: %q", err, g1)
	}
	g2, err := commentGenerate(doc2, gv)
	if err != nil {
		return g1, fmt.Errorf("generate G2: %w", err)
	}
	if g1 != g2 {
		return g1, fmt.Errorf("not idempotent\nD:  %q\nG1: %q\nG2: %q", src, g1, g2)
	}

	// invariant 2: the structure does not change
	plain, err := commentParse(src, explicit, false)
	if err != nil {
		return g1, fmt.Errorf("parse without comments: %w", err)
	}
	plain1, err := commentParse(g1, explicit, false)
	if err != nil {
		return g1, fmt.Errorf("G1 does not parse without comments: %w", err)
	}
	if err := compareNodes(plain.Nodes, plain1.Nodes, gv, "/"); err != nil {
		return g1, fmt.Errorf("structure changed: %v\nD:  %q\nG1: %q", err, src, g1)
	}

	// invariant 3: no comment is lost
	want, wantSlashes, err := scanComments(src, gv)
	if err != nil {
		return g1, fmt.Errorf("scan D: %w", err)
	}
	_, gotSlashes, err := scanComments(g1, gv)
	if err != nil {
		return g1, fmt.Errorf("scan G1: %w", err)
	}
	norm := normalizeComment(g1)
	for _, c := range want {
		if !strings.Contains(norm, normalizeComment(c)) {
			return g1, fmt.Errorf("comment %q lost\nD:  %q\nG1: %q", c, src, g1)
		}
	}
	if gotSlashes != wantSlashes {
		return g1, fmt.Errorf("%d slash-dashes in D, %d in G1\nD:  %q\nG1: %q", wantSlashes, gotSlashes, src, g1)
	}
	return g1, nil
}

type commentCase struct {
	name string
	in   string
	// out is the expected G1; the empty string means it is the input (a canonically formatted document)
	out string
	// versions the case is valid in; zero means both
	v2only bool
	v1only bool
}

var commentCases = []commentCase{
	// the six reported problems
	{name: "1 trailing line comment", in: "a 1 // trailing\nb 2\n"},
	{name: "2 comment at EOF", in: "a 1\n// at end\n"},
	{name: "3 comment before closing brace", in: "a {\n\tb 1\n\t// end\n}\n"},
	{name: "4 multi-line comment", in: "/*\n * doc\n */\na 1\n"},
	{name: "5 comment between args", in: "a 1 /* mid */ 2\n"},
	{name: "6 slash-dashed node", in: "/-a 1\nb 2\n"},
	{name: "6 slash-dashed arg", in: "a /-1 2\n"},
	{name: "6 slash-dashed last arg", in: "a 1 /-2\n"},
	{name: "6 slash-dashed prop", in: "a k=1 /-j=2 m=3\n"},
	{name: "6 slash-dashed child block", in: "a /-{ x }\n"},
	{name: "6 slash-dashed child block after arg", in: "a 1 /-{ x }\n"},
	{name: "6 multi-line slash-dashed node", in: "/-a {\n\tb 2\n}\nc\n"},
	{name: "6 multi-line slash-dashed child block", in: "a 1 /-{\n\tb\n}\n"},
	{name: "6 nested multi-line slash-dashed child block", in: "x {\n\ta 1 /-{\n\t\tb\n\t}\n}\n"},
	{name: "6 slash-dashed node in block", in: "a {\n\t/-b 1\n\tc 2\n}\n"},
	{name: "6 slash-dashed node last in block", in: "a {\n\tc 2\n\t/-b 1\n}\n"},
	{name: "6 slash-dashed node at EOF", in: "a 1\n/-b 2\n"},
	{name: "6 only slash-dashed", in: "/-a 1\n"},
	{name: "6 slash-dashed node with children inside block", in: "a {\n\t/-b {\n\t\tc\n\t}\n\td\n}\n"},

	// structure
	{name: "nested blocks", in: "a {\n\t// c1\n\tb {\n\t\t// c2\n\t\tc 1 // t\n\t\t// inside b\n\t}\n\t// inside a\n}\n"},
	{name: "blank lines between comments", in: "// c1\n\n// c2\n\na 1\n\nb 2\n"},
	{name: "blank line before EOF comment", in: "a 1\n\n// end\n"},
	{name: "blank line inside block", in: "a {\n\tb 1\n\n\t// end\n}\n"},
	{name: "trailing comment on node with children", in: "a 1 { // x\n\tb 2\n}\n"},
	{name: "trailing block comment on node with children", in: "a 1 { /* x */\n\tb 2\n}\n"},
	{name: "empty block with comment", in: "a {\n\t// only\n}\n"},
	{name: "empty block with comment and args", in: "a 1 k=2 { // t\n\t// only\n}\n"},
	{name: "comment after closing brace", in: "a {\n\tb\n} // c\n"},
	{name: "comment before and after block", in: "a 1 /* x */ { // y\n\tb\n} // z\n"},
	{name: "comment after closing brace of empty block", in: "a { // y\n} // z\n", out: "a // z // y\n"},
	{name: "block comment multi-line in block", in: "a {\n\t/*\n\t * x\n\t */\n\tb 1\n}\n"},
	{name: "block comment multi-line blank interior lines", in: "/* a\n\n\n b */\nx\n"},
	{name: "multi-line trailing comment", in: "a 1 /* x\n   y */\nb 2\n"},
	{name: "comments in every position", in: "// before\n(t)a /* 1 */ 1 /* 2 */ k=2 /* 3 */ (u)3 /* 4 */ { // 5\n\t// 6\n\tb /* 7 */ 1 // 8\n\t// 9\n} // 10\n// 11\n", out: "// before\n(t)a /* 1 */ 1 /* 3 */ (u)3 /* 2 */ k=2 /* 4 */ { // 5\n\t// 6\n\tb /* 7 */ 1 // 8\n\t// 9\n} // 10\n// 11\n", v2only: true},
	{name: "comment before property", in: "a /* c */ k=1\n"},
	{name: "comment before property after arg", in: "a 1 /* c */ k=1\n"},
	{name: "comment between key and value", in: "a k = /* c */ 1\n", out: "a /* c */ k=1\n", v2only: true},
	{name: "comment after name", in: "a /* c */\n"},
	{name: "comment after name without space", in: "a/* c */ 1\n", out: "a /* c */ 1\n", v2only: true},
	{name: "several trailing comments", in: "a 1 /* x */ /* y */ // z\n"},
	{name: "line comment inside continuation", in: "a 1 \\ // c\n 2\n"},
	{name: "line comment inside continuation then end", in: "a 1 \\ // c\n\nb\n", out: "a 1 // c\n\nb\n"},
	{name: "line comment in continuation then slash-dash", in: "a \\ // c\n/-1\n"},
	{name: "slash-dashed blocks", in: "a {\n\tb\n} /-{\n\tone\n} \\\n/-{\n\ttwo\n}\n"},
	{name: "dup property keeps comment", in: "a /* x */ k=1 /* y */ k=2\n", out: "a /* x */ /* y */ k=2\n"},
	{name: "type annotation comments", in: "(/* c */t)a\n", out: "/* c */\n(t)a\n", v2only: true},
	{name: "v2 literals", in: "a #true /* c */ #null // t\n", v2only: true},
	{name: "v1 literals", in: "a true /* c */ null // t\n", v1only: true},
	{name: "v2 strings", in: "a \"x\" /* c */ #\"raw\"# // t\n", v2only: true},
	{name: "v1 strings", in: "a \"x\" /* c */ \"y\" // t\n", v1only: true},
	{name: "slash-dashed multi-line string arg", in: "a /-\"\"\"\n\tx\n\t\"\"\" 2\n", v2only: true},
	{name: "block comment with slash-dash text", in: "a /* /-b */ 1\n"},
	{name: "line comment with braces", in: "a { // }\n\tb\n}\n"},

	// not canonical
	{name: "semicolons", in: "a 1; // x\nb 2\n", out: "a 1\n// x\nb 2\n"},
	{name: "semicolon-separated nodes", in: "a 1; b 2 // t\n", out: "a 1\nb 2 // t\n"},
	{name: "semicolon before comment", in: "a 1 /* x */;\nb\n", out: "a 1 /* x */\nb\n"},
	{name: "extra indentation", in: "a {\n        // c\n        b 1\n}\n", out: "a {\n\t// c\n\tb 1\n}\n"},
	{name: "multi-line comment indented", in: "a {\n    /*\n     * x\n     */\n    b 1\n}\n", out: "a {\n\t/*\n\t * x\n\t */\n\tb 1\n}\n"},
	{name: "many blank lines collapse", in: "a 1\n\n\n\n// c\n\n\n\nb 2\n", out: "a 1\n\n// c\n\nb 2\n"},
	{name: "trailing whitespace trimmed", in: "a 1 // c   \n// d \t\nb\n", out: "a 1 // c\n// d\nb\n"},
	{name: "crlf", in: "a 1 // t\r\n// c\r\n/* x\r\n y */\r\nb 2\r\n", out: "a 1 // t\n// c\n/* x\n y */\nb 2\n"},
	{name: "no trailing newline", in: "a 1 // t", out: "a 1 // t\n"},
	{name: "no trailing newline comment", in: "a 1\n// t", out: "a 1\n// t\n"},
	{name: "comment only", in: "// just a comment\n"},
	{name: "slash-dash only multi-line", in: "/-a {\n\tb\n}\n"},
	{name: "block comment then node on same line", in: "/* c */ a 1\n", out: "/* c */\na 1\n"},
	{name: "slash-dashed child block then children", in: "a { b } /-{ c }\n", out: "a {\n\tb\n} /-{ c }\n"},
	{name: "slash-dashed arg with children", in: "a 1 /-2 { b }\n", out: "a 1 /-2 {\n\tb\n}\n"},
	{name: "slash-dashed arg and prop with children", in: "a 1 /-2 /-k=3 { b }\n", out: "a 1 /-2 /-k=3 {\n\tb\n}\n"},
	{name: "slash-dashed arg with children and comments", in: "a /-2 { // t\n\tb\n} // c\n", out: "a /-2 { // t\n\tb\n} // c\n"},
	{name: "slash-dashed child block before real block", in: "a 1 /-{ y } { x }\n", out: "a 1 /-{ y } {\n\tx\n}\n", v2only: true},
	{name: "slash-dashed child blocks around real block", in: "a /-{ y } { x } /-{ z } // c\n", out: "a /-{ y } {\n\tx\n} /-{ z } // c\n", v2only: true},
	{name: "slash-dashed arg after real block moves before it", in: "a { x } /-2\n", out: "a /-2 {\n\tx\n}\n", v1only: true},
	{name: "nested slash-dash text in block", in: "a {\n  b 1 /-2 /* x */ 3\n  /-c\n}\n", out: "a {\n\tb 1 /-2 /* x */ 3\n\t/-c\n}\n"},
	{name: "mix", in: "// top\n\n/-skip 1\n(t)a 1 /* m */ k=v /-x=y // end\n/* block\n   two */\nb { // open\n    /-c 1\n\n    d /-1 { e // e-trail\n    // e-inside\n    } // d-after\n    // b-inside\n}\n\n// eof\n", out: "// top\n\n/-skip 1\n(t)a 1 /* m */ k=v /-x=y // end\n/* block\n   two */\nb { // open\n\t/-c 1\n\n\td /-1 {\n\t\te // e-trail\n\t\t// e-inside\n\t} // d-after\n\t// b-inside\n}\n\n// eof\n", v2only: true},
}

func TestCommentRoundTrip(t *testing.T) {
	for _, tc := range commentCases {
		tc := tc
		want := tc.out
		if want == "" {
			want = tc.in
		}
		for _, pv := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
			if (tc.v2only && pv == ParseVersionV1) || (tc.v1only && pv == ParseVersionV2) {
				continue
			}
			name := fmt.Sprintf("%s/v%d", tc.name, pv)
			t.Run(name, func(t *testing.T) {
				g1, err := checkCommentRoundTrip(tc.in, pv)
				if err != nil {
					t.Fatal(err)
				}
				if g1 != want {
					t.Errorf("unexpected output\nin:   %q\nwant: %q\ngot:  %q", tc.in, want, g1)
				}
			})
		}
	}
}

// TestCommentRoundTripOptions checks the generator options that change how nodes are written.
func TestCommentRoundTripOptions(t *testing.T) {
	src := "a 1 // t\nb 2 /-3 { c // x\n\t// in\n}\n/-d\n// end\n"
	doc, err := commentParse(src, ParseVersionV2, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []GenerateOptions{
		{Indent: "  ", Version: tokenizer.VersionV2},
		{Indent: "\t", Version: tokenizer.VersionV2, AddSemicolons: true},
		{Indent: "\t", Version: tokenizer.VersionV2, IgnoreFlags: true},
	} {
		var b bytes.Buffer
		if err := GenerateWithOptions(doc, &b, opts); err != nil {
			t.Fatal(err)
		}
		g1 := b.String()
		doc2, err := commentParse(g1, ParseVersionV2, true)
		if err != nil {
			t.Fatalf("%+v: G1 does not parse: %v\n%s", opts, err, g1)
		}
		b.Reset()
		if err := GenerateWithOptions(doc2, &b, opts); err != nil {
			t.Fatal(err)
		}
		if b.String() != g1 {
			t.Errorf("%+v: not idempotent\nG1: %q\nG2: %q", opts, g1, b.String())
		}
		for _, want := range []string{"// t", "// x", "// in", "/-3", "/-d", "// end"} {
			if !strings.Contains(g1, want) {
				t.Errorf("%+v: %q lost: %q", opts, want, g1)
			}
		}
	}
}

// TestCommentsOffUnchanged checks that without ParseComments the comments are dropped as they always were.
func TestCommentsOffUnchanged(t *testing.T) {
	doc, err := commentParse("// c\na 1 /* m */ 2 /-3 // t\n/-b\n// e\n", ParseVersionV2, false)
	if err != nil {
		t.Fatal(err)
	}
	g, err := commentGenerate(doc, tokenizer.VersionV2)
	if err != nil {
		t.Fatal(err)
	}
	if g != "a 1 2\n" {
		t.Errorf("got %q", g)
	}
}

func TestCommentOnlyDocument(t *testing.T) {
	doc, err := commentParse("// a\n\n/* b */\n/-c 1\n", ParseVersionV2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 0 || len(doc.Comment) == 0 {
		t.Fatalf("nodes %d, comment %q", len(doc.Nodes), doc.Comment)
	}
	g, err := commentGenerate(doc, tokenizer.VersionV2)
	if err != nil {
		t.Fatal(err)
	}
	if want := "// a\n\n/* b */\n/-c 1\n"; g != want {
		t.Errorf("got %q want %q", g, want)
	}
}

// TestCommentModel checks where the parser puts each comment.
func TestCommentModel(t *testing.T) {
	doc, err := commentParse("// b\na 1 /* v */ 2 // t\n/-s\nb { // open\n\t/* x */\n\tc\n\t// in\n}\n// after\n", ParseVersionV2, true)
	if err != nil {
		t.Fatal(err)
	}
	a, b := doc.Nodes[0], doc.Nodes[1]
	check := func(what string, got []byte, want string) {
		t.Helper()
		if string(got) != want {
			t.Errorf("%s: got %q, want %q", what, got, want)
		}
	}
	check("a.Before", a.Comment.Before, "// b")
	check("a.Arguments[0].Comment", a.Arguments[0].Comment, "")
	check("a.Arguments[1].Comment", a.Arguments[1].Comment, "/* v */")
	check("a.Trailing", a.Comment.Trailing, "// t")
	check("b.Before", b.Comment.Before, "/-s")
	check("b.Trailing", b.Comment.Trailing, "// open")
	check("b.Children[0].Before", b.Children[0].Comment.Before, "/* x */")
	check("b.Inside", b.Comment.Inside, "// in")
	check("b.After", b.Comment.After, "// after")
	if b.Children[0].Comment.After != nil || b.Children[0].Comment.Inside != nil {
		t.Errorf("child comment: %+v", b.Children[0].Comment)
	}
}

// TestCommentUserAfter checks that After set by hand is written on its own lines, never glued to the node.
func TestCommentUserAfter(t *testing.T) {
	doc, err := commentParse("a {\n\tb\n}\nc\n", ParseVersionV2, false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Comment = &document.Comment{After: []byte("// after a")}
	doc.Nodes[0].Children[0].Comment = &document.Comment{After: []byte("// after b")}
	g, err := commentGenerate(doc, tokenizer.VersionV2)
	if err != nil {
		t.Fatal(err)
	}
	if want := "a {\n\tb\n\t// after b\n}\n// after a\nc\n"; g != want {
		t.Errorf("got %q want %q", g, want)
	}
}

func commentSeeds(t testing.TB) []string {
	var seeds []string
	for _, c := range commentCases {
		seeds = append(seeds, c.in)
	}
	for _, dir := range []string{
		filepath.Join("kdl-org-v2", "tests", "test_cases", "input"),
		filepath.Join("kdl-org-v1", "tests", "test_cases", "input"),
	} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.kdl"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			seeds = append(seeds, string(data))
		}
	}
	return seeds
}

// TestCommentRoundTripCorpus checks invariants 1 to 3 for every document of the official test suites that parses.
func TestCommentRoundTripCorpus(t *testing.T) {
	n := 0
	for _, src := range commentSeeds(t) {
		if _, err := commentParse(src, ParseVersionAuto, true); err != nil {
			continue
		}
		n++
		if _, err := checkCommentRoundTrip(src, ParseVersionAuto); err != nil {
			t.Errorf("%v\ninput: %q", err, src)
		}
	}
	if n < 100 {
		t.Errorf("only %d documents parsed", n)
	}
}

// hugeExponent matches numbers that are too big for a float64 (a long exponent, or 300 digits). Those are parsed as
// big.Float with a 64-bit mantissa and are not written back with the same value ("A 1e2781" is generated as
// "A 1E+2782"), a number-formatting problem that has nothing to do with comments.
var hugeExponent = regexp.MustCompile(`[eE][+-]?[0-9_]{3,}|[0-9]{300,}`)

// identBackslash matches a backslash right after a bare identifier.
var identBackslash = regexp.MustCompile(`[^\s"\\#/*;{}()=]\\`)

// FuzzCommentRoundTrip asserts invariants 1 to 3 for any input that parses with comments on.
func FuzzCommentRoundTrip(f *testing.F) {
	for _, s := range commentSeeds(f) {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// a backslash right after a bare identifier is part of the identifier in v1, so "/-A\\ \\" is a node named
		// "A\\" followed by a line continuation, which cannot be told apart from "/-A" and a continuation once written
		// back
		if hugeExponent.MatchString(src) {
			return
		}
		if identBackslash.MatchString(src) || strings.Contains(src, "\\\\") {
			return
		}
		if _, err := commentParse(src, ParseVersionAuto, true); err != nil {
			return
		}
		if _, err := checkCommentRoundTrip(src, ParseVersionAuto); err != nil {
			t.Fatalf("%v\ninput: %q", err, src)
		}
	})
}

// TestCommentEntriesNeverAfterBlock checks that slash-dashed arguments and properties are written before the "{", and
// that the parser rejects them after the block in v2.
func TestCommentEntriesNeverAfterBlock(t *testing.T) {
	for _, src := range []string{"a { x } /-2", "a { x } /-k=2", "a { x } /-(t)2", "a {\n x\n} /-2 // c"} {
		if _, err := commentParse(src, ParseVersionV2, true); err == nil {
			t.Errorf("v2 %q: expected an error", src)
		}
		if _, err := commentParse(src, ParseVersionV2, false); err == nil {
			t.Errorf("v2 %q without comments: expected an error", src)
		}
	}
	for _, src := range []string{"a { x } /-{ y }", "a /-{ y } { x }", "a /-{ y } { x } /-{ z }", "a /-2 { x }"} {
		if _, err := commentParse(src, ParseVersionV2, true); err != nil {
			t.Errorf("v2 %q: %v", src, err)
		}
	}
	// v1 accepts them after the block, and the generator writes them before it
	doc, err := commentParse("a { x } /-2 // c", ParseVersionV1, true)
	if err != nil {
		t.Fatal(err)
	}
	g, err := commentGenerate(doc, tokenizer.VersionV1)
	if err != nil {
		t.Fatal(err)
	}
	if want := "a /-2 { // c\n\tx\n}\n"; g != "a /-2 {\n\tx\n} // c\n" && g != want {
		t.Errorf("got %q", g)
	}
	// the writer never puts entries after a closing brace, whatever the comment fields hold
	n := doc.Nodes[0]
	n.Comment = &document.Comment{BeforeBlock: []byte("/-3 /* b */"), Trailing: []byte("// t"), Closing: []byte("/-{ z } // c")}
	g, err = commentGenerate(doc, tokenizer.VersionV2)
	if err != nil {
		t.Fatal(err)
	}
	if want := "a /-3 /* b */ { // t\n\tx\n} /-{ z } // c\n"; g != want {
		t.Errorf("got %q want %q", g, want)
	}
}

// TestCommentClosing checks the Closing and BeforeBlock fields.
func TestCommentClosing(t *testing.T) {
	doc, err := commentParse("a 1 /* x */ { // t\n\tb\n} // end\n", ParseVersionV2, true)
	if err != nil {
		t.Fatal(err)
	}
	c := doc.Nodes[0].Comment
	if string(c.BeforeBlock) != "/* x */" || string(c.Trailing) != "// t" || string(c.Closing) != "// end" {
		t.Errorf("%q %q %q", c.BeforeBlock, c.Trailing, c.Closing)
	}
	for src, want := range map[string]string{
		"a {\n  x\n} // end":     "a {\n\tx\n} // end\n",
		"a { x } /* a */ // end": "a {\n\tx\n} /* a */ // end\n",
	} {
		g, err := checkCommentRoundTrip(src, ParseVersionV2)
		if err != nil {
			t.Fatal(err)
		}
		if g != want {
			t.Errorf("%q: got %q want %q", src, g, want)
		}
	}
}

// TestEmitVersionMarkerOnce checks that a version marker that was kept as a comment is not written a second time.
func TestEmitVersionMarkerOnce(t *testing.T) {
	for _, src := range []string{"/- kdl-version 2\na 1\n", "\n/- kdl-version 2\n// c\na 1\n"} {
		doc, err := commentParse(src, ParseVersionAuto, true)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := GenerateWithOptions(doc, &b, GenerateOptions{Indent: "\t", Version: tokenizer.VersionV2, EmitVersionMarker: true}); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(b.String(), "kdl-version"); n != 1 {
			t.Errorf("%q: %d markers in %q", src, n, b.String())
		}
	}
	// without the marker in the document, it is added
	doc, _ := commentParse("// c\na 1\n", ParseVersionV2, true)
	var b bytes.Buffer
	if err := GenerateWithOptions(doc, &b, GenerateOptions{Indent: "\t", Version: tokenizer.VersionV2, EmitVersionMarker: true}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "/- kdl-version 2\n// c\na 1\n" {
		t.Errorf("got %q", b.String())
	}
}
