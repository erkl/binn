package binn

import (
	"fmt"
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
