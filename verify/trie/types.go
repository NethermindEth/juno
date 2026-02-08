package trie

import (
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

func (t TrieType) IsValid() bool {
	switch t {
	case ContractTrie, ClassTrie, ContractStorageTrie:
		return true
	default:
		return false
	}
}

type TrieInfo struct {
	Name       string
	Prefix     []byte
	HashFn     crypto.HashFn
	ReaderFunc func(db.KeyValueReader, []byte, uint8) (trie.TrieReader, error)
	Height     uint8
}
