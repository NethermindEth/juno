package db

import "errors"

var (
	// ErrInternal represents an internal database error.
	ErrInternal = errors.New("internal error")
	// ErrNotFound is used when a key is not found in the database.
	ErrNotFound = errors.New("not found error")
	// ErrTx is returned when a transaction fails for some reason.
	ErrTx = errors.New("transaction error")
	// ErrUnmarshal is returned when unmarshall error fails sor some reason.
	ErrUnmarshal = errors.New("unmarshal error")
	// ErrMarshal is returned when unmarshall error fails sor some reason.
	ErrMarshal = errors.New("marshal error")
)

// IsNotFound checks is the given error is an ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
