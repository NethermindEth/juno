package db

import "github.com/dgraph-io/badger/v3"

func NewDb(path string) (*badger.DB, error) {
	opt := badger.DefaultOptions(path)
	return badger.Open(opt)
}

func NewInMemoryDb() (*badger.DB, error) {
	opt := badger.DefaultOptions("").WithInMemory(true)
	return badger.Open(opt)
}

func NewTestDb() *badger.DB {
	db, err := NewInMemoryDb()
	if err != nil {
		panic(err)
	}
	return db
}
