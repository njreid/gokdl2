package kdl

import (
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

// U+FFFD (the Unicode replacement character, encoded EF BF BD) is a valid code point that KDL allows literally. It
// must not be confused with invalid UTF-8, which the decoder also reports as U+FFFD but with a size of one byte.
func TestLiteralReplacementCharacter(t *testing.T) {
	const r = "�"
	tests := []struct {
		input string
		v     ParseVersion
		want  string // first argument's value; "" means the document only needs to parse
	}{
		{`n "a` + r + `b"`, ParseVersionV1, "a" + r + "b"},
		{`n "a` + r + `b"`, ParseVersionV2, "a" + r + "b"},
		{`n r"a` + r + `b"`, ParseVersionV1, "a" + r + "b"},
		{`n r#"a` + r + `b"#`, ParseVersionV1, "a" + r + "b"},
		{`n #"a` + r + `b"#`, ParseVersionV2, "a" + r + "b"},
		{"n \"\"\"\n  a" + r + "b\n  \"\"\"", ParseVersionV2, "a" + r + "b"},
		{"n #\"\"\"\n  a" + r + "b\n  \"\"\"#", ParseVersionV2, "a" + r + "b"},
		{"n a" + r + "b", ParseVersionV2, "a" + r + "b"},
		{r + "node 1", ParseVersionV2, "1"},
		{"// " + r + "\nn 1", ParseVersionV1, "1"},
		{"/* " + r + " */ n 1", ParseVersionV2, "1"},
	}
	for _, tt := range tests {
		doc, err := ParseWithOptions(strings.NewReader(tt.input), ParseOptions{Version: tt.v})
		if err != nil {
			t.Errorf("v%d %q: %v", tt.v, tt.input, err)
			continue
		}
		if got := doc.Nodes[0].Arguments[0].ValueString(); got != tt.want {
			t.Errorf("v%d %q: got %q, want %q", tt.v, tt.input, got, tt.want)
		}
	}

	// invalid UTF-8 is still rejected
	for _, input := range []string{"n \"a\xffb\"", "n a\xffb", "n #\"a\xef\xbf\"#"} {
		for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
			if _, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: v}); err == nil {
				t.Errorf("v%d %q: expected an error for invalid UTF-8", v, input)
			}
		}
	}
}

// A value containing U+FFFD round-trips through the generator, and a raw multi-line value keeps its raw form.
func TestReplacementCharacterRoundTrip(t *testing.T) {
	const val = "line � one\nline two"
	n := document.NewNode()
	n.SetName("n")
	v := n.AddArgument(val, "")
	v.Flag = document.FlagRaw | document.FlagMultiLine
	doc := document.New()
	doc.AddNode(n)

	var b strings.Builder
	opts := DefaultGenerateOptions
	opts.Version = 2
	if err := GenerateWithOptions(doc, &b, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `#"""`) || !strings.Contains(b.String(), "�") {
		t.Errorf("expected a raw multi-line string with a literal U+FFFD, got %q", b.String())
	}
	back, err := ParseWithOptions(strings.NewReader(b.String()), ParseOptions{Version: ParseVersionV2})
	if err != nil {
		t.Fatalf("re-parse %q: %v", b.String(), err)
	}
	if got := back.Nodes[0].Arguments[0].ValueString(); got != val {
		t.Errorf("round trip: got %q, want %q", got, val)
	}
}
