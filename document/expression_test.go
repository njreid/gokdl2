package document

import (
	"testing"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

func TestValueFromExpressionToken(t *testing.T) {
	v, err := ValueFromToken(tokenizer.Token{ID: tokenizer.ExpressionString, Data: []byte("`request.auth.claims.sub`")})
	if err != nil {
		t.Fatalf("ValueFromToken() error = %v", err)
	}

	expr, ok := v.ResolvedValue().(Expression)
	if !ok {
		t.Fatalf("ResolvedValue() type = %T, want document.Expression", v.ResolvedValue())
	}
	if expr != Expression("request.auth.claims.sub") {
		t.Fatalf("ResolvedValue() = %q", expr)
	}
	if got := v.StringV2(); got != "`request.auth.claims.sub`" {
		t.Fatalf("StringV2() = %q", got)
	}
}

func TestValueFromMultilineExpressionToken(t *testing.T) {
	v, err := ValueFromToken(tokenizer.Token{ID: tokenizer.ExpressionString, Data: []byte("```\nfoo &&\n  bar\n```")})
	if err != nil {
		t.Fatalf("ValueFromToken() error = %v", err)
	}

	expr, ok := v.ResolvedValue().(Expression)
	if !ok {
		t.Fatalf("ResolvedValue() type = %T, want document.Expression", v.ResolvedValue())
	}
	if expr != Expression("foo &&\n  bar") {
		t.Fatalf("ResolvedValue() = %q", expr)
	}
	if got := v.StringV2(); got != "```\nfoo &&\n  bar\n```" {
		t.Fatalf("StringV2() = %q", got)
	}
}
