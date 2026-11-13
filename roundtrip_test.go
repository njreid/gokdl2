package kdl

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

// roundTripSkipV2 and roundTripSkipV1 list corpus files that are skipped because the failure lies with the parser,
// not the generator. Each entry must say why.
var roundTripSkipV2 = map[string]string{}

var roundTripSkipV1 = map[string]string{}

func TestRoundTripV2Corpus(t *testing.T) {
	roundTripCorpus(t, filepath.Join("kdl-org-v2", "tests", "test_cases", "input"), ParseVersionV2, tokenizer.VersionV2, roundTripSkipV2)
}

func TestRoundTripV1Corpus(t *testing.T) {
	roundTripCorpus(t, filepath.Join("kdl-org-v1", "tests", "test_cases", "input"), ParseVersionV1, tokenizer.VersionV1, roundTripSkipV1)
}

func roundTripCorpus(t *testing.T, dir string, pv ParseVersion, gv tokenizer.Version, skip map[string]string) {
	files, err := filepath.Glob(filepath.Join(dir, "*.kdl"))
	if err != nil || len(files) == 0 {
		t.Skipf("no corpus in %s", dir)
	}
	parsed := 0
	for _, f := range files {
		name := filepath.Base(f)
		if strings.HasPrefix(name, "_") {
			continue
		}
		if why, ok := skip[name]; ok {
			t.Logf("skipping %s: %s", name, why)
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := ParseWithOptions(bytes.NewReader(data), ParseOptions{Version: pv})
		if err != nil {
			continue // not valid input for this version; covered by the parser tests
		}
		parsed++
		for _, mode := range []struct {
			name string
			opts GenerateOptions
		}{
			{"formatted", GenerateOptions{Indent: "  ", Version: gv}},
			{"ignoreflags", GenerateOptions{Indent: "\t", Version: gv, IgnoreFlags: true}},
		} {
			t.Run(name+"/"+mode.name, func(t *testing.T) {
				var out bytes.Buffer
				if err := GenerateWithOptions(doc, &out, mode.opts); err != nil {
					t.Fatalf("generate: %v", err)
				}
				doc2, err := ParseWithOptions(bytes.NewReader(out.Bytes()), ParseOptions{Version: pv})
				if err != nil {
					t.Fatalf("re-parse failed: %v\ninput:\n%s\ngenerated:\n%s", err, data, out.Bytes())
				}
				if err := compareNodes(doc.Nodes, doc2.Nodes, gv, "/"); err != nil {
					t.Fatalf("not equal: %v\ninput:\n%s\ngenerated:\n%s", err, data, out.Bytes())
				}
			})
		}
	}
	if parsed == 0 {
		t.Fatal("no corpus file parsed")
	}
}

func resolved(v *document.Value, gv tokenizer.Version) interface{} {
	if gv == tokenizer.VersionV2 {
		return v.ResolvedValueV2()
	}
	return v.ResolvedValue()
}

func compareValues(a, b *document.Value, gv tokenizer.Version) error {
	if a == nil || b == nil {
		if a == b {
			return nil
		}
		return fmt.Errorf("nil mismatch")
	}
	if a.Type != b.Type {
		return fmt.Errorf("type %q != %q", a.Type, b.Type)
	}
	x, y := resolved(a, gv), resolved(b, gv)
	switch xv := x.(type) {
	case float64:
		yv, ok := y.(float64)
		if !ok {
			return fmt.Errorf("%T(%v) != %T(%v)", x, x, y, y)
		}
		if math.IsNaN(xv) && math.IsNaN(yv) {
			return nil
		}
		if xv != yv {
			return fmt.Errorf("%v != %v", xv, yv)
		}
	case *big.Int:
		yv, ok := y.(*big.Int)
		if !ok || xv.Cmp(yv) != 0 {
			return fmt.Errorf("%v != %v", x, y)
		}
	case *big.Float:
		yv, ok := y.(*big.Float)
		if !ok || xv.Cmp(yv) != 0 {
			return fmt.Errorf("%v != %v", x, y)
		}
	default:
		if x != y {
			return fmt.Errorf("%T(%#v) != %T(%#v)", x, x, y, y)
		}
	}
	return nil
}

func compareNodes(a, b []*document.Node, gv tokenizer.Version, path string) error {
	if len(a) != len(b) {
		return fmt.Errorf("%s: %d nodes != %d nodes", path, len(a), len(b))
	}
	for i := range a {
		x, y := a[i], b[i]
		p := fmt.Sprintf("%s%d/", path, i)
		if x.Type != y.Type {
			return fmt.Errorf("%s: node type %q != %q", p, x.Type, y.Type)
		}
		if err := compareValues(x.Name, y.Name, gv); err != nil {
			return fmt.Errorf("%s: node name: %w", p, err)
		}
		if len(x.Arguments) != len(y.Arguments) {
			return fmt.Errorf("%s: %d args != %d args", p, len(x.Arguments), len(y.Arguments))
		}
		for j := range x.Arguments {
			if err := compareValues(x.Arguments[j], y.Arguments[j], gv); err != nil {
				return fmt.Errorf("%s: arg %d: %w", p, j, err)
			}
		}
		if x.Properties.Len() != y.Properties.Len() {
			return fmt.Errorf("%s: %d props != %d props", p, x.Properties.Len(), y.Properties.Len())
		}
		for k, xv := range x.Properties.Unordered() {
			yv, ok := y.Properties.Get(k)
			if !ok {
				return fmt.Errorf("%s: missing prop %q", p, k)
			}
			if err := compareValues(xv, yv, gv); err != nil {
				return fmt.Errorf("%s: prop %q: %w", p, k, err)
			}
		}
		if err := compareNodes(x.Children, y.Children, gv, p); err != nil {
			return err
		}
	}
	return nil
}
