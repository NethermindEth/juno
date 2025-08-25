package commontrie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

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
