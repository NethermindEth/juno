package migration_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestMigrateIfNeeded(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Run("Migration should not happen on cancelled ctx", func(t *testing.T) {
		require.ErrorIs(t, migration.MigrateIfNeeded(ctx, testDB, &utils.Mainnet, utils.NewNopZapLogger()), ctx.Err())
	})

	meta, err := migration.SchemaMetadata(testDB)
	require.NoError(t, err)
	require.Equal(t, uint64(0), meta.Version)
	require.Nil(t, meta.IntermediateState)

	t.Run("Migration should happen on empty DB", func(t *testing.T) {
		require.NoError(t, migration.MigrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger()))
	})

	meta, err = migration.SchemaMetadata(testDB)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), meta.Version)
	require.Nil(t, meta.IntermediateState)

	t.Run("subsequent calls to MigrateIfNeeded should not change the DB version", func(t *testing.T) {
		require.NoError(t, migration.MigrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger()))
		postVersion, postErr := migration.SchemaMetadata(testDB)
		require.NoError(t, postErr)
		require.Equal(t, meta, postVersion)
	})
}
