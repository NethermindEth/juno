package triedb

import (
	"bytes"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var dbBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type Database struct {
	txn    db.Transaction
	prefix db.Bucket
}

func New(txn db.Transaction, prefix db.Bucket) *Database {
	return &Database{txn: txn, prefix: prefix}
}

func (d *Database) Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.BitArray) (int, error) {
	dbBuf := dbBufferPool.Get().(*bytes.Buffer)
	dbBuf.Reset()
	defer func() {
		dbBuf.Reset()
		dbBufferPool.Put(dbBuf)
	}()

	if err := d.dbKey(dbBuf, owner, path); err != nil {
		return 0, err
	}

	err := d.txn.Get(dbBuf.Bytes(), func(blob []byte) error {
		buf.Write(blob)
		return nil
	})
	if err != nil {
		return 0, err
	}

	return buf.Len(), nil
}

func (d *Database) Put(owner felt.Felt, path trieutils.BitArray, blob []byte) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path); err != nil {
		return err
	}

	return d.txn.Set(buffer.Bytes(), blob)
}

func (d *Database) Delete(owner felt.Felt, path trieutils.BitArray) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path); err != nil {
		return err
	}

	return d.txn.Delete(buffer.Bytes())
}

func (d *Database) dbKey(buf *bytes.Buffer, owner felt.Felt, path trieutils.BitArray) error {
	_, err := buf.Write(d.prefix.Key())
	if err != nil {
		return err
	}

	if owner != (felt.Felt{}) {
		oBytes := owner.Bytes()
		_, err = buf.Write(oBytes[:])
		if err != nil {
			return err
		}
	}

	_, err = path.Write(buf)
	if err != nil {
		return err
	}

	return nil
}
