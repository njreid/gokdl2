package kdl

import (
	"strings"
	"sync"
	"testing"
)

type issueText string

func (t issueText) MarshalText() ([]byte, error) { return []byte(t), nil }

type issueTextHolder struct {
	V issueText `kdl:"v,arg"`
}

func TestMarshalTextValueWholeToken(t *testing.T) {
	cases := map[string]string{
		"hello world":       `v "hello world"`,
		"10 Downing Street": `v "10 Downing Street"`,
		"true story":        `v "true story"`,
		"42":                `v 42`,
		"true":              `v true`,
		"null":              `v null`,
		"hello":             `v "hello"`,
	}
	for in, want := range cases {
		out, err := Marshal(issueTextHolder{V: issueText(in)})
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		got := strings.TrimSpace(string(out))
		if got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

type innerT struct {
	Alpha int `kdl:"alpha"`
}

type issueDyn struct {
	Alpha int    `kdl:"alpha"`
	Name  string `kdl:"name"`
}

func TestMarshalSameNamedTypes(t *testing.T) {
	type innerT struct{ X, Y, Beta string }
	out, err := Marshal(struct {
		L innerT `kdl:"l"`
		G any    `kdl:"g"`
	}{L: innerT{X: "x", Beta: "b"}, G: pkgInner()})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `beta="b"`) || !strings.Contains(s, "alpha=4") {
		t.Errorf("types shared field tables: %s", s)
	}
}

func pkgInner() any { return innerT{Alpha: 4} }

func TestMarshalDynamicTypes(t *testing.T) {
	out, err := Marshal(map[string]any{"k": issueDyn{Alpha: 7, Name: "n"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "alpha=7") {
		t.Errorf("map value: %s", out)
	}
	out, err = Marshal(struct {
		A any `kdl:"a"`
	}{A: &issueDyn{Alpha: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "alpha=3") {
		t.Errorf("field: %s", out)
	}
}

func TestUnmarshalIntoAny(t *testing.T) {
	var m map[string]any
	if err := Unmarshal([]byte("a 1\nb x=2"), &m); err != nil {
		t.Fatal(err)
	}
	var s struct {
		A any `kdl:"a"`
	}
	if err := Unmarshal([]byte(`a 1`), &s); err != nil {
		t.Fatal(err)
	}
	var s2 struct {
		A any `kdl:"a"`
	}
	s2.A = &issueDyn{}
	_ = Unmarshal([]byte("a alpha=5 name=\"q\""), &s2)
}

func TestConcurrentMarshalUnmarshal(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				out, err := Marshal(map[string]any{"k": issueDyn{Alpha: j}, "t": innerT{Alpha: j}})
				if err != nil {
					t.Error(err)
					return
				}
				var m map[string]any
				if err := Unmarshal(out, &m); err != nil {
					t.Error(err)
					return
				}
				var d struct {
					A issueDyn `kdl:"a"`
				}
				if err := Unmarshal([]byte("a alpha=1 name=x"), &d); err != nil || d.A.Alpha != 1 {
					t.Error(err, d)
					return
				}
			}
		}()
	}
	wg.Wait()
}
