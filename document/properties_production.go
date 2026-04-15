//go:build !kdldeterministic

//
// properties_unordered.go could otherwise be implemented as a simple map, but provides additional methods to make it a
// drop-in replacement for properties_ordered.go for use during testing.

package document

import (
	"sort"

	"github.com/njreid/gokdl2/internal/tokenizer"
)

// Properties represents a list of properties for a Node
type Properties map[string]*Value

// Allocated indicates whether the property list has been allocated
func (p Properties) Allocated() bool {
	return p != nil
}

// Alloc allocates the property list
func (p *Properties) Alloc() {
	*p = make(map[string]*Value)
}

// Get returns Properties[key]
func (p Properties) Get(key string) (*Value, bool) {
	v, ok := p[key]
	return v, ok
}

// Len returns the number of properties
func (p Properties) Len() int {
	return len(p)
}

// Unordered returns the unordered property map; this simply passes through p in this implementation but is provided
// as it is necessary in the deterministic version
func (p Properties) Unordered() map[string]*Value {
	return p
}

// Add adds a property to the list
func (p Properties) Add(name string, val *Value) {
	p[name] = val
}

// Exist indicates whether any properties exist
func (p Properties) Exist() bool {
	return len(p) > 0
}

// String returns the KDL representation of the property list, formatting numbers per their flags
func (p Properties) String() string {
	return p.string(false, tokenizer.VersionV1)
}

func (p Properties) string(unformatted bool, version tokenizer.Version) string {
	b := make([]byte, 0, len(p)*(1+8+1+8))
	for _, k := range p.sortedKeys() {
		v := p[k]
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
func (p Properties) UnformattedString() string {
	return p.string(true, tokenizer.VersionV1)
}

// StringV2 returns the KDL representation of the property list in KDL v2 syntax.
func (p Properties) StringV2() string {
	return p.string(false, tokenizer.VersionV2)
}

// UnformattedStringV2 returns the KDL representation of the property list in KDL v2 syntax, formatting numbers in decimal.
func (p Properties) UnformattedStringV2() string {
	return p.string(true, tokenizer.VersionV2)
}

// AppendTo appends the KDL representation of the property list to b, formatting numbers in decimal, and returns b
func (p Properties) AppendTo(b []byte) []byte {
	required := len(p) * (1 + 8 + 1 + 8)
	if cap(b)-len(b) < required {
		r := make([]byte, 0, len(b)+required)
		r = append(r, b...)
		b = r
	}
	for _, k := range p.sortedKeys() {
		v := p[k]
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

func (p Properties) sortedKeys() []string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
