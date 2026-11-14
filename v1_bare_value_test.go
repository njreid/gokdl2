package kdl

import (
	"strings"
	"testing"

	"github.com/njreid/gokdl2/relaxed"
)

// Issue #33: in KDL v1 a bare identifier is not a valid property value.
func TestV1BareIdentifierPropertyValue(t *testing.T) {
	for _, input := range []string{"node x=bad", "node x=(int)bad", "node a=1 x=bad", "node {\n child x=bad\n}"} {
		if _, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: ParseVersionV1}); err == nil {
			t.Errorf("v1 %q: expected an error", input)
		}
	}
	// still valid: v1 keywords and strings as values, v2 bare identifiers, and relaxed NGINX mode
	valid := []struct {
		input string
		opts  ParseOptions
	}{
		{"node x=true y=null z=\"s\" w=1", ParseOptions{Version: ParseVersionV1}},
		{"node x=bad", ParseOptions{Version: ParseVersionV2}},
		{"node x=(int)bad", ParseOptions{Version: ParseVersionV2}},
		{"node x=bad", ParseOptions{Version: ParseVersionV1, RelaxedNonCompliant: relaxed.NGINXSyntax}},
	}
	for _, tt := range valid {
		if _, err := ParseWithOptions(strings.NewReader(tt.input), tt.opts); err != nil {
			t.Errorf("%q with %+v: %v", tt.input, tt.opts, err)
		}
	}
	// auto mode no longer silently reinterprets v1-invalid input: it parses as v2, where bare values are allowed
	doc, err := Parse(strings.NewReader("node x=bad"))
	if err != nil || doc.Version != 2 {
		t.Errorf("auto: version=%v err=%v", doc, err)
	}
}
