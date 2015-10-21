package binn

import (
	"fmt"
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
