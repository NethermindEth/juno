package trie

import (
	"slices"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

const (
	StarknetTrieHeight  uint8 = 251
	ConcurrencyMaxDepth uint8 = 8
)

type TrieType string

const (
	ContractTrie        TrieType = "contract"
	ClassTrie           TrieType = "class"
	ContractStorageTrie TrieType = "contract-storage"
)

var allTrieTypes = []TrieType{ContractTrie, ClassTrie, ContractStorageTrie}

func (t TrieType) IsValid() bool {
	return slices.Contains(allTrieTypes, t)
}

type TrieInfo struct {
	Name       string
	Prefix     []byte
	HashFn     crypto.HashFn
	ReaderFunc func(db.KeyValueReader, uint8) (trie.TrieReader, error)
	Height     uint8
}
