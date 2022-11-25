package db

import (
	"fmt"

	"github.com/torquem-ch/mdbx-go/mdbx"
)

type MdbxDb struct {
	env *mdbx.Env
}

var _ Database = &MdbxDb{}

func NewMdbxDb(path string, maxBuckets uint64) (*MdbxDb, error) {
	env, err := mdbx.NewEnv()
	if err != nil {
		return nil, err
	}

	// By default, mdbx will only create enough buckets for its
	// two CORE_DBs [1]. The first core bucket is used to track free
	// pages and the second is the "main" bucket [2].
	// [1] mdbx_env_create in mdbx.c
	// [2] https://blog.separateconcerns.com/2016-04-03-lmdb-format.html
	env.SetOption(mdbx.OptMaxDB, maxBuckets)

	// TODO more options?

	// TODO which flags and mode?
	if err := env.Open(path, 0 /* flags */, 0o664 /* FileMode */); err != nil {
		env.Close()
		return nil, err
	}

	return &MdbxDb{
		env: env,
	}, nil
}

func (db *MdbxDb) Update(op TxOp) error {
	return db.env.Update(func(tx *mdbx.Txn) error {
		return op(newMdbxTx(tx))
	})
}

func (db *MdbxDb) View(op TxOp) error {
	return db.env.View(func(tx *mdbx.Txn) error {
		return op(newMdbxTx(tx))
	})
}

func (db *MdbxDb) Close() error {
	db.env.Close()
	return nil
}

type mdbxTx struct {
	tx *mdbx.Txn
}

var _ Transaction = &mdbxTx{}

func newMdbxTx(tx *mdbx.Txn) *mdbxTx {
	return &mdbxTx{
		tx: tx,
	}
}

func (tx *mdbxTx) CreateBucketIfNotExists(name string) (Bucket, error) {
	b, err := tx.tx.CreateDBI(name)
	if err != nil {
		return nil, err // TODO wrap error
	}
	return newMdbxBucket(tx.tx, b), nil
}

func (tx *mdbxTx) Bucket(name string) (Bucket, error) {
	b, err := tx.tx.OpenDBI(name, 0, nil, nil) // TODO params
	if err != nil {
		return nil, err // TODO wrap error
	}
	return newMdbxBucket(tx.tx, b), nil
}

type mdbxBucket struct {
	tx *mdbx.Txn
	b  mdbx.DBI
}

var _ Bucket = &mdbxBucket{}

func newMdbxBucket(tx *mdbx.Txn, b mdbx.DBI) *mdbxBucket {
	return &mdbxBucket{
		tx: tx,
		b:  b,
	}
}

func (b *mdbxBucket) Get(key []byte) ([]byte, error) {
	return b.tx.Get(b.b, key)
}

func (b *mdbxBucket) Put(key []byte, value []byte) error {
	return b.tx.Put(b.b, key, value, 0 /* flags */) // TODO: flags
}

func (b *mdbxBucket) Cursor() (Cursor, error) {
	c, err := b.tx.OpenCursor(b.b)
	if err != nil {
		return nil, err
	}
	return newMdbxCursor(c), nil
}

type mdbxCursor struct {
	c *mdbx.Cursor
}

var _ Cursor = &mdbxCursor{}

func newMdbxCursor(c *mdbx.Cursor) *mdbxCursor {
	return &mdbxCursor{
		c: c,
	}
}

func (c *mdbxCursor) Seek(prefix []byte) ([]byte, []byte, error) {
	// See https://github.com/ledgerwatch/erigon-lib/blob/661ef494cf0035ddb4caa804394f66532d5cb2d3/kv/mdbx/kv_mdbx.go#L1159-L1178
	var k, v []byte
	var err error
	if len(prefix) == 0 {
		k, v, err = c.get(nil, nil, mdbx.First)
	} else {
		k, v, err = c.get(prefix, nil, mdbx.SetRange)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed seek on prefix %x: %w", prefix, err)
	}
	return k, v, nil
}

func (c *mdbxCursor) Next() ([]byte, []byte, error) {
	return c.get(nil, nil, mdbx.Next)
}

func (c *mdbxCursor) Prev() ([]byte, []byte, error) {
	return c.get(nil, nil, mdbx.Prev)
}

func (c *mdbxCursor) First() ([]byte, []byte, error) {
	return c.get(nil, nil, mdbx.First)
}

func (c *mdbxCursor) Last() ([]byte, []byte, error) {
	return c.get(nil, nil, mdbx.Last)
}

func (c *mdbxCursor) get(prefix []byte, value []byte, op uint) ([]byte, []byte, error) {
	k, v, err := c.c.Get(prefix, value, op)
	// See TODO
	if err != nil {
		if mdbx.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed cursor.get(): %w", err)
	}
	return k, v, nil
}

func (c *mdbxCursor) Close() error {
	c.c.Close()
	return nil
}
