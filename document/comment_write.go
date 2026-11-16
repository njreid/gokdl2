package document

import (
	"bytes"
	"unicode"
)

func trimRightSpace(b []byte) []byte {
	return bytes.TrimRightFunc(b, unicode.IsSpace)
}

func appendIndentN(b, indent []byte, depth int) []byte {
	for i := 0; i < depth; i++ {
		b = append(b, indent...)
	}
	return b
}

// appendCommentLines appends text as whole lines, each followed by a newline. Every non-empty line is preceded by
// depth copies of indent, and has its trailing whitespace removed; leading whitespace is kept, because it is part of
// the comment (the continuation lines of a block comment, for example).
func appendCommentLines(b, text, indent []byte, depth int) []byte {
	for {
		line := text
		i := bytes.IndexByte(text, '\n')
		if i >= 0 {
			line = text[:i]
		}
		line = trimRightSpace(line)
		if len(line) > 0 {
			b = appendIndentN(b, indent, depth)
			b = append(b, line...)
		}
		b = append(b, '\n')
		if i < 0 {
			return b
		}
		text = text[i+1:]
	}
}

// appendInlineComment appends text that is written in the middle of a line. Trailing whitespace is removed from every
// line, and every line after the first is preceded by depth copies of indent.
func appendInlineComment(b, text, indent []byte, depth int) []byte {
	first := true
	for {
		line := text
		i := bytes.IndexByte(text, '\n')
		if i >= 0 {
			line = text[:i]
		}
		line = trimRightSpace(line)
		if !first {
			b = append(b, '\n')
			if len(line) > 0 {
				b = appendIndentN(b, indent, depth)
			}
		}
		first = false
		b = append(b, line...)
		if i < 0 {
			return b
		}
		text = text[i+1:]
	}
}

// commentWriter carries what is needed to write comments in the middle of a node's line.
type commentWriter struct {
	indent []byte
	depth  int
}
