package parser

import (
	"fmt"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

func tokenLabel(id tokenizer.TokenID) string {
	switch id {
	case tokenizer.BareIdentifier:
		return "identifier"
	case tokenizer.QuotedString:
		return "string"
	case tokenizer.RawString:
		return "raw string"
	case tokenizer.ExpressionString:
		return "expression string"
	case tokenizer.Decimal, tokenizer.Hexadecimal, tokenizer.Octal, tokenizer.Binary, tokenizer.SuffixedDecimal:
		return "number"
	case tokenizer.Boolean:
		return "boolean"
	case tokenizer.Null:
		return "null"
	case tokenizer.Keyword:
		return "keyword"
	case tokenizer.Whitespace:
		return "whitespace"
	case tokenizer.Newline:
		return "newline"
	case tokenizer.SingleLineComment, tokenizer.MultiLineComment:
		return "comment"
	case tokenizer.TokenComment:
		return "token comment '/-'"
	case tokenizer.Continuation:
		return "line continuation '\\'"
	case tokenizer.BraceOpen:
		return "'{'"
	case tokenizer.BraceClose:
		return "'}'"
	case tokenizer.ParensOpen:
		return "'('"
	case tokenizer.ParensClose:
		return "')'"
	case tokenizer.Equals:
		return "'='"
	case tokenizer.Semicolon:
		return "';'"
	case tokenizer.EOF:
		return "end of input"
	default:
		return id.String()
	}
}

func expectedInState(c *ParseContext) string {
	switch c.state {
	case stateDocument:
		return "a node name, type annotation, comment, newline, ';', or end of input"
	case stateChildren:
		return "a child node, comment, type annotation, newline, or '}'"
	case stateTypeAnnot:
		return "a type name or string type annotation"
	case stateTypeDone:
		return "')' to close the type annotation"
	case stateNode:
		return "whitespace, comment, '{', newline, ';', line continuation, or end of input"
	case stateNodeParams:
		if c.typeAnnot.Valid() {
			return "a value after the type annotation"
		}
		return "an argument, property, child block, comment, newline, ';', or end of input"
	case stateNodeEnd:
		return "newline or end of input"
	case stateProperty:
		return "'=' after the property name"
	case stateArgProp:
		return "'=' for a property, whitespace or newline to finish the argument, '{' for children, or end of input"
	case statePropertyValue:
		if c.typeAnnot.Valid() {
			return "a property value after the type annotation"
		}
		return "a property value"
	default:
		return "valid KDL syntax"
	}
}

func unexpectedToken(c *ParseContext, t tokenizer.Token) error {
	if t.ID == tokenizer.EOF {
		return fmt.Errorf("unexpected end of input; expected %s", expectedInState(c))
	}
	return fmt.Errorf("unexpected %s; expected %s", tokenLabel(t.ID), expectedInState(c))
}

func expectedValueAfterType(c *ParseContext, t tokenizer.Token) error {
	if t.ID == tokenizer.EOF {
		return fmt.Errorf("unexpected end of input; expected a value after the type annotation")
	}
	return fmt.Errorf("unexpected %s; expected a value after the type annotation", tokenLabel(t.ID))
}

func missingSeparatorError(t tokenizer.Token) error {
	return fmt.Errorf("missing whitespace, newline, comment, or ';' before %s", tokenLabel(t.ID))
}

func unexpectedAfterIgnoredChildBlock(t tokenizer.Token) error {
	if t.ID == tokenizer.EOF {
		return fmt.Errorf("unexpected end of input after ignored child block; expected newline, line continuation, or end of input")
	}
	return fmt.Errorf("unexpected %s after ignored child block; expected newline, line continuation, or end of input", tokenLabel(t.ID))
}
