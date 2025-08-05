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

// TrieAdapter wraps trie.Trie to implement commontrie.CommonTrie
type TrieAdapter struct {
	Trie *trie.Trie
}

func NewTrieAdapter(t *trie.Trie) *TrieAdapter {
	return &TrieAdapter{Trie: t}
}

func (ta *TrieAdapter) Update(key, value *felt.Felt) error {
	_, err := ta.Trie.Put(key, value)
	return err
}

func (ta *TrieAdapter) Get(key *felt.Felt) (felt.Felt, error) {
	value, err := ta.Trie.Get(key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (ta *TrieAdapter) Hash() (felt.Felt, error) {
	root, err := ta.Trie.Root()
	if err != nil {
		return felt.Zero, err
	}
	return *root, nil
}

func (ta *TrieAdapter) HashFn() crypto.HashFn {
	return ta.Trie.HashFn()
}

// Trie2Adapter wraps trie2.Trie to implement commontrie.CommonTrie
type Trie2Adapter struct {
	Trie *trie2.Trie
}

func NewTrie2Adapter(t *trie2.Trie) *Trie2Adapter {
	return &Trie2Adapter{Trie: t}
}

func (ta *Trie2Adapter) Update(key, value *felt.Felt) error {
	return ta.Trie.Update(key, value)
}

func (ta *Trie2Adapter) Get(key *felt.Felt) (felt.Felt, error) {
	return ta.Trie.Get(key)
}

func (ta *Trie2Adapter) Hash() (felt.Felt, error) {
	return ta.Trie.Hash(), nil
}

func (ta *Trie2Adapter) HashFn() crypto.HashFn {
	return ta.Trie.HashFn()
}
