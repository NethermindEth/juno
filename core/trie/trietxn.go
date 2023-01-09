package trie

import (
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
)

// TrieBadgerTxn is a database transaction on a trie.
type TrieBadgerTxn struct {
	badgerTxn *badger.Txn
	prefix    []byte
}

func NewTrieBadgerTxn(badgerTxn *badger.Txn, prefix []byte) *TrieBadgerTxn {
	return &TrieBadgerTxn{
		badgerTxn: badgerTxn,
		prefix:    prefix,
	}
}

// dbKey creates a byte array to be used as a key to our KV store
// it simply appends the given key to the configured prefix
func (t *TrieBadgerTxn) dbKey(key *bitset.BitSet) ([]byte, error) {
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return append(t.prefix, keyBytes...), nil
}

func (t *TrieBadgerTxn) Put(key *bitset.BitSet, value *Node) error {
	dbKey, err := t.dbKey(key)
	if err != nil {
		return err
	}

	valueBytes, err := value.MarshalBinary()
	if err != nil {
		return err
	}

	return t.badgerTxn.Set(dbKey, valueBytes)
}

func (t *TrieBadgerTxn) Get(key *bitset.BitSet) (*Node, error) {
	dbKey, err := t.dbKey(key)
	if err != nil {
		return nil, err
	}

	if item, err := t.badgerTxn.Get(dbKey); err != nil {
		return nil, err
	} else {
		node := new(Node)
		return node, item.Value(func(val []byte) error {
			return node.UnmarshalBinary(val)
		})
	}
}

func (t *TrieBadgerTxn) Delete(key *bitset.BitSet) error {
	dbKey, err := t.dbKey(key)
	if err != nil {
		return err
	}
	return t.badgerTxn.Delete(dbKey)
}
