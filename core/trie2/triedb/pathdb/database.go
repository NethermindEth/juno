package pathdb

import (
	"bytes"
	"errors"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

var ErrCallEmptyDatabase = errors.New("call to empty database")

var dbBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var (
	_ TrieDB = (*Database)(nil)
	_ TrieDB = (*EmptyDatabase)(nil)
)

type TrieDB interface {
	Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, isLeaf bool) (int, error)
	Put(owner felt.Felt, path trieutils.Path, blob []byte, isLeaf bool) error
	Delete(owner felt.Felt, path trieutils.Path, isLeaf bool) error
	NewIterator(owner felt.Felt) (db.Iterator, error)
}

type Database struct {
	txn    db.Transaction
	prefix db.Bucket
}

func New(txn db.Transaction, prefix db.Bucket) *Database {
	return &Database{txn: txn, prefix: prefix}
}

func (d *Database) Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, isLeaf bool) (int, error) {
	dbBuf := dbBufferPool.Get().(*bytes.Buffer)
	dbBuf.Reset()
	defer func() {
		dbBuf.Reset()
		dbBufferPool.Put(dbBuf)
	}()

	if err := d.dbKey(dbBuf, owner, path, isLeaf); err != nil {
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

func (d *Database) Put(owner felt.Felt, path trieutils.Path, blob []byte, isLeaf bool) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path, isLeaf); err != nil {
		return err
	}

	return d.txn.Set(buffer.Bytes(), blob)
}

func (d *Database) Delete(owner felt.Felt, path trieutils.Path, isLeaf bool) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path, isLeaf); err != nil {
		return err
	}

	return d.txn.Delete(buffer.Bytes())
}

func (d *Database) NewIterator(owner felt.Felt) (db.Iterator, error) {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	_, err := buffer.Write(d.prefix.Key())
	if err != nil {
		return nil, err
	}

	if owner != (felt.Felt{}) {
		oBytes := owner.Bytes()
		_, err := buffer.Write(oBytes[:])
		if err != nil {
			return nil, err
		}
	}

	return d.txn.NewIterator(buffer.Bytes(), true)
}

// Construct key bytes to insert a trie node. The format is as follows:
//
// ClassTrie/ContractTrie:
// [1 byte prefix][1 byte node-type][path]
//
// StorageTrie of a Contract :
// [1 byte prefix][32 bytes owner][1 byte node-type][path]
func (d *Database) dbKey(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, isLeaf bool) error {
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

	var nodeType []byte
	if isLeaf {
		nodeType = leaf.Bytes()
	} else {
		nodeType = nonLeaf.Bytes()
	}

	_, err = buf.Write(nodeType)
	if err != nil {
		return err
	}

	_, err = path.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

type EmptyDatabase struct{}

func (EmptyDatabase) Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, isLeaf bool) (int, error) {
	return 0, ErrCallEmptyDatabase
}

func (EmptyDatabase) Put(owner felt.Felt, path trieutils.Path, blob []byte, isLeaf bool) error {
	return ErrCallEmptyDatabase
}

func (EmptyDatabase) Delete(owner felt.Felt, path trieutils.Path, isLeaf bool) error {
	return ErrCallEmptyDatabase
}

func (EmptyDatabase) NewIterator(owner felt.Felt) (db.Iterator, error) {
	return nil, ErrCallEmptyDatabase
}
