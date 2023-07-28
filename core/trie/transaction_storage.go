package trie

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
)

var _ Storage = (*TransactionStorage)(nil)
var notSameDb error = errors.New("not same db")

// bufferPool caches unused buffer objects for later reuse.
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// nodePool caches unused node objects for later reuse.
var nodePool = sync.Pool{
	New: func() any {
		return new(Node)
	},
}

func getBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

// TransactionStorage is a database transaction on a trie.
type TransactionStorage struct {
	txn    db.Transaction
	prefix []byte
}

func NewTransactionStorage(txn db.Transaction, prefix []byte) *TransactionStorage {
	return &TransactionStorage{
		txn:    txn,
		prefix: prefix,
	}
}

// dbKey creates a byte array to be used as a key to our KV store
// it simply appends the given key to the configured prefix
func (t *TransactionStorage) dbKey(key *bitset.BitSet, buffer *bytes.Buffer) (int64, error) {
	_, err := buffer.Write(t.prefix)
	if err != nil {
		return 0, err
	}

	keyLen, err := key.WriteTo(buffer)
	return int64(len(t.prefix)) + keyLen, err
}

func (t *TransactionStorage) keyToBitset(key []byte) (*bitset.BitSet, error) {
	if !bytes.Equal(t.prefix, key[:len(t.prefix)]) {
		return nil, notSameDb
	}

	buff := bytes.NewBuffer(key)

	_, err := io.CopyN(io.Discard, buff, int64(len(t.prefix)))
	if err != nil {
		return nil, err
	}

	bts := &bitset.BitSet{}
	_, err = bts.ReadFrom(buff)
	if err != nil {
		return nil, err
	}

	return bts, nil
}

func (t *TransactionStorage) Put(key *bitset.BitSet, value *Node) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	keyLen, err := t.dbKey(key, buffer)
	if err != nil {
		return err
	}

	_, err = value.WriteTo(buffer)
	if err != nil {
		return err
	}

	encodedBytes := buffer.Bytes()
	return t.txn.Set(encodedBytes[:keyLen], encodedBytes[keyLen:])
}

func (t *TransactionStorage) Get(key *bitset.BitSet) (*Node, error) {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return nil, err
	}

	var node *Node
	if err = t.txn.Get(buffer.Bytes(), func(val []byte) error {
		node = nodePool.Get().(*Node)
		return node.UnmarshalBinary(val)
	}); err != nil {
		return nil, err
	}
	return node, err
}

func (t *TransactionStorage) IterateFrom(key *bitset.BitSet, consumer func(*bitset.BitSet, *Node) (bool, error)) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return err
	}

	it, err := t.txn.NewIterator()
	if err != nil {
		return err
	}

	ok := it.Seek(buffer.Bytes())
	for ok {
		btset, err := t.keyToBitset(it.Key())
		if err == notSameDb {
			break
		}
		if err != nil {
			return err
		}

		bts, err := it.Value()
		if err != nil {
			return err
		}

		keystr := hex.EncodeToString(it.Key())
		valstr := hex.EncodeToString(bts)
		fmt.Printf("R %s -> %s\n", keystr, valstr)

		node := nodePool.Get().(*Node)
		err = node.UnmarshalBinary(bts)
		if err != nil {
			return err
		}

		cont, err := consumer(btset, node)
		if err != nil {
			return err
		}
		if !cont {
			break
		}

		ok = it.Next()
	}

	return nil
}

func (t *TransactionStorage) Delete(key *bitset.BitSet) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return err
	}
	return t.txn.Delete(buffer.Bytes())
}

func (t *TransactionStorage) RootKey() (*bitset.BitSet, error) {
	var rootKey *bitset.BitSet
	if err := t.txn.Get(t.prefix, func(val []byte) error {
		rootKey = new(bitset.BitSet)
		return rootKey.UnmarshalBinary(val)
	}); err != nil {
		return nil, err
	}
	return rootKey, nil
}

func (t *TransactionStorage) PutRootKey(newRootKey *bitset.BitSet) error {
	newRootKeyBytes, err := newRootKey.MarshalBinary()
	if err != nil {
		return err
	}
	return t.txn.Set(t.prefix, newRootKeyBytes)
}

func (t *TransactionStorage) DeleteRootKey() error {
	return t.txn.Delete(t.prefix)
}

func newMemStorage() Storage {
	return NewTransactionStorage(db.NewMemTransaction(), nil)
}
