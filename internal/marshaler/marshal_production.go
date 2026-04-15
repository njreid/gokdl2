//go:build !kdldeterministic

package marshaler

import (
	"reflect"
	"sort"

	"github.com/njreid/gokdl2/internal/coerce"
)

func sortMapKeys(v []reflect.Value) []reflect.Value {
	sort.SliceStable(v, func(i, j int) bool {
		return coerce.ToString(v[i].Interface()) < coerce.ToString(v[j].Interface())
	})
	return v
}
