package tokenizer

import "testing"

func TestScanExpressionString(t *testing.T) {
	tok, err := ScanOne([]byte("`request.auth.claims.sub`"))
	if err != nil {
		t.Fatalf("ScanOne() error = %v", err)
	}
	if tok.ID != ExpressionString {
		t.Fatalf("ScanOne() token = %s, want %s", tok.ID, ExpressionString)
	}
}

func TestScanMultilineExpressionString(t *testing.T) {
	tok, err := ScanOne([]byte("```\nrequest.auth != nil\n```"))
	if err != nil {
		t.Fatalf("ScanOne() error = %v", err)
	}
	if tok.ID != ExpressionString {
		t.Fatalf("ScanOne() token = %s, want %s", tok.ID, ExpressionString)
	}
}
