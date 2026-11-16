package parser

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
)

// This file holds the comment bookkeeping that is only active when ParseComments is set. Nothing here runs, and
// nothing here allocates, on the default parse path.
//
// Comments are routed by where they appear:
//
//   - at node position (document or child block) they are collected as "pieces" in ParseContext.comment, a buffer of
//     whole lines whose blank-line structure is preserved. The buffer becomes Comment.Before of the next node,
//     Comment.Inside of the node that owns the block when a '}' comes next, or Comment.After of the last top-level
//     node (Document.Comment when there is none) at EOF.
//   - within a node (after its name, before its terminator) they are collected in the inline buffer, which becomes
//     Value.Comment of the next value that is added, or Comment.Trailing of the node when it ends first.
//   - a comment on the same line as a block's '{' belongs to the node that owns the block (Comment.Trailing).
//
// Slash-dashed nodes, arguments, properties and child blocks are captured verbatim from the token stream and routed as
// if they were comments of the same position.

// captureKind says what a verbatim slash-dash capture is collecting.
type captureKind uint8

const (
	capNone captureKind = iota
	capNode
	capArg
	capBlock
)

// inlinePiece is one comment (or slash-dashed item) collected inside a node.
type inlinePiece struct {
	text []byte
	// line is true for a // comment on a line that is continued with a backslash: more entries follow on the next line
	line bool
	// block is true for a slash-dashed child block. Nothing but a comment may follow one on its line (except after a
	// line continuation), so whatever comes next is started on a continued line.
	block bool
}

type inlineBuf struct {
	pieces []inlinePiece
}

func (b *inlineBuf) add(p inlinePiece) { b.pieces = append(b.pieces, p) }
func (b *inlineBuf) reset()            { b.pieces = b.pieces[:0] }

// appendJoined appends piece to dst, separated from what is already there by one space unless dst is empty or ends
// with a newline.
func appendJoined(dst, piece []byte) []byte {
	if len(dst) > 0 && dst[len(dst)-1] != '\n' {
		dst = append(dst, ' ')
	}
	return append(dst, piece...)
}

// appendAfterBlock appends piece to dst, which ends with a slash-dashed child block: a comment may follow on the same
// line, anything else is moved to a continued line.
func appendAfterBlock(dst, piece []byte) []byte {
	if bytes.HasPrefix(piece, []byte("/-")) {
		dst = append(dst, " \\\n"...)
		return append(dst, piece...)
	}
	return appendJoined(dst, piece)
}

// value renders the pieces as Value.Comment text: a // comment that is followed by more entries is preceded by a line
// continuation and followed by the newline it needs, and a value never follows a slash-dashed child block on its line.
func (b *inlineBuf) value() []byte {
	if len(b.pieces) == 0 {
		return nil
	}
	var out []byte
	prevBlock := false
	for _, p := range b.pieces {
		text := p.text
		if p.line {
			text = append(append([]byte{'\\', ' '}, p.text...), '\n')
		}
		if prevBlock {
			out = appendAfterBlock(out, text)
		} else {
			out = appendJoined(out, text)
		}
		prevBlock = p.block
	}
	if prevBlock {
		out = append(out, " \\\n"...)
	}
	return out
}

// trailing appends the pieces to dst as Comment.Trailing text. prevBlock says that dst ends with a slash-dashed child
// block; the result says whether the new dst does. Nothing follows Trailing on its line, so a // comment is plain text
// unless a slash-dashed item comes after it, which then has to be on a continued line to stay one.
func (b *inlineBuf) trailing(dst []byte, prevBlock bool) ([]byte, bool) {
	for i, p := range b.pieces {
		text := p.text
		if p.line && b.slashAfter(i) {
			text = append(append([]byte{'\\', ' '}, p.text...), '\n')
		}
		if prevBlock {
			dst = appendAfterBlock(dst, text)
		} else {
			dst = appendJoined(dst, text)
		}
		prevBlock = p.block
	}
	return dst, prevBlock
}

// slashAfter reports whether a slash-dashed item comes after piece i.
func (b *inlineBuf) slashAfter(i int) bool {
	for _, p := range b.pieces[i+1:] {
		if bytes.HasPrefix(p.text, []byte("/-")) {
			return true
		}
	}
	return false
}

func isKDLSpace(r rune) bool {
	switch r {
	case '\t', ' ', '\u00A0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007',
		'\u2008', '\u2009', '\u200A', '\u202F', '\u205F', '\u3000':
		return true
	}
	return false
}

// trimRightSpace removes trailing whitespace, including newline characters.
func trimRightSpace(b []byte) []byte {
	return bytes.TrimRightFunc(b, unicode.IsSpace)
}

// commentText returns a copy of data with CRLF normalised to LF and, on every line after the first, up to indent leading
// whitespace characters removed.
func commentText(data []byte, indent int) []byte {
	if bytes.IndexByte(data, '\n') < 0 && bytes.IndexByte(data, '\r') < 0 {
		return append([]byte(nil), data...)
	}
	out := make([]byte, 0, len(data))
	first := true
	for len(data) > 0 {
		var line []byte
		more := false
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data, more = data[:i], data[i+1:], true
		} else {
			line, data = data, nil
		}
		// a "\r" before a line feed is half of a CRLF pair
		if n := len(line); more && n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if !first {
			out = append(out, '\n')
			for k := 0; k < indent && len(line) > 0; k++ {
				r, size := utf8.DecodeRune(line)
				if !isKDLSpace(r) {
					break
				}
				line = line[size:]
			}
		}
		first = false
		out = append(out, line...)
	}
	return out
}

// newlinesIn returns the number of line breaks inside a token's data.
func newlinesIn(data []byte) int {
	n := 0
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		switch r {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				size++
			}
			n++
		case '\n', '\u000b', '\u000c', '\u0085', ' ', ' ':
			n++
		}
		i += size
	}
	return n
}

// lineRel reports where a piece starting on startLine sits relative to the previous content: -1 if it is on the same
// line, otherwise the number of blank lines between them.
func (c *ParseContext) lineRel(startLine int) int {
	if startLine <= c.prevEnd {
		return -1
	}
	return startLine - c.prevEnd - 1
}

// afterToken updates the comment bookkeeping once a token has been fully handled.
func (c *ParseContext) afterToken(t tokenizer.Token) {
	if c.capKind != capNone {
		c.capBuf = append(c.capBuf, t.Data...)
	}
	switch t.ID {
	case tokenizer.Newline:
		c.lineIndent = 0
		c.atLineStart = true
		return
	case tokenizer.Whitespace:
		if c.atLineStart {
			c.lineIndent = utf8.RuneCount(t.Data)
			c.atLineStart = false
		}
		return
	case tokenizer.EOF:
		return
	}
	c.atLineStart = false
	end := t.Line
	switch t.ID {
	case tokenizer.MultiLineComment, tokenizer.QuotedString, tokenizer.RawString:
		end += newlinesIn(t.Data)
	}
	c.prevEnd = end
}

// addLinePiece appends a piece to the line buffer, separated from the previous piece according to how far apart they
// were in the source.
func (c *ParseContext) addLinePiece(text []byte, g int) {
	if c.comment.Len() == 0 {
		if g >= 1 {
			c.comment.WriteByte('\n')
		}
	} else {
		switch {
		case g < 0:
			c.comment.WriteByte(' ')
		case g == 0:
			c.comment.WriteByte('\n')
		default:
			c.comment.WriteString("\n\n")
		}
	}
	c.comment.Write(text)
}

func (c *ParseContext) ensureComment(n *document.Node) *document.Comment {
	if n.Comment == nil {
		n.Comment = &document.Comment{}
	}
	return n.Comment
}

// noteComment collects a comment token, routing it by the current state.
func (c *ParseContext) noteComment(t tokenizer.Token) {
	if c.capKind != capNone {
		return
	}
	switch c.state {
	case stateDocument, stateChildren:
		c.noteLineComment(t, c.state == stateChildren)
	case stateTypeAnnot, stateTypeDone:
		if n := len(c.states); n > 0 && (c.states[n-1] == stateDocument || c.states[n-1] == stateChildren) {
			c.noteLineComment(t, false)
		} else {
			c.noteInlineComment(t)
		}
	default:
		c.noteInlineComment(t)
	}
}

func (c *ParseContext) noteLineComment(t tokenizer.Token, inBlock bool) {
	text := commentText(t.Data, c.lineIndent)
	if inBlock && c.comment.Len() == 0 && len(c.braces) > 0 && c.braces[len(c.braces)-1].Line == t.Line {
		// a comment on the line of the block's '{' belongs to the node that owns the block
		if owner := c.currentNode(); owner != nil && len(owner.Children) == 0 {
			cm := c.ensureComment(owner)
			cm.Trailing = appendJoined(cm.Trailing, text)
			return
		}
	}
	c.addLinePiece(text, c.lineRel(t.Line))
}

func (c *ParseContext) noteInlineComment(t tokenizer.Token) {
	p := inlinePiece{
		text: commentText(t.Data, c.lineIndent),
		line: t.ID == tokenizer.SingleLineComment && c.continuation,
	}
	if c.state == stateArgProp && c.ident.Valid() {
		// the value just before the comment is not added yet, and the comment follows it
		c.inl2.add(p)
	} else {
		c.inl.add(p)
	}
}

// mergeInline moves comments collected behind a pending identifier back before it: it turned out to be a property
// name, and the comments are part of that property.
func (c *ParseContext) mergeInline() {
	if len(c.inl2.pieces) == 0 {
		return
	}
	c.inl.pieces = append(c.inl.pieces, c.inl2.pieces...)
	c.inl2.reset()
}

// takeInline returns the comment text collected for the value that is being added, and moves comments that were
// waiting behind it up to be the next value's.
func (c *ParseContext) takeInline() []byte {
	r := c.inl.value()
	c.inl.reset()
	c.inl, c.inl2 = c.inl2, c.inl
	return r
}

// flushTrailing makes the pending inline comments the Trailing comment of n.
func (c *ParseContext) flushTrailing(n *document.Node) {
	c.mergeInline()
	if len(c.inl.pieces) == 0 || n == nil {
		return
	}
	cm := c.ensureComment(n)
	var ends bool
	cm.Trailing, ends = c.inl.trailing(cm.Trailing, c.trailBlockNode == n)
	if ends {
		c.trailBlockNode = n
	} else if c.trailBlockNode == n {
		c.trailBlockNode = nil
	}
	c.inl.reset()
}

// attachBefore gives a newly created node the comments and blank lines that precede it.
func (c *ParseContext) attachBefore(n *document.Node, t tokenizer.Token) {
	g := c.lineRel(t.Line)
	if c.typeGValid {
		g = c.typeG
		c.typeGValid = false
	}
	if c.comment.Len() == 0 {
		if g >= 1 {
			c.ensureComment(n).Before = []byte{}
		}
		return
	}
	if g >= 1 {
		c.comment.WriteByte('\n')
	}
	c.ensureComment(n).Before = c.comment.CopyBytes()
	c.comment.Reset()
}

// noteTypeStart remembers how far a node's type annotation is from the previous content, since the blank lines before
// a node are measured at its first token.
func (c *ParseContext) noteTypeStart(t tokenizer.Token) {
	if c.capKind != capNone {
		return
	}
	c.typeG = c.lineRel(t.Line)
	c.typeGValid = true
}

func (c *ParseContext) startCapture(kind captureKind, t tokenizer.Token) {
	if c.capKind != capNone {
		return
	}
	c.capKind = kind
	c.capDepth = len(c.node)
	c.capOwner = c.currentNode()
	c.capBuf = c.capBuf[:0]
	c.capIndent = c.lineIndent
	c.capG = c.lineRel(t.Line)
}

// endCapture finishes the slash-dash capture in progress. The current token is part of the captured text only if
// include is set.
func (c *ParseContext) endCapture(include bool, t tokenizer.Token) {
	if include {
		c.capBuf = append(c.capBuf, t.Data...)
	}
	kind := c.capKind
	c.capKind = capNone
	text := commentText(trimRightSpace(c.capBuf), c.capIndent)
	if string(text) == "/-" {
		// a slash-dash that slash-dashes nothing, because the end of the block or of the document follows it; the
		// parser tolerates that, but it cannot be written back
		return
	}
	if kind == capNode {
		c.addLinePiece(text, c.capG)
	} else {
		c.inl.add(inlinePiece{text: text, block: kind == capBlock})
	}
}

// endArgCapture ends the capture of a slash-dashed argument or property, if one is in progress.
func (c *ParseContext) endArgCapture(include bool, t tokenizer.Token) {
	if c.capKind == capArg {
		c.endCapture(include, t)
	}
}

// flushEOF attaches the comments left at the end of the document.
func (c *ParseContext) flushEOF() {
	if c.comment.Len() == 0 {
		return
	}
	text := c.comment.CopyBytes()
	c.comment.Reset()
	if n := len(c.doc.Nodes); n > 0 {
		cm := c.ensureComment(c.doc.Nodes[n-1])
		cm.After = append(cm.After, text...)
		return
	}
	c.doc.Comment = append(c.doc.Comment, text...)
}

// flushInside attaches the comments before a '}' to the node that owns the block.
func (c *ParseContext) flushInside() {
	if c.comment.Len() == 0 {
		return
	}
	owner := c.currentNode()
	if owner == nil {
		return
	}
	cm := c.ensureComment(owner)
	cm.Inside = append(cm.Inside, c.comment.CopyBytes()...)
	c.comment.Reset()
}

// joinPropertyComment keeps the comment of a property value that a later duplicate of the same key replaces.
func joinPropertyComment(old, cur *document.Value) {
	if old == nil || len(old.Comment) == 0 {
		return
	}
	if len(cur.Comment) == 0 {
		cur.Comment = append([]byte(nil), old.Comment...)
		return
	}
	cur.Comment = appendJoined(append([]byte(nil), old.Comment...), cur.Comment)
}
