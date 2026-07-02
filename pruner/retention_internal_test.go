package pruner

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOldestRetainedBlock(t *testing.T) {
	t.Run("empty database returns ErrKeyNotFound", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		_, err := oldestRetainedBlock(database)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("returns lowest block number with commitments", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		for i := uint64(5); i <= 7; i++ {
			testutils.StoreBlock(t, database, i)
		}

		num, err := oldestRetainedBlock(database)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), num)
	})
}
