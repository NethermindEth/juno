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

func getBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

// TransactionStorage is a database transaction on a trie.
type TransactionStorage struct {
	txn    db.Transaction
	prefix []byte
	cache  map[string]*Node
}

func NewTransactionStorage(txn db.Transaction, prefix []byte) *TransactionStorage {
	return &TransactionStorage{
		txn:    txn,
		prefix: prefix,
		cache:  map[string]*Node{},
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
	if err = t.txn.Set(encodedBytes[:keyLen], encodedBytes[keyLen:]); err != nil {
		return err
	}

	if t.cache != nil {
		keyStr := string(encodedBytes[:keyLen])
		t.cache[keyStr] = value
	}
	return nil
}

func (t *TransactionStorage) Get(key *bitset.BitSet) (*Node, error) {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return nil, err
	}

	if t.cache != nil {
		keyStr := buffer.String()
		if node, hit := t.cache[keyStr]; hit {
			return node, nil
		}
	}

	var node *Node
	if err = t.txn.Get(buffer.Bytes(), func(val []byte) error {
		node = new(Node)
		return encoder.Unmarshal(val, node)
	}); err != nil {
		return nil, err
	}

	if t.cache != nil {
		keyStr := buffer.String()
		t.cache[keyStr] = node
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

	if err = t.txn.Delete(buffer.Bytes()); err != nil {
		return err
	}

	if t.cache != nil {
		keyStr := buffer.String()
		delete(t.cache, keyStr)
	}
	return nil
}

func newMemStorage() Storage {
	return &TransactionStorage{
		txn: db.NewMemTransaction(),
	}
}
