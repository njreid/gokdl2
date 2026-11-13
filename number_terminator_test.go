package kdl

import (
	"strings"
	"testing"

	"github.com/njreid/gokdl2/relaxed"
)

// Issue #32: a number may be directly followed by anything the grammar allows after a value: '}', '{', ';', a
// comment, a slashdash or a line continuation.
func TestNumberDirectlyFollowedByTerminator(t *testing.T) {
	tests := []struct {
		input string
		want  string // generated v2 output
	}{
		{"n {x 1}", "n {\n\tx 1\n}\n"},
		{"n {x 1.5}", "n {\n\tx 1.5\n}\n"},
		{"n {x 0xff}", "n {\n\tx 0xff\n}\n"},
		{"n {x -0b101}", "n {\n\tx -0b101\n}\n"},
		{"n {x 0o17}", "n {\n\tx 0o17\n}\n"},
		{"n {x 1e3}", "n {\n\tx 1000.0\n}\n"},
		{"n {x k=7}", "n {\n\tx k=7\n}\n"},
		{"n 1;", "n 1\n"},
		{"n 0xff;", "n 0xff\n"},
		{"n 0xff;m 2", "n 0xff\nm 2\n"},
		{"n 1{\n c\n}", "n 1 {\n\tc\n}\n"},
		{"n 0x10{\n c\n}", "n 0x10 {\n\tc\n}\n"},
		{"n 1/* c */ 2", "n 1 2\n"},
		{"n 0xff/* c */", "n 0xff\n"},
		{"n 1// c", "n 1\n"},
		{"n 0b1// c", "n 0b1\n"},
		{"n 1/-2 3", "n 1 3\n"},
		{"n 0xff/-0x1 3", "n 0xff 3\n"},
		{"n 1\\\n  2", "n 1 2\n"},
		{"n 0o7\\\n  2", "n 0o7 2\n"},
		{`("ty")"node" 0xDEADbeef;`, "(ty)node 0xdeadbeef\n"},
	}
	for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				doc, err := ParseWithOptions(strings.NewReader(tt.input), ParseOptions{Version: v})
				if err != nil {
					t.Fatalf("v%d: %v", v, err)
				}
				var b strings.Builder
				opts := DefaultGenerateOptions
				opts.Version = 2
				if err := GenerateWithOptions(doc, &b, opts); err != nil {
					t.Fatal(err)
				}
				if b.String() != tt.want {
					t.Errorf("v%d: got %q, want %q", v, b.String(), tt.want)
				}
			})
		}
	}
}

func TestNumberFollowedByInvalidCharacter(t *testing.T) {
	for _, input := range []string{"n 1/x", "n 0xff/x", "n 1)", "n 0xff)", "n 1=2", "n 0b1(", "n 1\"s\"", "n 0o7#true"} {
		for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
			if _, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: v}); err == nil {
				t.Errorf("v%d %q: expected an error", v, input)
			}
		}
	}
}

// In relaxed NGINX mode '/' and '\' are identifier characters, so a digit-led word containing them stays one token.
func TestNumberTerminatorNGINXMode(t *testing.T) {
	doc, err := ParseWithOptions(strings.NewReader("location 10/foo"), ParseOptions{Version: ParseVersionV1, RelaxedNonCompliant: relaxed.NGINXSyntax})
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Nodes[0].Arguments[0].ValueString(); got != "10/foo" {
		t.Errorf("got %q, want %q", got, "10/foo")
	}
}

// Issue #32 (parser side): in KDL v1 a comment directly after an argument ends the argument.
func TestV1CommentDirectlyAfterArgument(t *testing.T) {
	tests := []struct{ input, want string }{
		{`n "s"/* c */ 2`, "n \"s\" 2\n"},
		{`n "s"// c`, "n \"s\"\n"},
		{"n 1// c\nm 2", "n 1\nm 2\n"},
		{"n 1/* a */ /* b */2", "n 1 2\n"},
		{"n /-1/* c */ 2", "n 2\n"},
		{"n 1/* c */{\n c\n}", "n 1 {\n\tc\n}\n"},
	}
	for _, tt := range tests {
		doc, err := ParseWithOptions(strings.NewReader(tt.input), ParseOptions{Version: ParseVersionV1})
		if err != nil {
			t.Errorf("%q: %v", tt.input, err)
			continue
		}
		var b strings.Builder
		opts := DefaultGenerateOptions
		opts.Version = 2
		_ = GenerateWithOptions(doc, &b, opts)
		if b.String() != tt.want {
			t.Errorf("%q: got %q, want %q", tt.input, b.String(), tt.want)
		}
	}
}
