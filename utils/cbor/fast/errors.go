package fast

import "errors"

var (
	// ErrTruncated means the input ended inside an item.
	ErrTruncated = errors.New("cbor: unexpected end of input")
	// ErrSyntax means the input is not well-formed CBOR.
	ErrSyntax = errors.New("cbor: malformed item")
	// ErrType means a well-formed item has a different type than the reader expects,
	// or its value does not fit the Go type.
	ErrType = errors.New("cbor: item does not fit the destination")
	// ErrDepth means the items nest deeper than MaxNestedLevels.
	ErrDepth = errors.New("cbor: nesting too deep")
	// ErrUnsupported means the item starts with a form this package does not read, an
	// indefinite-length string or container. It says nothing about whether the rest of the
	// item is valid; the decoder that takes over decides that. Callers may hand the input
	// to another decoder only while the destination is untouched (the fallback contract).
	ErrUnsupported = errors.New("cbor: unsupported encoding")
)
