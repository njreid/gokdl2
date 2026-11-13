package kdl

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

// trickyStrings are values that cannot be written naively in some string forms.
var trickyStrings = []string{
	"",
	" ",
	"\n",
	"\n\n",
	"a",
	"a\nb",
	"a\n   \nb",
	"a\n \n",
	"a\n   ",
	"   ",
	"\t",
	"a\n\t\nb",
	"\na",
	"a\n",
	"\n a",
	"a\n\nb",
	"  a\n  b",
	"  a\nb",
	"a  \nb",
	"a\r\nb",
	"a\rb",
	"a\r",
	"\r\n",
	"\r",
	"a\r\n\r\nb",
	"a\u0085b",
	"a\u2028b",
	"a\u2029b",
	"a\vb",
	"a\fb",
	"\u2028",
	"a\n\u2028\nb",
	"a\n\u00a0\nb",
	"a\n\u3000b",
	"\u00a0",
	`"""`,
	`a """ b`,
	`"""#`,
	`a """# b`,
	`"""##`,
	`"`,
	`""`,
	`"#`,
	`#"`,
	`a"#b`,
	`a"##b`,
	`"""` + "\n" + `"""`,
	"a\\b",
	`a\`,
	"a\\\nb",
	"a \\\n b",
	"a\\\n\n",
	`\n`,
	`\s`,
	`\u{41}`,
	"tab\tinside",
	"a\x00b",
	"a\x7fb",
	"a\x01b",
	"\ufeffa",
	"a\ufeffb",
	"a\u200eb",
	"a\u202ab",
	"a\u2066b",
	"bad\xffutf8",
	"\ufffd",
	"héllo wörld 日本語 😀",
	"line1\nline2\nline3",
	"  indented\n    more\n  back",
	"trailing space \nnext",
	"a\n \nb\n  \nc",
	"a\n\n\n",
	"\n\n\na",
}

type stringForm struct {
	name string
	flag document.ValueFlag
}

var stringForms = []stringForm{
	{"none", document.FlagNone},
	{"quoted", document.FlagQuoted},
	{"raw", document.FlagRaw},
	{"multiline", document.FlagMultiLine},
	{"raw+multiline", document.FlagRaw | document.FlagMultiLine},
	{"quoted+multiline", document.FlagQuoted | document.FlagMultiLine},
	{"quoted+raw", document.FlagQuoted | document.FlagRaw},
}

// roundTripStringValue generates a node holding s (as an argument and as a property) with the given flag and
// version, re-parses it and returns a description of any mismatch.
func roundTripStringValue(s string, flag document.ValueFlag, gv tokenizer.Version, ignoreFlags bool) error {
	pv := ParseVersionV2
	if gv == tokenizer.VersionV1 {
		pv = ParseVersionV1
	}
	node := document.NewNode()
	node.SetName("n")
	node.AddArgumentValue(&document.Value{Value: s, Flag: flag})
	node.AddPropertyValue("p", &document.Value{Value: s, Flag: flag}, "")
	doc := &document.Document{Nodes: []*document.Node{node}}

	var out bytes.Buffer
	if err := GenerateWithOptions(doc, &out, GenerateOptions{Indent: "  ", Version: gv, IgnoreFlags: ignoreFlags}); err != nil {
		return fmt.Errorf("generate: %v", err)
	}
	doc2, err := ParseWithOptions(bytes.NewReader(out.Bytes()), ParseOptions{Version: pv})
	if err != nil {
		return fmt.Errorf("re-parse failed: %v\ngenerated:\n%s", err, out.Bytes())
	}
	if len(doc2.Nodes) != 1 || len(doc2.Nodes[0].Arguments) != 1 {
		return fmt.Errorf("unexpected structure\ngenerated:\n%s", out.Bytes())
	}
	// invalid UTF-8 cannot be represented; each invalid byte is replaced with U+FFFD
	want := replaceInvalidBytes(s)
	got, ok := doc2.Nodes[0].Arguments[0].Value.(string)
	if !ok {
		return fmt.Errorf("argument is %T, not string\ngenerated:\n%s", doc2.Nodes[0].Arguments[0].Value, out.Bytes())
	}
	if got != want {
		return fmt.Errorf("argument: got %q, want %q\ngenerated:\n%s", got, want, out.Bytes())
	}
	pv2, ok := doc2.Nodes[0].Properties.Get("p")
	if !ok {
		return fmt.Errorf("property missing\ngenerated:\n%s", out.Bytes())
	}
	if ps, _ := pv2.Value.(string); ps != want {
		return fmt.Errorf("property: got %q, want %q\ngenerated:\n%s", ps, want, out.Bytes())
	}
	return nil
}

func TestMultiLineStringRoundTrip(t *testing.T) {
	for _, s := range trickyStrings {
		for _, f := range stringForms {
			for _, gv := range []tokenizer.Version{tokenizer.VersionV2, tokenizer.VersionV1} {
				for _, ignore := range []bool{false, true} {
					name := fmt.Sprintf("%q/%s/v%d/ignore=%v", s, f.name, gv, ignore)
					t.Run(name, func(t *testing.T) {
						if err := roundTripStringValue(s, f.flag, gv, ignore); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

// TestRawMultiLineIssue31 covers the example from the issue.
func TestRawMultiLineIssue31(t *testing.T) {
	if err := roundTripStringValue("a\n   \nb", document.FlagRaw|document.FlagMultiLine, tokenizer.VersionV2, false); err != nil {
		t.Fatal(err)
	}
}

func FuzzStringRoundTrip(f *testing.F) {
	for _, s := range trickyStrings {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, form := range stringForms {
			for _, gv := range []tokenizer.Version{tokenizer.VersionV2, tokenizer.VersionV1} {
				if err := roundTripStringValue(s, form.flag, gv, false); err != nil {
					t.Fatalf("%s v%d: %v", form.name, gv, err)
				}
			}
		}
	})
}

// replaceInvalidBytes replaces each byte of s that is not part of a valid UTF-8 sequence with U+FFFD.
func replaceInvalidBytes(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
	}
	return b.String()
}

// TestRawStringV1ValueString checks the v1 raw string writer used by Value.String, which is not reached by the
// generator (it never emits raw strings for v1) and must delimit values that contain quotes and `"#` sequences.
func TestRawStringV1ValueString(t *testing.T) {
	for _, s := range trickyStrings {
		if !utf8.ValidString(s) || strings.ContainsRune(s, utf8.RuneError) {
			continue // the v1 tokenizer rejects U+FFFD even in raw strings
		}
		for _, f := range stringForms {
			if !f.flag.Has(document.FlagRaw) {
				continue
			}
			v := &document.Value{Value: s, Flag: f.flag}
			text := v.String()
			doc, err := ParseWithOptions(strings.NewReader("n "+text+"\n"), ParseOptions{Version: ParseVersionV1})
			if err != nil {
				t.Errorf("%q/%s: re-parse of %s failed: %v", s, f.name, text, err)
				continue
			}
			if got, _ := doc.Nodes[0].Arguments[0].Value.(string); got != s {
				t.Errorf("%q/%s: got %q from %s", s, f.name, got, text)
			}
		}
	}
}
