package db

import "errors"

var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrorDirtyShutdown = errors.New("dirty shutdown")
)
