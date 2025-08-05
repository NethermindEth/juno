package commontrie_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commontrie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrieAdapter_Update(t *testing.T) {
	trie := &commontrie.TrieAdapter{}
	key := &felt.Felt{}
	value := &felt.Felt{}

	err := trie.Update(key, value)
	require.NoError(t, err)
}

func TestTrieAdapter_Get(t *testing.T) {
	trie := &commontrie.TrieAdapter{}
	key := &felt.Felt{}

	value, err := trie.Get(key)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, value)
}

func TestTrieAdapter_Hash(t *testing.T) {
	trie := &commontrie.TrieAdapter{}

	hash, err := trie.Hash()
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, hash)
}

func TestTrieAdapter_HashFn(t *testing.T) {
	trie := &commontrie.TrieAdapter{}

	hashFn := trie.HashFn()
	assert.NotNil(t, hashFn)
}

func TestTrie2Adapter_Update(t *testing.T) {
	trie := &commontrie.Trie2Adapter{}
	key := &felt.Felt{}
	value := &felt.Felt{}

	err := trie.Update(key, value)
	require.NoError(t, err)
}

func TestTrie2Adapter_Get(t *testing.T) {
	trie := &commontrie.Trie2Adapter{}
	key := &felt.Felt{}

	value, err := trie.Get(key)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, value)
}

func TestTrie2Adapter_Hash(t *testing.T) {
	trie := &commontrie.Trie2Adapter{}

	hash, err := trie.Hash()
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, hash)
}

func TestTrie2Adapter_HashFn(t *testing.T) {
	trie := &commontrie.Trie2Adapter{}

	hashFn := trie.HashFn()
	assert.NotNil(t, hashFn)
}
