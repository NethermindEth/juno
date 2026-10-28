package compactfelt_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/compactfelt"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func migrate(t *testing.T, database db.KeyValueStore) {
	t.Helper()
	migrator := &compactfelt.Migrator{}
	require.NoError(t, migrator.Before(nil))
	state, err := migrator.Migrate(t.Context(), database, &networks.Mainnet, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, state)
}

func TestMigrator(t *testing.T) {
	t.Run("empty database starts compact at block 0", func(t *testing.T) {
		database := memory.New()
		migrate(t, database)

		height, err := core.CompactFeltHeightBucket.Get(database, struct{}{})
		require.NoError(t, err)
		require.Equal(t, uint64(0), height)
	})

	t.Run("synced database starts compact after the chain head", func(t *testing.T) {
		database := memory.New()
		require.NoError(t, core.WriteChainHeight(database, 41))
		migrate(t, database)

		height, err := core.CompactFeltHeightBucket.Get(database, struct{}{})
		require.NoError(t, err)
		require.Equal(t, uint64(42), height)
	})
}
