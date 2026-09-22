//
// Properties represents the list of properties (key=value pairs) attached to a Node. Property order is preserved in
// insertion order (the order Add is called: the order properties appear in a parsed KDL document, or the order
// struct fields are visited when marshaling a Go value), so that repeated generation of the same document produces
// byte-for-byte identical output. This behavior used to be gated behind the `kdldeterministic` build tag, with a
// plain, alphabetically-sorted map used by default; it is now always on, since discarding insertion order caused
// spurious diffs for anyone who generates and commits KDL files under version control.
//

package document

import (
	"github.com/njreid/gokdl2/internal/tokenizer"
)

// Properties represents an ordered list of properties for a Node
type Properties struct {
	order []string
	props map[string]*Value
}

// Allocated indicates whether the property list has been allocated
func (p *Properties) Allocated() bool {
	return p.order != nil
}

// Alloc allocates the property list
func (p *Properties) Alloc() {
	p.order = make([]string, 0, 8)
	p.props = make(map[string]*Value, 8)
}

// Get returns Properties[key]
func (p Properties) Get(key string) (*Value, bool) {
	v, ok := p.props[key]
	return v, ok
}

// Len returns the number of properties
func (p *Properties) Len() int {
	return len(p.order)
}

// Unordered returns the property map with no order guarantee; useful when only key/value lookups are needed and
// insertion order doesn't matter (e.g. when unmarshaling into a Go map)
func (p Properties) Unordered() map[string]*Value {
	return p.props
}

// Add adds a property to the list, preserving the order in which property names were first added
func (p *Properties) Add(name string, val *Value) {
	if _, exists := p.props[name]; !exists {
		p.order = append(p.order, name)
	}
	p.props[name] = val
}

// Exist indicates whether any properties exist
func (p *Properties) Exist() bool {
	return len(p.order) > 0
}

// String returns the KDL representation of the property list, formatting numbers per their flags
func (p *Properties) String() string {
	return p.string(false, tokenizer.VersionV1)
}

func (p *Properties) string(unformatted bool, version tokenizer.Version) string {
	b := make([]byte, 0, len(p.order)*(1+8+1+8))
	for _, k := range p.order {
		v := p.props[k]
		b = append(b, ' ')
		if len(k) > 0 && tokenizer.IsBareIdentifierVersion(k, 0, version) {
			b = append(b, k...)
		} else {
			b = AppendQuotedString(b, k, '"')
		}
		b = append(b, '=')
		// property values must always be quoted
		if unformatted {
			if version == tokenizer.VersionV2 {
				b = append(b, v.UnformattedStringV2()...)
			} else {
				b = append(b, v.UnformattedString()...)
			}
		} else {
			if version == tokenizer.VersionV2 {
				b = append(b, v.FormattedStringV2()...)
			} else {
				b = append(b, v.FormattedString()...)
			}
		}
	}
	return string(b)
}

// UnformattedString returns the KDL representation of the property list, formatting numbers in decimal
func (p *Properties) UnformattedString() string {
	return p.string(true, tokenizer.VersionV1)
}

// StringV2 returns the KDL representation of the property list in KDL v2 syntax.
func (p *Properties) StringV2() string {
	return p.string(false, tokenizer.VersionV2)
}

// UnformattedStringV2 returns the KDL representation of the property list in KDL v2 syntax, formatting numbers in decimal.
func (p *Properties) UnformattedStringV2() string {
	return p.string(true, tokenizer.VersionV2)
}

// AppendTo appends the KDL representation of the property list to b, formatting numbers in decimal, and returns b
func (p Properties) AppendTo(b []byte) []byte {
	required := len(p.order) * (1 + 8 + 1 + 8)
	if cap(b)-len(b) < required {
		r := make([]byte, 0, len(b)+required)
		r = append(r, b...)
		b = r
	}
	for _, k := range p.order {
		v := p.props[k]
		b = append(b, ' ')
		if len(k) > 0 && tokenizer.IsBareIdentifierVersion(k, 0, tokenizer.VersionV1) {
			b = append(b, k...)
		} else {
			b = AppendQuotedString(b, k, '"')
		}
		b = append(b, '=')
		// property values must always be quoted
		b = append(b, v.UnformattedString()...)
	}
	return b
}
