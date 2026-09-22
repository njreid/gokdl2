package marshaler

import (
	"reflect"
	"sort"

	"github.com/njreid/gokdl2/internal/coerce"
)

// sortMapKeys sorts the keys of a Go map by their string representation so that maps (which have no inherent
// order) are marshaled deterministically. This used to be gated behind the `kdldeterministic` build tag; it is now
// always on, since nondeterministic map key order caused spurious diffs for anyone who generates and commits KDL
// files under version control.
func sortMapKeys(v []reflect.Value) []reflect.Value {
	ss := make([]string, len(v))
	for i, rv := range v {
		ss[i] = coerce.ToString(rv.Interface())
	}

	idx := make([]int, len(v))
	for i := range idx {
		idx[i] = i
	}

	sort.SliceStable(idx, func(i, j int) bool {
		return ss[idx[i]] < ss[idx[j]]
	})

	sorted := make([]reflect.Value, len(v))
	for i, j := range idx {
		sorted[i] = v[j]
	}
	copy(v, sorted)

	return v
}
