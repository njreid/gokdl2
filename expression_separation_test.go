package kdl

import (
	"strings"
	"testing"
)

// In KDL v2 values must be separated by whitespace; backtick expression strings follow the same rule as other
// values (v1 has no such check).
func TestV2ExpressionStringsNeedSeparation(t *testing.T) {
	for _, input := range []string{
		"rule `a``b`",
		"rule \"a\"`b`",
		"rule `a`\"b\"",
		"rule `a`1",
		"rule 1`a`",
		"rule `a`#true",
		"rule `a`(t)`b`",
		"rule `a`k=1",
	} {
		if _, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: ParseVersionV2}); err == nil {
			t.Errorf("v2 %q: expected a missing-separator error", input)
		} else if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("v2 %q: unexpected error %v", input, err)
		}
	}
	for _, input := range []string{
		"rule `a` `b`",
		"rule `a`;next `b`",
		"rule `a`/* c */ `b`",
		"rule `a`// c",
		"rule `a`/-`b` `c`",
		"rule (t)`a` (u)`b`",
		"rule k=`a` j=`b`",
		"n {\n  r `a`}",
		"rule ```\n  a\n  ``` `b`",
	} {
		if _, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: ParseVersionV2}); err != nil {
			t.Errorf("v2 %q: %v", input, err)
		}
	}
}
