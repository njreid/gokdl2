package kdl

import (
	"io"

	"github.com/sblinch/kdl-go/document"
	"github.com/sblinch/kdl-go/internal/generator"
	"github.com/sblinch/kdl-go/internal/parser"
	"github.com/sblinch/kdl-go/internal/tokenizer"
)

func parse(s *tokenizer.Scanner) (*document.Document, error) {
	if s.Version != tokenizer.VersionAuto {
		return parseOne(s)
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
	doc, err := parseOne(s2)
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
	return parseOne(s1)
}

func parseOne(s *tokenizer.Scanner) (*document.Document, error) {
	defer s.Close()

	p := parser.New()
	opts := parser.ParseContextOptions{
		RelaxedNonCompliant: s.RelaxedNonCompliant,
		Version:             s.Version,
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

type ParseOptions = parser.ParseContextOptions

var DefaultParseOptions = parser.ParseContextOptions{}

// Parse parses a KDL document from r and returns the parsed Document, or a non-nil error on failure
func Parse(r io.Reader) (*document.Document, error) {
	return ParseWithOptions(r, DefaultParseOptions)
}

func ParseWithOptions(r io.Reader, opts ParseOptions) (*document.Document, error) {
	s := tokenizer.New(r)
	s.RelaxedNonCompliant = opts.RelaxedNonCompliant
	s.ParseComments = opts.Flags.Has(parser.ParseComments)
	s.Version = opts.Version
	return parse(s)
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
