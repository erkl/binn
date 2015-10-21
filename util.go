package binn

import (
	"fmt"
	"reflect"
	"sort"
)

const (
	intBits = 32 << (^uint(0) >> 63)
	maxInt  = 1<<(intBits-1) - 1
)

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
