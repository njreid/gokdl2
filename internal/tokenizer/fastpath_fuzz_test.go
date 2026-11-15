package tokenizer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/njreid/gokdl2/relaxed"
)

// FuzzFastPathMatchesRunePath checks that the ASCII fast paths (used when the whole input is in memory) produce
// exactly the same tokens, positions and error presence as the rune-at-a-time paths (used while a reader is
// attached). Run it beyond the seed corpus with: go test -run '^$' -fuzz FuzzFastPathMatchesRunePath ./internal/tokenizer
func FuzzFastPathMatchesRunePath(f *testing.F) {
	for _, dir := range []string{"kdl-org-v1", "kdl-org-v2"} {
		files, _ := filepath.Glob(filepath.Join("..", "..", dir, "tests", "test_cases", "input", "*.kdl"))
		for _, file := range files {
			if data, err := os.ReadFile(file); err == nil {
				f.Add(data)
			}
		}
	}
	for _, s := range []string{
		"node \"a\\\n   b\" \t x",
		"node \"\\/\"",
		"node \"abc",
		"node \"abc\"\xff",
		"node \"\xef\xbf\xbd\"",
		"a b 　 \" \" c\r\nd\v e\f",
		"node \"\\s\\u{1F600}\\\"\" k=\"\"",
		"ab\xffcd",
	} {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, v := range []Version{VersionV1, VersionV2} {
			for _, flags := range []relaxed.Flags{0, relaxed.NGINXSyntax} {
				fast := NewSlice(data)
				slow := NewBuffer(bytes.NewReader(data), make([]byte, 16))
				for _, s := range []*Scanner{fast, slow} {
					s.Version = v
					s.RelaxedNonCompliant = flags
				}
				for i := 0; ; i++ {
					fok, sok := fast.Scan(), slow.Scan()
					if fok != sok {
						t.Fatalf("v%d flags=%d token %d: fast ok=%v err=%v, slow ok=%v err=%v", v, flags, i, fok, fast.Err(), sok, slow.Err())
					}
					if !fok {
						break
					}
					ft, st := fast.Token(), slow.Token()
					if ft.ID != st.ID || !bytes.Equal(ft.Data, st.Data) || ft.Line != st.Line || ft.Column != st.Column {
						t.Fatalf("v%d flags=%d token %d: fast %v@%d:%d, slow %v@%d:%d", v, flags, i, ft, ft.Line, ft.Column, st, st.Line, st.Column)
					}
					if ft.ID == EOF {
						break
					}
				}
			}
		}
	})
}
