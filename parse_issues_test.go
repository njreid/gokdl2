package kdl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

func parseV(t *testing.T, v ParseVersion, s string) (nodeType string, argType string, err error) {
	t.Helper()
	doc, err := ParseWithOptions(strings.NewReader(s), ParseOptions{Version: v})
	if err != nil {
		return "", "", err
	}
	n := doc.Nodes[0]
	if len(n.Arguments) > 0 {
		argType = string(n.Arguments[0].Type)
	}
	return string(n.Type), argType, nil
}

func TestQuotedTypeAnnotations(t *testing.T) {
	nt, at, err := parseV(t, ParseVersionV2, `("my type")node ("u8")1`)
	if err != nil || nt != "my type" || at != "u8" {
		t.Fatalf("got %q %q %v", nt, at, err)
	}
	_, at, err = parseV(t, ParseVersionV2, `node (#"raw"#)1`)
	if err != nil || at != "raw" {
		t.Fatalf("raw: %q %v", at, err)
	}
	_, at, err = parseV(t, ParseVersionV1, `node ("u8")1`)
	if err != nil || at != "u8" {
		t.Fatalf("v1: %q %v", at, err)
	}
}

func TestTypeAnnotationQuotedOnGenerate(t *testing.T) {
	doc, err := ParseWithOptions(strings.NewReader(`("my type")node ("u8")1 p=("a b")2`), ParseOptions{Version: ParseVersionV2})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	opts := DefaultGenerateOptions
	opts.Version = tokenizer.VersionV2
	if err := GenerateWithOptions(doc, &buf, opts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`("my type")node`, `(u8)1`, `("a b")2`} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
	doc2, err := ParseWithOptions(strings.NewReader(out), ParseOptions{Version: ParseVersionV2})
	if err != nil {
		t.Fatal(err)
	}
	if doc2.Nodes[0].Type != "my type" {
		t.Errorf("round trip type %q", doc2.Nodes[0].Type)
	}
}
