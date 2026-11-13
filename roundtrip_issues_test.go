package kdl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

func TestRoundTripGeneratorIssues(t *testing.T) {
	inputs := []string{
		"node -0xff +0b101 -0o17 key=-0x10\n",
		"node (u8)0xff\nnode (i8)-0xff\n",
		"node \"\"\"\n  a\n   b\n  \"\"\"\n",
		"node #\"\"\"\n  a\n   b\n  \"\"\"#\n",
		"node \"\"\"\n  trailing\n\n  \"\"\"\n",
		"node \"a\\u{1}b\\u{7f}c\\u{1b}d\\u{fffd}\"\n",
		"node (\"my type\")1 k=(\"\")2\n",
	}
	for _, in := range inputs {
		doc, err := ParseWithOptions(strings.NewReader(in), ParseOptions{Version: ParseVersionV2})
		if err != nil {
			t.Errorf("parse %q: %v", in, err)
			continue
		}
		var out bytes.Buffer
		if err := GenerateWithOptions(doc, &out, GenerateOptions{Indent: "  ", Version: tokenizer.VersionV2}); err != nil {
			t.Fatal(err)
		}
		doc2, err := ParseWithOptions(bytes.NewReader(out.Bytes()), ParseOptions{Version: ParseVersionV2})
		if err != nil {
			t.Errorf("re-parse of %q failed: %v\ngenerated: %q", in, err, out.String())
			continue
		}
		if err := compareNodes(doc.Nodes, doc2.Nodes, tokenizer.VersionV2, "/"); err != nil {
			t.Errorf("%q: %v\ngenerated: %q", in, err, out.String())
		}
	}
}

func TestGenerateInvalidUTF8(t *testing.T) {
	n := document.NewNode()
	n.SetName("node")
	n.AddArgument("a\xffb", "")
	doc := document.New()
	doc.AddNode(n)
	var out bytes.Buffer
	if err := GenerateWithOptions(doc, &out, GenerateOptions{Version: tokenizer.VersionV2}); err != nil {
		t.Fatal(err)
	}
	doc2, err := ParseWithOptions(bytes.NewReader(out.Bytes()), ParseOptions{Version: ParseVersionV2})
	if err != nil {
		t.Fatalf("%v: %q", err, out.String())
	}
	if got := doc2.Nodes[0].Arguments[0].Value; got != "a�b" {
		t.Errorf("got %q", got)
	}
}
