package binn

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"time"
)

const (
	intBits = 32 << (^uint(0) >> 63)
	maxInt  = 1<<(intBits-1) - 1
)

var (
	typNil      = reflect.TypeOf((*interface{})(nil)).Elem()
	typBool     = reflect.TypeOf(false)
	typTime     = reflect.TypeOf(time.Time{})
	typDuration = reflect.TypeOf(time.Duration(0))
	typFloat    = reflect.TypeOf(float64(0))
	typInt      = reflect.TypeOf(int64(0))
	typMap      = reflect.TypeOf(map[interface{}]interface{}{})
	typList     = reflect.TypeOf([]interface{}{})
	typString   = reflect.TypeOf("")
	typBinary   = reflect.TypeOf([]byte{})
)

var errRangeRegexp = regexp.MustCompile("^runtime error: (index|slice bounds) out of range$")

// The throwf function allows for rudimentary exception-style error handling.
// By wrapping the error in a custom struct type we can easily distinguish
// our own thrown errors from other panics.
func throwf(format string, args ...interface{}) {
	panic(thrown{errorf(format, args...)})
}

type thrown struct {
	err error
}

// errorf is shorthand for fmt.Errorf.
func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// isRangePanic returns true if x is a runtime panic error caused by reading
// a slice index beyond its length.
func isRangePanic(x interface{}) bool {
	err, ok := x.(runtime.Error)
	return ok && errRangeRegexp.MatchString(err.Error())
}

// enumStructFields enumerates all fields with "binn" tags in a struct type,
// including fields of embedded structs.
func enumStructFields(t reflect.Type) []reflect.StructField {
	// We use this queue to visit every field in the struct (both immediate
	// ones and those in embedded structs), breadth-first.
	queue := make([][]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		queue[i] = []int{i}
	}

	var names = make(map[string]bool)
	var fields []reflect.StructField

	// Work through the queue.
	for ; len(queue) > 0; queue = queue[1:] {
		index := queue[0]
		field := t.FieldByIndex(index)

		// todo: Distinguish between empty struct tags and ones that are
		// simply missing.
		name := field.Tag.Get("binn")

		// Visit the fields any embedded structs.
		if field.Anonymous && field.Type.Kind() == reflect.Struct && name == "" {
			index = index[:len(index):len(index)]
			for j := 0; j < field.Type.NumField(); j++ {
				queue = append(queue, append(index, field.Type.Field(j).Index...))
			}
			continue
		}

		// Ignore unexported fields and fields without a "binn" tag.
		if field.PkgPath != "" && name == "" {
			continue
		}

		field.Name = name
		field.Index = index
		fields = append(fields, field)

		names[name] = true
	}

	// Order the fields by their position in the root struct.
	sort.Sort(fieldsByIndex(fields))

	return fields
}

type fieldsByIndex []reflect.StructField

func (x fieldsByIndex) Len() int      { return len(x) }
func (x fieldsByIndex) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x fieldsByIndex) Less(i, j int) bool {
	a, b := x[i].Index, x[j].Index

	for i := range a {
		if i >= len(b) {
			return false
		}
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}

	return len(a) < len(b)
}

// The enumMapKeys function returns the keys in a map. Using a variable allows
// to change the function when running tests to return keys in a deterministic
// order (this is done in util_test.go's init function).
var enumMapKeys = enumMapKeysFast

func enumMapKeysFast(v reflect.Value) []reflect.Value {
	return v.MapKeys()
}

// The describe function produces a string describing the type of
// the first value in b.
func describe(b []byte) string {
	switch k := b[0]; {
	case k <= 0x07:
		return "positive integer"
	case k <= 0x0f:
		return "negative integer"
	case k <= 0x10:
		return "nil"
	case k <= 0x12:
		return "bool"
	case k <= 0x13:
		// todo: Check for "duration".
		return "timestamp"
	case k <= 0x1f:
		return "floating-point number"
	case k <= 0x27:
		return "negative integer"
	case k <= 0x2f:
		return "positive integer"
	case k <= 0x4f:
		return "map"
	case k <= 0x6f:
		return "list"
	case k <= 0xdf:
		return "string"
	case k <= 0xef:
		return "binary"
	default:
		return "extension"
	}
}

// rev32 reverses the lower four bytes in a 64-bit integer.
func rev32(x uint64) uint64 {
	rev := (x & 0x000000ff) << 24
	rev |= (x & 0x0000ff00) << 8
	rev |= (x & 0x00ff0000) >> 8
	rev |= (x & 0xff000000) >> 24

	return rev
}

// rev64 reverses the bytes in a 64-bit integer.
func rev64(x uint64) uint64 {
	rev := (x & 0x00000000000000ff) << 56
	rev |= (x & 0x000000000000ff00) << 40
	rev |= (x & 0x0000000000ff0000) << 24
	rev |= (x & 0x00000000ff000000) << 8
	rev |= (x & 0x000000ff00000000) >> 8
	rev |= (x & 0x0000ff0000000000) >> 24
	rev |= (x & 0x00ff000000000000) >> 40
	rev |= (x & 0xff00000000000000) >> 56

	return rev
}
