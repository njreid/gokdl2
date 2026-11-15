package document

import (
	"errors"
	"math/big"
	"testing"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

func TestHexTypeAnnotationPreserved(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    *Value
		want string
	}{
		{"hex", &Value{Type: "u8", Value: int64(255), Flag: FlagHexadecimal}, "(u8)0xff"},
		{"octal", &Value{Type: "u8", Value: int64(255), Flag: FlagOctal}, "(u8)0o377"},
		{"binary", &Value{Type: "u8", Value: int64(5), Flag: FlagBinary}, "(u8)0b101"},
		{"hex uint8", &Value{Type: "u8", Value: uint8(255), Flag: FlagHexadecimal}, "(u8)0xff"},
	} {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("%s: String() = %q, want %q", tc.name, got, tc.want)
		}
		if got := tc.v.FormattedStringV2(); got != tc.want {
			t.Errorf("%s: FormattedStringV2() = %q, want %q", tc.name, got, tc.want)
		}
	}

	n := NewNode()
	n.SetName("node")
	n.AddArgument(int64(255), "u8").Flag = FlagHexadecimal
	if got := n.String(); got != "node (u8)0xff" {
		t.Errorf("node.String() = %q", got)
	}
}

func TestNegativePrefixedNumbers(t *testing.T) {
	huge := new(big.Int)
	huge.SetString("-123456789012345678901234567890", 10)
	minInt := int64(-1 << 63)
	for _, tc := range []struct {
		v    *Value
		want string
	}{
		{&Value{Value: int64(-255), Flag: FlagHexadecimal}, "-0xff"},
		{&Value{Value: int(-8), Flag: FlagOctal}, "-0o10"},
		{&Value{Value: int8(-5), Flag: FlagBinary}, "-0b101"},
		{&Value{Type: "i8", Value: int64(-255), Flag: FlagHexadecimal}, "(i8)-0xff"},
		{&Value{Value: minInt, Flag: FlagHexadecimal}, "-0x8000000000000000"},
		{&Value{Value: big.NewInt(-255), Flag: FlagHexadecimal}, "-0xff"},
		{&Value{Value: big.NewInt(255), Flag: FlagHexadecimal}, "0xff"},
		{&Value{Value: huge, Flag: FlagOctal}, "-0o" + new(big.Int).Abs(huge).Text(8)},
		{&Value{Value: int64(-255)}, "-255"},
		{&Value{Value: huge}, huge.String()},
	} {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
	if got := (&Value{Value: int64(-255), Flag: FlagHexadecimal}).UnformattedString(); got != "-255" {
		t.Errorf("UnformattedString() = %q", got)
	}
}

// failOnceWriter fails on the failAt-th Write call (0-based) and succeeds otherwise.
type failOnceWriter struct {
	calls  int
	failAt int
}

var errWrite = errors.New("write failed")

func (w *failOnceWriter) Write(p []byte) (int, error) {
	i := w.calls
	w.calls++
	if i == w.failAt {
		return 0, errWrite
	}
	return len(p), nil
}

func TestWriteToOptionsPropagatesFirstError(t *testing.T) {
	build := func() *Node {
		n := NewNode()
		n.SetName("parent")
		n.AddArgument("a", "")
		n.AddArgument("b", "")
		n.AddProperty("k", "v", "")
		for _, name := range []string{"c1", "c2"} {
			c := NewNode()
			c.SetName(name)
			c.AddArgument(int64(1), "")
			n.AddNode(c)
		}
		return n
	}
	opts := NodeWriteOptions{
		LeadingTrailingSpace: true, NameAndType: true, Indent: []byte("  "), Version: tokenizer.VersionV2,
	}

	// count the writes of a clean run
	counter := &failOnceWriter{failAt: -1}
	if _, err := build().WriteToOptions(counter, opts); err != nil {
		t.Fatal(err)
	}
	if counter.calls == 0 {
		t.Fatal("no writes")
	}
	// fail at every possible write: the error must always be reported
	for i := 0; i < counter.calls; i++ {
		w := &failOnceWriter{failAt: i}
		_, err := build().WriteToOptions(w, opts)
		if !errors.Is(err, errWrite) {
			t.Errorf("failAt=%d: err = %v, want errWrite", i, err)
		}
	}
}

func TestPropertiesSemantics(t *testing.T) {
	var p Properties
	if p.Allocated() || p.Exist() || p.Len() != 0 {
		t.Fatal("zero value should be empty")
	}
	if _, ok := p.Get("x"); ok {
		t.Fatal("Get on empty")
	}
	p.Alloc()
	if !p.Allocated() || p.Exist() {
		t.Fatal("after Alloc")
	}
	p.Add("b", &Value{Value: int64(1)})
	p.Add("a", &Value{Value: int64(2)})
	p.Add("b", &Value{Value: int64(3)})
	if p.Len() != 2 {
		t.Fatalf("Len = %d", p.Len())
	}
	if got := p.String(); got != " b=3 a=2" {
		t.Errorf("String = %q", got)
	}
	if got := string(p.AppendTo(nil)); got != " b=3 a=2" {
		t.Errorf("AppendTo = %q", got)
	}
	if v, ok := p.Get("b"); !ok || v.Value != int64(3) {
		t.Errorf("Get(b) = %v %v", v, ok)
	}
	m := p.Unordered()
	if len(m) != 2 || m["a"].Value != int64(2) {
		t.Errorf("Unordered = %v", m)
	}

	// many entries: exercise the lazily built index
	var q Properties
	q.Alloc()
	for i := 0; i < 100; i++ {
		q.Add(string(rune('a'+i%26))+string(rune('a'+i/26)), &Value{Value: int64(i)})
	}
	q.Add("aa", &Value{Value: int64(-1)})
	if q.Len() != 100 {
		t.Fatalf("Len = %d", q.Len())
	}
	if v, ok := q.Get("aa"); !ok || v.Value != int64(-1) {
		t.Errorf("Get(aa) = %v", v)
	}
	if v, ok := q.Get("zc"); !ok || v.Value != int64(25+2*26) {
		t.Errorf("Get(zc) = %v", v)
	}
	if _, ok := q.Get("nope"); ok {
		t.Error("Get(nope)")
	}
	if len(q.Unordered()) != 100 {
		t.Error("Unordered len")
	}
	if got := q.String()[:9]; got != " aa=-1 ba" {
		t.Errorf("order: %q", got)
	}
}
