package parser

import (
	"strconv"
)

type parserState int

const (
	stateDocument parserState = iota
	stateNode
	stateNodeParams
	stateNodeEnd
	stateArgProp
	stateProperty
	statePropertyValue
	stateChildren
	stateTypeAnnot
	stateTypeDone
)

func (p parserState) String() string {
	switch p {
	case stateDocument:
		return "document"
	case stateNode:
		return "node"
	case stateNodeParams:
		return "node parameters"
	case stateNodeEnd:
		return "node end"
	case stateArgProp:
		return "argument or property"
	case stateProperty:
		return "property"
	case statePropertyValue:
		return "property value"
	case stateChildren:
		return "children"
	case stateTypeAnnot:
		return "type annotation"
	case stateTypeDone:
		return "type annotation close"
	default:
		return strconv.FormatInt(int64(p), 10)
	}
}
