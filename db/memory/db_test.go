package memory_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestMemoryDB(t *testing.T) {
	t.Run("test suite", func(t *testing.T) {
		db.TestKeyValueStoreSuite(t, func() db.KeyValueStore {
			return memory.New()
		})
	})
}

func TestMemoryDBPathLifecycle(t *testing.T) {
	memoryDB := memory.New()

	path := memoryDB.Path()
	require.NotEmpty(t, path)

	require.NoError(t, memoryDB.Close())
}

func TestMemoryDBCopyGetsIndependentPath(t *testing.T) {
	memoryDB := memory.New()

	copyDB := memoryDB.Copy()
	copyPath := copyDB.Path()
	require.NotEmpty(t, copyPath)
	require.NotEqual(t, memoryDB.Path(), copyPath)

	require.NoError(t, memoryDB.Close())
	require.NoError(t, copyDB.Close())
}
