package kdl

import (
	"strings"
	"testing"
)

func TestQuotedStringErrors(t *testing.T) {
	tests := []struct {
		name, in string
		ver      ParseVersion
		want     string
	}{
		{"newline v2", "node \"ab\ncd\"\n", ParseVersionV2, "unexpected newline in string at line 1, column 9"},
		{"newline in children v2", "node {\n  a \"x\ny\"\n}", ParseVersionV2, "unexpected newline in string at line 2, column 7"},
		{"unterminated v2", "node \"abc", ParseVersionV2, "unterminated string at line 1, column 6"},
		{"unterminated v1", "node \"abc", ParseVersionV1, "unterminated string at line 1, column 6"},
		{"escaped slash v2", "node \"a\\/b\"", ParseVersionV2, "unexpected character / at line 1, column 8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseWithOptions(strings.NewReader(tt.in), ParseOptions{Version: tt.ver})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}
