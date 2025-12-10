package migration_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestMigrateIfNeeded(t *testing.T) {
	testDB := memory.New()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	t.Run("Migration should not happen on cancelled ctx", func(t *testing.T) {
		require.ErrorIs(t, migration.MigrateIfNeeded(ctx, testDB, &utils.Mainnet, utils.NewNopZapLogger(), &migration.HTTPConfig{}), ctx.Err())
	})

	meta, err := migration.SchemaMetadata(utils.NewNopZapLogger(), testDB)
	require.NoError(t, err)
	require.Equal(t, uint64(0), meta.Version)
	require.Nil(t, meta.IntermediateState)

	t.Run("Migration should happen on empty DB", func(t *testing.T) {
		require.NoError(t, migration.MigrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), &migration.HTTPConfig{}))
	})

	meta, err = migration.SchemaMetadata(utils.NewNopZapLogger(), testDB)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), meta.Version)
	require.Nil(t, meta.IntermediateState)

	t.Run("subsequent calls to MigrateIfNeeded should not change the DB version", func(t *testing.T) {
		require.NoError(t, migration.MigrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), &migration.HTTPConfig{}))
		postVersion, postErr := migration.SchemaMetadata(utils.NewNopZapLogger(), testDB)
		require.NoError(t, postErr)
		require.Equal(t, meta, postVersion)
	})
}
