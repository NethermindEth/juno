package migration

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemovePending(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	pendingBlockBytes := []byte("some pending block bytes")
	require.NoError(t, testDB.Update(func(txn db.Transaction) error {
		if err := txn.Set(db.Unused.Key(), pendingBlockBytes); err != nil {
			return err
		}

		if err := txn.Get(db.Unused.Key(), func(_ []byte) error { return nil }); err != nil {
			return err
		}

		if err := removePendingBlock(txn, nil); err != nil {
			return err
		}

		assert.EqualError(t, db.ErrKeyNotFound, testDB.View(func(txn db.Transaction) error {
			return txn.Get(db.Unused.Key(), nil)
		}).Error())

		return nil
	}))
}

func TestL1HandlerTxns(t *testing.T) {
	testdb := pebble.NewMemTest(t)
	chain := blockchain.New(testdb, &utils.Sepolia, nil)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	for i := uint64(0); i <= 6; i++ { // First l1 handler txn is in block 6
		b, err := gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err)
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))
	}

	msgHash := common.HexToHash("0x42e76df4e3d5255262929c27132bd0d295a8d3db2cfe63d2fcd061c7a7a7ab34")

	// Delete the L1 handler txn hash from the database
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		return txn.Delete(db.L1HandlerTxnHashByMsgHash.Key(msgHash.Bytes()))
	}))

	// Ensure the key has been deleted
	_, err := chain.L1HandlerTxnHash(&msgHash)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	// Recalculate and store the L1 message hashes
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		return calculateL1MsgHashes(txn, &utils.Sepolia)
	}))

	msgHash = common.HexToHash("0x42e76df4e3d5255262929c27132bd0d295a8d3db2cfe63d2fcd061c7a7a7ab34")
	l1HandlerTxnHash, err := chain.L1HandlerTxnHash(&msgHash)
	require.NoError(t, err)
	assert.Equal(t, l1HandlerTxnHash.String(), "0x785c2ada3f53fbc66078d47715c27718f92e6e48b96372b36e5197de69b82b5")
}

func TestSchemaMetadata(t *testing.T) {
	t.Run("conversion", func(t *testing.T) {
		t.Run("version not set", func(t *testing.T) {
			testDB := pebble.NewMemTest(t)
			metadata, err := SchemaMetadata(testDB)
			require.NoError(t, err)
			require.Equal(t, uint64(0), metadata.Version)
			require.Nil(t, metadata.IntermediateState)
		})

		t.Run("version set", func(t *testing.T) {
			testDB := pebble.NewMemTest(t)
			var version [8]byte
			binary.BigEndian.PutUint64(version[:], 1)
			require.NoError(t, testDB.Update(func(txn db.Transaction) error {
				return txn.Set(db.SchemaVersion.Key(), version[:])
			}))

			metadata, err := SchemaMetadata(testDB)
			require.NoError(t, err)
			require.Equal(t, uint64(1), metadata.Version)
			require.Nil(t, metadata.IntermediateState)
		})
	})
	t.Run("update", func(t *testing.T) {
		t.Run("Intermediate nil", func(t *testing.T) {
			testDB := pebble.NewMemTest(t)
			version := uint64(5)
			require.NoError(t, testDB.Update(func(txn db.Transaction) error {
				return updateSchemaMetadata(txn, schemaMetadata{
					Version:           version,
					IntermediateState: nil,
				})
			}))
			metadata, err := SchemaMetadata(testDB)
			require.NoError(t, err)
			require.Equal(t, version, metadata.Version)
			require.Nil(t, metadata.IntermediateState)
		})

		t.Run("Intermediate not nil", func(t *testing.T) {
			testDB := pebble.NewMemTest(t)
			var (
				intermediateState = []byte{1, 2, 3, 4}
				version           = uint64(5)
			)
			require.NoError(t, testDB.Update(func(txn db.Transaction) error {
				return updateSchemaMetadata(txn, schemaMetadata{
					Version:           version,
					IntermediateState: intermediateState,
				})
			}))
			metadata, err := SchemaMetadata(testDB)
			require.NoError(t, err)
			require.Equal(t, version, metadata.Version)
			require.Equal(t, intermediateState, metadata.IntermediateState)
		})

		t.Run("Intermediate empty", func(t *testing.T) {
			testDB := pebble.NewMemTest(t)
			var (
				intermediateState = make([]byte, 0)
				version           = uint64(5)
			)
			require.NoError(t, testDB.Update(func(txn db.Transaction) error {
				return updateSchemaMetadata(txn, schemaMetadata{
					Version:           version,
					IntermediateState: intermediateState,
				})
			}))
			metadata, err := SchemaMetadata(testDB)
			require.NoError(t, err)
			require.Equal(t, version, metadata.Version)
			require.Equal(t, intermediateState, metadata.IntermediateState)
		})
	})
}

type testMigration struct {
	exec   func(context.Context, db.Transaction, *utils.Network) ([]byte, error)
	before func([]byte) error
}

func (f testMigration) Migrate(ctx context.Context, txn db.Transaction, network *utils.Network, _ utils.SimpleLogger) ([]byte, error) {
	return f.exec(ctx, txn, network)
}

func (f testMigration) Before(state []byte) error { return f.before(state) }

func TestMigrateIfNeededInternal(t *testing.T) {
	t.Run("failure at schema", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.Transaction, *utils.Network) ([]byte, error) {
					return nil, errors.New("foo")
				},
				before: func([]byte) error {
					return errors.New("bar")
				},
			},
		}
		require.ErrorContains(t, migrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations), "bar")
	})

	t.Run("call with new tx", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		var counter int
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.Transaction, *utils.Network) ([]byte, error) {
					if counter == 0 {
						counter++
						return nil, ErrCallWithNewTransaction
					}
					return nil, nil
				},
				before: func([]byte) error {
					return nil
				},
			},
		}
		require.NoError(t, migrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations))
	})

	t.Run("error during migration", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.Transaction, *utils.Network) ([]byte, error) {
					return nil, errors.New("foo")
				},
				before: func([]byte) error {
					return nil
				},
			},
		}
		require.ErrorContains(t, migrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations), "foo")
	})

	t.Run("error if using new db on old version of juno", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.Transaction, *utils.Network) ([]byte, error) {
					return nil, nil
				},
				before: func([]byte) error {
					return nil
				},
			},
		}
		require.NoError(t, migrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations))
		want := "db is from a newer, incompatible version of Juno"
		require.ErrorContains(t, migrateIfNeeded(context.Background(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), []Migration{}), want)
	})
}
