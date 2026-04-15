package kdl

import (
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

func TestParseGenerateExpressionStrings(t *testing.T) {
	input := "rule `request.auth.claims.sub` match=```\nrequest.auth != nil\n```\n"

	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	arg, ok := doc.Nodes[0].Arguments[0].ResolvedValue().(document.Expression)
	if !ok {
		t.Fatalf("argument type = %T, want document.Expression", doc.Nodes[0].Arguments[0].ResolvedValue())
	}
	if arg != document.Expression("request.auth.claims.sub") {
		t.Fatalf("argument = %q", arg)
	}

	prop, ok := doc.Nodes[0].Properties.Get("match")
	if !ok {
		t.Fatal("missing match property")
	}
	if _, ok := prop.ResolvedValue().(document.Expression); !ok {
		t.Fatalf("property type = %T, want document.Expression", prop.ResolvedValue())
	}

	var b strings.Builder
	if err := Generate(doc, &b); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := b.String(); got != input {
		t.Fatalf("Generate() = %q, want %q", got, input)
	}
}

func TestMarshalExpressionString(t *testing.T) {
	type config struct {
		Match document.Expression `kdl:"match"`
	}

	b, err := Marshal(config{Match: document.Expression("request.auth != nil")})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got := string(b); got != "match `request.auth != nil`\n" {
		t.Fatalf("Marshal() = %q", got)
	}
}

func TestUnmarshalExpressionIntoInterface(t *testing.T) {
	type config struct {
		Match interface{} `kdl:"match"`
	}

	var got config
	if err := Unmarshal([]byte("match `request.auth != nil`\n"), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if _, ok := got.Match.(document.Expression); !ok {
		t.Fatalf("Match type = %T, want document.Expression", got.Match)
	}
}
