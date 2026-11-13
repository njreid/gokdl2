package kdl

import (
	"strings"
	"testing"
)

func TestCRLFErrorPositionsMatchLF(t *testing.T) {
	cases := []struct {
		name    string
		version ParseVersion
		lf      string
	}{
		{"v2 plain", ParseVersionV2, "a 1\nb 2\nc }\n"},
		{"v2 multiline string", ParseVersionV2, "a \"\"\"\n  x\n  y\n  \"\"\"\nb }\n"},
		{"v2 error inside multiline string", ParseVersionV2, "a \"\"\"\n  x\n  \\q\n  \"\"\"\n"},
		{"v2 comment", ParseVersionV2, "a 1\n/* one\ntwo\nthree */ b }\n"},
		{"v2 escaped fold", ParseVersionV2, "a \"x\\\n   y\"\nb }\n"},
		{"v1 plain", ParseVersionV1, "a 1\nb 2\nc }\n"},
		{"v1 multiline string", ParseVersionV1, "a \"x\ny\nz\"\nb }\n"},
		{"v1 comment", ParseVersionV1, "a 1\n/* one\ntwo\nthree */ b }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := ParseOptions{Version: tc.version}
			_, lfErr := ParseWithOptions(strings.NewReader(tc.lf), opts)
			if lfErr == nil {
				t.Fatal("expected an error for the LF document")
			}
			crlf := strings.ReplaceAll(tc.lf, "\n", "\r\n")
			_, crlfErr := ParseWithOptions(strings.NewReader(crlf), opts)
			if crlfErr == nil {
				t.Fatal("expected an error for the CRLF document")
			}
			if lfErr.Error() != crlfErr.Error() {
				t.Errorf("error differs\nLF:   %q\nCRLF: %q", lfErr, crlfErr)
			}
			// lone CR documents report the same line as well
			cr := strings.ReplaceAll(tc.lf, "\n", "\r")
			_, crErr := ParseWithOptions(strings.NewReader(cr), opts)
			if crErr == nil || crErr.Error() != lfErr.Error() {
				t.Errorf("lone CR error differs\nLF: %q\nCR: %v", lfErr, crErr)
			}
		})
	}
}
