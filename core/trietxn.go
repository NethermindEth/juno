package core

import (
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
)

// TrieTxn is a database transaction on a trie.
type TrieTxn struct {
	badgerTxn *badger.Txn
	prefix    []byte
}

func NewTrieTxn(badgerTxn *badger.Txn, prefix []byte) *TrieTxn {
	return &TrieTxn{
		badgerTxn: badgerTxn,
		prefix:    prefix,
	}
}

// dbKey creates a byte array to be used as a key to our KV store
// it simply appends the given key to the configured prefix
func (txn *TrieTxn) dbKey(key *bitset.BitSet) ([]byte, error) {
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return append(txn.prefix, keyBytes...), nil
}

func (txn *TrieTxn) Put(key *bitset.BitSet, value *TrieNode) error {
	dbKey, err := txn.dbKey(key)
	if err != nil {
		return err
	}

	valueBytes, err := value.MarshalBinary()
	if err != nil {
		return err
	}

	return txn.badgerTxn.Set(dbKey, valueBytes)
}

func (txn *TrieTxn) Get(key *bitset.BitSet) (*TrieNode, error) {
	dbKey, err := txn.dbKey(key)
	if err != nil {
		return nil, err
	}

	if item, err := txn.badgerTxn.Get(dbKey); err == nil {
		node := new(TrieNode)
		return node, item.Value(func(val []byte) error {
			return node.UnmarshalBinary(val)
		})
	} else {
		return nil, err
	}
}

func (txn *TrieTxn) Delete(key *bitset.BitSet) error {
	dbKey, err := txn.dbKey(key)
	if err != nil {
		return err
	}
	return txn.badgerTxn.Delete(dbKey)
}

func (txn *TrieTxn) IsEmpty() bool {
	it := txn.badgerTxn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	it.Rewind()
	return !it.Valid()
}
