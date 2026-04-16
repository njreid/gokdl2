package parser

import (
	"bytes"
	"errors"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
	"github.com/njreid/gokdl2/relaxed"
)

type ParseFlags uint8

func (p ParseFlags) Has(f ParseFlags) bool {
	return (p & f) != 0
}

const (
	ParseComments ParseFlags = 1 << iota
)

type ParseContextOptions struct {
	RelaxedNonCompliant relaxed.Flags
	Flags               ParseFlags
	Version             tokenizer.Version // VersionAuto, VersionV1, or VersionV2
	InputSizeEstimate   int
}

var defaultParseContextOptions = ParseContextOptions{
	RelaxedNonCompliant: 0,
}

// ParseContext maintains the parser context for a KDL document
type ParseContext struct {
	// document being generated
	doc *document.Document
	// state stack; current state is pushed onto this when a child block is encountered
	states []parserState
	// current state
	state parserState
	// node stack; new nodes are pushed onto this when a child block is encountered; current node is last
	node []*document.Node
	// temporary storage for identifier (usually node name or property key)
	ident tokenizer.Token
	// temporary storage for type annotation
	typeAnnot tokenizer.Token
	// true if a continuation backslash has been encountered and the next newline should be ignored
	continuation bool
	// true once the first newline after a continuation has been consumed
	continuationNewlineSeen bool
	// true for the first significant token parsed after a continuation
	justContinuedLine bool
	// true if a /- was encountered and the next entire node should be ignored
	ignoreNextNode bool
	// true if a /- was encountered and the next arg/prop should be ignored
	ignoreNextArgProp bool
	// true if a /- was encountered and the next child block should be ignored
	ignoreChildren int
	// true after finishing an ignored child block; only terminators may follow
	afterIgnoredChildBlock bool
	// tracks whether an ignored child block was introduced from a continued line
	ignoreChildBlockFromContinuation bool
	// marks the next slashdash as coming from a continued line
	ignoreFromContinuation bool
	opts                   ParseContextOptions

	comment pendingComment

	lastAddedNode *document.Node
	recent        recentTokens
	alloc         *document.Allocator

	// VersionSetter is called when a /- kdl-version N marker is detected; used to update the scanner version
	VersionSetter func(tokenizer.Version)
	// versionMarkerStep tracks state for version marker detection (0=initial, -1=aborted, 5=done)
	versionMarkerStep int
}

type pendingComment struct {
	bytes.Buffer
}

func (p pendingComment) CopyBytes() []byte {
	if p.Len() == 0 {
		return nil
	}

	r := make([]byte, p.Len())
	copy(r, p.Bytes())
	return r
}

func (c *ParseContext) RelaxedNonCompliant() relaxed.Flags {
	return c.opts.RelaxedNonCompliant
}

// Document returns the current parsed document
func (c *ParseContext) Document() *document.Document {
	return c.doc
}

func (c *ParseContext) addNode() *document.Node {
	n := c.newNode()
	if len(c.node) > 0 {
		c.node[len(c.node)-1].AddNode(n)
	} else {
		c.doc.AddNode(n)
	}
	c.node = append(c.node, n)
	c.lastAddedNode = n
	return n
}

func (c *ParseContext) createNode() *document.Node {
	n := c.newNode()
	c.node = append(c.node, n)
	c.lastAddedNode = n
	return n
}

func (c *ParseContext) newNode() *document.Node {
	if c.alloc != nil {
		return c.alloc.NewNode()
	}
	return document.NewNode()
}

func (c *ParseContext) newValue() *document.Value {
	if c.alloc != nil {
		return c.alloc.NewValue()
	}
	return &document.Value{}
}

func (c *ParseContext) setNodeNameToken(n *document.Node, t tokenizer.Token) error {
	v := c.newValue()
	if err := document.ValueFromTokenInto(v, t); err != nil {
		return err
	}
	n.SetNameValue(v)
	return nil
}

func (c *ParseContext) addArgumentToken(n *document.Node, t tokenizer.Token, typeAnnot tokenizer.Token) error {
	v := c.newValue()
	if err := document.ValueFromTokenInto(v, t); err != nil {
		return err
	}
	if typeAnnot.Valid() {
		v.Type = document.TypeAnnotation(typeAnnot.Data)
	}
	n.AddArgumentValue(v)
	return nil
}

func (c *ParseContext) addPropertyToken(n *document.Node, name tokenizer.Token, value tokenizer.Token, typeAnnot tokenizer.Token) (*document.Value, error) {
	nt := c.newValue()
	if err := document.ValueFromTokenInto(nt, name); err != nil {
		return nil, err
	}
	vt := c.newValue()
	if err := document.ValueFromTokenInto(vt, value); err != nil {
		return nil, err
	}
	if typeAnnot.Valid() {
		vt.Type = document.TypeAnnotation(typeAnnot.Data)
	}
	return n.AddPropertyValue(nt.ValueString(), vt, ""), nil
}

var errNodeStackEmpty = errors.New("node stack empty")

func (c *ParseContext) popNode() (*document.Node, error) {
	if len(c.node) == 0 {
		return nil, errNodeStackEmpty
	}
	node := c.currentNode()
	c.node = c.node[0 : len(c.node)-1]
	return node, nil
}

func (c *ParseContext) popNodeAndState() (parserState, *document.Node, error) {
	ps, err := c.popState()
	if err != nil {
		return ps, nil, err
	}
	node, err := c.popNode()
	return ps, node, err
}

func (c *ParseContext) currentNode() *document.Node {
	if len(c.node) == 0 {
		return nil
	}
	return c.node[len(c.node)-1]
}

func (c *ParseContext) pushState(newState parserState) {
	c.states = append(c.states, c.state)
	c.state = newState
}

func (c *ParseContext) startContinuation() {
	c.continuation = true
	c.continuationNewlineSeen = false
}

func (c *ParseContext) stopContinuation() {
	c.continuation = false
	c.continuationNewlineSeen = false
}

func (c *ParseContext) markIgnoreFromContinuation() {
	c.ignoreFromContinuation = c.justContinuedLine
	c.justContinuedLine = false
}

var errStateStackEmpty = errors.New("state stack empty")

func (c *ParseContext) popState() (parserState, error) {
	if len(c.states) == 0 {
		return c.state, errStateStackEmpty
	}
	c.state = c.states[len(c.states)-1]
	c.states = c.states[0 : len(c.states)-1]
	return c.state, nil
}
