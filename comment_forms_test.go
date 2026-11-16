package kdl

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

// structureDump renders node names, types, arguments, properties and children, ignoring comments and formatting.
func structureDump(nodes []*document.Node) string {
	var parts []string
	for _, n := range nodes {
		s := ""
		if n.Type != "" {
			s += "(" + string(n.Type) + ")"
		}
		s += n.Name.ValueString()
		for _, a := range n.Arguments {
			if a.Type != "" {
				s += " (" + string(a.Type) + ")" + a.ValueString()
			} else {
				s += " " + a.ValueString()
			}
		}
		var keys []string
		for k := range n.Properties.Unordered() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v, _ := n.Properties.Get(k)
			s += " " + k + "=" + v.ValueString()
		}
		if len(n.Children) > 0 {
			s += "{" + structureDump(n.Children) + "}"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; ")
}

func parseStructure(src string, v ParseVersion, comments bool) string {
	doc, err := ParseWithOptions(strings.NewReader(src), ParseOptions{Version: v, ParseComments: comments})
	if err != nil {
		return "ERR"
	}
	return structureDump(doc.Nodes)
}

// Every comment form in every position the grammar allows, checked in both versions with ParseComments off and on;
// the two modes must agree on the document structure.
func TestCommentForms(t *testing.T) {
	tests := []struct {
		name, src string
		v2, v1    string // expected structure, "ERR" for a parse error; v1 "" means same as v2
	}{
		// /* */ is whitespace (node-space), so it can be the only separator and may span lines
		{"block comment as only separator", "a/*c*/1", "a 1", ""},
		{"block comment before children", "a/*c*/{ b }", "a{b}", ""},
		{"block comment after property value", "a k=1/*c*/2", "a 2 k=1", ""},
		{"block comment between args", "a 1/*c*/2", "a 1 2", ""},
		{"block comment between strings", "a \"x\"/*c*/\"y\"", "a x y", ""},
		{"multi-line block comment inside node", "a 1 /* multi\nline */ 2", "a 1 2", ""},
		{"multi-line comment before children", "a /*\n*/ { b }", "a{b}", ""},
		{"nested block comment", "a /* /* nested */ still */ 1", "a 1", ""},
		{"deeply nested block comment", "a /* 1 /* 2 /* 3 */ 2 */ 1 */ 9", "a 9", ""},
		{"slash-star-slash", "a /*/ 1 */ 2", "a 2", ""},
		{"stars inside", "a /* ** * */ 1", "a 1", ""},
		{"empty block comment", "a /**/ 1", "a 1", ""},
		{"block comment containing quote", "a /* \" */ 1", "a 1", ""},
		{"block comment containing //", "a /* // */ 1", "a 1", ""},
		{"unterminated block comment", "a /* unterminated", "ERR", ""},
		{"unterminated nested block comment", "a /* /* */ 1", "ERR", ""},
		{"stray close after comment", "a /* */ */", "ERR", ""},
		// // ends at any KDL newline or the end of input, and terminates the node
		{"block comment only", "/* c */", "", ""},
		{"line comment only", "// c", "", ""},
		{"line comment at EOF, no newline", "a 1 // c", "a 1", ""},
		{"line comment glued to arg", "a 1//c\nb 2", "a 1; b 2", ""},
		{"line comment glued to name", "a//c\nb", "a; b", ""},
		{"line comment CRLF", "a 1 // c\r\nb 2", "a 1; b 2", ""},
		{"line comment CR", "a 1 // c\rb 2", "a 1; b 2", ""},
		{"line comment U+2028", "a 1 // c b 2", "a 1; b 2", ""},
		{"line comment U+0085", "a 1 // c\u0085b 2", "a 1; b 2", ""},
		{"line comment FF", "a 1 // c\fb 2", "a 1; b 2", ""},
		{"line comment containing */", "a 1 // */\nb", "a 1; b", ""},
		{"line comment does not continue node", "a 1 // c\n 2", "ERR", ""},
		{"children on next line after comment", "a // c\n{ b }", "ERR", ""},
		// line continuations
		{"escline then line comment", "a 1 \\ // c\n  2", "a 1 2", ""},
		{"escline then block then line comment", "a 1 \\ /* c */ // d\n  2", "a 1 2", ""},
		{"block comment before escline", "a 1 /* c */ \\\n  2", "a 1 2", ""},
		// comments are not recognised inside strings
		{"comment in string kept", "a \"/* not */ // not\"", "a /* not */ // not", ""},
		{"comment in raw string kept", "a #\"/* x */\"#", "a /* x */", "ERR"},
		// v2 allows comments inside type annotations and around '='; v1 does not
		{"comments around prop equals", "a k /*c*/ = /*c*/ 1", "a k=1", "ERR"},
		{"comment before prop key", "a /*c*/k=1", "a k=1", ""},
		{"comment inside arg type", "a (/*c*/ u8 /*c*/)1", "a (u8)1", "ERR"},
		{"comment between type and value", "a (u8)/*c*/1", "a (u8)1", "ERR"},
		{"comment inside node type", "(/*c*/t)a", "(t)a", "ERR"},
		{"comment before node type", "/*c*/(t)a", "(t)a", ""},
		// blocks and terminators
		{"comment inside empty block", "a { /*c*/ }", "a", ""},
		{"comments glued inside block", "a {/*c*/b/*c*/}", "a{b}", ""},
		{"block comment hides closing brace", "a {\n /*\n }\n */\n b\n}", "a{b}", ""},
		{"line comment hides closing brace", "a { b // }\n}", "a{b}", ""},
		{"comment before semicolon", "a 1 /*c*/;b", "a 1; b", ""},
		{"comment between nodes on one line", "a /*c*/; /*d*/ b", "a; b", ""},
		{"line comments inside children", "a {\n  // x\n  b // y\n  // z\n}", "a{b}", ""},
		{"comment between slashdash and node", "/- /*c*/ a\nb", "b", ""},
		// values at the very end of the input are not dropped
		{"zero at EOF", "n 0", "n 0", ""},
		{"question mark at EOF", "n ?", "n ?", "ERR"},
		{"lone slash at EOF", "n /", "ERR", ""},
		{"only a slash", "/", "ERR", ""},
		{"slash after node at EOF", "n\n/", "ERR", ""},
	}
	for _, tt := range tests {
		for _, v := range []ParseVersion{ParseVersionV2, ParseVersionV1} {
			want := tt.v2
			if v == ParseVersionV1 && tt.v1 != "" {
				want = tt.v1
			}
			off, on := parseStructure(tt.src, v, false), parseStructure(tt.src, v, true)
			if off != want {
				t.Errorf("v%d %s: got %q, want %q", v, tt.name, off, want)
			}
			if on != off {
				t.Errorf("v%d %s: ParseComments changes the structure: %q vs %q", v, tt.name, on, off)
			}
		}
	}
}

// FuzzCommentModesAgree checks that, for any input, parsing with and without ParseComments either fails in both
// modes or yields the same document structure. Run beyond the seeds with:
// go test -run '^$' -fuzz FuzzCommentModesAgree .
func FuzzCommentModesAgree(f *testing.F) {
	for _, s := range []string{
		"a/*c*/1", "a 1 // c", "a { b // }\n}", "/- a { b }\nc", "a /* /* */ */ 1", "a k /*c*/ = 1",
		"a 1 \\ // c\n 2", "n 0", "a {\n  b 1", "a /-{ y } { x }",
	} {
		f.Add(s)
	}
	for _, dir := range []string{"kdl-org-v1", "kdl-org-v2"} {
		files, _ := filepathGlob(dir)
		for _, data := range files {
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, src string) {
		for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
			off, on := parseStructure(src, v, false), parseStructure(src, v, true)
			if off != on {
				t.Fatalf("v%d %q: ParseComments off gives %q, on gives %q", v, src, off, on)
			}
		}
	})
}

// filepathGlob returns the contents of the official test-case inputs under dir.
func filepathGlob(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "tests", "test_cases", "input", "*.kdl"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			out = append(out, string(b))
		}
	}
	return out, nil
}
