package deprecated_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/deprecated"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestMigrateIfNeeded(t *testing.T) {
	testDB := memory.New()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	t.Run("Migration should not happen on cancelled ctx", func(t *testing.T) {
		require.ErrorIs(
			t,
			deprecated.MigrateIfNeeded(
				ctx,
				testDB,
				&networks.Mainnet,
				utils.NewNopZapLogger(),
			),
			ctx.Err())
	})

	meta, err := deprecated.SchemaMetadata(utils.NewNopZapLogger(), testDB)
	require.NoError(t, err)
	require.Equal(t, uint64(0), meta.Version)
	require.Nil(t, meta.IntermediateState)

	t.Run("Migration should happen on empty DB", func(t *testing.T) {
		require.NoError(
			t,
			deprecated.MigrateIfNeeded(
				t.Context(),
				testDB,
				&networks.Mainnet,
				utils.NewNopZapLogger(),
			),
		)
	})

	meta, err = deprecated.SchemaMetadata(utils.NewNopZapLogger(), testDB)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), meta.Version)
	require.Nil(t, meta.IntermediateState)

	t.Run("subsequent calls to MigrateIfNeeded should not change the DB version", func(t *testing.T) {
		require.NoError(
			t,
			deprecated.MigrateIfNeeded(
				t.Context(),
				testDB,
				&networks.Mainnet,
				utils.NewNopZapLogger(),
			),
		)
		postVersion, postErr := deprecated.SchemaMetadata(utils.NewNopZapLogger(), testDB)
		require.NoError(t, postErr)
		require.Equal(t, meta, postVersion)
	})
}
