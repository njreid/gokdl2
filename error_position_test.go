package kdl

import (
	"strings"
	"testing"
)

// errPosCase describes where an error for a malformed document must be reported. line and col are 1-based. The
// documents are written with LF; each is also run with CRLF line endings, which must give identical positions.
type errPosCase struct {
	name    string
	input   string
	version ParseVersion
	msg     string // substring of the message
	line    int
	col     int
}

// The rule, matching kdl-rs: an error is reported at the start of the offending token. Exceptions: an error inside a
// string that the tokenizer detects (a newline in a v2 single-line string, "\/" in v2) and invalid UTF-8 point at the
// offending character; a bad escape is only detected when the string value is decoded, and is reported at the start of
// the string token. An unterminated string is reported at its opening quote, an unclosed '{' at the '{', and a v1
// bare identifier that is not followed by '=' at the identifier.
var errPosCases = []errPosCase{
	// tokenizer number/identifier errors
	{"v2 number suffix", "a 1\nnode 1asdf 2\n", ParseVersionV2, "", 2, 6},
	{"v2 hex suffix", "a 1\nnode 0x1asdf 2\n", ParseVersionV2, "", 2, 6},
	{"v2 float suffix", "a 1\nnode 1.0easdf 2\n", ParseVersionV2, "", 2, 6},
	{"v2 slash in identifier", "a 1\nno/de 1\n", ParseVersionV2, "", 2, 1},
	{"v1 slash in identifier", "a 1\nno/de 1\n", ParseVersionV1, "", 2, 1},

	// parser errors
	{"v2 unclosed block", "a 1\nnode {\n  x 1\n", ParseVersionV2, "unclosed child block", 2, 6},
	{"v1 unclosed block", "a 1\nnode {\n  x 1\n", ParseVersionV1, "unclosed child block", 2, 6},
	{"v2 nested unclosed block", "a {\n  b {\n    c 1\n  }\n", ParseVersionV2, "unclosed child block", 1, 3},
	{"v2 stray close", "a 1\nnode 1\n}\n", ParseVersionV2, "", 3, 1},
	{"v1 stray close", "a 1\nnode 1\n}\n", ParseVersionV1, "", 3, 1},
	{"v2 missing value after =", "a 1\nnode k=\n", ParseVersionV2, "expected a property value", 2, 8},
	{"v1 missing value after =", "a 1\nnode k=\n", ParseVersionV1, "expected a property value", 2, 8},
	{"v1 bare identifier argument", "a 1\nnode \"ok\" wait \"fine\"\n", ParseVersionV1, "expected '='", 2, 11},
	{"v1 bare identifier argument at EOF", "a 1\nbad evil", ParseVersionV1, "expected '='", 2, 5},
	{"v2 missing separator", "a 1\nnode \"foo\"1\n", ParseVersionV2, "missing whitespace", 2, 11},

	// errors inside strings
	{"v2 bad escape", "a 1\nnode \"ab\\qcd\"\n", ParseVersionV2, "invalid quoted string", 2, 6},
	{"v1 bad escape", "a 1\nnode \"ab\\qcd\"\n", ParseVersionV1, "invalid quoted string", 2, 6},
	{"v2 escaped slash", "a 1\nnode \"a\\/b\"\n", ParseVersionV2, "unexpected character /", 2, 8},
	{"v2 newline in string", "a 1\nnode \"ab\ncd\"\n", ParseVersionV2, "unexpected newline in string", 2, 9},
	{"v2 unterminated string", "a 1\nnode \"abc", ParseVersionV2, "unterminated string", 2, 6},
	{"v1 unterminated string", "a 1\nnode \"abc", ParseVersionV1, "unterminated string", 2, 6},
	{"v2 unterminated string then newline", "a 1\nnode \"abc\n", ParseVersionV2, "", 2, 10},

	// invalid UTF-8 points at the bad byte
	{"v2 invalid UTF-8 between tokens", "a 1\nnode \xff 1\n", ParseVersionV2, "invalid UTF8", 2, 6},
	{"v1 invalid UTF-8 between tokens", "a 1\nnode \xff 1\n", ParseVersionV1, "invalid UTF8", 2, 6},
	{"v2 invalid UTF-8 in string", "a 1\nnode \"ab\xffcd\"\n", ParseVersionV2, "invalid UTF8", 2, 9},
	{"v1 invalid UTF-8 in string", "a 1\nnode \"ab\xffcd\"\n", ParseVersionV1, "invalid UTF8", 2, 9},
}

func TestErrorPositions(t *testing.T) {
	for _, tc := range errPosCases {
		for _, eol := range []string{"\n", "\r\n"} {
			name := tc.name
			if eol == "\r\n" {
				name += " CRLF"
			}
			t.Run(name, func(t *testing.T) {
				input := strings.ReplaceAll(tc.input, "\n", eol)
				_, err := ParseWithOptions(strings.NewReader(input), ParseOptions{Version: tc.version})
				if err == nil {
					t.Fatalf("expected an error for %q", input)
				}
				msg := err.Error()
				if tc.msg != "" && !strings.Contains(msg, tc.msg) {
					t.Errorf("error %q does not contain %q", msg, tc.msg)
				}
				if want := "at line " + itoa(tc.line) + ", column " + itoa(tc.col) + "\n"; !strings.Contains(msg, want) {
					t.Errorf("error %q does not contain %q", msg, want)
				}
				checkCaret(t, msg, strings.Split(strings.ReplaceAll(tc.input, "\r", ""), "\n")[tc.line-1], tc.col)
			})
		}
	}
}

// checkCaret checks the source excerpt that follows the first line of an error message: the offending source line
// and a caret under column col of it.
func checkCaret(t *testing.T, msg, srcLine string, col int) {
	t.Helper()
	lines := strings.Split(msg, "\n")
	if len(lines) < 3 {
		t.Fatalf("error has no source excerpt: %q", msg)
	}
	excerpt, caret := lines[len(lines)-2], lines[len(lines)-1]
	if strings.TrimRight(excerpt, " ") != strings.TrimRight(strings.ReplaceAll(srcLine, "\xff", "\xff"), " ") && !strings.HasPrefix(excerpt, "...") {
		t.Errorf("excerpt %q does not show source line %q", excerpt, srcLine)
	}
	if strings.HasPrefix(excerpt, "...") {
		return
	}
	if strings.TrimRight(caret, " ") != caret || caret != strings.Repeat(" ", col-1)+"^" {
		t.Errorf("caret %q is not under column %d of %q", caret, col, srcLine)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
