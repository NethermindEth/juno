// Package cborerr defines the errors shared by typed CBOR readers and the planner.
package cborerr

import (
	"errors"
	"fmt"
)

// Error sentinels are shared with fastcbor and must be treated as read-only.
var (
	ErrUnsupported    = errors.New("fastcbor: unsupported")
	ErrExtraneousData = errors.New("fastcbor: extraneous data")
)

// DecodeError describes a typed reader failure. Formatting is outside the read path.
type DecodeError struct {
	Kind   error
	Detail string
	Extra  int
}

func (e *DecodeError) Error() string {
	if e.Kind == ErrExtraneousData {
		return fmt.Sprintf("%s: %d bytes", e.Kind, e.Extra)
	}
	return e.Kind.Error() + ": " + e.Detail
}

func (e *DecodeError) Unwrap() error { return e.Kind }
