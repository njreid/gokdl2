package kdl

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/njreid/gokdl2/document"
)

type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func TestParseReaderError(t *testing.T) {
	boom := errors.New("disk on fire")
	for _, v := range []ParseVersion{ParseVersionAuto, ParseVersionV1, ParseVersionV2} {
		doc, err := ParseWithOptions(&failingReader{data: []byte("node 1\nnode 2\n"), err: boom}, ParseOptions{Version: v})
		if !errors.Is(err, boom) || doc != nil {
			t.Errorf("version %d: got doc=%v err=%v, want read error", v, doc, err)
		}
	}
	var v struct{}
	if err := NewDecoder(&failingReader{data: []byte("a 1\n"), err: boom}).Decode(&v); !errors.Is(err, boom) {
		t.Errorf("Decode: got %v", err)
	}
}

func TestParseEmptyReader(t *testing.T) {
	for _, r := range []io.Reader{strings.NewReader(""), io.MultiReader()} {
		doc, err := Parse(r)
		if err != nil || len(doc.Nodes) != 0 {
			t.Fatalf("got %v, %v", doc, err)
		}
	}
}

func TestUnmarshalWithOptionsHonoursOptions(t *testing.T) {
	type S struct {
		A int `kdl:"a"`
	}
	var s S
	if err := Unmarshal([]byte("a 1\nb 2\n"), &s); err == nil {
		t.Fatal("expected unhandled node error")
	}
	if err := UnmarshalWithOptions([]byte("a 1\nb 2\n"), &s, UnmarshalOptions{AllowUnhandledNodes: true}); err != nil || s.A != 1 {
		t.Fatalf("got %v, %+v", err, s)
	}
	s = S{}
	d := NewDecoder(strings.NewReader("a 1\nb 2\n"))
	d.Options.AllowUnhandledNodes = true
	if err := d.Decode(&s); err != nil || s.A != 1 {
		t.Fatalf("decode: %v, %+v", err, s)
	}
}

func TestAutoVersionMarker(t *testing.T) {
	if _, err := Parse(strings.NewReader("/- kdl-version 1\nnode #true\n")); err == nil {
		t.Error("v1 marker followed by v2 syntax should fail")
	}
	if _, err := Parse(strings.NewReader("/- kdl-version 2\nnode true\n")); err == nil {
		t.Error("v2 marker followed by v1 syntax should fail")
	}
	doc, err := Parse(strings.NewReader("/- kdl-version 1\nnode true\n"))
	if err != nil || doc.Version != 1 {
		t.Errorf("v1 marker: %v %v", doc, err)
	}
	doc, err = Parse(strings.NewReader("/- kdl-version 2\nnode #true\n"))
	if err != nil || doc.Version != 2 {
		t.Errorf("v2 marker: %v %v", doc, err)
	}
}

func TestAutoVersionFallbackAndError(t *testing.T) {
	_, err := Parse(strings.NewReader("node \"unterminated\nnode2 {\n"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "KDL v1") {
		t.Errorf("error should mention the v1 attempt: %v", err)
	}
	doc, err := Parse(strings.NewReader("node true\n"))
	if err != nil || doc.Version != 1 {
		t.Fatalf("v1 fallback: %v %v", doc, err)
	}
}

func TestInputMutationAfterParse(t *testing.T) {
	const input = "name \"hello world\" key=\"some value\" 12\nother #true\n"

	src := []byte(input)
	var m map[string]interface{}
	if err := Unmarshal(src, &m); err != nil {
		t.Fatal(err)
	}
	var want map[string]interface{}
	if err := Unmarshal([]byte(input), &want); err != nil {
		t.Fatal(err)
	}
	for i := range src {
		src[i] = 'X'
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("Unmarshal result changed after mutating input:\n got %v\nwant %v", m, want)
	}

	src2 := []byte(input)
	doc, err := Parse(bytes.NewReader(src2))
	if err != nil {
		t.Fatal(err)
	}
	for i := range src2 {
		src2[i] = 'X'
	}
	n := doc.Nodes[0]
	if n.Name.ValueString() != "name" || n.Arguments[0].ValueString() != "hello world" ||
		propString(n, "key") != "some value" {
		t.Errorf("parsed document changed after mutating input: %v", doc)
	}
}

func propString(n *document.Node, key string) string {
	v, _ := n.Properties.Get(key)
	return v.ValueString()
}
