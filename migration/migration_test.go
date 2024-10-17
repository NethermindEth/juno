package migration_test

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
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

func TestMigrateContractFields(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)

	// Test data
	contracts := []struct {
		addr             *felt.Felt
		nonce            *felt.Felt
		classHash        *felt.Felt
		deploymentHeight uint64
	}{
		{new(felt.Felt).SetUint64(1), new(felt.Felt).SetUint64(11), new(felt.Felt).SetUint64(111), 1111},
		{new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(22), new(felt.Felt).SetUint64(222), 2222},
		{new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(33), new(felt.Felt).SetUint64(333), 3333},
	}

	// Set up initial data
	for _, c := range contracts {
		addrBytes := c.addr.Marshal()
		hBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(hBytes, c.deploymentHeight)

		require.NoError(t, txn.Set(db.ContractNonce.Key(addrBytes), c.nonce.Marshal()))
		require.NoError(t, txn.Set(db.ContractClassHash.Key(addrBytes), c.classHash.Marshal()))
		require.NoError(t, txn.Set(db.ContractDeploymentHeight.Key(addrBytes), hBytes))
	}

	// Run migration
	require.NoError(t, migration.MigrateContractFields(txn, nil))

	// Verify results
	for _, c := range contracts {
		var contract core.StateContract
		require.NoError(t, txn.Get(db.Contract.Key(c.addr.Marshal()), func(value []byte) error {
			return encoder.Unmarshal(value, &contract)
		}))
		require.Equal(t, c.nonce, contract.Nonce)
		require.Equal(t, c.classHash, contract.ClassHash)
		require.Equal(t, c.deploymentHeight, contract.DeployHeight)
	}
}
