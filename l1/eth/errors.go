package eth

import "errors"

// ErrNotFound is returned when a requested resource (block, receipt, ...)
// does not exist on the remote endpoint. Replaces ethereum.NotFound.
var ErrNotFound = errors.New("not found")
