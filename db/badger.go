package db

import (
	"errors"

	"github.com/dgraph-io/badger/v3"
)

// Todo: there is no reason to create badgerDb since interfaces implementations are implicit in
// golang
type badgerDb struct {
	badger *badger.DB
}

// NewTransaction : see db.DB.NewTransaction
func (db *badgerDb) NewTransaction(update bool) Transaction {
	return &badgerTxn{db.badger.NewTransaction(update)}
}

// Close : see io.Closer.Close
func (db *badgerDb) Close() error {
	return db.badger.Close()
}

// View : see db.DB.View
func (db *badgerDb) View(fn func(txn Transaction) error) error {
	return db.badger.View(func(txn *badger.Txn) error {
		return fn(&badgerTxn{txn})
	})
}

// Update : see db.DB.Update
func (db *badgerDb) Update(fn func(txn Transaction) error) error {
	return db.badger.Update(func(txn *badger.Txn) error {
		return fn(&badgerTxn{txn})
	})
}

// Impl : see db.DB.Impl
func (db *badgerDb) Impl() any {
	return db.badger
}

// Todo: badgerTxn should be made public and only Get needs to have a definition since it is
// "overriding" the badger txn Get function.
type badgerTxn struct {
	badger *badger.Txn
}

// Discard : see db.Transaction.Discard
func (t *badgerTxn) Discard() {
	t.badger.Discard()
}

// Commit : see db.Transaction.Commit
func (t *badgerTxn) Commit() error {
	return t.badger.Commit()
}

// Set : see db.Transaction.Set
func (t *badgerTxn) Set(key, val []byte) error {
	return t.badger.Set(key, val)
}

// Delete : see db.Transaction.Delete
func (t *badgerTxn) Delete(key []byte) error {
	return t.badger.Delete(key)
}

// Get : see db.Transaction.Get
func (t *badgerTxn) Get(key []byte) ([]byte, error) {
	item, err := t.badger.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrKeyNotFound
		}

		return nil, err
	}

	var value []byte
	return value, item.Value(func(val []byte) error {
		value = append([]byte{}, val...)
		return nil
	})
}

// Seek : see db.Transaction.Seek
func (t *badgerTxn) Seek(key []byte) (*Entry, error) {
	it := t.badger.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek(key)
	if it.Valid() {
		item := it.Item()
		next := &Entry{
			Key: item.Key(),
		}
		return next, item.Value(func(val []byte) error {
			next.Value = append([]byte{}, val...)
			return nil
		})
	}

	return nil, nil
}

// Impl : see db.Transaction.Impl
func (t *badgerTxn) Impl() any {
	return t.badger
}

// NewDb opens a new database at the given path
func NewDb(path string) (DB, error) {
	opt := badger.DefaultOptions(path)
	db, err := badger.Open(opt)
	return &badgerDb{db}, err
}

// NewInMemoryDb opens a new in-memory database
func NewInMemoryDb() (DB, error) {
	opt := badger.DefaultOptions("").WithInMemory(true)
	opt.Logger = nil
	db, err := badger.Open(opt)
	return &badgerDb{db}, err
}

// NewTestDb opens a new in-memory database, panics on error
func NewTestDb() DB {
	db, err := NewInMemoryDb()
	if err != nil {
		panic(err)
	}
	return db
}
