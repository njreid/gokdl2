package kdl

import (
	"fmt"
	"io"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/generator"
	"github.com/njreid/gokdl2/internal/parser"
	"github.com/njreid/gokdl2/internal/tokenizer"
	"github.com/njreid/gokdl2/relaxed"
)

// ParseVersion controls which KDL syntax version the public parser accepts.
type ParseVersion int

const (
	// ParseVersionAuto tries KDL v2 first and falls back to v1 if v2 parsing fails.
	ParseVersionAuto ParseVersion = ParseVersion(tokenizer.VersionAuto)
	// ParseVersionV1 only accepts KDL v1 syntax.
	ParseVersionV1 ParseVersion = ParseVersion(tokenizer.VersionV1)
	// ParseVersionV2 only accepts KDL v2 syntax.
	ParseVersionV2 ParseVersion = ParseVersion(tokenizer.VersionV2)
)

func (v ParseVersion) tokenizerVersion() tokenizer.Version {
	return tokenizer.Version(v)
}

// parseBytes parses data, which it takes ownership of: parsed strings alias data, so callers must pass a buffer that
// nothing else will modify.
func parseBytes(data []byte, opts ParseOptions) (*document.Document, error) {
	newScanner := func(v tokenizer.Version) *tokenizer.Scanner {
		s := tokenizer.NewSlice(data)
		s.RelaxedNonCompliant = opts.RelaxedNonCompliant
		s.ParseComments = opts.ParseComments
		s.Version = v
		return s
	}

	v := opts.Version.tokenizerVersion()
	if v == tokenizer.VersionAuto {
		// an explicit version marker is authoritative: no fallback
		v = tokenizer.DetectVersion(data)
	}
	if v != tokenizer.VersionAuto {
		return parseOne(newScanner(v), len(data))
	}

	// no marker: v2 is the default version, but fall back to v1 if the document does not parse as v2
	doc, err2 := parseOne(newScanner(tokenizer.VersionV2), len(data))
	if err2 == nil {
		return doc, nil
	}
	doc, err1 := parseOne(newScanner(tokenizer.VersionV1), len(data))
	if err1 == nil {
		return doc, nil
	}
	return nil, fmt.Errorf("%w\n(the document also failed to parse as KDL v1: %v)", err2, err1)
}

// parseReader reads r to completion exactly once and parses the result.
func parseReader(r io.Reader, opts ParseOptions) (*document.Document, error) {
	data, err := readAll(r)
	if err != nil {
		return nil, err
	}
	return parseBytes(data, opts)
}

// readAll reads r until EOF into a single buffer sized from the reader's Len/Size hint when available.
func readAll(r io.Reader) ([]byte, error) {
	hint := estimateReaderSize(r)
	if hint < 511 {
		hint = 511
	}
	// one spare byte lets us see EOF without growing the buffer when the hint is exact
	data := make([]byte, 0, hint+1)
	for {
		n, err := r.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				return data, nil
			}
			return nil, err
		}
		if len(data) == cap(data) {
			data = append(data, 0)[:len(data)]
		}
	}
}

func parseOne(s *tokenizer.Scanner, inputSizeEstimate int) (*document.Document, error) {
	defer s.Close()

	p := parser.New()
	opts := parser.ParseContextOptions{
		RelaxedNonCompliant: s.RelaxedNonCompliant,
		Version:             s.Version,
		InputSizeEstimate:   inputSizeEstimate,
	}
	if s.ParseComments {
		opts.Flags |= parser.ParseComments
	}
	c := p.NewContextOptions(opts)
	for s.Scan() {
		tok := s.Token()
		for {
			err := p.Parse(c, tok)
			if err == nil {
				break
			}
			if err == parser.ErrReenterState {
				continue
			}
			return nil, fmt.Errorf("parse failed: %w at line %d, column %d\n%s", err, tok.Line+1, tok.Column+1, s.ExtractLineAtPosition(tok.Line+1, tok.Column+1))
		}
	}
	if s.Err() != nil {
		return nil, s.Err()
	}

	doc := c.Document()
	doc.Version = int(s.Version)
	return doc, nil
}

func estimateReaderSize(r io.Reader) int {
	type lenner interface{ Len() int }
	type sizer interface{ Size() int64 }

	if lr, ok := r.(lenner); ok {
		return lr.Len()
	}
	if sr, ok := r.(sizer); ok {
		sz := sr.Size()
		if sz > 0 && sz <= int64(^uint(0)>>1) {
			return int(sz)
		}
	}
	return 0
}

type ParseOptions struct {
	// RelaxedNonCompliant enables optional non-standard parsing behaviors.
	RelaxedNonCompliant relaxed.Flags
	// ParseComments preserves comments in the parsed document.
	ParseComments bool
	// Version selects whether parsing is automatic, v1-only, or v2-only.
	Version ParseVersion
}

var DefaultParseOptions = ParseOptions{
	Version: ParseVersionAuto,
}

// Parse parses a KDL document from r and returns the parsed Document, or a non-nil error on failure
func Parse(r io.Reader) (*document.Document, error) {
	return ParseWithOptions(r, DefaultParseOptions)
}

func ParseWithOptions(r io.Reader, opts ParseOptions) (*document.Document, error) {
	return parseReader(r, opts)
}

type GenerateOptions = generator.Options

var DefaultGenerateOptions = generator.DefaultOptions

// Generate writes to w a well-formatted KDL document generated from doc, or a non-nil error on failure
func Generate(doc *document.Document, w io.Writer) error {
	return GenerateWithOptions(doc, w, DefaultGenerateOptions)
}

// GenerateWithOptions writes to w a well-formatted KDL document generated from doc, or a non-nil error on failure
func GenerateWithOptions(doc *document.Document, w io.Writer, opts GenerateOptions) error {
	g := generator.NewOptions(w, opts)
	return g.Generate(doc)
}
