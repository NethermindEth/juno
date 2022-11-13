package core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
)

type (
	StoragePath  = bitset.BitSet
	StorageValue = []byte
)

type (
	TrieKey   = felt.Felt
	TrieValue = felt.Felt
)

type TrieStorage interface {
	Put(key *StoragePath, value StorageValue) error
	Get(key *StoragePath) (StorageValue, error)
}

type Trie struct {
	root    StoragePath
	storage TrieStorage
}

func PathFromKey(k *TrieKey) *StoragePath {
	regularK := k.ToRegular()
	return bitset.FromWithLength(felt.Bits, regularK[:])
}
