package db

type Databaser interface {
	Has(key []byte) (bool, error)
	GetOne(key []byte) (val []byte, err error)
	Get(key []byte) ([]byte, error)

	Put(key, value []byte) error

	Delete(k, v []byte) error

	Begin()
	Rollback()
	Close()
}
