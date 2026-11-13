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

// fail returns kind, wrapped with the current position and the reason.
func (c *Cursor) fail(kind error, format string, args ...any) error {
	return fmt.Errorf("%w at byte %d: %s", kind, c.pos, fmt.Sprintf(format, args...))
}

func errDepth(pos int) error {
	return fmt.Errorf("%w at byte %d: nested more than %d levels", ErrDepth, pos, maxNestedLevels)
}
