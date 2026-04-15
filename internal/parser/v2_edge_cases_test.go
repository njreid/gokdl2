package parser

import (
	"strings"
	"testing"

	"github.com/njreid/gokdl2/internal/generator"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

func TestParserV2EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version tokenizer.Version
		wantErr bool
		expect  string
	}{
		{
			name:    "bare identifier as value",
			input:   "node bare-val prop=bare-prop",
			version: tokenizer.VersionV2,
			expect:  "node bare-val prop=bare-prop",
		},
		{
			name:    "triple-quote with leading whitespace",
			input:   "node \"\"\"  \n  content\n  \"\"\"",
			version: tokenizer.VersionV2,
			expect:  "node \"\"\"\ncontent\"\"\"",
		},
		{
			name:    "triple-quote must have newline",
			input:   "node \"\"\"content\"\"\"",
			version: tokenizer.VersionV2,
			wantErr: true,
		},
		{
			name:    "non-greedy raw string",
			input:   "node #\"foo\"# #\"bar\"#",
			version: tokenizer.VersionV2,
			expect:  "node #\"foo\"# #\"bar\"#",
		},
		{
			name:    "complex non-greedy raw string",
			input:   "node ##\"foo\"#\"bar\"##",
			version: tokenizer.VersionV2,
			expect:  "node ##\"foo\"#\"bar\"##",
		},
		{
			name:    "vertical tab as newline",
			input:   "node1\vnode2",
			version: tokenizer.VersionV2,
			expect:  "node1\nnode2",
		},
		{
			name:    "comma rejection",
			input:   "node 1, 2",
			version: tokenizer.VersionV2,
			wantErr: true,
		},
		{
			name:    "raw triple-quote",
			input:   "node #\"\"\"\n    raw\\scontent\n    \"\"\"#",
			version: tokenizer.VersionV2,
			expect:  "node #\"\"\"\nraw\\scontent\n\"\"\"#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tokenizer.NewSlice([]byte(tt.input))
			s.Version = tt.version

			p := New()
			opts := ParseContextOptions{Version: tt.version}
			c := p.NewContextOptions(opts)

			for s.Scan() {
				if err := p.Parse(c, s.Token()); err != nil {
					if tt.wantErr {
						return
					}
					t.Fatalf("unexpected parse error: %v", err)
				}
			}
			if err := s.Err(); err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("unexpected scanner error: %v", err)
			}

			if tt.wantErr {
				t.Fatal("expected error, got success")
			}

			b := strings.Builder{}
			gopts := generator.Options{Version: tt.version, IgnoreFlags: false}
			g := generator.NewOptions(&b, gopts)
			if err := g.Generate(c.doc); err != nil {
				t.Fatalf("failed to generate: %v", err)
			}

			got := strings.TrimSpace(b.String())
			if got != tt.expect {
				t.Errorf("expected:\n%q\ngot:\n%q", tt.expect, got)
			}
		})
	}
}
