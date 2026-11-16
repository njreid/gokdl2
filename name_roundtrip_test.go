package kdl

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/njreid/gokdl2/document"
)

// nameRoundTrip generates a node whose name, single property key and single argument are all s, re-parses the
// output with the same KDL version, and reports the generated text and any mismatch.
func nameRoundTrip(t *testing.T, s string, v ParseVersion, ignoreFlags bool) {
	t.Helper()
	n := document.NewNode()
	n.SetName(s)
	n.AddProperty(s, int64(1), "")
	n.AddArgument(s, "")
	doc := document.New()
	doc.AddNode(n)

	var b strings.Builder
	if err := GenerateWithOptions(doc, &b, GenerateOptions{Version: v.tokenizerVersion(), IgnoreFlags: ignoreFlags}); err != nil {
		t.Fatalf("v%d %q: generate: %v", v, s, err)
	}
	out := b.String()
	got, err := ParseWithOptions(strings.NewReader(out), ParseOptions{Version: v})
	if err != nil {
		t.Fatalf("v%d ignoreFlags=%v %q generated %q, which does not parse: %v", v, ignoreFlags, s, out, err)
	}
	g := got.Nodes[0]
	if name := g.Name.ValueString(); name != s {
		t.Errorf("v%d %q generated %q: name came back as %q", v, s, out, name)
	}
	if _, ok := g.Properties.Get(s); !ok {
		t.Errorf("v%d %q generated %q: property key not found", v, s, out)
	}
	if arg := g.Arguments[0].ValueString(); arg != s {
		t.Errorf("v%d %q generated %q: argument came back as %q", v, s, out, arg)
	}
}

var nameRoundTripSeeds = []string{
	"node", "--flag", "é", "a b", "", "1", "-", "+", ".", "..5", "+-1", "-nan", "a=b", "a/b", "#x",
	// v2 keywords and number-like names (#5)
	"true", "false", "null", "inf", "-inf", "nan", "+1", "-1x", ".5", "-.5", "+.5", "-.5x",
	"a" + string(rune(0xFEFF)) + "b", string(rune(0xFEFF)),
	// #39: v1 keywords and sign-digit names
	"-1", "+0x1", "-1.5", "r#", "r#x", "r", "rx#",
	// #40: a leading backtick starts an expression string
	"`x`", "`", "a`b",
	// #41: a comma is an identifier character in v2 only
	",", ",x", "a,b",
}

func TestNameRoundTrip(t *testing.T) {
	for _, s := range nameRoundTripSeeds {
		for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
			for _, ignore := range []bool{false, true} {
				nameRoundTrip(t, s, v, ignore)
			}
		}
	}
}

// FuzzNameRoundTrip checks that any string survives generation and re-parsing as a node name, a property key and an
// argument, in both KDL versions. Run beyond the seeds with: go test -run '^$' -fuzz FuzzNameRoundTrip .
func FuzzNameRoundTrip(f *testing.F) {
	for _, s := range nameRoundTripSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
			for _, ignore := range []bool{false, true} {
				nameRoundTrip(t, s, v, ignore)
			}
		}
	})
}

// #41: in KDL v2 an identifier may start with a comma; in v1 a comma is not an identifier character.
func TestV2CommaIdentifiers(t *testing.T) {
	doc, err := ParseWithOptions(strings.NewReader(",node ,arg ,key=,val a,b"), ParseOptions{Version: ParseVersionV2})
	if err != nil {
		t.Fatal(err)
	}
	n := doc.Nodes[0]
	if got := n.Name.ValueString(); got != ",node" {
		t.Errorf("name = %q", got)
	}
	if got := n.Arguments[0].ValueString(); got != ",arg" {
		t.Errorf("arg = %q", got)
	}
	if got := n.Arguments[1].ValueString(); got != "a,b" {
		t.Errorf("arg 2 = %q", got)
	}
	if v, ok := n.Properties.Get(",key"); !ok || v.ValueString() != ",val" {
		t.Errorf("property ,key = %v, %v", v, ok)
	}

	for _, input := range []string{",node 1", "node ,x", "node k=,x"} {
		if _, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: ParseVersionV1}); err == nil {
			t.Errorf("v1 %q: expected an error", input)
		}
	}
	// a comma glued to a number is still not a valid number or separator
	if _, err := ParseWithOptions(strings.NewReader("node 1,2"), ParseOptions{Version: ParseVersionV2}); err == nil {
		t.Error(`v2 "node 1,2": expected an error`)
	}
}

// #39 and #40: names the generator must quote.
func TestGeneratorQuotesAmbiguousNames(t *testing.T) {
	tests := []struct {
		name string
		v    ParseVersion
		want string
	}{
		{"true", ParseVersionV1, `"true" "true" "true"=1` + "\n"},
		{"null", ParseVersionV1, `"null" "null" "null"=1` + "\n"},
		{"+1", ParseVersionV1, `"+1" "+1" "+1"=1` + "\n"},
		{"-1x", ParseVersionV1, `"-1x" "-1x" "-1x"=1` + "\n"},
		{"`x`", ParseVersionV1, "\"`x`\" \"`x`\" \"`x`\"=1\n"},
		{"`x`", ParseVersionV2, "\"`x`\" \"`x`\" \"`x`\"=1\n"},
		// still bare
		{"inf", ParseVersionV1, `inf "inf" inf=1` + "\n"},
		{".5", ParseVersionV1, `.5 ".5" .5=1` + "\n"},
		{"a`b", ParseVersionV2, "a`b \"a`b\" a`b=1\n"},
		{",x", ParseVersionV2, `,x ",x" ,x=1` + "\n"},
	}
	for _, tt := range tests {
		n := document.NewNode()
		n.SetName(tt.name)
		n.AddArgument(tt.name, "")
		n.AddProperty(tt.name, int64(1), "")
		doc := document.New()
		doc.AddNode(n)
		var b strings.Builder
		if err := GenerateWithOptions(doc, &b, GenerateOptions{Version: tt.v.tokenizerVersion()}); err != nil {
			t.Fatal(err)
		}
		if b.String() != tt.want {
			t.Errorf("v%d %q: got %q, want %q", tt.v, tt.name, b.String(), tt.want)
		}
	}
}
