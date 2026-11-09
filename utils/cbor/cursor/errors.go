package cursor

import (
	"errors"
	"fmt"
)

var (
	ErrTruncated   = errors.New("cbor: unexpected end of input")
	ErrSyntax      = errors.New("cbor: malformed item")
	ErrType        = errors.New("cbor: item does not fit the destination")
	ErrDepth       = errors.New("cbor: nesting too deep")
	ErrUnsupported = errors.New("cbor: unsupported encoding")
)

// DecodeError keeps the error category and position separate from diagnostic formatting.
// Unwrap preserves errors.Is; Error formats the message only when a caller requests it.
type DecodeError struct {
	Kind   error
	Offset int
	reason string
	args   []any
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s at byte %d: %s", e.Kind, e.Offset, fmt.Sprintf(e.reason, e.args...))
}

func (e *DecodeError) Unwrap() error { return e.Kind }

// fail records the category, position and diagnostic arguments without formatting.
func (c *Cursor) fail(kind error, format string, args ...any) error {
	return &DecodeError{Kind: kind, Offset: c.pos, reason: format, args: args}
}

func errDepth(pos int) error {
	return &DecodeError{Kind: ErrDepth, Offset: pos, reason: "nested more than %d levels", args: []any{maxNestedLevels}}
}
