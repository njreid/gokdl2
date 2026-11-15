package parser

import (
	"testing"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

// Issue #28: in KDL v2, a type-annotated value following another argument must parse.
func TestV2TypedArgumentAfterArgument(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		args    []string // expected argument values with their type annotations
	}{
		{input: "node 1 (u8)2", args: []string{"1", "(u8)2"}},
		{input: "node (u8)1 (u8)2", args: []string{"(u8)1", "(u8)2"}},
		{input: `node "a" (t)"b"`, args: []string{`"a"`, `(t)"b"`}},
		{input: "node a (t)b", args: []string{"a", "(t)b"}},
		{input: "node a (t) b", args: []string{"a", "(t)b"}},
		{input: "node /-1 (t)2 3", args: []string{"(t)2", "3"}},
		{input: "node 1 /-(t)2 3", args: []string{"1", "3"}},
		{input: "node 1 (t)2 k=(u)3", args: []string{"1", "(t)2"}},
		{input: "node (t)2 (u)3 {\n child\n}", args: []string{"(t)2", "(u)3"}},
		{input: "node 1(u8)2", wantErr: true},
		{input: "node 1 (u8)", wantErr: true},
		{input: "node 1 (u8)(u8)2", wantErr: true},
		{input: "node a (t)=1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s := tokenizer.NewSlice([]byte(tt.input))
			s.Version = tokenizer.VersionV2
			tokens, err := s.ScanAll()
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("scan: %v", err)
				}
				return
			}
			p := New()
			c := p.NewContextOptions(ParseContextOptions{Version: tokenizer.VersionV2})
			doc, err := p.ParseAllContext(c, tokens)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %d node(s)", len(doc.Nodes))
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := doc.Nodes[0].Arguments
			if len(got) != len(tt.args) {
				t.Fatalf("got %d args, want %d", len(got), len(tt.args))
			}
			for i, a := range got {
				if s := a.FormattedStringV2(); s != tt.args[i] {
					t.Errorf("arg %d = %s, want %s", i, s, tt.args[i])
				}
			}
		})
	}
}
