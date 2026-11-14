package kdl

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

// upstreamCase is a single test case ported from the official KDL spec
// repository (kdl-org/kdl) or the official Rust implementation (kdl-rs).
//
// want is a canonical dump of the parsed document (see dumpUpstreamDoc); an
// empty want means only the parse outcome (valid / invalid) is checked.
type upstreamCase struct {
	name   string
	source string
	input  string
	v1     bool // parse as KDL v1 instead of v2
	valid  bool
	want   string
	// errAt, if set, is a "line L, column C" substring the error must contain.
	errAt string
	// errAtKnownFailure, if set, skips only the errAt location check.
	errAtKnownFailure string
	// knownFailure, if set, records a gap: the case is skipped. The reason starts
	// with a classification: (a) real gokdl2 bug, (b) upstream differs from the
	// spec text, (c) intentional gokdl2 difference, (d) test-harness limitation.
	knownFailure string
}

const errLocDiff = "(d) harness limitation: kdl-rs spans point at the start of the offending token; gokdl2 reports the position where scanning/parsing failed"

const (
	srcSpec = "kdl-org/kdl tests/test_cases/"
	specRev = "@89c1087d"
	srcRS   = "kdl-rs "
	rsRev   = "@443af893"
)

func specSrc(file string) string { return srcSpec + file + specRev }
func rsSrc(where string) string  { return srcRS + where + rsRev }

// upstreamCases is the table of ported cases.
var upstreamCases = []upstreamCase{
	// The kdl-org/kdl spec suite is vendored in kdl-org-v2/ (see kdl-org-v2/VENDORED.txt) and run by
	// kdl_test_suite_test.go; specSrc is kept for citing individual spec cases.

	// ---- kdl-rs tests/test_cases (older copy of the suite, files absent upstream) ----
	{name: "prop_ambiguous_number", source: rsSrc("tests/test_cases/input/prop_ambiguous_number.kdl"),
		input: `node key1="+7" key2="-7" key3="+.7" key4="-.7" key5=".7" key6="--this" key7=".md"` + "\n",
		valid: true,
		want:  `node key1="+7" key2="-7" key3="+.7" key4="-.7" key5=".7" key6="--this" key7=".md"`},
	{name: "space_after_node", source: rsSrc("tests/test_cases/input/space_after_node.kdl"),
		input: "node1 {\n    child\n} \nnode2 {\n    child\n}\n", valid: true,
		want: "node1 {child}\nnode2 {child}"},
	{name: "brackets_in_bare_id_fail", source: rsSrc("tests/test_cases/input/brackets_in_bare_id_fail.kdl"),
		input: "foo123{bar}foo weeee\n", valid: false},

	// ---- kdl-rs src/v2_parser.rs: multi_line_comment_test ----
	{name: "multiline comment spaces", source: rsSrc("src/v2_parser.rs: multi_line_comment_test"),
		input: "/* foo */\nnode", valid: true, want: "node"},
	{name: "multiline comment empty", source: rsSrc("src/v2_parser.rs: multi_line_comment_test"),
		input: "/**/ node", valid: true, want: "node"},
	{name: "multiline comment with newlines", source: rsSrc("src/v2_parser.rs: multi_line_comment_test"),
		input: "/*\nfoo\n*/node", valid: true, want: "node"},
	{name: "multiline comment end on text line", source: rsSrc("src/v2_parser.rs: multi_line_comment_test"),
		input: "/*\nfoo*/node", valid: true, want: "node"},
	{name: "multiline comment start on text line", source: rsSrc("src/v2_parser.rs: multi_line_comment_test"),
		input: "/*foo\n*/node", valid: true, want: "node"},
	{name: "multiline comment nested", source: rsSrc("src/v2_parser.rs: multi_line_comment_test"),
		input: "/* /*bar*/ foo\n*/node", valid: true, want: "node"},
	{name: "multiline comment unterminated", source: rsSrc("src/v2_parser.rs: multi_line_comment_test (negative variant)"),
		input: "/* foo node", valid: false},
	{name: "multiline comment nested unterminated", source: rsSrc("src/v2_parser.rs: multi_line_comment_test (negative variant)"),
		input: "/* /* foo */ node", valid: false},

	// ---- kdl-rs src/v2_parser.rs: slashdash_tests ----
	{name: "slashdash node", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/- foo bar", valid: true, want: ""},
	{name: "slashdash node semicolon", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/- foo bar;", valid: true, want: ""},
	{name: "slashdash no space before name", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/-n 1;", valid: true, want: ""},
	{name: "slashdash node then node", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/- foo\nbar baz", valid: true, want: `bar "baz"`},
	{name: "slashdash entry no space", source: rsSrc("src/v2_parser.rs: slashdash_tests (node_entry)"),
		input: "node /-commented tada", valid: true, want: `node "tada"`},
	{name: "slashdash empty children", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "foo /- { }", valid: true, want: "foo"},
	{name: "slashdash children block", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "foo /- { bar }", valid: true, want: "foo"},
	{name: "slashdash first arg then children", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/- foo bar\nnode /-1 2 { x }", valid: true, want: "node i:2 {x}"},
	{name: "slashdash later arg then children", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/- foo bar\nnode 2 /-3 { x }", valid: true, want: "node i:2 {x}"},
	{name: "slashdash two args then children", source: rsSrc("src/v2_parser.rs: slashdash_tests"),
		input: "/- foo bar\nnode /-1 2 /-3 { x }", valid: true, want: "node i:2 {x}"},
	{name: "slashdash props with spaced equals", source: rsSrc("src/v2_parser.rs: entry_test"),
		input: "node /- foo=1 bar = 2", valid: true, want: "node bar=i:2"},
	{name: "slashdash line continuation then prop", source: rsSrc("src/v2_parser.rs: entry_test"),
		input: "node /- \\\nfoo = 1 bar = 2", valid: true, want: "node bar=i:2"},
	// SPEC.md: slashdash := '/-' line-space*, and line-space includes newlines.
	{name: "slashdash newline inside slashdash space", source: rsSrc("src/v2_parser.rs: entry_test"),
		input: "node /- \nfoo = 1 bar = 2", valid: true, want: "node bar=i:2"},
	{name: "slashdash unattached eof", source: rsSrc("src/v2_parser.rs: slashdash_tests (negative variant)"),
		input: "node /-", valid: false},

	// ---- kdl-rs src/v2_parser.rs: entry_test ----
	{name: "entry prop bare value", source: rsSrc("src/v2_parser.rs: entry_test"),
		input: "n foo=bar", valid: true, want: `n foo="bar"`},
	{name: "entry bare string arg", source: rsSrc("src/v2_parser.rs: entry_test"),
		input: "n foo", valid: true, want: `n "foo"`},

	// ---- kdl-rs src/v2_parser.rs: float_test ----
	{name: "float underscore inside", source: rsSrc("src/v2_parser.rs: float_test"),
		input: "n 12_34.56", valid: true, want: "n f:1234.56"},
	{name: "float underscore trailing integer part", source: rsSrc("src/v2_parser.rs: float_test"),
		input: "n 1234_.56", valid: true, want: "n f:1234.56"},
	{name: "float leading underscore is a string", source: rsSrc("src/v2_parser.rs: float_test"),
		input: "n _1234.56", valid: true, want: `n "_1234.56"`},
	{name: "float with letter in integer part", source: rsSrc("src/v2_parser.rs: float_test"),
		input: "n 1234a.56", valid: false},
	{name: "float 2.5", source: rsSrc("src/v2_parser.rs: float_test"),
		input: "n 2.5", valid: true, want: "n f:2.5"},

	// ---- kdl-rs src/v2_parser.rs: decimal_test ----
	{name: "decimal underscore", source: rsSrc("src/v2_parser.rs: decimal_test"),
		input: "n 12_34", valid: true, want: "n i:1234"},
	{name: "decimal trailing underscore", source: rsSrc("src/v2_parser.rs: decimal_test"),
		input: "n 1234_", valid: true, want: "n i:1234"},
	{name: "decimal leading underscore is a string", source: rsSrc("src/v2_parser.rs: decimal_test"),
		input: "n _1234", valid: true, want: `n "_1234"`},
	{name: "decimal with letter", source: rsSrc("src/v2_parser.rs: decimal_test"),
		input: "n 1234a", valid: false},

	// ---- kdl-rs src/v2_parser.rs: test_hex ----
	{name: "hex lowercase underscore", source: rsSrc("src/v2_parser.rs: test_hex"),
		input: "n 0xdead_beef123", valid: true, want: "n i:15302363377955"},
	{name: "hex mixed case underscore", source: rsSrc("src/v2_parser.rs: test_hex"),
		input: "n 0xDeAd_BeEf123", valid: true, want: "n i:15302363377955"},
	{name: "hex trailing underscore", source: rsSrc("src/v2_parser.rs: test_hex"),
		input: "n 0xdeadbeef123_", valid: true, want: "n i:15302363377955"},
	{name: "hex beyond i128 (kdl-rs rejects, spec allows)", source: rsSrc("src/v2_parser.rs: test_hex (i128 overflow)"),
		input: "n 0xABCDEF0123456789abcdef0123456789", valid: true, want: "n i:228367255721259569362527394270995113865"},
	{name: "hex leading underscore", source: rsSrc("src/v2_parser.rs: test_hex"),
		input: "n 0x_deadbeef123", valid: false},
	{name: "hex invalid digit", source: rsSrc("src/v2_parser.rs: test_hex"),
		input: "n 0xbeefg1", valid: false},

	// ---- kdl-rs src/v2_parser.rs: test_octal ----
	{name: "octal underscore", source: rsSrc("src/v2_parser.rs: test_octal"),
		input: "n 0o12_34", valid: true, want: "n i:668"},
	{name: "octal trailing underscore", source: rsSrc("src/v2_parser.rs: test_octal"),
		input: "n 0o1234_", valid: true, want: "n i:668"},
	{name: "octal leading underscore", source: rsSrc("src/v2_parser.rs: test_octal"),
		input: "n 0o_12_34", valid: false},
	{name: "octal invalid digit", source: rsSrc("src/v2_parser.rs: test_octal"),
		input: "n 0o89", valid: false},

	// ---- kdl-rs src/v2_parser.rs: test_binary ----
	{name: "binary underscore", source: rsSrc("src/v2_parser.rs: test_binary"),
		input: "n 0b10_01", valid: true, want: "n i:9"},
	{name: "binary trailing underscore", source: rsSrc("src/v2_parser.rs: test_binary"),
		input: "n 0b1001_", valid: true, want: "n i:9"},
	{name: "binary leading underscore", source: rsSrc("src/v2_parser.rs: test_binary"),
		input: "n 0b_10_01", valid: false},
	{name: "binary invalid trailing digits", source: rsSrc("src/v2_parser.rs: test_binary"),
		input: "n 0b12389", valid: false},
	{name: "decimal not binary", source: rsSrc("src/v2_parser.rs: test_binary"),
		input: "n 123", valid: true, want: "n i:123"},

	// Radix integers directly followed by a node terminator or comment. Derived from
	// kdl-rs src/node.rs test::parsing (`0xDEADbeef;`) and the radix tests above.
	{name: "hex before semicolon", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;)"),
		input: "n 0xDEADbeef;", valid: true, want: "n i:3735928559"},
	{name: "octal before semicolon", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;, octal variant)"),
		input: "n 0o7;", valid: true, want: "n i:7"},
	{name: "binary before semicolon", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;, binary variant)"),
		input: "n 0b1;", valid: true, want: "n i:1"},
	{name: "hex prop before semicolon", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;, property variant)"),
		input: "n k=0xff;", valid: true, want: "n k=i:255"},
	{name: "hex before closing brace", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;, child-block variant)"),
		input: "n {x 0xff}", valid: true, want: "n {x i:255}"},
	{name: "hex before block comment", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;, comment variant)"),
		input: "n 0xff/* c */", valid: true, want: "n i:255"},
	{name: "hex before newline", source: rsSrc("src/node.rs: test::parsing (0xDEADbeef;, newline variant)"),
		input: "n 0xDEADbeef\n", valid: true, want: "n i:3735928559"},
	{name: "v1 hex before semicolon", source: rsSrc("src/v1_parser.rs: value_tests::hexadecimal_val (terminator variant)"),
		input: "n 0xff;", v1: true, valid: true, want: "n i:255"},

	// ---- kdl-rs src/v2_parser.rs: failure_tests (inputs and error locations) ----
	// errAt is derived from the byte span start of the first kdl-rs diagnostic.
	{name: "bad_node_name child block as name", source: rsSrc("src/v2_parser.rs: failure_tests::bad_node_name_test"),
		input: "foo { bar; { baz; }; }", valid: false, errAt: "line 1, column 12"},
	{name: "bad_node_name slash in name", source: rsSrc("src/v2_parser.rs: failure_tests::bad_node_name_test"),
		input: "no/de 1 {\n    1 2 foo\n    bad#\n}", valid: false, errAt: "line 1, column 1", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 1asdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 1asdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 0x1asdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 0x1asdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 0o1asdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 0o1asdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 0b1asdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 0b1asdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 1.0asdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 1.0asdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 1.asdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 1.asdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_entry_number 1.0easdf", source: rsSrc("src/v2_parser.rs: failure_tests::bad_entry_number_test"),
		input: "node 1.0easdf 2", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_string unterminated", source: rsSrc("src/v2_parser.rs: failure_tests::bad_string_test"),
		input: "node \" 1", valid: false},
	{name: "bad_string junk after string", source: rsSrc("src/v2_parser.rs: failure_tests::bad_string_test"),
		input: "node \"foo\"1", valid: false, errAt: "line 1, column 11"},
	{name: "bad_string newline in single-line string", source: rsSrc("src/v2_parser.rs: failure_tests::bad_string_test"),
		input: "node \"\nlet's do multiline!\"", valid: false},
	{name: "bad_child unclosed", source: rsSrc("src/v2_parser.rs: failure_tests::bad_child_test"),
		input: "node {", valid: false, errAt: "line 1, column 6", errAtKnownFailure: errLocDiff},
	{name: "bad_child extra close", source: rsSrc("src/v2_parser.rs: failure_tests::bad_child_test"),
		input: "node {}}", valid: false, errAt: "line 1, column 8"},
	{name: "bad_child close then open", source: rsSrc("src/v2_parser.rs: failure_tests::bad_child_test"),
		input: "node }{", valid: false, errAt: "line 1, column 6"},
	{name: "bad_child unclosed with newline", source: rsSrc("src/v2_parser.rs: failure_tests::bad_child_test"),
		input: "node {\n", valid: false},
	{name: "bad_child nested unclosed", source: rsSrc("src/v2_parser.rs: failure_tests::bad_child_test"),
		input: "node {\nnode2{{}}", valid: false},

	// ---- kdl-rs src/value.rs / src/identifier.rs ----
	{name: "plain ident with comma", source: rsSrc("src/value.rs: plain_ident_test"),
		input: "foo123,bar", valid: true, want: "foo123,bar"},
	{name: "plain ident with many symbols", source: rsSrc("src/value.rs: plain_ident_test"),
		input: "foo123~!@$%^&*.:'|?+<>,", valid: true, want: "foo123~!@$%^&*.:'|?+<>,"},
	{name: "identifier quoted with escape", source: rsSrc("src/identifier.rs: test::parsing"),
		input: `"foo\"bar" 1`, valid: true, want: `"foo\"bar" i:1`},
	{name: "identifier starting with digit", source: rsSrc("src/identifier.rs: test::parsing"),
		input: "123", valid: false},
	{name: "identifier unterminated quote", source: rsSrc("src/identifier.rs: test::parsing"),
		input: `"x`, valid: false},

	// ---- kdl-rs src/entry.rs: test::parsing ----
	{name: "entry type with continuation and hex", source: rsSrc("src/entry.rs: test::parsing"),
		input: "n \\\n (\"m\\\"eh\")0xDEADbeef\t\\\n", valid: true, want: `n (m"eh)i:3735928559`},
	{name: "entry prop type with continuation and hex", source: rsSrc("src/entry.rs: test::parsing"),
		input: "n \\\n \"foo\"=(\"m\\\"eh\")0xDEADbeef\t\\\n", valid: true, want: `n foo=(m"eh)i:3735928559`},
	{name: "entry equal formatting 1.03e2", source: rsSrc("src/entry.rs: test::reset_value_repr"),
		input: "n name=1.03e2", valid: true, want: "n name=f:103"},

	// ---- kdl-rs src/node.rs ----
	{name: "node type and quoted name", source: rsSrc("src/node.rs: test::parsing"),
		input: "\n\t  (\"ty\")\"node\" 0xDEADbeef;\n", valid: true, want: "(ty)node i:3735928559"},
	{name: "node type and quoted name (no semicolon)", source: rsSrc("src/node.rs: test::parsing"),
		input: "\n\t  (\"ty\")\"node\" 0xDEADbeef\n", valid: true, want: "(ty)node i:3735928559"},
	{name: "node with children and self value", source: rsSrc("src/node.rs: test::parsing"),
		input: "node test {\n    link \"blah\" anything=self\n}\n", valid: true,
		want: `node "test" {link "blah" anything="self"}`},
	{name: "node canonical comments and slashdash", source: rsSrc("src/node.rs: test::canonical_clear_fmt"),
		input: "node /-\"commented\" param_name=103.000 {\n            // This is a nested node\n            nested 1 2 3\n        }",
		valid: true, want: "node param_name=f:103 {nested i:1 i:2 i:3}"},
	{name: "node canonical compact", source: rsSrc("src/node.rs: test::canonical_clear_fmt"),
		input: "node param_name=103.0 { nested 1 2 3; }", valid: true,
		want: "node param_name=f:103 {nested i:1 i:2 i:3}"},

	// ---- kdl-rs src/document.rs: parse_examples (examples/*.kdl) ----
	{name: "example Cargo.kdl", source: rsSrc("examples/Cargo.kdl (src/document.rs: parse_examples)"),
		input: exampleCargoKDL, valid: true,
		want: `package {name "kdl"; version "0.0.0"; description "The kdl document language"; authors "Kat Marchán <kzm@zkat.tech>"; license-file "LICENSE"; edition "2018"}` + "\n" +
			`dependencies {nom "6.0.1"; thiserror "1.0.22"}`},
	{name: "example ci.kdl", source: rsSrc("examples/ci.kdl (src/document.rs: parse_examples)"),
		input: exampleCIKDL, valid: true},
	{name: "example website.kdl", source: rsSrc("examples/website.kdl (src/document.rs: parse_examples)"),
		input: exampleWebsiteKDL, valid: true},

	// ---- kdl-rs src/document.rs: v1_v2_conversions (same tree in v1 and v2) ----
	{name: "zellij v1", source: rsSrc("src/document.rs: v1_v2_conversions (v1 half)"),
		input: zellijV1, v1: true, valid: true, want: zellijWant},
	{name: "zellij v2", source: rsSrc("src/document.rs: v1_v2_conversions (v2 half)"),
		input: zellijV2, valid: true, want: zellijWant},

	// ---- kdl-rs src/v1_parser.rs (KDL v1) ----
	{name: "v1 node basic", source: rsSrc("src/v1_parser.rs: node_tests::basic"),
		input: `foo 1 "bar"=false`, v1: true, valid: true, want: `foo i:1 bar=#false`},
	{name: "v1 errant ident struct Vec", source: rsSrc("src/v1_parser.rs: node_tests::errant_ident1"),
		input: "struct Vec { }", v1: true, valid: false, errAt: "line 1, column 8", errAtKnownFailure: errLocDiff},
	{name: "v1 errant ident evil", source: rsSrc("src/v1_parser.rs: node_tests::errant_ident2"),
		input: "\n    some_node\n    bad evil", v1: true, valid: false, errAt: "line 3, column 9", errAtKnownFailure: errLocDiff},
	{name: "v1 errant ident wait", source: rsSrc("src/v1_parser.rs: node_tests::errant_ident3"),
		input: `node "ok" wait "fine"`, v1: true, valid: false, errAt: "line 1, column 11", errAtKnownFailure: errLocDiff},
	{name: "v1 errant ident oof", source: rsSrc("src/v1_parser.rs: node_tests::errant_ident4"),
		input: `node x="ok" oof z="5`, v1: true, valid: false},
	{name: "v1 bare prop value", source: rsSrc("src/v1_parser.rs: node_tests::errant_ident5"),
		input: "node x=bad", v1: true, valid: false},
	{name: "v1 typed bare value", source: rsSrc("src/v1_parser.rs: node_tests::errant_ident6"),
		input: "node (int)bad", v1: true, valid: false},
	{name: "v1 whitespace only", source: rsSrc("src/v1_parser.rs: whitespace_tests::basic"),
		input: " \t\n\r", v1: true, valid: true, want: ""},
	{name: "v2 whitespace only", source: rsSrc("src/v1_parser.rs: whitespace_tests::basic (as v2)"),
		input: " \t\n\r", valid: true, want: ""},
	{name: "v1 single line comment", source: rsSrc("src/v1_parser.rs: comment_tests::single_line"),
		input: "// Hello world", v1: true, valid: true, want: ""},
	{name: "v1 multiline comment", source: rsSrc("src/v1_parser.rs: comment_tests::multi_line"),
		input: "/* Hello world */", v1: true, valid: true, want: ""},
	{name: "v1 nested multiline comment", source: rsSrc("src/v1_parser.rs: comment_tests::multi_line"),
		input: "/* Hello /* world */ blah */", v1: true, valid: true, want: ""},
	{name: "v1 slashdash node", source: rsSrc("src/v1_parser.rs: comment_tests::slashdash"),
		input: "/-foo 1 2", v1: true, valid: true, want: ""},
	{name: "v1 true", source: rsSrc("src/v1_parser.rs: value_tests::boolean_val"),
		input: "n true", v1: true, valid: true, want: "n #true"},
	{name: "v1 false", source: rsSrc("src/v1_parser.rs: value_tests::boolean_val"),
		input: "n false", v1: true, valid: true, want: "n #false"},
	{name: "v1 null", source: rsSrc("src/v1_parser.rs: value_tests::null_val"),
		input: "n null", v1: true, valid: true, want: "n #null"},
	{name: "v1 binary", source: rsSrc("src/v1_parser.rs: value_tests::binary_val"),
		input: "n 0b0101 0b0101_1111 -0b0101 +0b0101", v1: true, valid: true, want: "n i:5 i:95 i:-5 i:5"},
	{name: "v1 octal", source: rsSrc("src/v1_parser.rs: value_tests::octal_val"),
		input: "n 0o01234567 0o123_4567 -0o123 +0o123", v1: true, valid: true, want: "n i:342391 i:342391 i:-83 i:83"},
	{name: "v1 hex", source: rsSrc("src/v1_parser.rs: value_tests::hexadecimal_val"),
		input: "n 0x0123456789abcdef 0x0123_4567_89ab_cdef 0x123_4567 -0x123 +0x123", v1: true, valid: true,
		want: "n i:81985529216486895 i:81985529216486895 i:19088743 i:-291 i:291"},
	{name: "v1 integer", source: rsSrc("src/v1_parser.rs: value_tests::integer_val"),
		input: "n 123_456 -123 +123", v1: true, valid: true, want: "n i:123456 i:-123 i:123"},
	{name: "v1 float", source: rsSrc("src/v1_parser.rs: value_tests::float_val"),
		input: "n 123_456.789e-10 -123.456 +123.456", v1: true, valid: true,
		want: "n f:1.23456789e-05 f:-123.456 f:123.456"},
	{name: "v1 string escapes", source: rsSrc("src/v1_parser.rs: value_tests::string_val"),
		input: `n "Hello \n\u{2020}world"`, v1: true, valid: true, want: `n "Hello \n†world"`},
	{name: "v1 raw string", source: rsSrc("src/v1_parser.rs: value_tests::raw_string_val"),
		input: `n r"Hello \n\u{2020}world"`, v1: true, valid: true, want: `n "Hello \\n\\u{2020}world"`},
	{name: "v1 raw string hashes", source: rsSrc("src/v1_parser.rs: value_tests::raw_string_val"),
		input: `n r##"Hello \n\u{2020}world"##`, v1: true, valid: true, want: `n "Hello \\n\\u{2020}world"`},
}

func TestUpstreamCases(t *testing.T) {
	for _, tc := range upstreamCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.knownFailure != "" {
				t.Skipf("known failure: %s (source: %s)", tc.knownFailure, tc.source)
			}
			ver := ParseVersionV2
			if tc.v1 {
				ver = ParseVersionV1
			}
			doc, err := ParseWithOptions(strings.NewReader(tc.input), ParseOptions{Version: ver})
			if !tc.valid {
				if err == nil {
					t.Fatalf("expected parse error, got none (source: %s)\ninput: %q\ngot: %s", tc.source, tc.input, dumpUpstreamDoc(doc))
				}
				if tc.errAt != "" {
					if tc.errAtKnownFailure != "" {
						t.Logf("error location check skipped: %s (source: %s); gokdl2 error: %q", tc.errAtKnownFailure, tc.source, err.Error())
					} else if !strings.Contains(err.Error(), tc.errAt) {
						t.Fatalf("error %q does not contain location %q (source: %s)", err.Error(), tc.errAt, tc.source)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error: %v (source: %s)\ninput: %q", err, tc.source, tc.input)
			}
			if tc.want == "" {
				return
			}
			if got := dumpUpstreamDoc(doc); got != tc.want {
				t.Fatalf("parsed tree mismatch (source: %s)\ninput: %q\nwant: %s\n got: %s", tc.source, tc.input, tc.want, got)
			}
		})
	}
}

// TestUpstreamCasesBookkeeping guards the table's bookkeeping.
func TestUpstreamCasesBookkeeping(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range upstreamCases {
		if tc.source == "" {
			t.Errorf("case %q has no source", tc.name)
		}
		if seen[tc.name] {
			t.Errorf("duplicate case name %q", tc.name)
		}
		seen[tc.name] = true
		if tc.knownFailure != "" && !strings.HasPrefix(tc.knownFailure, "(") {
			t.Errorf("case %q: knownFailure must start with a classification (a)-(d)", tc.name)
		}
	}
}

// dumpUpstreamDoc renders a document as one line per top-level node:
//
//	[(type)]name [(type)]arg ... key=[(type)]value ... {child; child}
//
// Strings are Go-quoted, numbers are i:<int> or f:<float>, plus #true, #false
// and #null.
func dumpUpstreamDoc(doc *document.Document) string {
	if doc == nil {
		return "<nil>"
	}
	lines := make([]string, 0, len(doc.Nodes))
	for _, n := range doc.Nodes {
		lines = append(lines, dumpUpstreamNode(n))
	}
	return strings.Join(lines, "\n")
}

func dumpUpstreamNode(n *document.Node) string {
	var b strings.Builder
	if n.Type != "" {
		fmt.Fprintf(&b, "(%s)", unquoteUpstreamType(string(n.Type)))
	}
	name, _ := n.Name.ResolvedValue().(string)
	b.WriteString(dumpUpstreamName(name))
	for _, a := range n.Arguments {
		b.WriteByte(' ')
		b.WriteString(dumpUpstreamValue(a))
	}
	props := n.Properties.Unordered()
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, " %s=%s", dumpUpstreamName(k), dumpUpstreamValue(props[k]))
	}
	if len(n.Children) > 0 {
		kids := make([]string, 0, len(n.Children))
		for _, c := range n.Children {
			kids = append(kids, dumpUpstreamNode(c))
		}
		b.WriteString(" {" + strings.Join(kids, "; ") + "}")
	}
	return b.String()
}

func unquoteUpstreamType(s string) string {
	if u, err := document.UnquoteString(s); err == nil {
		return u
	}
	return s
}

// dumpUpstreamName quotes names containing whitespace, quotes or backslashes, and empty names.
func dumpUpstreamName(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\n\"\\") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func dumpUpstreamValue(v *document.Value) string {
	prefix := ""
	if v.Type != "" {
		prefix = "(" + unquoteUpstreamType(string(v.Type)) + ")"
	}
	switch x := v.Value.(type) {
	case nil:
		return prefix + "#null"
	case bool:
		return prefix + fmt.Sprintf("#%t", x)
	case string:
		return prefix + fmt.Sprintf("%q", x)
	case document.Expression:
		return prefix + fmt.Sprintf("%q", string(x))
	case *big.Int:
		return prefix + "i:" + x.String()
	case *big.Float:
		return prefix + "f:" + x.Text('g', -1)
	case float32:
		return prefix + fmt.Sprintf("f:%g", float64(x))
	case float64:
		return prefix + fmt.Sprintf("f:%g", x)
	default:
		return prefix + fmt.Sprintf("i:%v", x)
	}
}

const exampleCargoKDL = `package {
    name kdl
    version "0.0.0"
    description "The kdl document language"
    authors "Kat Marchán <kzm@zkat.tech>"
    license-file LICENSE
    edition "2018"
}

dependencies {
    nom "6.0.1"
    thiserror "1.0.22"
}
`

const exampleCIKDL = `// This example is a GitHub Action if it used KDL syntax.
// See .github/workflows/ci.yml for the file this was based on.
name CI

on push pull_request

env {
  RUSTFLAGS -Dwarnings
}

jobs {
  fmt_and_docs "Check fmt & build docs" {
    runs-on ubuntu-latest
    steps {
      step uses="actions/checkout@v1"
      step "Install Rust" uses="actions-rs/toolchain@v1" {
        profile minimal
        toolchain stable
        components rustfmt
        override #true
      }
      step rustfmt { run cargo fmt --all -- --check }
      step docs { run cargo doc --no-deps }
    }
  }
  build_and_test "Build & Test" {
    runs-on "${{ matrix.os }}"
    strategy {
      matrix {
        rust "1.46.0" stable
        os ubuntu-latest macOS-latest windows-latest
      }
    }

    steps {
      step uses="actions/checkout@v1"
      step "Install Rust" uses="actions-rs/toolchain@v1" {
        profile minimal
        toolchain "${{ matrix.rust }}"
        components clippy
        override #true
      }
      step Clippy { run cargo clippy --all -- -D warnings }
      step "Run tests" { run cargo test --all --verbose }
      step "Other Stuff" run="""
        echo foo
        echo bar
        echo baz
        """
    }
  }
}
`

const exampleWebsiteKDL = `!doctype html
html lang=en {
    head {
        meta charset=utf-8
        meta name=viewport content="width=device-width, initial-scale=1.0"
        meta \
            name=description \
            content="kdl is a document language, mostly based on SDLang, with xml-like semantics that looks like you're invoking a bunch of CLI commands!"
        title "kdl - The KDL Document Language"
        link rel=stylesheet href="/styles/global.css"
    }
    body {
        main {
            header class="py-10 bg-gray-300" {
                h1 class="text-4xl text-center" "kdl - The KDL Document Language"
            }
            section class=kdl-section id=description {
                p {
                    - "kdl is a document language, mostly based on "
                    a href="https://sdlang.org" "SDLang"
                    - " with xml-like semantics that looks like you're invoking a bunch of CLI commands"
                }
                p "It's meant to be used both as a serialization format and a configuration language, and is relatively light on syntax compared to XML."
            }
            section class=kdl-section id=design-and-discussion {
                h2 "Design and Discussion"
                p {
                    - "kdl is still extremely new, and discussion about the format should happen over on the "
                    a href="https://github.com/kdoclang/kdl/discussions" {
                        - "discussions"
                    }
                    - " page in the Github repo. Feel free to jump in and give us your 2 cents!"
                }
            }
            section class=kdl-section id=design-principles {
                h2 "Design Principles"
                ol {
                    li Maintainability
                    li Flexibility
                    li "Cognitive simplicity and Learnability"
                    li "Ease of de/serialization"
                    li "Ease of implementation"
                }
            }
        }
    }
}
`

const zellijV1 = `
// If you'd like to override the default keybindings completely, be sure to change "keybinds" to "keybinds clear-defaults=true"
keybinds {
    normal {
        // uncomment this and adjust key if using copy_on_select=false
        // bind "Alt c" { Copy; }
    }
    locked {
        bind "Ctrl g" { SwitchToMode "Normal"; }
    }
    resize {
        bind "Ctrl n" { SwitchToMode "Normal"; }
        bind "h" "Left" { Resize "Increase Left"; }
        bind "j" "Down" { Resize "Increase Down"; }
        bind "k" "Up" { Resize "Increase Up"; }
        bind "l" "Right" { Resize "Increase Right"; }
        bind "H" { Resize "Decrease Left"; }
        bind "J" { Resize "Decrease Down"; }
        bind "K" { Resize "Decrease Up"; }
        bind "L" { Resize "Decrease Right"; }
        bind "=" "+" { Resize "Increase"; }
        bind "-" { Resize "Decrease"; }
    }
}
// Plugin aliases - can be used to change the implementation of Zellij
// changing these requires a restart to take effect
plugins {
    tab-bar location="zellij:tab-bar"
    status-bar location="zellij:status-bar"
    welcome-screen location="zellij:session-manager" {
        welcome_screen true
    }
    filepicker location="zellij:strider" {
        cwd "/"
    }
}
mouse_mode false
mirror_session true
`

const zellijV2 = `
// If you'd like to override the default keybindings completely, be sure to change "keybinds" to "keybinds clear-defaults=true"
keybinds {
    normal {
        // uncomment this and adjust key if using copy_on_select=false
        // bind "Alt c" { Copy; }
    }
    locked {
        bind "Ctrl g" { SwitchToMode Normal; }
    }
    resize {
        bind "Ctrl n" { SwitchToMode Normal; }
        bind h Left { Resize "Increase Left"; }
        bind j Down { Resize "Increase Down"; }
        bind k Up { Resize "Increase Up"; }
        bind l Right { Resize "Increase Right"; }
        bind H { Resize "Decrease Left"; }
        bind J { Resize "Decrease Down"; }
        bind K { Resize "Decrease Up"; }
        bind L { Resize "Decrease Right"; }
        bind "=" + { Resize Increase; }
        bind - { Resize Decrease; }
    }
}
// Plugin aliases - can be used to change the implementation of Zellij
// changing these requires a restart to take effect
plugins {
    tab-bar location=zellij:tab-bar
    status-bar location=zellij:status-bar
    welcome-screen location=zellij:session-manager {
        welcome_screen #true
    }
    filepicker location=zellij:strider {
        cwd "/"
    }
}
mouse_mode #false
mirror_session #true
`

const zellijWant = `keybinds {normal; locked {bind "Ctrl g" {SwitchToMode "Normal"}}; resize {bind "Ctrl n" {SwitchToMode "Normal"}; bind "h" "Left" {Resize "Increase Left"}; bind "j" "Down" {Resize "Increase Down"}; bind "k" "Up" {Resize "Increase Up"}; bind "l" "Right" {Resize "Increase Right"}; bind "H" {Resize "Decrease Left"}; bind "J" {Resize "Decrease Down"}; bind "K" {Resize "Decrease Up"}; bind "L" {Resize "Decrease Right"}; bind "=" "+" {Resize "Increase"}; bind "-" {Resize "Decrease"}}}
plugins {tab-bar location="zellij:tab-bar"; status-bar location="zellij:status-bar"; welcome-screen location="zellij:session-manager" {welcome_screen #true}; filepicker location="zellij:strider" {cwd "/"}}
mouse_mode #false
mirror_session #true`
