// Package errors provides typed errors for ktrace.
package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound        = errors.New("resource not found")
	ErrUnsupportedKind = errors.New("unsupported resource kind")
	ErrInvalidArgs     = errors.New("invalid arguments")
	ErrForbidden       = errors.New("forbidden")
)

// NotFound wraps ErrNotFound with resource context.
func NotFound(kind, name, namespace string) error {
	return fmt.Errorf("%w: %s/%s in namespace %q", ErrNotFound, kind, name, namespace)
}

// UnsupportedKind wraps ErrUnsupportedKind with kind context.
func UnsupportedKind(kind string) error {
	return fmt.Errorf("%w: %q", ErrUnsupportedKind, kind)
}

// InvalidArgs wraps ErrInvalidArgs with a message.
func InvalidArgs(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgs, msg)
}

// Forbidden wraps ErrForbidden with context.
func Forbidden(msg string) error {
	return fmt.Errorf("%w: %s", ErrForbidden, msg)
}

// IsNotFound reports whether err is a not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnsupportedKind reports whether err is an unsupported kind error.
func IsUnsupportedKind(err error) bool {
	return errors.Is(err, ErrUnsupportedKind)
}

// IsInvalidArgs reports whether err is an invalid arguments error.
func IsInvalidArgs(err error) bool {
	return errors.Is(err, ErrInvalidArgs)
}
