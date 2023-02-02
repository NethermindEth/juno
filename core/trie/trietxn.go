package trie

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bitset"
)

// TrieTxn is a database transaction on a trie.
type TrieTxn struct {
	txn    db.Transaction
	prefix []byte
}

func NewTrieTxn(txn db.Transaction, prefix []byte) *TrieTxn {
	return &TrieTxn{
		txn:    txn,
		prefix: prefix,
	}
}

// dbKey creates a byte array to be used as a key to our KV store
// it simply appends the given key to the configured prefix
func (t *TrieTxn) dbKey(key *bitset.BitSet) ([]byte, error) {
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return append(t.prefix, keyBytes...), nil
}

func (t *TrieTxn) Put(key *bitset.BitSet, value *Node) error {
	dbKey, err := t.dbKey(key)
	if err != nil {
		return err
	}

	valueBytes, err := encoder.Marshal(value)
	if err != nil {
		return err
	}

	return t.txn.Set(dbKey, valueBytes)
}

func (t *TrieTxn) Get(key *bitset.BitSet) (*Node, error) {
	dbKey, err := t.dbKey(key)
	if err != nil {
		return nil, err
	}

	if val, err := t.txn.Get(dbKey); err != nil {
		return nil, err
	} else {
		node := new(Node)
		return node, encoder.Unmarshal(val, node)
	}
}

func (t *TrieTxn) Delete(key *bitset.BitSet) error {
	dbKey, err := t.dbKey(key)
	if err != nil {
		return err
	}
	return t.txn.Delete(dbKey)
}
