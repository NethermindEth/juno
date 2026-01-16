package migration

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/migration/l1handlermapping"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bitset"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0000(t *testing.T) {
	testDB := memory.New()

	t.Run("empty DB", func(t *testing.T) {
		txn := testDB.NewIndexedBatch()
		require.NoError(t, migration0000(txn, &utils.Mainnet))
	})

	t.Run("non-empty DB", func(t *testing.T) {
		txn := testDB.NewIndexedBatch()
		require.NoError(t, txn.Put([]byte("asd"), []byte("123")))
		require.NoError(t, txn.Write())

		txn = testDB.NewIndexedBatch()
		require.EqualError(t, migration0000(txn, &utils.Mainnet), "initial DB should be empty")
	})
}

func TestRelocateContractStorageRootKeys(t *testing.T) {
	testDB := memory.New()

	txn := testDB.NewIndexedBatch()
	numberOfContracts := 5

	// Populate the database with entries in the old location.
	for i := range numberOfContracts {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()
		// Use exampleBytes for the key suffix (the contract address) and the value.
		err := txn.Put(db.Peer.Key(exampleBytes[:]), exampleBytes[:])
		require.NoError(t, err)
	}

	require.NoError(t, relocateContractStorageRootKeys(txn, &utils.Mainnet))

	// Each root-key entry should have been moved to its new location
	// and the old entry should not exist.
	for i := range numberOfContracts {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()

		// New entry exists.
		err := txn.Get(db.ContractStorage.Key(exampleBytes[:]), func(data []byte) error {
			require.Equal(
				t,
				exampleBytes[:],
				data,
				"the correct value was not transferred to the new location",
			)
			return nil
		})
		require.NoError(t, err)

		// Old entry does not exist.
		oldKey := db.Peer.Key(exampleBytes[:])
		err = txn.Get(oldKey, func([]byte) error { return nil })
		require.ErrorIs(t, db.ErrKeyNotFound, err)
	}

	// Commit the transaction to release resources
	require.NoError(t, txn.Write())
}

func TestRecalculateBloomFilters(t *testing.T) {
	testDB := memory.New()
	chain := blockchain.New(testDB, &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	for i := range uint64(3) {
		b, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)

		b.EventsBloom = nil
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))
	}

	require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
		return recalculateBloomFilters(txn, &utils.Mainnet)
	}))

	for i := range uint64(3) {
		b, err := chain.BlockByNumber(i)
		require.NoError(t, err)
		assert.Equal(t, core.EventsBloom(b.Receipts), b.EventsBloom)
	}
}

func TestRemovePending(t *testing.T) {
	testDB := memory.New()
	pendingBlockBytes := []byte("some pending block bytes")
	require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
		if err := txn.Put(db.Unused.Key(), pendingBlockBytes); err != nil {
			return err
		}

		err := txn.Get(db.Unused.Key(), func([]byte) error { return nil })
		require.NoError(t, err)

		if err := removePendingBlock(txn, nil); err != nil {
			return err
		}

		err = txn.Get(db.Unused.Key(), func([]byte) error { return nil })
		require.ErrorIs(t, db.ErrKeyNotFound, err)

		return nil
	}))
}

func TestChangeTrieNodeEncoding(t *testing.T) {
	testdb := memory.New()

	buckets := []db.Bucket{db.ClassesTrie, db.StateTrie, db.ContractStorage}

	var n struct {
		Value *felt.Felt
		Left  *bitset.BitSet
		Right *bitset.BitSet
	}
	require.NoError(t, testdb.Update(func(txn db.IndexedBatch) error {
		// contract root keys, if changeTrieNodeEncoding tries to migrate these it
		// will fail with an error since they are not valid trie.Node encodings.
		require.NoError(t, txn.Put(db.ClassesTrie.Key(), []byte{1, 2, 3}))
		require.NoError(t, txn.Put(db.StateTrie.Key(), []byte{1, 2, 3}))
		require.NoError(t, txn.Put(db.ContractStorage.Key(make([]byte, felt.Bytes)), []byte{1, 2, 3}))

		for _, bucket := range buckets {
			for i := range 5 {
				n.Value = new(felt.Felt).SetUint64(uint64(i))

				encodedNode, err := encoder.Marshal(n)
				if err != nil {
					return err
				}

				if err = txn.Put(bucket.Key([]byte{byte(i)}), encodedNode); err != nil {
					return err
				}
			}
		}

		return nil
	}))

	m := new(changeTrieNodeEncoding)
	require.NoError(t, m.Before(nil))
	_, err := m.Migrate(t.Context(), testdb, &utils.Mainnet, nil)
	require.NoError(t, err)

	require.NoError(t, testdb.Update(func(txn db.IndexedBatch) error {
		for _, bucket := range buckets {
			for i := range 5 {
				var coreNode trie.Node
				err := txn.Get(bucket.Key([]byte{byte(i)}), coreNode.UnmarshalBinary)
				require.NoError(t, err)
			}
		}

		return nil
	}))
}

func TestCalculateBlockCommitments(t *testing.T) {
	testdb := memory.New()
	chain := blockchain.New(testdb, &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	for i := range uint64(3) {
		b, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))
	}

	require.NoError(t, testdb.Update(func(txn db.IndexedBatch) error {
		return calculateBlockCommitments(txn, &utils.Mainnet)
	}))
	for i := range uint64(3) {
		b, err := chain.BlockCommitmentsByNumber(i)
		require.NoError(t, err)
		assert.NotNil(t, b.TransactionCommitment)
	}
}

func TestL1HandlerTxns(t *testing.T) {
	testdb := memory.New()
	chain := blockchain.New(testdb, &utils.Sepolia)
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	for i := range uint64(7) { // First l1 handler txn is in block 6
		b, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))
	}

	msgHash := common.HexToHash("0x42e76df4e3d5255262929c27132bd0d295a8d3db2cfe63d2fcd061c7a7a7ab34")

	// Delete the L1 handler txn hash from the database
	require.NoError(t, testdb.Update(func(txn db.IndexedBatch) error {
		return txn.Delete(db.L1HandlerTxnHashByMsgHash.Key(msgHash.Bytes()))
	}))

	// Ensure the key has been deleted
	_, err := chain.L1HandlerTxnHash(&msgHash)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	// Recalculate and store the L1 message hashes
	require.NoError(t, testdb.Update(func(txn db.IndexedBatch) error {
		return calculateL1MsgHashes2(txn, &utils.Sepolia)
	}))

	msgHash = common.HexToHash("0x42e76df4e3d5255262929c27132bd0d295a8d3db2cfe63d2fcd061c7a7a7ab34")
	l1HandlerTxnHash, err := chain.L1HandlerTxnHash(&msgHash)
	require.NoError(t, err)
	assert.Equal(t, l1HandlerTxnHash.String(), "0x785c2ada3f53fbc66078d47715c27718f92e6e48b96372b36e5197de69b82b5")
}

func TestMigrateTrieRootKeysFromBitsetToTrieKeys(t *testing.T) {
	memTxn := memory.New()

	bs := bitset.New(251)
	bsBytes, err := bs.MarshalBinary()
	require.NoError(t, err)

	key := []byte{0}
	err = memTxn.Put(key, bsBytes)
	require.NoError(t, err)

	require.NoError(t, migrateTrieRootKeysFromBitsetToTrieKeys(memTxn, key, bsBytes, &utils.Mainnet))

	var trieKey trie.BitArray
	err = memTxn.Get(key, trieKey.UnmarshalBinary)
	require.NoError(t, err)
	require.Equal(t, bs.Len(), uint(trieKey.Len()))
	require.Equal(t, felt.Zero, trieKey.Felt())
}

func TestMigrateCairo1CompiledClass(t *testing.T) {
	txn := memory.New()

	key := []byte("key")
	class := oldCairo1Class{
		Abi:     "some cairo abi",
		AbiHash: felt.NewRandom[felt.Felt](),
		EntryPoints: core.SierraEntryPointsByType{
			Constructor: []core.SierraEntryPoint{
				{
					Index:    0,
					Selector: felt.NewRandom[felt.Felt](),
				},
			},
			External: []core.SierraEntryPoint{
				{
					Index:    0,
					Selector: felt.NewRandom[felt.Felt](),
				},
			},
			L1Handler: []core.SierraEntryPoint{
				{
					Index:    0,
					Selector: felt.NewRandom[felt.Felt](),
				},
			},
		},
		Program:         randSlice(t),
		ProgramHash:     felt.NewRandom[felt.Felt](),
		SemanticVersion: "0.1.0",
	}
	expectedDeclared := declaredClass{
		At:    777,
		Class: class,
	}

	for _, test := range []struct {
		compiledJSON        string
		checkCompiledExists bool
	}{
		{
			compiledJSON: `{
				"prime": "123"
			}`,
			checkCompiledExists: true,
		},
		{
			compiledJSON: `{
				"program" : "shouldnotexist"
			}`,
		},
	} {
		expectedDeclared.Class.Compiled = json.RawMessage(test.compiledJSON)
		classBytes, err := encoder.Marshal(expectedDeclared)
		require.NoError(t, err)
		err = txn.Put(key, classBytes)
		require.NoError(t, err)

		require.NoError(t, migrateCairo1CompiledClass2(txn, key, classBytes, &utils.Mainnet))

		var actualDeclared core.DeclaredClassDefinition
		err = txn.Get(key, func(data []byte) error {
			return encoder.Unmarshal(data, &actualDeclared)
		})
		require.NoError(t, err)

		assert.Equal(t, actualDeclared.At, expectedDeclared.At)

		actualClass := actualDeclared.Class.(*core.SierraClass)
		expectedClass := expectedDeclared.Class
		assert.Equal(t, expectedClass.Abi, actualClass.Abi)
		assert.Equal(t, expectedClass.AbiHash, actualClass.AbiHash)
		assert.Equal(t, expectedClass.EntryPoints, actualClass.EntryPoints)
		assert.Equal(t, expectedClass.Program, actualClass.Program)
		assert.Equal(t, expectedClass.ProgramHash, actualClass.ProgramHash)
		assert.Equal(t, expectedClass.SemanticVersion, actualClass.SemanticVersion)

		if test.checkCompiledExists {
			assert.NotNil(t, actualClass.Compiled)
		} else {
			assert.Nil(t, actualClass.Compiled)
		}
	}
}

func TestMigrateTrieNodesFromBitsetToBitArray(t *testing.T) {
	migrator := migrateTrieNodesFromBitsetToTrieKey(db.ClassesTrie)
	memDB := memory.New()
	memTxn := memDB.NewIndexedBatch()

	bs := bitset.New(251)
	bsBytes, err := bs.MarshalBinary()
	require.NoError(t, err)

	n := node{
		Value: new(felt.Felt).SetUint64(123),
		Left:  bitset.New(37),
		Right: bitset.New(44),
	}

	var nodeBytes bytes.Buffer
	wrote, err := n._WriteTo(&nodeBytes)
	require.True(t, wrote > 0)
	require.NoError(t, err)

	nodeKey := db.ClassesTrie.Key(bsBytes)
	err = memTxn.Put(nodeKey, nodeBytes.Bytes())
	require.NoError(t, err)

	require.NoError(t, migrator(memTxn, nodeKey, nodeBytes.Bytes(), &utils.Mainnet))

	err = memTxn.Get(db.ClassesTrie.Key(bsBytes), func([]byte) error { return nil })
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	var nodeKeyBuf bytes.Buffer
	newNodeKey := bitset2BitArray(bs)
	bWrite, err := newNodeKey.Write(&nodeKeyBuf)
	require.True(t, bWrite > 0)
	require.NoError(t, err)

	var trieNode trie.Node
	err = memTxn.Get(db.Temporary.Key(nodeKeyBuf.Bytes()), trieNode.UnmarshalBinary)
	require.NoError(t, err)

	require.Equal(t, n.Value, trieNode.Value)
	require.Equal(t, n.Left.Len(), uint(trieNode.Left.Len()))
	require.Equal(t, n.Right.Len(), uint(trieNode.Right.Len()))
	require.Equal(t, felt.Zero, trieNode.Left.Felt())
	require.Equal(t, felt.Zero, trieNode.Right.Felt())
}

func TestSchemaMetadata(t *testing.T) {
	t.Run("conversion", func(t *testing.T) {
		t.Run("version not set", func(t *testing.T) {
			testDB := memory.New()
			metadata, err := SchemaMetadata(utils.NewNopZapLogger(), testDB)
			require.NoError(t, err)
			require.Equal(t, uint64(0), metadata.Version)
			require.Nil(t, metadata.IntermediateState)
		})

		t.Run("version set", func(t *testing.T) {
			testDB := memory.New()
			var version [8]byte
			binary.BigEndian.PutUint64(version[:], 1)
			require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
				return txn.Put(db.SchemaVersion.Key(), version[:])
			}))

			metadata, err := SchemaMetadata(utils.NewNopZapLogger(), testDB)
			require.NoError(t, err)
			require.Equal(t, uint64(1), metadata.Version)
			require.Nil(t, metadata.IntermediateState)
		})
	})
	t.Run("update", func(t *testing.T) {
		t.Run("Intermediate nil", func(t *testing.T) {
			testDB := memory.New()
			version := uint64(5)
			require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
				return updateSchemaMetadata(txn, schemaMetadata{
					Version:           version,
					IntermediateState: nil,
				})
			}))
			metadata, err := SchemaMetadata(utils.NewNopZapLogger(), testDB)
			require.NoError(t, err)
			require.Equal(t, version, metadata.Version)
			require.Nil(t, metadata.IntermediateState)
		})

		t.Run("Intermediate not nil", func(t *testing.T) {
			testDB := memory.New()
			var (
				intermediateState = []byte{1, 2, 3, 4}
				version           = uint64(5)
			)
			require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
				return updateSchemaMetadata(txn, schemaMetadata{
					Version:           version,
					IntermediateState: intermediateState,
				})
			}))
			metadata, err := SchemaMetadata(utils.NewNopZapLogger(), testDB)
			require.NoError(t, err)
			require.Equal(t, version, metadata.Version)
			require.Equal(t, intermediateState, metadata.IntermediateState)
		})

		t.Run("Intermediate empty", func(t *testing.T) {
			testDB := memory.New()
			var (
				intermediateState = make([]byte, 0)
				version           = uint64(5)
			)
			require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
				return updateSchemaMetadata(txn, schemaMetadata{
					Version:           version,
					IntermediateState: intermediateState,
				})
			}))
			metadata, err := SchemaMetadata(utils.NewNopZapLogger(), testDB)
			require.NoError(t, err)
			require.Equal(t, version, metadata.Version)
			require.Equal(t, intermediateState, metadata.IntermediateState)
		})
	})
}

type testMigration struct {
	exec   func(context.Context, db.KeyValueStore, *utils.Network) ([]byte, error)
	before func([]byte) error
}

func (f testMigration) Migrate(ctx context.Context, database db.KeyValueStore, network *utils.Network, _ utils.SimpleLogger) ([]byte, error) {
	return f.exec(ctx, database, network)
}

func (f testMigration) Before(state []byte) error { return f.before(state) }

func TestMigrateIfNeeded(t *testing.T) {
	t.Run("failure at schema", func(t *testing.T) {
		testDB := memory.New()
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.KeyValueStore, *utils.Network) ([]byte, error) {
					return nil, errors.New("foo")
				},
				before: func([]byte) error {
					return errors.New("bar")
				},
			},
		}
		require.ErrorContains(t, migrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations, &HTTPConfig{}), "bar")
	})

	t.Run("call with new tx", func(t *testing.T) {
		testDB := memory.New()
		var counter int
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.KeyValueStore, *utils.Network) ([]byte, error) {
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
		require.NoError(t, migrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations, &HTTPConfig{}))
	})

	t.Run("error during migration", func(t *testing.T) {
		testDB := memory.New()
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.KeyValueStore, *utils.Network) ([]byte, error) {
					return nil, errors.New("foo")
				},
				before: func([]byte) error {
					return nil
				},
			},
		}
		require.ErrorContains(t, migrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations, &HTTPConfig{}), "foo")
	})

	t.Run("error if using new db on old version of juno", func(t *testing.T) {
		testDB := memory.New()
		migrations := []Migration{
			testMigration{
				exec: func(context.Context, db.KeyValueStore, *utils.Network) ([]byte, error) {
					return nil, nil
				},
				before: func([]byte) error {
					return nil
				},
			},
		}
		require.NoError(t, migrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), migrations, &HTTPConfig{}))
		want := "db is from a newer, incompatible version of Juno"
		require.ErrorContains(t, migrateIfNeeded(t.Context(), testDB, &utils.Mainnet, utils.NewNopZapLogger(), []Migration{}, &HTTPConfig{}), want)
	})
}

func TestChangeStateDiffStructEmptyDB(t *testing.T) {
	testdb := memory.New()
	migrator := NewBucketMigrator(db.StateUpdatesByBlockNumber, changeStateDiffStruct2)
	require.NoError(t, migrator.Before(nil))
	intermediateState, err := migrator.Migrate(t.Context(), testdb, &utils.Mainnet, nil)
	require.NoError(t, err)
	require.Nil(t, intermediateState)

	// DB is still empty.
	iter, err := testdb.NewIterator(nil, false)
	defer func() {
		require.NoError(t, iter.Close())
	}()
	require.NoError(t, err)
	require.False(t, iter.Valid())
}

func TestChangeStateDiffStruct(t *testing.T) {
	testdb := memory.New()

	// Initialise DB with two state diffs.
	zero := make([]byte, 8)
	binary.BigEndian.PutUint64(zero, 0)
	su0Key := db.StateUpdatesByBlockNumber.Key(zero)
	one := make([]byte, 8)
	binary.BigEndian.PutUint64(one, 1)
	su1Key := db.StateUpdatesByBlockNumber.Key(one)
	require.NoError(t, testdb.Update(func(txn db.IndexedBatch) error {
		//nolint: dupl
		su0 := oldStateUpdate{
			BlockHash: felt.NewUnsafeFromString[felt.Felt]("0x0"),
			NewRoot:   felt.NewUnsafeFromString[felt.Felt]("0x1"),
			OldRoot:   felt.NewUnsafeFromString[felt.Felt]("0x2"),
			StateDiff: &oldStateDiff{
				StorageDiffs: map[felt.Felt][]oldStorageDiff{
					*felt.NewUnsafeFromString[felt.Felt]("0x3"): {{Key: felt.NewUnsafeFromString[felt.Felt]("0x4"), Value: felt.NewUnsafeFromString[felt.Felt]("0x5")}},
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*felt.NewUnsafeFromString[felt.Felt]("0x6"): felt.NewUnsafeFromString[felt.Felt]("0x7"),
				},
				DeployedContracts: []oldAddressClassHashPair{{Address: felt.NewUnsafeFromString[felt.Felt]("0x8"), ClassHash: felt.NewUnsafeFromString[felt.Felt]("0x9")}},
				DeclaredV0Classes: []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x10")},
				DeclaredV1Classes: []oldDeclaredV1Class{{ClassHash: felt.NewUnsafeFromString[felt.Felt]("0x11"), CompiledClassHash: felt.NewUnsafeFromString[felt.Felt]("0x12")}},
				ReplacedClasses:   []oldAddressClassHashPair{{Address: felt.NewUnsafeFromString[felt.Felt]("0x13"), ClassHash: felt.NewUnsafeFromString[felt.Felt]("0x14")}},
			},
		}
		su0Bytes, err := encoder.Marshal(su0)
		require.NoError(t, err)
		require.NoError(t, txn.Put(su0Key, su0Bytes))

		//nolint: dupl
		su1 := oldStateUpdate{
			BlockHash: felt.NewUnsafeFromString[felt.Felt]("0x15"),
			NewRoot:   felt.NewUnsafeFromString[felt.Felt]("0x16"),
			OldRoot:   felt.NewUnsafeFromString[felt.Felt]("0x17"),
			StateDiff: &oldStateDiff{
				StorageDiffs: map[felt.Felt][]oldStorageDiff{
					*felt.NewUnsafeFromString[felt.Felt]("0x18"): {{Key: felt.NewUnsafeFromString[felt.Felt]("0x19"), Value: felt.NewUnsafeFromString[felt.Felt]("0x20")}},
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*felt.NewUnsafeFromString[felt.Felt]("0x21"): felt.NewUnsafeFromString[felt.Felt]("0x22"),
				},
				DeployedContracts: []oldAddressClassHashPair{{Address: felt.NewUnsafeFromString[felt.Felt]("0x23"), ClassHash: felt.NewUnsafeFromString[felt.Felt]("0x24")}},
				DeclaredV0Classes: []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x25")},
				DeclaredV1Classes: []oldDeclaredV1Class{{ClassHash: felt.NewUnsafeFromString[felt.Felt]("0x26"), CompiledClassHash: felt.NewUnsafeFromString[felt.Felt]("0x27")}},
				ReplacedClasses:   []oldAddressClassHashPair{{Address: felt.NewUnsafeFromString[felt.Felt]("0x28"), ClassHash: felt.NewUnsafeFromString[felt.Felt]("0x29")}},
			},
		}
		su1Bytes, err := encoder.Marshal(su1)
		require.NoError(t, err)
		require.NoError(t, txn.Put(su1Key, su1Bytes))
		return nil
	}))

	// Migrate.
	migrator := NewBucketMigrator(db.StateUpdatesByBlockNumber, changeStateDiffStruct2)
	require.NoError(t, migrator.Before(nil))
	intermediateState, err := migrator.Migrate(t.Context(), testdb, &utils.Mainnet, nil)
	require.NoError(t, err)
	require.Nil(t, intermediateState)

	// Assert:
	// - Both state diffs have been updated.
	// - There are no extraneous entries in the DB.
	require.NoError(t, testdb.View(func(txn db.Snapshot) error {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, iter.Close())
		}()

		updates := []struct {
			key  []byte
			want *core.StateUpdate
		}{
			//nolint: dupl
			{
				key: su0Key,
				want: &core.StateUpdate{
					BlockHash: felt.NewUnsafeFromString[felt.Felt]("0x0"),
					NewRoot:   felt.NewUnsafeFromString[felt.Felt]("0x1"),
					OldRoot:   felt.NewUnsafeFromString[felt.Felt]("0x2"),
					StateDiff: &core.StateDiff{
						StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x3"): {
								*felt.NewUnsafeFromString[felt.Felt]("0x4"): felt.NewUnsafeFromString[felt.Felt]("0x5"),
							},
						},
						Nonces: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x6"): felt.NewUnsafeFromString[felt.Felt]("0x7"),
						},
						DeployedContracts: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x8"): felt.NewUnsafeFromString[felt.Felt]("0x9"),
						},
						DeclaredV0Classes: []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x10")},
						DeclaredV1Classes: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x11"): felt.NewUnsafeFromString[felt.Felt]("0x12"),
						},
						ReplacedClasses: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x13"): felt.NewUnsafeFromString[felt.Felt]("0x14"),
						},
					},
				},
			},
			//nolint: dupl
			{
				key: su1Key,
				want: &core.StateUpdate{
					BlockHash: felt.NewUnsafeFromString[felt.Felt]("0x15"),
					NewRoot:   felt.NewUnsafeFromString[felt.Felt]("0x16"),
					OldRoot:   felt.NewUnsafeFromString[felt.Felt]("0x17"),
					StateDiff: &core.StateDiff{
						StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x18"): {
								*felt.NewUnsafeFromString[felt.Felt]("0x19"): felt.NewUnsafeFromString[felt.Felt]("0x20"),
							},
						},
						Nonces: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x21"): felt.NewUnsafeFromString[felt.Felt]("0x22"),
						},
						DeployedContracts: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x23"): felt.NewUnsafeFromString[felt.Felt]("0x24"),
						},
						DeclaredV0Classes: []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x25")},
						DeclaredV1Classes: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x26"): felt.NewUnsafeFromString[felt.Felt]("0x27"),
						},
						ReplacedClasses: map[felt.Felt]*felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x28"): felt.NewUnsafeFromString[felt.Felt]("0x29"),
						},
					},
				},
			},
		}
		for _, update := range updates {
			require.True(t, iter.Next())
			key := iter.Key()
			require.Equal(t, update.key, key)
			value, err := iter.Value()
			require.NoError(t, err)
			got := new(core.StateUpdate)
			require.NoError(t, encoder.Unmarshal(value, got))
			require.Equal(t, update.want, got)
		}
		require.False(t, iter.Next())
		return nil
	}))
}

func randSlice(t *testing.T) []*felt.Felt {
	t.Helper()

	n := rand.Intn(10)
	sl := make([]*felt.Felt, n)

	for i := range sl {
		sl[i] = felt.NewRandom[felt.Felt]()
	}
	return sl
}

func TestRecalculateL1HandlerMsgHashesToTxnHashes(t *testing.T) {
	t.Run("empty DB", func(t *testing.T) {
		testdb := memory.New()
		m := l1handlermapping.Migrator{}
		require.NoError(t, m.Before(nil))
		intermediateState, err := m.Migrate(t.Context(), testdb, &utils.Sepolia, nil)
		require.NoError(t, err)
		require.Nil(t, intermediateState)
	})

	t.Run("calculate L1Handler transactions per block with mixed tx types", func(t *testing.T) {
		testdb := memory.New()

		const numBlocks uint64 = 10
		const minTxsPerBlock = 5
		const maxTxsPerBlock = 15

		// Track all L1Handler transactions for verification
		type l1HandlerInfo struct {
			msgHash []byte
			txHash  *felt.Felt
		}
		var allL1Handlers []l1HandlerInfo

		// Use weights with higher L1Handler probability to ensure we get some
		weights := txWeights{
			Invoke:        0.30,
			DeployAccount: 0.10,
			Declare:       0.10,
			L1Handler:     0.50,
		}

		batch := testdb.NewBatch()

		for blockNum := range numBlocks {
			// Random number of transactions per block
			numTxs := minTxsPerBlock + rand.Intn(maxTxsPerBlock-minTxsPerBlock+1)

			// Generate transactions with weighted sampling
			txs, receipts := generateWeightedTestTxs(t, &utils.Sepolia, numTxs, weights)

			// Write transactions and track L1Handlers
			for txIndex, tx := range txs {
				require.NoError(t, core.WriteTxAndReceipt(
					batch,
					blockNum,
					uint64(txIndex),
					tx,
					receipts[txIndex],
				))

				// Track L1Handler transactions for verification
				if l1Handler, ok := tx.(*core.L1HandlerTransaction); ok {
					allL1Handlers = append(allL1Handlers, l1HandlerInfo{
						msgHash: l1Handler.MessageHash(),
						txHash:  l1Handler.Hash(),
					})
				}
			}
		}

		// Ensure we have at least some L1Handlers to verify
		require.NotEmpty(t, allL1Handlers, "should have generated some L1Handler transactions")

		require.NoError(t, core.WriteChainHeight(batch, numBlocks-1))
		require.NoError(t, batch.Write())

		// Verify bucket is empty (WriteTxAndReceipt didn't write them)
		iter, err := testdb.NewIterator(db.L1HandlerTxnHashByMsgHash.Key(), true)
		require.NoError(t, err)
		defer iter.Close()
		require.False(
			t,
			iter.First(),
			"L1HandlerTxnHashByMsgHash bucket should be empty before migration",
		)

		// Run migration
		m := l1handlermapping.Migrator{}
		require.NoError(t, m.Before(nil))
		intermediateState, err := m.Migrate(t.Context(), testdb, &utils.Sepolia, nil)
		require.NoError(t, err)
		require.Nil(t, intermediateState)

		// Verify ALL L1Handler mappings are now present and correct
		for _, info := range allL1Handlers {
			mappedTxHash, err := core.GetL1HandlerTxnHashByMsgHash(testdb, info.msgHash)
			require.NoError(t, err)
			require.True(t, mappedTxHash.Equal(info.txHash))
		}
	})
}

// txWeights defines weights for each transaction type
type txWeights struct {
	Declare       float64
	DeployAccount float64
	Invoke        float64
	L1Handler     float64
}

// generateWeightedTestTxs generates n random transactions with receipts based on weights
func generateWeightedTestTxs(
	t *testing.T,
	network *utils.Network,
	n int,
	weights txWeights,
) ([]core.Transaction, []*core.TransactionReceipt) {
	t.Helper()

	builder := &testutils.SyncTransactionBuilder[core.Transaction, struct{}]{
		ToCore: func(tx core.Transaction, _ core.ClassDefinition, _ *felt.Felt) core.Transaction {
			return tx
		},
	}

	// Build cumulative distribution
	cumulative := []float64{
		weights.Declare,
		weights.Declare + weights.DeployAccount,
		weights.Declare + weights.DeployAccount + weights.Invoke,
		weights.Declare + weights.DeployAccount + weights.Invoke + weights.L1Handler,
	}
	total := cumulative[3]

	txs := make([]core.Transaction, n)
	receipts := make([]*core.TransactionReceipt, n)

	for i := range n {
		r := rand.Float64() * total

		var tx core.Transaction
		switch {
		case r < cumulative[0]:
			tx, _ = builder.GetTestDeclareV3Transaction(t, network)
		case r < cumulative[1]:
			tx, _ = builder.GetTestDeployAccountTransactionV3(t, network)
		case r < cumulative[2]:
			tx, _ = builder.GetTestInvokeTransactionV3(t, network)
		default:
			// L1Handler: retry until non-empty CallData
			for {
				tx, _ = builder.GetTestL1HandlerTransaction(t, network)
				if len(tx.(*core.L1HandlerTransaction).CallData) > 0 {
					break
				}
			}
		}

		receipts[i] = &core.TransactionReceipt{
			TransactionHash: tx.Hash(),
			Fee:             felt.NewRandom[felt.Felt](),
			FeeUnit:         core.WEI,
		}
		txs[i] = tx
	}

	return txs, receipts
}
