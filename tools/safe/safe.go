package safe

import (
	"fmt"
	"reflect"
)

// MustNotNil panics if the given value is nil.
// Useful for enforcing required fields during struct initialization.
func MustNotNil(v any, name string) {
	if reflect.ValueOf(v).IsNil() {
		panic(fmt.Sprintf("%s must not be nil", name))
	}
}

// DefaultString returns the dereferenced value of a string pointer,
// or the fallback if the pointer is nil.
func DefaultString(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

// DefaultInt returns the dereferenced value of an int pointer,
// or the fallback if the pointer is nil.
func DefaultInt(i *int, fallback int) int {
	if i == nil {
		return fallback
	}
	return *i
}

// SafeGo starts a new goroutine that recovers from panic,
// so that panics don't crash the entire program.
func SafeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[SafeGo] panic recovered: %v\n", r)
			}
		}()
		f()
	}()
}
