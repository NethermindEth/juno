package trie

import (
	"bytes"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
)

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

// Storage is a database transaction on a trie.
type Storage struct {
	txn    db.SnapshotBatch
	prefix []byte
}

func NewStorage(txn db.SnapshotBatch, prefix []byte) *Storage {
	return &Storage{
		txn:    txn,
		prefix: prefix,
	}
}

// dbKey creates a byte array to be used as a key to our KV store
// it simply appends the given key to the configured prefix
func (t *Storage) dbKey(key *BitArray, buffer *bytes.Buffer) (int, error) {
	_, err := buffer.Write(t.prefix)
	if err != nil {
		return 0, err
	}

	keyLen, err := key.Write(buffer)
	return len(t.prefix) + keyLen, err
}

func (t *Storage) Put(key *BitArray, value *Node) error {
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
	return t.txn.Put(encodedBytes[:keyLen], encodedBytes[keyLen:])
}

func (t *Storage) Get(key *BitArray) (*Node, error) {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return nil, err
	}

	var node *Node
	err = t.txn.Get(buffer.Bytes(), func(val []byte) error {
		node = nodePool.Get().(*Node)
		return node.UnmarshalBinary(val)
	})

	return node, err
}

func (t *Storage) Delete(key *BitArray) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return err
	}
	return t.txn.Delete(buffer.Bytes())
}

func (t *Storage) RootKey() (*BitArray, error) {
	var rootKey *BitArray
	err := t.txn.Get(t.prefix, func(val []byte) error {
		rootKey = new(BitArray)
		return rootKey.UnmarshalBinary(val)
	})
	return rootKey, err
}

func (t *Storage) PutRootKey(newRootKey *BitArray) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := newRootKey.Write(buffer)
	if err != nil {
		return err
	}
	return t.txn.Put(t.prefix, buffer.Bytes())
}

func (t *Storage) DeleteRootKey() error {
	return t.txn.Delete(t.prefix)
}

func (t *Storage) SyncedStorage() *Storage {
	return &Storage{
		txn:    db.NewSyncBatch(t.txn),
		prefix: t.prefix,
	}
}

func newMemStorage() *Storage {
	memoryDB := memory.New()
	return NewStorage(memoryDB.NewSnapshotBatch(), nil)
}
