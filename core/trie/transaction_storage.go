package trie

import (
	"bytes"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bitset"
)

var _ Storage = (*TransactionStorage)(nil)

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

func (t *TransactionStorage) Put(key *bitset.BitSet, value *Node) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	keyLen, err := t.dbKey(key, buffer)
	if err != nil {
		return err
	}

	enc := encoder.NewEncoder(buffer)
	err = enc.Encode(value)
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
		return encoder.Unmarshal(val, node)
	}); err != nil {
		return nil, err
	}
	return node, err
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

func newMemStorage() Storage {
	return NewTransactionStorage(db.NewMemTransaction(), nil)
}
