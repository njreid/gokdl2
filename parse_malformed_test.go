package kdl

import (
	"math/big"
	"strings"
	"testing"
)

func TestMalformedInputRejected(t *testing.T) {
	inputs := []string{
		"node { a b= }",
		"node { a b=\n}",
		"node { b= }",
		"node { a=1 b= }",
		"node \"\\u{zz}\"",
		"node \"\\u{}\"",
		"node \"\\u{12g}\"",
		"node \"\"\"\n  \\u{zz}\n  \"\"\"",
	}
	for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
		for _, in := range inputs {
			if v == ParseVersionV1 && strings.Contains(in, `"""`) {
				continue
			}
			if _, err := ParseWithOptions(strings.NewReader(in), ParseOptions{Version: v}); err == nil {
				t.Errorf("version %v: %q should be rejected", v, in)
			}
		}
	}
}

func TestValidUnicodeEscapeStillWorks(t *testing.T) {
	doc, err := ParseWithOptions(strings.NewReader(`node "\u{41}\u{1F600}"`), ParseOptions{Version: ParseVersionV2})
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Nodes[0].Arguments[0].Value; got != "A\U0001F600" {
		t.Errorf("got %q", got)
	}
}

func TestSignedPrefixedNumbers(t *testing.T) {
	big1, _ := new(big.Int).SetString("-ffffffffffffffffff", 16)
	tests := []struct {
		in   string
		want interface{}
	}{
		{"-0xff", int64(-255)},
		{"+0xff", int64(255)},
		{"-0o7", int64(-7)},
		{"+0o7", int64(7)},
		{"-0b1", int64(-1)},
		{"+0b101", int64(5)},
		{"-0x_1", nil},
		{"-0xf_f", int64(-255)},
		{"-0xffffffffffffffffff", big1},
	}
	for _, v := range []ParseVersion{ParseVersionV1, ParseVersionV2} {
		for _, tt := range tests {
			doc, err := ParseWithOptions(strings.NewReader("node "+tt.in+"\n"), ParseOptions{Version: v})
			if tt.want == nil {
				if err == nil {
					t.Errorf("v%v %s: expected error", v, tt.in)
				}
				continue
			}
			if err != nil {
				t.Errorf("v%v %s: %v", v, tt.in, err)
				continue
			}
			got := doc.Nodes[0].Arguments[0].Value
			if b, ok := tt.want.(*big.Int); ok {
				gb, ok := got.(*big.Int)
				if !ok || gb.Cmp(b) != 0 {
					t.Errorf("v%v %s: got %v want %v", v, tt.in, got, b)
				}
			} else if got != tt.want {
				t.Errorf("v%v %s: got %v (%T) want %v", v, tt.in, got, got, tt.want)
			}
		}
	}
}
