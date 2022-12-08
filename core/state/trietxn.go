package state

import (
	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
)

type TrieTxn struct {
	badgerTxn *badger.Txn
	prefix    []byte
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

func (txn *TrieTxn) Put(key *bitset.BitSet, value *core.TrieNode) error {
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

func (txn *TrieTxn) Get(key *bitset.BitSet) (*core.TrieNode, error) {
	dbKey, err := txn.dbKey(key)
	if err != nil {
		return nil, err
	}

	if item, err := txn.badgerTxn.Get(dbKey); err == nil {
		node := new(core.TrieNode)
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
