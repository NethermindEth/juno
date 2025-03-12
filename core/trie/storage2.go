package trie

import (
	"bytes"
	// "sync"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
)

// // bufferPool caches unused buffer objects for later reuse.
// var bufferPool = sync.Pool{
// 	New: func() any {
// 		return new(bytes.Buffer)
// 	},
// }

// // nodePool caches unused node objects for later reuse.
// var nodePool = sync.Pool{
// 	New: func() any {
// 		return new(Node)
// 	},
// }

// func getBuffer() *bytes.Buffer {
// 	buffer := bufferPool.Get().(*bytes.Buffer)
// 	buffer.Reset()
// 	return buffer
// }

// Storage2 is a database transaction on a trie.
type Storage2 struct {
	txn    db.IndexedBatch
	prefix []byte
}

func NewStorage2(txn db.IndexedBatch, prefix []byte) *Storage2 {
	return &Storage2{
		txn:    txn,
		prefix: prefix,
	}
}

// dbKey creates a byte array to be used as a key to our KV store
// it simply appends the given key to the configured prefix
func (t *Storage2) dbKey(key *BitArray, buffer *bytes.Buffer) (int, error) {
	_, err := buffer.Write(t.prefix)
	if err != nil {
		return 0, err
	}

	keyLen, err := key.Write(buffer)
	return len(t.prefix) + keyLen, err
}

func (t *Storage2) Put(key *BitArray, value *Node) error {
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

func (t *Storage2) Get(key *BitArray) (*Node, error) {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return nil, err
	}

	var node *Node
	val, err := t.txn.Get2(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	node = nodePool.Get().(*Node)
	if err := node.UnmarshalBinary(val); err != nil {
		return nil, err
	}
	return node, nil
}

func (t *Storage2) Delete(key *BitArray) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := t.dbKey(key, buffer)
	if err != nil {
		return err
	}
	return t.txn.Delete(buffer.Bytes())
}

func (t *Storage2) RootKey() (*BitArray, error) {
	var rootKey *BitArray
	val, err := t.txn.Get2(t.prefix)
	if err != nil {
		return nil, err
	}
	rootKey = new(BitArray)
	if err := rootKey.UnmarshalBinary(val); err != nil {
		return nil, err
	}
	return rootKey, nil
}

func (t *Storage2) PutRootKey(newRootKey *BitArray) error {
	buffer := getBuffer()
	defer bufferPool.Put(buffer)
	_, err := newRootKey.Write(buffer)
	if err != nil {
		return err
	}
	return t.txn.Put(t.prefix, buffer.Bytes())
}

func (t *Storage2) DeleteRootKey() error {
	return t.txn.Delete(t.prefix)
}

func (t *Storage2) SyncedStorage() *Storage2 {
	return &Storage2{
		txn:    db.NewSyncBatch(t.txn),
		prefix: t.prefix,
	}
}

func newMemStorage2() *Storage2 {
	memoryDB := memory.New()
	return NewStorage2(memoryDB.NewIndexedBatch(), nil)
}
