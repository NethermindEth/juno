package commontrie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
)

type Trie interface {
	Update(key, value *felt.Felt) error
	Get(key *felt.Felt) (felt.Felt, error)
	Hash() (felt.Felt, error)
	HashFn() crypto.HashFn
}

type DeprecatedTrieAdapter trie.Trie

func NewDeprecatedTrieAdapter(t *trie.Trie) *DeprecatedTrieAdapter {
	return (*DeprecatedTrieAdapter)(t)
}

func (dta *DeprecatedTrieAdapter) Update(key, value *felt.Felt) error {
	_, err := (*trie.Trie)(dta).Put(key, value)
	return err
}

func (dta *DeprecatedTrieAdapter) Get(key *felt.Felt) (felt.Felt, error) {
	value, err := (*trie.Trie)(dta).Get(key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (dta *DeprecatedTrieAdapter) Hash() (felt.Felt, error) {
	root, err := (*trie.Trie)(dta).Root()
	if err != nil {
		return felt.Zero, err
	}
	return *root, nil
}

func (dta *DeprecatedTrieAdapter) HashFn() crypto.HashFn {
	return (*trie.Trie)(dta).HashFn()
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
