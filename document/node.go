package document

import (
	"bytes"
	"io"
	"strings"
	"sync"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

// TypeAnnotation represents a type annotation in a KDL document
type TypeAnnotation string

// Comment holds the comments of a node, as collected when parsing with ParseComments and written when generating a
// document. Each field is comment text, not including indentation, and may span several lines: a line break inside the
// text is written as a line break, and the lines after the first are prefixed with the indentation of the node (of its
// child block, for Inside). The text is written as it is, except that trailing whitespace is removed from every line.
// The text of slash-dashed nodes, arguments, properties and child blocks is kept verbatim in the field of the comment
// they would be in if they were a comment.
type Comment struct {
	// Before is the comment lines before the node, including blank lines: a blank line is an empty line in the
	// text, and Before ending in a newline (or being empty but non-nil) means a blank line before the node. Written
	// on lines of their own before the node. Slash-dashed nodes before this node are part of it.
	Before []byte
	// After is the comment lines after the node. The parser uses it only for the comments after the last top-level
	// node at the end of the document. The generator writes it on lines of its own after the node's last line.
	After []byte
	// Trailing is the comment text on the same line as the node, after its last entry and before the end of the
	// line, such as "// x" or "/* x */" (several are separated by a space), and slash-dashed arguments, properties
	// and child blocks that no entry follows. A comment on the line of the "{" that opens the child block, or after
	// the "}" that closes it, is Trailing as well. The generator writes it at the end of the node's first line,
	// separated by one space: after the last entry, or after " {" if the node has a child block, as in "a 1 { // x".
	// If it contains a slash-dash it is written after the closing "}" instead (the only place in which slash-dashed
	// arguments and child blocks are valid after a child block). When AddSemicolons is set, a node with a Trailing
	// comment is not given a semicolon.
	Trailing []byte
	// Inside is the comment lines at the end of the node's child block, after the last child and before "}", also
	// for a block without children ("a {\n // only\n}"). The generator writes them indented at child depth just
	// before the "}", and writes a block for Inside even if the node has no children.
	Inside []byte
}

// Node represents a single node in a KDL document
type Node struct {
	// Name is name of the node
	Name *Value
	// Type is the type annotation of the node, or an empty string if none
	Type TypeAnnotation
	// EmptyType is true when an explicitly empty type annotation, eg `("")node`, was present
	EmptyType bool
	// Arguments is the list of arguments for the node, or nil if none
	Arguments []*Value
	// Properties is the list of properties for the node, or nil if none
	Properties Properties
	// Children is the list of child nodes for the node, or nil if none
	Children []*Node
	// Comment is the comment for the node, or nil if none
	Comment *Comment
}

func (n *Node) ShallowCopy() *Node {
	r := &Node{}
	*r = *n
	return r
}

func (n *Node) ExpectChildren(count int) {
	want := len(n.Children) + count
	if cap(n.Children) < want {
		c := make([]*Node, 0, want)
		c = append(c, n.Children...)
		n.Children = c
	}
}

func (n *Node) ExpectArguments(count int) {
	want := len(n.Arguments) + count
	if cap(n.Arguments) < want {
		a := make([]*Value, 0, want)
		a = append(a, n.Arguments...)
		n.Arguments = a
	}
}

// AddNode adds a Node as a child of this node
func (n *Node) AddNode(child *Node) {
	n.Children = append(n.Children, child)
}

// NewNode creates and returns a new Node
func NewNode() *Node {
	return &Node{}
}

// SetName sets the name of the node
func (n *Node) SetName(name string) {
	n.Name = &Value{Value: name}
}

func (n *Node) SetNameValue(v *Value) {
	n.Name = v
}

// SetNameToken sets the name of the node from a Token, and returns a non-nil error on failure
func (n *Node) SetNameToken(t tokenizer.Token) error {
	v, err := ValueFromToken(t)
	if err != nil {
		return err
	}
	n.Name = v
	return nil
}

// AddArgument adds an argument to this node, with the given type annotation (which may be ""), and returns the added
// Value
func (n *Node) AddArgument(value interface{}, typeAnnot TypeAnnotation) *Value {
	v := &Value{
		Value: value,
		Type:  typeAnnot,
	}
	n.Arguments = append(n.Arguments, v)
	return v
}

func (n *Node) AddArgumentValue(v *Value) *Value {
	n.Arguments = append(n.Arguments, v)
	return v
}

// AddArgumentToken adds an argument to this node from a Token, with the given type annotation (which may be ""), and
// returns a non-nil error on failure
func (n *Node) AddArgumentToken(t tokenizer.Token, typeAnnot tokenizer.Token) error {
	v, err := ValueFromToken(t)
	if err != nil {
		return err
	}
	if typeAnnot.Valid() {
		ta, err := TypeAnnotationFromToken(typeAnnot)
		if err != nil {
			return err
		}
		v.Type = ta
		v.EmptyType = ta == ""
	}
	n.Arguments = append(n.Arguments, v)
	return nil
}

// AddProperty adds a property to this node with the given name, value, and type annotation (which may be ""), and
// returns the added Value
func (n *Node) AddProperty(name string, value interface{}, typeAnnot TypeAnnotation) *Value {
	v := &Value{
		Type:  typeAnnot,
		Value: value,
		Flag:  0,
	}
	if !n.Properties.Allocated() {
		n.Properties.Alloc()
	}
	n.Properties.Add(name, v)
	return v
}

func (n *Node) AddPropertyValue(name string, value *Value, typeAnnot TypeAnnotation) *Value {
	if typeAnnot != "" {
		value.Type = typeAnnot
	}
	if !n.Properties.Allocated() {
		n.Properties.Alloc()
	}
	n.Properties.Add(name, value)
	return value
}

// AddPropertyToken adds a property to this node from the given name and value Token and type annotation (which may be
// "") and returns the added Value and a non-nil error on failure
func (n *Node) AddPropertyToken(name tokenizer.Token, value tokenizer.Token, typeAnnot tokenizer.Token) (*Value, error) {
	nt, err := ValueFromToken(name)
	if err != nil {
		return nil, err
	}
	vt, err := ValueFromToken(value)
	if err != nil {
		return nil, err
	}
	if typeAnnot.Valid() {
		ta, err := TypeAnnotationFromToken(typeAnnot)
		if err != nil {
			return nil, err
		}
		vt.Type = ta
		vt.EmptyType = ta == ""
	}

	if !n.Properties.Allocated() {
		n.Properties.Alloc()
	}
	n.Properties.Add(nt.ValueString(), vt)

	return vt, nil
}

// NodeWriteOptions controls how a node is written using WriteToOptions.
type NodeWriteOptions struct {
	// LeadingTrailingSpace specifies whether leading space (indentation) and newlines are included in the output
	LeadingTrailingSpace bool
	// NameAndType specifies whether the node's name and type annotation are included in the output
	NameAndType bool
	// Depth specifies the indentation depth
	Depth int
	// Indent specifies the byte string to use for each indentation level
	Indent []byte
	// IgnoreFlags specifies that the formatting flags for the node's value(s) should be ignored
	IgnoreFlags bool
	// AddSemicolons causes lines to be terminated with semicolons
	AddSemicolons bool
	// AddEquals causes '=' symbols to be inserted between nodes and their values, which is noncompliant with the KDL spec
	AddEquals bool
	// AddEquals causes ':' symbols to be inserted between nodes and their values, which is noncompliant with the KDL spec
	AddColons bool
	// Version specifies the KDL version for output
	Version tokenizer.Version
}

var defaultNodeWriteOptions = NodeWriteOptions{
	LeadingTrailingSpace: false,
	NameAndType:          true,
	Depth:                0,
	Indent:               []byte{'\t'},
	IgnoreFlags:          false,
	Version:              tokenizer.VersionV1,
}

// String returns the complete KDL representation of this node, including its type annotation and name
func (n *Node) String() string {
	b := strings.Builder{}
	_, _ = n.WriteTo(&b)
	return b.String()
}

// ValueString returns the KDL representation of this node, without its type annotation or name.
func (n *Node) ValueString() string {
	b := strings.Builder{}
	_, _ = n.WriteValueTo(&b)
	return b.String()
}

// TextString returns a text representation of this node, without its type annotation or name. If the node contains
// exactly one argument, zero properties, and zero children, it writes the unquoted string representation of the only
// argument.
func (n *Node) TextString() string {
	b := strings.Builder{}
	_, _ = n.WriteTextValueTo(&b)
	return b.String()
}

// WriteValueTo writes the KDL representation of this node, without its type annotation or name.
func (n *Node) WriteValueTo(w io.Writer) (int64, error) {
	opts := defaultNodeWriteOptions
	opts.NameAndType = false
	return n.WriteToOptions(w, opts)
}

// WriteTextValueTo writes a text representation of the arguments, properties, and children of node. If node contains
// exactly one argument, zero properties, and zero children, it writes the unquoted string representation of the only
// argument.
func (n *Node) WriteTextValueTo(w io.Writer) (int64, error) {
	if len(n.Arguments) == 1 && n.Properties.Len() == 0 && len(n.Children) == 0 {
		nw, err := w.Write([]byte(n.Arguments[0].ValueString()))
		return int64(nw), err
	}

	nw, err := n.WriteToOptions(w, NodeWriteOptions{
		LeadingTrailingSpace: false,
		NameAndType:          false,
		Depth:                0,
		Indent:               []byte{},
		IgnoreFlags:          false,
	})
	return int64(nw), err

}

// WriteTo writes the complete KDL representation of this node, including its type annotation or name.
func (n *Node) WriteTo(w io.Writer) (int64, error) {
	return n.WriteToOptions(w, defaultNodeWriteOptions)
}

// nodeBufPool holds reusable output buffers for WriteToOptions.
var nodeBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 512)
		return &b
	},
}

// maxPooledBuf is the largest buffer that is returned to nodeBufPool.
const maxPooledBuf = 64 << 10

// WriteToOptions writes the KDL representation of this node with the specified options. The output for each node is
// assembled in a single buffer and written with one Write call (children are written by their own calls). Writing
// stops at the first error, which is returned along with the number of bytes written so far.
func (n *Node) WriteToOptions(w io.Writer, opts NodeWriteOptions) (int64, error) {
	var (
		nw  int64
		err error
	)
	bp := nodeBufPool.Get().(*[]byte)
	buf := (*bp)[:0]
	defer func() {
		if cap(buf) <= maxPooledBuf {
			*bp = buf[:0]
			nodeBufPool.Put(bp)
		}
	}()

	flush := func() {
		if len(buf) > 0 && err == nil {
			var written int
			written, err = w.Write(buf)
			nw += int64(written)
		}
		buf = buf[:0]
	}
	appendIndent := func(b []byte, depth int) []byte {
		for i := 0; i < depth; i++ {
			b = append(b, opts.Indent...)
		}
		return b
	}

	indented := opts.Depth > 0 && opts.LeadingTrailingSpace
	v2 := opts.Version == tokenizer.VersionV2

	// comments inside a line are only written when writing a document; String and friends leave them out
	inline := opts.LeadingTrailingSpace
	commentDepth := 0
	if indented {
		commentDepth = opts.Depth
	}
	if n.Comment != nil && n.Comment.Before != nil {
		buf = appendCommentLines(buf, n.Comment.Before, opts.Indent, commentDepth)
	}

	if indented {
		buf = appendIndent(buf, opts.Depth)
	}
	if opts.NameAndType {
		if len(n.Type) > 0 || n.EmptyType {
			buf = append(buf, '(')
			buf = appendTypeAnnotationName(buf, n.Type, opts.Version)
			buf = append(buf, ')')
		}
		// node names don't need to be quoted unless they include non-Identifier characters
		if v2 {
			buf = n.Name.appendString(buf, voSimpleString, tokenizer.VersionV2)
		} else {
			buf = n.Name.appendString(buf, voSimpleString, tokenizer.VersionV1)
		}
	}

	if opts.AddEquals && len(n.Arguments) > 0 && !n.Properties.Exist() && len(n.Children) == 0 {
		buf = append(buf, ' ', '=')
	} else if opts.AddColons && len(n.Arguments) > 0 && !n.Properties.Exist() && len(n.Children) == 0 {
		buf = append(buf, ':')
	}

	// arguments must always be quoted
	argOpts := argValueOpts(opts.IgnoreFlags, opts.Version)
	for i, arg := range n.Arguments {
		if opts.NameAndType || i > 0 {
			buf = append(buf, ' ')
		}
		if inline && len(arg.Comment) > 0 {
			buf = appendInlineComment(buf, arg.Comment, opts.Indent, commentDepth)
			buf = append(buf, ' ')
		}
		buf = arg.appendString(buf, argOpts, opts.Version)
	}
	if n.Properties.Exist() {
		var cw *commentWriter
		if inline {
			cw = &commentWriter{indent: opts.Indent, depth: commentDepth}
		}
		buf = n.Properties.appendToComments(buf, opts.IgnoreFlags, opts.Version, cw)
	}

	var trailing, inside, after []byte
	if n.Comment != nil {
		after = n.Comment.After
		if inline {
			trailing = n.Comment.Trailing
			inside = n.Comment.Inside
		}
	}
	// a slash-dash is only valid after the closing brace of a child block, so a Trailing comment that has one goes there
	slashed := bytes.Contains(trailing, []byte("/-"))
	hasBlock := len(n.Children) > 0 || len(inside) > 0

	if hasBlock {
		buf = append(buf, ' ', '{')
		if len(trailing) > 0 && !slashed {
			buf = append(buf, ' ')
			buf = appendInlineComment(buf, trailing, opts.Indent, commentDepth)
		}
		buf = append(buf, '\n')
		flush()
		if err != nil {
			return nw, err
		}

		childOpts := opts
		childOpts.Depth++
		for _, child := range n.Children {
			cnw, cerr := child.WriteToOptions(w, childOpts)
			nw += cnw
			if cerr != nil {
				return nw, cerr
			}
		}

		if len(inside) > 0 {
			buf = appendCommentLines(buf, inside, opts.Indent, opts.Depth+1)
		}
		if opts.Depth > 0 {
			buf = appendIndent(buf, opts.Depth)
		}
		buf = append(buf, '}')
		if len(trailing) > 0 && slashed {
			buf = append(buf, ' ')
			buf = appendInlineComment(buf, trailing, opts.Indent, commentDepth)
		}
	} else if len(trailing) > 0 {
		buf = append(buf, ' ')
		buf = appendInlineComment(buf, trailing, opts.Indent, commentDepth)
	} else if opts.AddSemicolons {
		buf = append(buf, ';')
	}

	if opts.LeadingTrailingSpace || len(after) > 0 {
		buf = append(buf, '\n')
	}
	if len(after) > 0 {
		buf = appendCommentLines(buf, after, opts.Indent, commentDepth)
	}
	flush()

	return nw, err
}
