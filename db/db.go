package db

import (
	"io"
)

// TODO: define custom error types

type TxOp func(Transaction) error

type Database interface {
	io.Closer

	Update(TxOp) error
	View(TxOp) error
}

type Transaction interface {
	CreateBucketIfNotExists(string) (Bucket, error)
	Bucket(string) (Bucket, error)
}

// TODO we can remove this bucket interface
type Bucket interface {
	Get([]byte) ([]byte, error)
	Put([]byte, []byte) error
	Cursor() (Cursor, error)
}

type Cursor interface {
	io.Closer

	Seek([]byte) ([]byte, []byte, error)
	First() ([]byte, []byte, error)
	Last() ([]byte, []byte, error)
	Next() ([]byte, []byte, error)
	Prev() ([]byte, []byte, error)
}
