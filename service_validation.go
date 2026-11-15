package khipu

import (
	"reflect"
)

// Service interfaces may contain typed nils. Reject them before invoking a
// backend method; an interface comparison with nil alone is insufficient.
func nilRunService(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}
