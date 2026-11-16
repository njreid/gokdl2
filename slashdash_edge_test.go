package kdl

import (
	"strings"
	"testing"
)

func genV2(t *testing.T, src string, v ParseVersion) (string, error) {
	t.Helper()
	doc, err := ParseWithOptions(strings.NewReader(src), ParseOptions{Version: v})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := GenerateWithOptions(doc, &b, GenerateOptions{Indent: "\t", Version: 2}); err != nil {
		t.Fatal(err)
	}
	return b.String(), nil
}

// Slashdash ('/-') comments out a node, entry or child block, including everything nested inside it.
func TestSlashdashEdgeCases(t *testing.T) {
	deep := strings.Repeat("x {\n", 60) + "leaf 1\n" + strings.Repeat("}\n", 60)
	tests := []struct {
		name, src string
		want      string // generated v2 output; "" means a parse error is expected
		v1        bool   // also check KDL v1
	}{
		{"nested node removed with all children", "a {\n  /-b {\n    c {\n      d 1\n    }\n  }\n  e 2\n}", "a {\n\te 2\n}\n", true},
		{"slashdash inside slashdashed node", "/-a {\n  /-b 1\n  c {\n    /-d\n  }\n}\nz", "z\n", true},
		{"60-deep tree", "/-" + deep + "keep 1", "keep 1\n", true},
		{"typed node", "/-(t)a 1\nb", "b\n", true},
		{"typed arg", "a /-(u8)1 2", "a 2\n", true},
		{"after semicolon", "a 1; /-b 2; c 3", "a 1\nc 3\n", true},
		{"comment after slashdash", "/- // why\na 1\nb 2", "b 2\n", false},
		{"only slashdashed children", "a {\n  /-b\n  /-c\n}", "a\n", true},
		// v2 allows ignored child blocks before and after the one real block
		{"ignored block before real block", "a /-{ y } { x }", "a {\n\tx\n}\n", false},
		{"several ignored blocks around real block", "a /-{ y } /-{ z } { x } /-{ w }", "a {\n\tx\n}\n", false},
		{"slashdash inside ignored block", "a /-{\n  /-b\n  c /-{ d }\n} {\n  real 1\n}", "a {\n\treal 1\n}\n", false},
		{"ignored block before real block, multi-line", "a /-{\n  y\n} {\n  x\n}", "a {\n\tx\n}\n", false},
		// still invalid
		{"two real blocks", "a { x } { y }", "", true},
		{"real block, ignored, real", "a { x } /-{ y } { z }", "", false},
		{"entry after ignored block", "a /-{ y } 1", "", true},
		{"slashdash before }", "a {\n  b /-\n}", "", true},
		{"invalid syntax inside slashdashed node", "/-a {\n  b \"unterminated\n}\nc", "", true},
		// the input ends in the middle of a construct
		{"slashdash at end of input", "a\n/-", "", true},
		{"only a slashdash", "/-", "", true},
		{"slashdash and comment at end of input", "a\n/- // nothing follows", "", false},
		{"unclosed block, no trailing newline", "a {\n  b 1", "", true},
		{"unclosed nested block, no trailing newline", "a {\n  b {\n    c 1", "", true},
		{"unclosed block inside slashdashed node", "/-a {\n  b {\n}\nc", "", true},
		{"unclosed slashdashed block", "/-a {\n  b 1", "", true},
	}
	for _, tt := range tests {
		versions := []ParseVersion{ParseVersionV2}
		if tt.v1 {
			versions = append(versions, ParseVersionV1)
		}
		for _, v := range versions {
			got, err := genV2(t, tt.src, v)
			if tt.want == "" {
				if err == nil {
					t.Errorf("v%d %s: expected an error, got %q", v, tt.name, got)
				}
				continue
			}
			if err != nil {
				t.Errorf("v%d %s: %v", v, tt.name, err)
			} else if got != tt.want {
				t.Errorf("v%d %s: got %q, want %q", v, tt.name, got, tt.want)
			}
		}
	}
}

// Errors for input that ends mid-construct point at the unclosed '{' or the dangling '/-'.
func TestEndOfInputErrorPositions(t *testing.T) {
	tests := []struct{ src, want string }{
		{"a {\n  b 1", "unclosed child block: unexpected end of input; expected '}' at line 1, column 3"},
		{"a {\n  b {\n    c 1", "at line 2, column 5"},
		{"top\n/-", "unexpected end of input after '/-'; expected a node at line 2, column 1"},
	}
	for _, tt := range tests {
		_, err := ParseWithOptions(strings.NewReader(tt.src), ParseOptions{Version: ParseVersionV2})
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: got %v, want it to contain %q", tt.src, err, tt.want)
		}
	}
}
