package kdl

import (
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

// Raw multi-line strings have no escapes, so a final line consisting of a backslash is content, not an escaped
// closing line.
func TestRawMultiLineTrailingBackslashLine(t *testing.T) {
	tests := []struct{ input, want string }{
		{"n #\"\"\"\n  a\n  \\\n  \"\"\"#", "a\n\\"},
		{"n #\"\"\"\n  \\\n  \"\"\"#", "\\"},
		{"n ##\"\"\"\n  a\n  \\  \n  \"\"\"##", "a\n\\  "},
	}
	for _, tt := range tests {
		doc, err := ParseWithOptions(strings.NewReader(tt.input), ParseOptions{Version: ParseVersionV2})
		if err != nil {
			t.Errorf("%q: %v", tt.input, err)
			continue
		}
		if got := doc.Nodes[0].Arguments[0].ValueString(); got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.input, got, tt.want)
		}
	}
}

// AppendRawString must pick enough hashes for the string to parse back as KDL v1.
func TestAppendRawStringV1RoundTrip(t *testing.T) {
	for _, s := range []string{`plain`, `This "is" a test`, `This #"is"# a test`, `a"##b`, `"`, `""#"`, ``} {
		src := "n " + string(document.AppendRawString(nil, s, -1))
		doc, err := ParseWithOptions(strings.NewReader(src), ParseOptions{Version: ParseVersionV1})
		if err != nil {
			t.Errorf("%q -> %s: %v", s, src, err)
			continue
		}
		if got := doc.Nodes[0].Arguments[0].ValueString(); got != s {
			t.Errorf("%q -> %s -> %q", s, src, got)
		}
	}
}
