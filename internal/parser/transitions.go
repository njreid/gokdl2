package parser

import (
	"bytes"
	"fmt"

	"github.com/njreid/gokdl2/document"
	"github.com/njreid/gokdl2/internal/tokenizer"
	"github.com/njreid/gokdl2/relaxed"
)

type stateTransitionFunc func(*ParseContext, tokenizer.Token) error

// stateTransitions maps a given parser state to the tokens allowed in that state, and provides a transition function
// that accepts a token and a context, processes the token, and updates the parser state
//
// TODO: benchmark this; it's likely faster (though likely much less readable) to do this using switch statements
var stateTransitions = map[parserState]map[tokenizer.TokenID]stateTransitionFunc{
	stateDocument: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			// cannot insert whitespace immediately after type annotation in v1, for... reasons
			if c.typeAnnot.Valid() && c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.ClassIdentifier: func(c *ParseContext, t tokenizer.Token) error {
			// an Identifier in the outermost context is always a node declaration
			var node *document.Node
			if c.ignoreNextNode {
				node = c.createNode()
				c.ignoreNextNode = false
			} else {
				node = c.addNode()
			}

			if err := c.setNodeNameToken(node, t); err != nil {
				return err
			}

			if c.opts.Flags.Has(ParseComments) {
				c.comment.Write(c.recent.TrailingNewlines())
				if c.comment.Len() > 0 {
					node.Comment = &document.Comment{
						Before: bytes.TrimSuffix(c.comment.CopyBytes(), []byte{'\n'}),
					}
					c.comment.Reset()
				}
			}
			if c.typeAnnot.Valid() {
				node.Type = document.TypeAnnotation(c.typeAnnot.Data)
				c.typeAnnot.Clear()
			}
			c.pushState(stateNode)
			return nil
		},
		tokenizer.ParensOpen: func(c *ParseContext, t tokenizer.Token) error {
			// a ( in the outermost context is the beginning of a type annotation for a node
			c.pushState(stateTypeAnnot)
			return nil
		},
		tokenizer.ClassTerminator: func(c *ParseContext, t tokenizer.Token) error {
			if c.typeAnnot.Valid() {
				return fmt.Errorf("expected value after type, found %s in state %s", t.ID, c.state)
			}

			// ignore extraneous newlines, semicolons, and EOF
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.typeAnnot.Valid() && c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			if c.opts.Flags.Has(ParseComments) {
				c.comment.Write(c.recent.TrailingNewlines())
				c.comment.Write(t.Data)
			}
			return nil
		},
		tokenizer.TokenComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.typeAnnot.Valid() {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			c.markIgnoreFromContinuation()
			c.ignoreNextNode = true
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			c.startContinuation()
			return nil
		},
	},
	stateChildren: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			// cannot insert whitespace immediately after type annotation in v1, for... reasons
			if c.typeAnnot.Valid() && c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.typeAnnot.Valid() && c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}

			if c.opts.Flags.Has(ParseComments) {
				trailing := c.recent.TrailingNewlines()
				c.comment.Write(trailing)
				c.comment.Write(t.Data)
			}
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			c.startContinuation()
			return nil
		},
		tokenizer.TokenComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.typeAnnot.Valid() {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			c.markIgnoreFromContinuation()
			c.ignoreNextNode = true
			return nil
		},
		tokenizer.ParensOpen: func(c *ParseContext, t tokenizer.Token) error {
			// a ( inside a node declaration is the beginning of a type annotation for a node
			c.pushState(stateTypeAnnot)
			return nil
		},
		tokenizer.Newline: func(c *ParseContext, t tokenizer.Token) error {
			// ignore extraneous newlines
			return nil
		},
		tokenizer.ClassIdentifier: func(c *ParseContext, t tokenizer.Token) error {
			// an Identifier in the child context is always a node declaration
			var node *document.Node
			if c.ignoreNextNode || c.ignoreChildren > 0 {
				node = c.createNode()
				c.ignoreNextNode = false
			} else {
				node = c.addNode()
			}
			if err := c.setNodeNameToken(node, t); err != nil {
				return err
			}

			if c.opts.Flags.Has(ParseComments) {
				c.comment.Write(c.recent.TrailingNewlines())
				if c.comment.Len() > 0 {
					node.Comment = &document.Comment{
						Before: bytes.TrimSuffix(c.comment.CopyBytes(), []byte{'\n'}),
					}
					c.comment.Reset()
				}
			}
			if c.typeAnnot.Valid() {
				node.Type = document.TypeAnnotation(c.typeAnnot.Data)
				c.typeAnnot.Clear()
			}
			c.pushState(stateNode)
			return nil
		},
		tokenizer.BraceClose: func(c *ParseContext, t tokenizer.Token) error {
			if c.ignoreNextNode {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			if c.ignoreChildren > 0 {
				c.ignoreChildren--
				if c.ignoreChildren == 0 {
					c.afterIgnoredChildBlock = !c.ignoreChildBlockFromContinuation
					c.ignoreChildBlockFromContinuation = false
				}
			}

			if c.opts.Flags.Has(ParseComments) {
				c.comment.Write(c.recent.TrailingNewlines())
				if c.comment.Len() > 0 {
					lastNode := c.lastAddedNode
					if lastNode.Comment == nil {
						lastNode.Comment = &document.Comment{}
					}
					lastNode.Comment.After = append(lastNode.Comment.After, bytes.TrimSuffix(c.comment.CopyBytes(), []byte{'\n'})...)
					c.comment.Reset()
				}
			}

			_, err := c.popState()
			return err
		},
	},

	stateTypeAnnot: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			c.startContinuation()
			return nil
		},
		tokenizer.BareIdentifier: func(c *ParseContext, t tokenizer.Token) error {
			c.typeAnnot = t
			c.state = stateTypeDone
			return nil
		},
		tokenizer.ClassString: func(c *ParseContext, t tokenizer.Token) error {
			c.typeAnnot = t
			c.state = stateTypeDone
			return nil
		},
	},
	stateTypeDone: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			c.startContinuation()
			return nil
		},
		tokenizer.ParensClose: func(c *ParseContext, t tokenizer.Token) error {
			_, err := c.popState()
			return err
		},
	},
	stateNode: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			c.state = stateNodeParams
			return nil
		},
		tokenizer.ClassTerminator: func(c *ParseContext, t tokenizer.Token) error {
			if c.continuation {
				return nil
			} else {
				_, _, err := c.popNodeAndState()
				return err
			}
		},
		tokenizer.Equals: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.RelaxedNonCompliant.Permit(relaxed.YAMLTOMLAssignments) {
				c.state = stateNodeParams
				return nil
			} else {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
		},
		tokenizer.BraceOpen: func(c *ParseContext, t tokenizer.Token) error {
			if c.ignoreNextNode || c.ignoreChildren > 0 {
				c.ignoreChildBlockFromContinuation = c.ignoreFromContinuation
				c.ignoreFromContinuation = false
				c.ignoreNextNode = false
				c.ignoreChildren++
			}
			c.pushState(stateChildren)
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			c.startContinuation()
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version == tokenizer.VersionV2 {
				return nil
			}
			return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
		},
		tokenizer.BraceClose: func(c *ParseContext, t tokenizer.Token) error {
			_, _, err := c.popNodeAndState()
			if err != nil {
				return err
			}
			return ErrReenterState
		},
	},
	stateNodeParams: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			// cannot insert whitespace immediately after type annotation in v1, for... reasons
			if c.typeAnnot.Valid() && c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.Equals: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.RelaxedNonCompliant.Permit(relaxed.YAMLTOMLAssignments) && !c.typeAnnot.Valid() && !c.ident.Valid() {
				// ignore
				return nil
			} else {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
		},
		tokenizer.TokenComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.typeAnnot.Valid() {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			c.markIgnoreFromContinuation()
			c.ignoreNextArgProp = true
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			// cannot insert comment immediately after type annotation in v1, for... reasons
			if c.typeAnnot.Valid() && c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.SingleLineComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.ignoreNextArgProp {
				return nil
			}
			c.state = stateNodeEnd
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			c.startContinuation()
			return nil
		},
		tokenizer.ParensOpen: func(c *ParseContext, t tokenizer.Token) error {
			// a ( inside a node declaration is hte beginning of a type annotation for a node
			c.pushState(stateTypeAnnot)
			return nil
		},
		tokenizer.BareIdentifier: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.RelaxedNonCompliant.Permit(relaxed.NGINXSyntax) || c.opts.Version == tokenizer.VersionV2 {
				// a bare identifier inside a node declaration in nginx syntax mode or KDL v2 is either an argument or a property name; save it
				c.ident = t
				c.state = stateArgProp
			} else {
				// a bare identifier inside a KDL node declaration is a property name; save it
				c.ident = t
				c.state = stateProperty
			}

			return nil
		},
		tokenizer.BraceClose: func(c *ParseContext, t tokenizer.Token) error {
			_, _, err := c.popNodeAndState()
			if err != nil {
				return err
			}
			return ErrReenterState
		},

		tokenizer.SuffixedDecimal: func(c *ParseContext, t tokenizer.Token) error {
			// a suffixed identifier inside a node declaration can only be an argument
			c.typeAnnot.Clear()
			c.ident.Clear()

			if c.ignoreNextArgProp {
				c.ignoreNextArgProp = false
			} else if err := c.addArgumentToken(c.currentNode(), t, c.typeAnnot); err != nil {
				return err
			}

			c.state = stateNodeParams
			return nil
		},
		tokenizer.ClassString: func(c *ParseContext, t tokenizer.Token) error {
			// a string value inside a node declaration is either an argument or a property name; save it
			c.ident = t
			c.state = stateArgProp
			return nil
		},
		tokenizer.ClassNonStringValue: func(c *ParseContext, t tokenizer.Token) error {
			// a non-string value inside a node declaration is always an argument, but we save it just to make sure it isn't followed by an equal sign
			c.ident = t
			c.state = stateArgProp
			return nil

			// a numeric value inside a node declaration is always an argument
			if c.ignoreNextArgProp {
				c.ignoreNextArgProp = false
			} else if err := c.addArgumentToken(c.currentNode(), t, c.typeAnnot); err != nil {
				return err
			}

			c.typeAnnot.Clear()
			c.ident.Clear()
			return nil
		},
		tokenizer.BraceOpen: func(c *ParseContext, t tokenizer.Token) error {
			if c.ignoreNextArgProp || c.ignoreChildren > 0 {
				c.ignoreChildBlockFromContinuation = c.ignoreFromContinuation
				c.ignoreFromContinuation = false
				c.ignoreNextArgProp = false
				c.ignoreChildren++
			}
			c.pushState(stateChildren)
			return nil
		},
		tokenizer.ClassTerminator: func(c *ParseContext, t tokenizer.Token) error {
			if c.continuation {
				return nil
			} else if c.ignoreNextArgProp {
				if t.ID == tokenizer.Newline {
					return nil
				}
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			} else if c.typeAnnot.Valid() {
				return fmt.Errorf("expected value after type, found %s in state %s", t.ID, c.state)
			} else {
				_, _, err := c.popNodeAndState()
				return err
			}
		},
	},
	stateNodeEnd: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			return nil
		},
		tokenizer.ClassEndOfLine: func(c *ParseContext, t tokenizer.Token) error {
			if c.continuation {
				c.stopContinuation()
				c.state = stateNodeParams
				return nil
			} else {
				_, _, err := c.popNodeAndState()
				return err
			}
		},
	},
	stateProperty: {
		tokenizer.Equals: func(c *ParseContext, t tokenizer.Token) error {
			// cannot cannot use a type annotation on a property key
			if c.typeAnnot.Valid() {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}

			// equals is the only valid value after a bare-identifier property name
			c.state = statePropertyValue
			return nil
		},
		tokenizer.BraceClose: func(c *ParseContext, t tokenizer.Token) error {
			_, _, err := c.popNodeAndState()
			if err != nil {
				return err
			}
			return ErrReenterState
		},
	},
	stateArgProp: {
		tokenizer.TokenComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.ignoreNextArgProp {
				c.ignoreNextArgProp = false
			} else if c.ident.Valid() {
				if err := c.addArgumentToken(c.currentNode(), c.ident, c.typeAnnot); err != nil {
					return err
				}
			}
			if c.ident.Valid() {
				c.typeAnnot.Clear()
				c.ident.Clear()
			} else if c.typeAnnot.Valid() {
				return fmt.Errorf("expected value after type, found %s in state %s", t.ID, c.state)
			}

			c.ignoreNextArgProp = true
			c.markIgnoreFromContinuation()
			c.state = stateNodeParams
			return nil
		},

		tokenizer.BraceOpen: func(c *ParseContext, t tokenizer.Token) error {
			// if we're at the end of the node and didn't find an equal sign, it was just an argument

			if c.ident.Valid() {
				if c.ignoreNextArgProp {
					c.ignoreNextArgProp = false
				} else if err := c.addArgumentToken(c.currentNode(), c.ident, c.typeAnnot); err != nil {
					return err
				}
				c.typeAnnot.Clear()
				c.ident.Clear()
			}

			if c.ignoreNextArgProp || c.ignoreChildren > 0 {
				c.ignoreNextArgProp = false
				c.ignoreChildren++
			}

			c.pushState(stateChildren)
			return nil
		},
		tokenizer.Equals: func(c *ParseContext, t tokenizer.Token) error {
			// cannot cannot use a type annotation on a property key
			// cannot use anything but an identifier or string as a property name
			if c.typeAnnot.Valid() || (c.ident.ID != tokenizer.BareIdentifier && c.ident.ID != tokenizer.QuotedString && c.ident.ID != tokenizer.RawString) {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}

			// equals indicates that it's a property
			c.state = statePropertyValue
			return nil
		},

		tokenizer.Whitespace: func(c *ParseContext, p tokenizer.Token) error {
			if c.opts.Version == tokenizer.VersionV2 {
				return nil
			}

			// whitespace indicates it was definitely an arg, not a prop
			if c.ignoreNextArgProp {
				c.ignoreNextArgProp = false
			} else if err := c.addArgumentToken(c.currentNode(), c.ident, c.typeAnnot); err != nil {
				return err
			}
			c.typeAnnot.Clear()
			c.ident.Clear()

			c.state = stateNodeParams
			return nil
		},

		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version == tokenizer.VersionV2 {
				return nil
			}
			return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
		},

		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			c.startContinuation()
			return nil
		},

		tokenizer.ClassTerminator: func(c *ParseContext, t tokenizer.Token) error {
			if c.continuation {
				return nil
			}
			if c.ident.Valid() {
				// if we're at the end of the node and have an identifier but didn't find an equal sign, it was just an argument
				if c.ignoreNextArgProp {
					c.ignoreNextArgProp = false
				} else if err := c.addArgumentToken(c.currentNode(), c.ident, c.typeAnnot); err != nil {
					return err
				}
				c.typeAnnot.Clear()
				c.ident.Clear()
			}

			// and the node is done
			_, _, err := c.popNodeAndState()
			return err
		},
		tokenizer.ClassValue: func(c *ParseContext, t tokenizer.Token) error {
			// if we found a value, but we already have an identifier queued, it was an argument, so save it
			if c.ignoreNextArgProp {
				c.ignoreNextArgProp = false
			} else if err := c.addArgumentToken(c.currentNode(), c.ident, c.typeAnnot); err != nil {
				return err
			}
			c.typeAnnot.Clear()
			c.ident.Clear()

			c.ident = t
			// c.state stays the same, because we're still determining if this is an arg or prop
			return nil
		},
		tokenizer.BraceClose: func(c *ParseContext, t tokenizer.Token) error {
			if c.ident.Valid() {
				// if we're at the end of the node and have an identifier but didn't find an equal sign, it was just an argument
				if c.ignoreNextArgProp {
					c.ignoreNextArgProp = false
				} else if err := c.addArgumentToken(c.currentNode(), c.ident, c.typeAnnot); err != nil {
					return err
				}
				c.typeAnnot.Clear()
				c.ident.Clear()
			}

			_, _, err := c.popNodeAndState()
			if err != nil {
				return err
			}
			return ErrReenterState
		},
	},
	statePropertyValue: {
		tokenizer.Whitespace: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.ClassComment: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			return nil
		},
		tokenizer.Continuation: func(c *ParseContext, t tokenizer.Token) error {
			if c.opts.Version != tokenizer.VersionV2 {
				return fmt.Errorf("unexpected %s in state %s", t.ID, c.state)
			}
			c.startContinuation()
			return nil
		},
		tokenizer.ParensOpen: func(c *ParseContext, t tokenizer.Token) error {
			// a ( inside a node declaration is hte beginning of a type annotation for a node
			c.pushState(stateTypeAnnot)
			return nil
		},
		tokenizer.ClassValue: func(c *ParseContext, t tokenizer.Token) error {
			if c.ignoreNextArgProp {
				c.ignoreNextArgProp = false
			} else if _, err := c.addPropertyToken(c.currentNode(), c.ident, t, c.typeAnnot); err != nil {
				return err
			}
			c.typeAnnot.Clear()
			c.ident.Clear()
			c.state = stateNode
			return nil
		},
		tokenizer.BraceClose: func(c *ParseContext, t tokenizer.Token) error {
			_, _, err := c.popNodeAndState()
			if err != nil {
				return err
			}
			return ErrReenterState
		},
	},
}
