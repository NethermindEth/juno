package migration_test

import (
	"testing"

	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/migration"
	"github.com/stretchr/testify/require"
)

func TestMigrateIfNeeded(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	t.Run("Migration should happen on empty DB", func(t *testing.T) {
		require.NoError(t, migration.MigrateIfNeeded(testDB))
	})

	version, err := migration.SchemaVersion(testDB)
	require.NoError(t, err)
	require.NotEqual(t, 0, version)

	t.Run("subsequent calls to MigrateIfNeeded should not change the DB version", func(t *testing.T) {
		require.NoError(t, migration.MigrateIfNeeded(testDB))
		postVersion, postErr := migration.SchemaVersion(testDB)
		require.NoError(t, postErr)
		require.Equal(t, version, postVersion)
	})
}
