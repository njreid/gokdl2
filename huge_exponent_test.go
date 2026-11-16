package kdl

import (
	"strings"
	"testing"
	"time"
)

// Numbers far outside the float64 range used to make math/big's decimal conversion run for minutes (a 13-byte
// document could hang the parser or generator). They must parse, generate and round-trip quickly.
func TestHugeExponentNumbers(t *testing.T) {
	tests := []struct{ in, out string }{
		{"1e-400", "1E-400"},
		{"1.5e400", "1.5E+400"},
		{"1.2345678901234567e999", "1.2345678901234567E+999"},
		{"-2.5e-10000", "-2.5E-10000"},
		{"6.02214076e9999", "6.02214076E+9999"},
		{"3.25e123456", "3.25E+123456"},
		{"1e-10210200", "1E-10210200"},
		{"-1e10210200", "-1E+10210200"},
		{"9.99999999999999999e99999", "1E+100000"},
		{"1e646000001", "#inf"},
		{"-1e646000001", "#-inf"},
		{"7e999999999999999999999", "#inf"},
		{"1e-999999999999999999999", "0.0"},
		{"-1e-999999999999999999999", "-0.0"},
	}
	start := time.Now()
	for _, tt := range tests {
		doc, err := ParseWithOptions(strings.NewReader("n "+tt.in), ParseOptions{Version: ParseVersionV2})
		if err != nil {
			t.Errorf("%s: %v", tt.in, err)
			continue
		}
		var b strings.Builder
		if err := GenerateWithOptions(doc, &b, GenerateOptions{Version: 2}); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(b.String()); got != "n "+tt.out {
			t.Errorf("%s: generated %q, want %q", tt.in, got, "n "+tt.out)
		}
		back, err := ParseWithOptions(strings.NewReader(b.String()), ParseOptions{Version: ParseVersionV2})
		if err != nil {
			t.Errorf("%s: re-parse of %q: %v", tt.in, b.String(), err)
		} else if back.Nodes[0].Arguments[0].ValueString() != doc.Nodes[0].Arguments[0].ValueString() {
			t.Errorf("%s: value changed on round trip", tt.in)
		}
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("parsing and generating huge exponents took %v; something is converting them digit by digit", d)
	}
}
