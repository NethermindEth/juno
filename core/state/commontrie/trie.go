package commontrie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
)

type Trie interface {
	Update(key, value *felt.Felt) error
	Get(key *felt.Felt) (felt.Felt, error)
	Hash() (felt.Felt, error)
	HashFn() crypto.HashFn
}

type TrieAdapter trie2.Trie

func NewTrieAdapter(t *trie2.Trie) *TrieAdapter {
	return (*TrieAdapter)(t)
}

func (ta *TrieAdapter) Update(key, value *felt.Felt) error {
	return (*trie2.Trie)(ta).Update(key, value)
}

func (ta *TrieAdapter) Get(key *felt.Felt) (felt.Felt, error) {
	return (*trie2.Trie)(ta).Get(key)
}

func (ta *TrieAdapter) Hash() (felt.Felt, error) {
	return (*trie2.Trie)(ta).Hash(), nil
}

func (ta *TrieAdapter) HashFn() crypto.HashFn {
	return (*trie2.Trie)(ta).HashFn()
}
