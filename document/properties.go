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

// propertyIndexThreshold is the number of properties above which a key->position map is built to speed up lookups.
const propertyIndexThreshold = 16

type property struct {
	Key   string
	Value *Value
}

// Properties represents an ordered list of properties for a Node
type Properties struct {
	list []property
	// index maps keys to positions in list; it is only built once the list grows past propertyIndexThreshold
	index map[string]int
}

// Allocated indicates whether the property list has been allocated
func (p *Properties) Allocated() bool {
	return p.list != nil
}

// Alloc allocates the property list
func (p *Properties) Alloc() {
	p.list = make([]property, 0, 4)
	p.index = nil
}

func (p *Properties) find(key string) int {
	if p.index != nil {
		if i, ok := p.index[key]; ok {
			return i
		}
		return -1
	}
	for i := range p.list {
		if p.list[i].Key == key {
			return i
		}
	}
	return -1
}

// Get returns Properties[key]
func (p Properties) Get(key string) (*Value, bool) {
	if i := p.find(key); i >= 0 {
		return p.list[i].Value, true
	}
	return nil, false
}

// Len returns the number of properties
func (p *Properties) Len() int {
	return len(p.list)
}

// Unordered returns a map of the properties with no order guarantee; useful when only key/value lookups are needed
// and insertion order doesn't matter (e.g. when unmarshaling into a Go map). The map is built on each call.
func (p Properties) Unordered() map[string]*Value {
	if p.list == nil {
		return nil
	}
	m := make(map[string]*Value, len(p.list))
	for _, e := range p.list {
		m[e.Key] = e.Value
	}
	return m
}

// Add adds a property to the list, preserving the order in which property names were first added. If name already
// exists its value is replaced and its position is kept.
func (p *Properties) Add(name string, val *Value) {
	if i := p.find(name); i >= 0 {
		p.list[i].Value = val
		return
	}
	p.list = append(p.list, property{Key: name, Value: val})
	if p.index != nil {
		p.index[name] = len(p.list) - 1
	} else if len(p.list) > propertyIndexThreshold {
		p.index = make(map[string]int, len(p.list)*2)
		for i, e := range p.list {
			p.index[e.Key] = i
		}
	}
}

// Exist indicates whether any properties exist
func (p *Properties) Exist() bool {
	return len(p.list) > 0
}

// String returns the KDL representation of the property list, formatting numbers per their flags
func (p *Properties) String() string {
	return p.string(false, tokenizer.VersionV1)
}

func (p *Properties) string(unformatted bool, version tokenizer.Version) string {
	return string(p.appendTo(make([]byte, 0, len(p.list)*(1+8+1+8)), unformatted, version))
}

// appendTo appends the KDL representation of the property list to b and returns the expanded buffer.
func (p *Properties) appendTo(b []byte, unformatted bool, version tokenizer.Version) []byte {
	valOpts := argValueOpts(unformatted, version)
	for _, e := range p.list {
		k, v := e.Key, e.Value
		b = append(b, ' ')
		if len(k) > 0 && tokenizer.IsBareIdentifierVersion(k, 0, version) {
			b = append(b, k...)
		} else {
			b = AppendQuotedString(b, k, '"')
		}
		b = append(b, '=')
		// property values must always be quoted
		b = v.appendString(b, valOpts, version)
	}
	return b
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
	required := len(p.list) * (1 + 8 + 1 + 8)
	if cap(b)-len(b) < required {
		r := make([]byte, 0, len(b)+required)
		r = append(r, b...)
		b = r
	}
	return p.appendTo(b, true, tokenizer.VersionV1)
}
