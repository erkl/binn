package binn

import (
	"reflect"
	"sort"
)

func init() {
	enumMapKeys = enumMapKeysStrict
}

// enumMapKeysStrict is almost identical to enumMapKeysFast, except for the
// fact that keys are consistently returned in the same, strict order. This
// makes testing much easier.
func enumMapKeysStrict(v reflect.Value) []reflect.Value {
	keys := v.MapKeys()
	sort.Sort(byCanonicalOrder(keys))
	return keys
}

type byCanonicalOrder []reflect.Value

func (x byCanonicalOrder) Len() int           { return len(x) }
func (x byCanonicalOrder) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byCanonicalOrder) Less(i, j int) bool { return mapKeyIsLess(x[i], x[j]) }

func mapKeyIsLess(a, b reflect.Value) bool {
	// Compare values across number sizes.
	switch {
	case reflect.Int <= a.Kind() && a.Kind() <= reflect.Int64:
		if reflect.Int <= b.Kind() && b.Kind() <= reflect.Int64 {
			return a.Int() < b.Int()
		}

	case reflect.Uint <= a.Kind() && a.Kind() <= reflect.Uint64:
		if reflect.Uint <= b.Kind() && b.Kind() <= reflect.Uint64 {
			return a.Uint() < b.Uint()
		}

	case reflect.Float32 <= a.Kind() && a.Kind() <= reflect.Float64:
		if reflect.Float32 <= b.Kind() && b.Kind() <= reflect.Float64 {
			return a.Float() < b.Float()
		}
	}

	// If the kinds are different, go by their natural order.
	if a.Kind() != b.Kind() {
		return a.Kind() < b.Kind()
	}

	switch a.Kind() {
	case reflect.Bool:
		return !a.Bool() && b.Bool()

	case reflect.String:
		return a.String() < b.String()

	case reflect.Ptr, reflect.Interface:
		switch {
		case a.IsNil():
			return !b.IsNil()
		case b.IsNil():
			return false
		default:
			return mapKeyIsLess(a.Elem(), b.Elem())
		}

	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if i >= b.Len() {
				return false
			} else if mapKeyIsLess(a.Index(i), b.Index(i)) {
				return true
			} else if mapKeyIsLess(b.Index(i), a.Index(i)) {
				return false
			}
		}
		return a.Len() < b.Len()

	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if i >= b.NumField() {
				return false
			} else if mapKeyIsLess(a.Field(i), b.Field(i)) {
				return true
			} else if mapKeyIsLess(b.Field(i), a.Field(i)) {
				return false
			}
		}
		return a.Len() < b.Len()
	}

	panic("unreachable")
}
