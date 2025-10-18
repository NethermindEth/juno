package commontrie

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeprecatedTrieAdapter(t *testing.T) {
	memDB := memory.New()
	txn := memDB.NewIndexedBatch()
	storage := trie.NewStorage(txn, db.ContractStorage.Key([]byte{0}))
	trie, err := trie.NewTriePedersen(storage, 0)
	require.NoError(t, err)
	adapter := (*DeprecatedTrieAdapter)(trie)

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
