package hashdb

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

type Database struct {
	txn    db.Transaction
	prefix db.Bucket
	config *Config

	CleanCache Cache
	DirtyCache Cache
}

func New(txn db.Transaction, prefix db.Bucket, config *Config) *Database {
	if config == nil {
		config = DefaultConfig
	}
	return &Database{
		txn:        txn,
		prefix:     prefix,
		config:     config,
		CleanCache: NewCache(config.CleanCacheType, config.CleanCacheSize),
		DirtyCache: NewCache(config.DirtyCacheType, config.DirtyCacheSize),
	}
}

func (d *Database) Get(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) (int, error) {
	dbBuf := dbBufferPool.Get().(*bytes.Buffer)
	dbBuf.Reset()
	defer func() {
		dbBuf.Reset()
		dbBufferPool.Put(dbBuf)
	}()

	if err := d.dbKey(dbBuf, owner, path, hash, isLeaf); err != nil {
		return 0, err
	}

	if hit := d.CleanCache.Get(buf, dbBuf.Bytes()); hit {
		return buf.Len(), nil
	}

	if hit := d.DirtyCache.Get(buf, dbBuf.Bytes()); hit {
		return buf.Len(), nil
	}

	err := d.txn.Get(dbBuf.Bytes(), func(blob []byte) error {
		buf.Write(blob)
		return nil
	})
	if err != nil {
		return 0, err
	}

	if d.CleanCache != nil && buf.Len() > 0 {
		d.CleanCache.Set(dbBuf.Bytes(), buf.Bytes())
	}

	return buf.Len(), nil
}

func (d *Database) Put(owner felt.Felt, path trieutils.Path, hash felt.Felt, blob []byte, isLeaf bool) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path, hash, isLeaf); err != nil {
		return err
	}

	d.DirtyCache.Set(buffer.Bytes(), blob)

	return d.txn.Set(buffer.Bytes(), blob)
}

func (d *Database) Delete(owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) error {
	buffer := dbBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer func() {
		buffer.Reset()
		dbBufferPool.Put(buffer)
	}()

	if err := d.dbKey(buffer, owner, path, hash, isLeaf); err != nil {
		return err
	}

	d.CleanCache.Delete(buffer.Bytes())
	d.DirtyCache.Delete(buffer.Bytes())

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
// ClassTrie :
// [1 byte prefix][1 section byte][1 byte node-type][8 byte from path][path length byte][32 byte hash]
//
// ContractTrie :
// [1 byte prefix][1 section byte][1 byte node-type][8 byte from path][path length byte][32 byte hash]
//
// StorageTrie of a Contract :
// [1 byte prefix][1 section byte][32 bytes owner][1 byte node-type][8 byte from path][path length byte][32 byte hash]
//
// Section:
// 0 if state and path length is <= 5.
// 1 if state and path length is > 5.
// 2 if storage.
//
// Hash: [Pedersen(path, value) + length] if length > 0 else [value].

func (d *Database) dbKey(buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) error {
	_, err := buf.Write(d.prefix.Key())
	if err != nil {
		return err
	}

	var section byte
	const shortPathLength = 5
	if d.prefix.Key()[0] == 2 {
		section = 2
	} else {
		if path.Len() <= shortPathLength {
			section = 0
		} else {
			section = 1
		}
	}
	if err := buf.WriteByte(section); err != nil {
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

	const pathSignificantBytes = 8
	pathBytes := make([]byte, pathSignificantBytes)
	pathBuf := bytes.NewBuffer(nil)
	if _, err := path.Write(pathBuf); err != nil {
		return err
	}
	copy(pathBytes, pathBuf.Bytes())
	_, err = buf.Write(pathBytes)
	if err != nil {
		return err
	}

	hashBytes := hash.Bytes()
	_, err = buf.Write(hashBytes[:])
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
