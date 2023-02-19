package trie

import (
	"encoding/hex"
	"fmt"

	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bitset"
)

var _ Storage = (*memStorage)(nil)

type memStorage struct {
	storage map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{storage: make(map[string][]byte)}
}

func (m *memStorage) Put(key *bitset.BitSet, value *Node) error {
	keyEnc, err := key.MarshalBinary()
	if err != nil {
		return err
	}

	vEnc, err := encoder.Marshal(value)
	if err != nil {
		return err
	}

	m.storage[hex.EncodeToString(keyEnc)] = vEnc
	return nil
}

func (m *memStorage) Get(key *bitset.BitSet) (*Node, error) {
	keyEnc, err := key.MarshalBinary()
	if err != nil {
		return nil, err
	}

	value, found := m.storage[hex.EncodeToString(keyEnc)]
	if !found {
		return nil, fmt.Errorf("key not found")
	}

	v := new(Node)
	if err = encoder.Unmarshal(value, v); err != nil {
		return nil, err
	}

	return v, err
}

func (m *memStorage) Delete(key *bitset.BitSet) error {
	keyEnc, err := key.MarshalBinary()
	if err != nil {
		return err
	}

	delete(m.storage, hex.EncodeToString(keyEnc))
	return nil
}
