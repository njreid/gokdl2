package kdl

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

// benchDoc generates a synthetic KDL document of n "server" blocks (~340 bytes each) exercising properties,
// nested children, quoted strings with escapes, hex numbers, floats, booleans and comments.
func benchDoc(v2 bool, n int) []byte {
	var b bytes.Buffer
	tr := "true"
	if v2 {
		tr = "#true"
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "server \"host-%d.example.com\" port=%d enabled=%s weight=1.5 {\n", i, 8000+i%100, tr)
		fmt.Fprintf(&b, "    // a comment about this server\n")
		fmt.Fprintf(&b, "    listen \"0.0.0.0\" %d\n", 443)
		fmt.Fprintf(&b, "    tags \"alpha\" \"beta\" \"gamma\"\n")
		fmt.Fprintf(&b, "    limits cpu=4 memory=0x4000 burst=%s {\n        window 60\n        policy \"token-bucket\"\n    }\n", tr)
		fmt.Fprintf(&b, "    description \"The quick brown fox jumps over the lazy dog \\\"quoted\\\" \\n\"\n")
		fmt.Fprintf(&b, "}\n")
	}
	return b.Bytes()
}

// readerOnly hides Len/Size so the parser takes the streaming path.
type readerOnly struct{ r io.Reader }

func (o readerOnly) Read(p []byte) (int, error) { return o.r.Read(p) }

func benchParse(b *testing.B, data []byte, opts ParseOptions, stream bool) {
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var r io.Reader = bytes.NewReader(data)
		if stream {
			r = readerOnly{r}
		}
		if _, err := ParseWithOptions(r, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseV2Auto(b *testing.B) {
	benchParse(b, benchDoc(true, 3000), DefaultParseOptions, false)
}
func BenchmarkParseV2(b *testing.B) {
	benchParse(b, benchDoc(true, 3000), ParseOptions{Version: ParseVersionV2}, false)
}
func BenchmarkParseV2Stream(b *testing.B) {
	benchParse(b, benchDoc(true, 3000), ParseOptions{Version: ParseVersionV2}, true)
}
func BenchmarkParseV1Auto(b *testing.B) {
	benchParse(b, benchDoc(false, 3000), DefaultParseOptions, false)
}
func BenchmarkParseV2Small(b *testing.B) {
	benchParse(b, benchDoc(true, 2), DefaultParseOptions, false)
}
func BenchmarkParseV2Comments(b *testing.B) {
	benchParse(b, benchDoc(true, 3000), ParseOptions{Version: ParseVersionV2, ParseComments: true}, false)
}

func BenchmarkGenerate(b *testing.B) {
	data := benchDoc(true, 3000)
	doc, err := Parse(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := Generate(doc, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

type benchLimits struct {
	CPU    int    `kdl:"cpu"`
	Memory int    `kdl:"memory"`
	Burst  bool   `kdl:"burst"`
	Window int    `kdl:"window,child"`
	Policy string `kdl:"policy,child"`
}

type benchServer struct {
	Host    string      `kdl:",arg"`
	Port    int         `kdl:"port"`
	Enabled bool        `kdl:"enabled"`
	Weight  float64     `kdl:"weight"`
	Listen  []any       `kdl:"listen"`
	Tags    []string    `kdl:"tags"`
	Limits  benchLimits `kdl:"limits"`
	Desc    string      `kdl:"description"`
}

type benchConfig struct {
	Servers []benchServer `kdl:"server,multiple"`
}

func BenchmarkUnmarshal(b *testing.B) {
	data := benchDoc(true, 3000)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var c benchConfig
		if err := Unmarshal(data, &c); err != nil {
			b.Fatal(err)
		}
		if len(c.Servers) != 3000 || c.Servers[5].Limits.Policy != "token-bucket" {
			b.Fatalf("unexpected result: %d servers, %+v", len(c.Servers), c.Servers[5])
		}
	}
}

func BenchmarkMarshal(b *testing.B) {
	var c benchConfig
	if err := Unmarshal(benchDoc(true, 3000), &c); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out, err := Marshal(&c)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(out)))
	}
}
