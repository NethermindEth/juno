package db

type Database interface {
	BucketExists(name string) (bool, error)

	Has(bucket string, key []byte) (bool, error)
	GetOne(bucket string, key []byte) (val []byte, err error)
	Get(bucket string, key []byte) ([]byte, error)

	Put(bucket string, key, value []byte) error

	Delete(bucket string, k, v []byte) error

	Begin()
	Rollback()
	Close()
}
