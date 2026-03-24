package kdl

import (
	"strings"
	"testing"
)

func TestParseVersionSelection(t *testing.T) {
	t.Run("auto falls back to v1", func(t *testing.T) {
		doc, err := Parse(strings.NewReader("node r\"raw\"\n"))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if doc.Version != 1 {
			t.Fatalf("doc.Version = %d, want 1", doc.Version)
		}
	})

	t.Run("auto prefers v2", func(t *testing.T) {
		doc, err := Parse(strings.NewReader("node .foo\n"))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if doc.Version != 2 {
			t.Fatalf("doc.Version = %d, want 2", doc.Version)
		}
	})

	t.Run("v2 rejects v1 only syntax", func(t *testing.T) {
		_, err := ParseWithOptions(strings.NewReader("node r\"raw\"\n"), ParseOptions{Version: ParseVersionV2})
		if err == nil {
			t.Fatal("ParseWithOptions() error = nil, want error")
		}
	})

	t.Run("v1 rejects v2 only syntax", func(t *testing.T) {
		_, err := ParseWithOptions(strings.NewReader("node .foo\n"), ParseOptions{Version: ParseVersionV1})
		if err == nil {
			t.Fatal("ParseWithOptions() error = nil, want error")
		}
	})

	t.Run("explicit versions succeed", func(t *testing.T) {
		v1Doc, err := ParseWithOptions(strings.NewReader("node r\"raw\"\n"), ParseOptions{Version: ParseVersionV1})
		if err != nil {
			t.Fatalf("ParseWithOptions(v1) error = %v", err)
		}
		if v1Doc.Version != 1 {
			t.Fatalf("v1Doc.Version = %d, want 1", v1Doc.Version)
		}

		v2Doc, err := ParseWithOptions(strings.NewReader("node .foo\n"), ParseOptions{Version: ParseVersionV2})
		if err != nil {
			t.Fatalf("ParseWithOptions(v2) error = %v", err)
		}
		if v2Doc.Version != 2 {
			t.Fatalf("v2Doc.Version = %d, want 2", v2Doc.Version)
		}
	})
}
