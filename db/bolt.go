package db

import (
	"os"

	"github.com/boltdb/bolt"
)

type BoltDb struct {
	db *bolt.DB
}

var _ Database = &BoltDb{}

func NewBoltDb(path string, mode os.FileMode, opts *bolt.Options) (*BoltDb, error) {
	db, err := bolt.Open(path, mode, opts)
	if err != nil {
		return nil, err
	}
	return &BoltDb{
		db: db,
	}, nil
}

func (b *BoltDb) Close() error {
	return b.db.Close()
}

func (b *BoltDb) Update(op TxOp) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		return op(newBoltTx(tx))
	})
}

func (b *BoltDb) View(op TxOp) error {
	return b.db.View(func(tx *bolt.Tx) error {
		return op(newBoltTx(tx))
	})
}

type boltTx struct {
	tx *bolt.Tx
}

var _ Transaction = &boltTx{}

func newBoltTx(tx *bolt.Tx) *boltTx {
	return &boltTx{
		tx: tx,
	}
}

func (tx *boltTx) CreateBucket(name []byte) (Bucket, error) {
	b, err := tx.tx.CreateBucket(name)
	if err != nil {
		return nil, err // TODO wrap error
	}
	return newBoltBucket(b), nil
}

func (tx *boltTx) Bucket(name []byte) (Bucket, error) {
	return newBoltBucket(tx.tx.Bucket(name)), nil // TODO wrap error (bolt returns nil if does not exist)
}

type boltBucket struct {
	b *bolt.Bucket
}

var _ Bucket = &boltBucket{}

func newBoltBucket(b *bolt.Bucket) *boltBucket {
	return &boltBucket{
		b: b,
	}
}

func (b *boltBucket) Get(key []byte) ([]byte, error) {
	// TODO Bolt returns nil on empty: we should look at other databases
	// and see if it makes more sense to mimic this behavior or return
	// an error as well
	return b.b.Get(key), nil
}

func (b *boltBucket) Put(key []byte, value []byte) error {
	// TODO Bolt returns nil on empty: we should look at other databases
	// and see if it makes more sense to mimic this behavior or return
	// an error as well
	return b.b.Put(key, value)
}

func (b *boltBucket) Cursor() Cursor {
	return b.b.Cursor()
}
