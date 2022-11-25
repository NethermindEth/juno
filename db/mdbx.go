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
	tx          *mdbx.Txn
	openBuckets map[string]mdbx.DBI
}

var _ Transaction = &mdbxTx{}

func newMdbxTx(tx *mdbx.Txn) *mdbxTx {
	return &mdbxTx{
		tx:          tx,
		openBuckets: make(map[string]mdbx.DBI),
	}
}

func (tx *mdbxTx) CreateBucketIfNotExists(name string) error {
	// CreateDBI will only create the bucket if it doesn't exist,
	// which is what we want.
	dbi, err := tx.tx.CreateDBI(name)
	if err != nil {
		return err
	}
	tx.openBuckets[name] = dbi
	return nil
}

func (tx *mdbxTx) Cursor(bucketName string) (Cursor, error) {
	if _, ok := tx.openBuckets[bucketName]; !ok {
		dbi, err := tx.tx.OpenDBI(bucketName, 0, nil, nil)
		if err != nil {
			return nil, err
		}
		tx.openBuckets[bucketName] = dbi
	}
	c, err := tx.tx.OpenCursor(tx.openBuckets[bucketName])
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

func (c *mdbxCursor) Get(key []byte) ([]byte, error) {
	_, v, err := c.get(key, nil, mdbx.SetKey)
	return v, err
}

func (c *mdbxCursor) Put(key []byte, value []byte) error {
	err := c.c.Put(key, value, mdbx.NoOverwrite)
	if mdbx.IsKeyExists(err) {
		// The key is present in the database. Mdbx has moved
		// the cursor to the key's position, so all we need to
		// do is overwrite the value.
		err = c.c.Put(key, value, mdbx.Current)
	}
	return err
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
	// See https://github.com/ledgerwatch/erigon-lib/blob/07fa94278fc8785f9bae9c6f92f0aeade5df3dfb/kv/mdbx/kv_mdbx.go#L1141-L1147
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
