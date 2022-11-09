package db

// TODO: define custom error types

type TxOp func(Transaction) error

type Database interface {
	Update(TxOp) error
	View(TxOp) error
	Close() error
}

type Transaction interface {
	CreateBucket([]byte) (Bucket, error)
	Bucket([]byte) (Bucket, error)
}

type Bucket interface {
	Get([]byte) ([]byte, error)
	Put([]byte, []byte) error
	Cursor() Cursor
}

type Cursor interface {
	Seek([]byte) ([]byte, []byte)
	Next() ([]byte, []byte)
	Prev() ([]byte, []byte)
}
