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
	// open child-block braces, innermost last; used to report an unclosed block at its opening brace
	braces []tokenizer.Token
	// the most recent node-level slashdash, reported if the input ends before the node it comments out
	slashdash tokenizer.Token
	// the node whose (non-ignored) child block closed most recently; a node may have only one such block
	realBlockClosed *document.Node
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

	// comments is true when ParseComments is set; all of the fields below are only used then (see comments.go)
	comments bool
	// prevEnd is the line on which the previous content token (anything but whitespace and newlines) ended
	prevEnd int
	// lineIndent is the number of whitespace characters that start the current line; atLineStart is true until the
	// first token that is not whitespace has been seen on the line
	lineIndent  int
	atLineStart bool
	// inl collects comments inside a node for the next value or the node's Trailing comment; inl2 holds those that
	// came behind a value that has not been added yet
	inl, inl2 inlineBuf
	// slash-dash capture in progress: kind, depth of the node stack when it began, raw text, the indentation of its
	// line and its distance from the previous content
	capKind   captureKind
	capDepth  int
	capOwner  *document.Node
	capBuf    []byte
	capIndent int
	capG      int
	// trailBlockNode is the node whose Trailing comment currently ends with a slash-dashed child block
	trailBlockNode *document.Node
	// typeG is the distance of a node's type annotation from the previous content, valid when typeGValid
	typeG      int
	typeGValid bool

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
		return errorAt(t, err)
	}
	n.SetNameValue(v)
	return nil
}

func (c *ParseContext) addArgumentToken(n *document.Node, t tokenizer.Token, typeAnnot tokenizer.Token) error {
	v := c.newValue()
	if err := document.ValueFromTokenInto(v, t); err != nil {
		return errorAt(t, err)
	}
	if typeAnnot.Valid() {
		ta, err := document.TypeAnnotationFromToken(typeAnnot)
		if err != nil {
			return err
		}
		v.Type = ta
		v.EmptyType = ta == ""
	}
	if c.comments && c.capKind == capNone {
		v.Comment = c.takeInline()
	}
	n.AddArgumentValue(v)
	return nil
}

func (c *ParseContext) addPropertyToken(n *document.Node, name tokenizer.Token, value tokenizer.Token, typeAnnot tokenizer.Token) (*document.Value, error) {
	nt := c.newValue()
	if err := document.ValueFromTokenInto(nt, name); err != nil {
		return nil, errorAt(name, err)
	}
	vt := c.newValue()
	if err := document.ValueFromTokenInto(vt, value); err != nil {
		return nil, errorAt(value, err)
	}
	if typeAnnot.Valid() {
		ta, err := document.TypeAnnotationFromToken(typeAnnot)
		if err != nil {
			return nil, err
		}
		vt.Type = ta
		vt.EmptyType = ta == ""
	}
	key := nt.ValueString()
	if c.comments && c.capKind == capNone {
		vt.Comment = c.takeInline()
		if old, ok := n.Properties.Get(key); ok {
			joinPropertyComment(old, vt)
		}
	}
	return n.AddPropertyValue(key, vt, ""), nil
}

var errNodeStackEmpty = errors.New("node stack empty")

func (c *ParseContext) popNode() (*document.Node, error) {
	if len(c.node) == 0 {
		return nil, errNodeStackEmpty
	}
	node := c.currentNode()
	if c.comments {
		if c.capKind == capNode && len(c.node)-1 == c.capDepth {
			// the slash-dashed node ends here; the terminator is not part of it
			c.endCapture(false, tokenizer.Token{})
		} else if c.capKind == capNone {
			c.flushTrailing(node)
		}
	}
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

// pushChildren enters a child block opened by the '{' token open
func (c *ParseContext) pushChildren(open tokenizer.Token) error {
	// a node has at most one real child block; ignored (slashdashed) blocks may come before or after it
	if c.ignoreChildren == 0 && c.realBlockClosed != nil && c.realBlockClosed == c.currentNode() {
		return unexpectedToken(c, open)
	}
	if c.comments && c.capKind == capNone {
		c.flushTrailing(c.currentNode())
	}
	c.braces = append(c.braces, open)
	c.pushState(stateChildren)
	return nil
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
	if c.state == stateChildren && len(c.braces) > 0 {
		c.braces = c.braces[:len(c.braces)-1]
	}
	c.state = c.states[len(c.states)-1]
	c.states = c.states[0 : len(c.states)-1]
	return c.state, nil
}
