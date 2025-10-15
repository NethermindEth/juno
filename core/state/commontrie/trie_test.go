package commontrie

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrieAdapter(t *testing.T) {
	trie, err := trie2.NewEmptyPedersen()
	require.NoError(t, err)
	adapter := (*TrieAdapter)(trie)

	t.Run("Update", func(t *testing.T) {
		err := adapter.Update(&felt.Zero, &felt.Zero)
		require.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		err := adapter.Update(&felt.Zero, &felt.Zero)
		require.NoError(t, err)

		gotValue, err := adapter.Get(&felt.Zero)
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, gotValue)
	})

	t.Run("Hash", func(t *testing.T) {
		hash, err := adapter.Hash()
		require.NoError(t, err)
		assert.Equal(t, felt.Zero, hash)
	})

	t.Run("HashFn", func(t *testing.T) {
		hashFn := adapter.HashFn()
		assert.NotNil(t, hashFn)
	})
}
