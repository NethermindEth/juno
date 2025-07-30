package commontrie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
)

type CommonTrie interface {
	Update(key, value *felt.Felt) error
	Get(key *felt.Felt) (felt.Felt, error)
	Hash() felt.Felt
	HashFn() crypto.HashFn
}

// TrieAdapter wraps trie.Trie to implement commontrie.CommonTrie
type TrieAdapter struct {
	trie *trie.Trie
}

func NewTrieAdapter(t *trie.Trie) *TrieAdapter {
	return &TrieAdapter{trie: t}
}

func (ta *TrieAdapter) Update(key, value *felt.Felt) error {
	_, err := ta.trie.Put(key, value)
	return err
}

func (ta *TrieAdapter) Get(key *felt.Felt) (felt.Felt, error) {
	value, err := ta.trie.Get(key)
	if err != nil {
		return felt.Zero, err
	}
	return *value, nil
}

func (ta *TrieAdapter) Hash() felt.Felt {
	root, _ := ta.trie.Root()
	return *root
}

func (ta *TrieAdapter) HashFn() crypto.HashFn {
	return ta.trie.HashFn()
}

// Trie2Adapter wraps trie2.Trie to implement commontrie.CommonTrie
type Trie2Adapter struct {
	trie *trie2.Trie
}

func NewTrie2Adapter(t *trie2.Trie) *Trie2Adapter {
	return &Trie2Adapter{trie: t}
}

func (ta *Trie2Adapter) Update(key, value *felt.Felt) error {
	return ta.trie.Update(key, value)
}

func (ta *Trie2Adapter) Get(key *felt.Felt) (felt.Felt, error) {
	return ta.trie.Get(key)
}

func (ta *Trie2Adapter) Hash() felt.Felt {
	return ta.trie.Hash()
}

func (ta *Trie2Adapter) HashFn() crypto.HashFn {
	return ta.trie.HashFn()
}
