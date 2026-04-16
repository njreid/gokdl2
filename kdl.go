package kdl

import (
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

func parse(s *tokenizer.Scanner, inputSizeEstimate int) (*document.Document, error) {
	if s.Version != tokenizer.VersionAuto {
		return parseOne(s, inputSizeEstimate)
	}

	// for VersionAuto, we need to be able to retry if the first attempt fails
	// and it was likely a v1 document
	data, err := io.ReadAll(s)
	if err != nil {
		return nil, err
	}

	// try v2 first
	s2 := tokenizer.NewSlice(data)
	s2.RelaxedNonCompliant = s.RelaxedNonCompliant
	s2.ParseComments = s.ParseComments
	s2.Version = tokenizer.VersionV2
	doc, err := parseOne(s2, len(data))
	if err == nil {
		return doc, nil
	}

	// if v2 failed, it might be a v1 document; if it had a v2 marker, don't fallback
	if s2.Version == tokenizer.VersionV2 && s2.Offset() > 0 {
		// if version was explicitly set to v2 by marker, don't fallback
		return nil, err
	}

	// fallback to v1
	s1 := tokenizer.NewSlice(data)
	s1.RelaxedNonCompliant = s.RelaxedNonCompliant
	s1.ParseComments = s.ParseComments
	s1.Version = tokenizer.VersionV1
	return parseOne(s1, len(data))
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
		if err := p.Parse(c, s.Token()); err != nil {
			return nil, err
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
	sizeEstimate := estimateReaderSize(r)
	s := tokenizer.New(r)
	s.RelaxedNonCompliant = opts.RelaxedNonCompliant
	s.ParseComments = opts.ParseComments
	s.Version = opts.Version.tokenizerVersion()
	return parse(s, sizeEstimate)
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
