package tokenizer

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

type posTok struct {
	ID           TokenID
	Data         string
	Line, Column int
}

func nonNewlineTokens(t *testing.T, src string, v Version, stream bool) []posTok {
	t.Helper()
	var s *Scanner
	if stream {
		s = NewBuffer(iotest.OneByteReader(strings.NewReader(src)), make([]byte, 16))
	} else {
		s = NewSlice([]byte(src))
	}
	s.Version = v
	var out []posTok
	for s.Scan() {
		tok := s.Token()
		if tok.ID == Newline || tok.ID == Whitespace {
			continue
		}
		data := strings.ReplaceAll(string(tok.Data), "\r\n", "\n")
		out = append(out, posTok{tok.ID, data, tok.Line, tok.Column})
		if tok.ID == EOF {
			break
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		t.Fatalf("scan error: %v", err)
	}
	return out
}

func TestCRLFTokenPositionsMatchLF(t *testing.T) {
	docs := map[Version]string{
		VersionV2: "node1 \"a\" 1\nnode2 \"\"\"\n  multi\n  line\n  \"\"\"\n/* c1\nc2 */ node3 \"q\\\n   z\" {\n  child 1 // x\n  /- skipped\n}\n#\"\"\"\n  raw\n  str\n  \"\"\"# end\n",
		VersionV1: "node1 \"a\" 1\nnode2 \"multi\nline\" r\"raw\nraw\"\n/* c1\nc2 */ node3 \"q\\\n   z\" {\n  child 1 // x\n  /-skipped\n}\nend\n",
	}
	for v, lf := range docs {
		crlf := strings.ReplaceAll(lf, "\n", "\r\n")
		var mixed strings.Builder
		for i, line := range strings.SplitAfter(lf, "\n") {
			if i%2 == 0 {
				line = strings.ReplaceAll(line, "\n", "\r\n")
			}
			mixed.WriteString(line)
		}
		for _, stream := range []bool{false, true} {
			want := nonNewlineTokens(t, lf, v, stream)
			if len(want) < 10 {
				t.Fatalf("v%d: too few tokens: %d", v, len(want))
			}
			for name, src := range map[string]string{"crlf": crlf, "mixed": mixed.String()} {
				got := nonNewlineTokens(t, src, v, stream)
				if len(got) != len(want) {
					t.Fatalf("v%d %s stream=%v: token count %d != %d", v, name, stream, len(got), len(want))
				}
				for i := range want {
					if got[i] != want[i] {
						t.Errorf("v%d %s stream=%v token %d: got %+v want %+v", v, name, stream, i, got[i], want[i])
					}
				}
			}
		}
	}
}

func TestLoneCRCountsAsOneLine(t *testing.T) {
	for _, nl := range []string{"\r", "\n", "\v", "\f", "\u0085", " ", " "} {
		for _, stream := range []bool{false, true} {
			toks := nonNewlineTokens(t, "a"+nl+"b"+nl+nl+"c", VersionV2, stream)
			var lines []int
			for _, tk := range toks {
				if tk.ID == BareIdentifier {
					lines = append(lines, tk.Line)
				}
			}
			if len(lines) != 3 || lines[1] != lines[0]+1 || lines[2] != lines[0]+3 {
				t.Errorf("%q stream=%v: lines %v", nl, stream, lines)
			}
		}
	}
	// \r\r\n is a lone \r followed by a CRLF pair: two lines.
	toks := nonNewlineTokens(t, "a\r\r\nb", VersionV2, false)
	if got := toks[len(toks)-2]; got.Line != toks[0].Line+2 {
		t.Errorf("\\r\\r\\n: b on line %d, a on %d", got.Line, toks[0].Line)
	}
	// \n\r is two line breaks.
	toks = nonNewlineTokens(t, "a\n\rb", VersionV2, false)
	if got := toks[len(toks)-2]; got.Line != toks[0].Line+2 {
		t.Errorf("\\n\\r: b on line %d, a on %d", got.Line, toks[0].Line)
	}
}

func TestExtractLineAtPositionCRLF(t *testing.T) {
	lf := NewSlice([]byte("first\nsecond line\nthird\n"))
	crlf := NewSlice([]byte("first\r\nsecond line\r\nthird\r\n"))
	for line := 1; line <= 4; line++ {
		for col := 1; col <= 14; col++ {
			if a, b := lf.ExtractLineAtPosition(line, col), crlf.ExtractLineAtPosition(line, col); a != b {
				t.Errorf("line %d col %d: LF %q, CRLF %q", line, col, a, b)
			}
		}
	}
	lone := NewSlice([]byte("first\rsecond line\rthird"))
	if got, want := lone.ExtractLineAtPosition(2, 3), "second line\n  ^"; got != want {
		t.Errorf("lone CR: got %q want %q", got, want)
	}
}
