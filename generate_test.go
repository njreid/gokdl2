package kdl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

func TestGenerateV2NamesRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		bare bool
	}{
		{"node", true},
		{"--flag", true},
		{"é", true},
		{"true", false},
		{"false", false},
		{"null", false},
		{"inf", false},
		{"-inf", false},
		{"nan", false},
		{"+1", false},
		{"-1x", false},
		{".5", false},
		{"-.5", false},
		{"+.5", false},
		{"a\uFEFFb", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := document.NewNode()
			n.SetName(tt.name)
			n.AddProperty(tt.name, int64(1), "")
			doc := document.New()
			doc.AddNode(n)

			var buf bytes.Buffer
			opts := GenerateOptions{Version: tokenizer.VersionV2}
			if err := GenerateWithOptions(doc, &buf, opts); err != nil {
				t.Fatalf("GenerateWithOptions() error = %v", err)
			}
			out := buf.String()
			if got := strings.HasPrefix(out, tt.name+" "); got != tt.bare {
				t.Fatalf("GenerateWithOptions() = %q, want bare name: %v", out, tt.bare)
			}

			got, err := ParseWithOptions(strings.NewReader(out), ParseOptions{Version: ParseVersionV2})
			if err != nil {
				t.Fatalf("ParseWithOptions(%q) error = %v", out, err)
			}
			if len(got.Nodes) != 1 {
				t.Fatalf("ParseWithOptions(%q) gave %d nodes, want 1", out, len(got.Nodes))
			}
			if name := got.Nodes[0].Name.ValueString(); name != tt.name {
				t.Errorf("node name = %q, want %q", name, tt.name)
			}
			if _, ok := got.Nodes[0].Properties.Get(tt.name); !ok {
				t.Errorf("property %q not found in %q", tt.name, out)
			}
		})
	}
}

func TestParseV2RejectsInvalidIdentifiers(t *testing.T) {
	for _, src := range []string{
		"-.5x 1\n",
		"+.5x 1\n",
		"a\uFEFFb 1\n",
		"node -.5x=1\n",
	} {
		if _, err := ParseWithOptions(strings.NewReader(src), ParseOptions{Version: ParseVersionV2}); err == nil {
			t.Errorf("ParseWithOptions(%q) error = nil, want error", src)
		}
	}
}
