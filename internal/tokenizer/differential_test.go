package tokenizer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
)

type diffTok struct {
	ID     TokenID
	Data   string
	Line   int
	Column int
}

// diffScan drains s and returns the token stream and whether scanning failed.
func diffScan(s *Scanner, v Version) ([]diffTok, bool) {
	s.Version = v
	var out []diffTok
	for i := 0; i < 1<<20; i++ {
		if !s.Scan() {
			return out, s.Err() != nil
		}
		t := s.Token()
		out = append(out, diffTok{t.ID, string(t.Data), t.Line, t.Column})
		if t.ID == EOF {
			return out, false
		}
	}
	return out, true
}

// TestDifferentialInputSources verifies that the token stream is identical no matter how the input reaches the
// scanner: a plain slice, buffered readers with awkward buffer sizes, or a reader that yields one byte at a time.
func TestDifferentialInputSources(t *testing.T) {
	var files []string
	for _, dir := range []string{"kdl-org-v1", "kdl-org-v2"} {
		m, err := filepath.Glob(filepath.Join("..", "..", dir, "tests", "test_cases", "input", "*.kdl"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no test case files found")
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		suite := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(f)))))
		for _, v := range []Version{VersionV1, VersionV2} {
			name := fmt.Sprintf("%s/v%d/%s", suite, v, filepath.Base(f))
			t.Run(name, func(t *testing.T) {
				want, wantErr := diffScan(NewSlice(data), v)

				check := func(label string, s *Scanner) {
					got, gotErr := diffScan(s, v)
					if gotErr != wantErr {
						t.Errorf("%s: error presence = %v, want %v", label, gotErr, wantErr)
					}
					n := len(got)
					if len(want) < n {
						n = len(want)
					}
					for i := 0; i < n; i++ {
						if got[i] != want[i] {
							t.Errorf("%s: token %d = %+v, want %+v", label, i, got[i], want[i])
							return
						}
					}
					if len(got) != len(want) {
						t.Errorf("%s: %d tokens, want %d", label, len(got), len(want))
					}
				}

				for _, n := range []int{16, 17, 23, 64} {
					check(fmt.Sprintf("buffer %d", n), NewBuffer(bytes.NewReader(data), make([]byte, n)))
				}
				check("one-byte reader", NewBuffer(iotest.OneByteReader(bytes.NewReader(data)), make([]byte, 64)))
			})
		}
	}
}
