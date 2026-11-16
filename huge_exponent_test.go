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

// Generating a huge number and parsing it again must give back exactly the same value, pass after pass: the
// formatter writes the shortest mantissa that parses back to the identical big.Float. Before that, "1e2781" was
// written as "1E+2782" and some values drifted by one unit in the last place on every pass.
func TestHugeExponentRoundTripIsStable(t *testing.T) {
	big700 := "1001022" + strings.Repeat("2", 600) + strings.Repeat("0", 93) + ".0"
	for _, src := range []string{"1e2781", "120e-10_1000", big700, "1e1024", "9.999999999999999e1500", "-7.25e-5000", "6.02214076e23000"} {
		gen := func(s string) string {
			doc, err := ParseWithOptions(strings.NewReader("A "+s), ParseOptions{Version: ParseVersionV2})
			if err != nil {
				t.Fatalf("%.30s: %v", s, err)
			}
			var b strings.Builder
			if err := GenerateWithOptions(doc, &b, GenerateOptions{Version: 2}); err != nil {
				t.Fatal(err)
			}
			return strings.TrimPrefix(strings.TrimSpace(b.String()), "A ")
		}
		g1 := gen(src)
		g2 := gen(g1)
		g3 := gen(g2)
		if g1 != g2 || g2 != g3 {
			t.Errorf("%.30s: not stable: %s -> %s -> %s", src, g1, g2, g3)
		}
	}
	if got := func() string {
		doc, _ := ParseWithOptions(strings.NewReader("A 1e2781"), ParseOptions{Version: ParseVersionV2})
		return doc.Nodes[0].Arguments[0].FormattedStringV2()
	}(); got != "1E+2781" {
		t.Errorf("1e2781 formatted as %s", got)
	}
}
