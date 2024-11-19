package migration

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bitset"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0000(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	t.Run("empty DB", func(t *testing.T) {
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			return migration0000(txn, &utils.Mainnet)
		}))
	})

	t.Run("non-empty DB", func(t *testing.T) {
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set([]byte("asd"), []byte("123"))
		}))
		require.EqualError(t, testDB.View(func(txn db.Transaction) error {
			return migration0000(txn, &utils.Mainnet)
		}), "initial DB should be empty")
	})
}

func TestRelocateContractStorageRootKeys(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	numberOfContracts := 5

	// Populate the database with entries in the old location.
	for i := 0; i < numberOfContracts; i++ {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()
		// Use exampleBytes for the key suffix (the contract address) and the value.
		err := txn.Set(db.Peer.Key(exampleBytes[:]), exampleBytes[:])
		require.NoError(t, err)
	}

	require.NoError(t, relocateContractStorageRootKeys(txn, &utils.Mainnet))

	// Each root-key entry should have been moved to its new location
	// and the old entry should not exist.
	for i := 0; i < numberOfContracts; i++ {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()

		// New entry exists.
		require.NoError(t, txn.Get(db.ContractStorage.Key(exampleBytes[:]), func(val []byte) error {
			require.Equal(t, exampleBytes[:], val, "the correct value was not transferred to the new location")
			return nil
		}))

		// Old entry does not exist.
		oldKey := db.Peer.Key(exampleBytes[:])
		err := txn.Get(oldKey, func(val []byte) error { return nil })
		require.ErrorIs(t, db.ErrKeyNotFound, err)
	}
}

func TestRecalculateBloomFilters(t *testing.T) {
	testdb := pebble.NewMemTest(t)
	chain := blockchain.New(testdb, &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	for i := uint64(0); i < 3; i++ {
		b, err := gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err)

		b.EventsBloom = nil
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))
	}

	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		return recalculateBloomFilters(txn, &utils.Mainnet)
	}))

	for i := uint64(0); i < 3; i++ {
		b, err := chain.BlockByNumber(i)
		require.NoError(t, err)
		assert.Equal(t, core.EventsBloom(b.Receipts), b.EventsBloom)
	}
}

func TestChangeTrieNodeEncoding(t *testing.T) {
	testdb := pebble.NewMemTest(t)

	buckets := []db.Bucket{db.ClassesTrie, db.StateTrie, db.ContractStorage}

	var n struct {
		Value *felt.Felt
		Left  *bitset.BitSet
		Right *bitset.BitSet
	}
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		// contract root keys, if changeTrieNodeEncoding tries to migrate these it
		// will fail with an error since they are not valid trie.Node encodings.
		require.NoError(t, txn.Set(db.ClassesTrie.Key(), []byte{1, 2, 3}))
		require.NoError(t, txn.Set(db.StateTrie.Key(), []byte{1, 2, 3}))
		require.NoError(t, txn.Set(db.ContractStorage.Key(make([]byte, felt.Bytes)), []byte{1, 2, 3}))

		for _, bucket := range buckets {
			for i := range 5 {
				n.Value = new(felt.Felt).SetUint64(uint64(i))

				encodedNode, err := encoder.Marshal(n)
				if err != nil {
					return err
				}

				if err = txn.Set(bucket.Key([]byte{byte(i)}), encodedNode); err != nil {
					return err
				}
			}
		}

		return nil
	}))

	m := new(changeTrieNodeEncoding)
	require.NoError(t, m.Before(nil))
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		_, err := m.Migrate(context.Background(), txn, &utils.Mainnet, nil)
		return err
	}))

	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		for _, bucket := range buckets {
			for i := range 5 {
				var coreNode trie.Node
				err := txn.Get(bucket.Key([]byte{byte(i)}), coreNode.UnmarshalBinary)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}))
}

func TestCalculateBlockCommitments(t *testing.T) {
	testdb := pebble.NewMemTest(t)
	chain := blockchain.New(testdb, &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	for i := uint64(0); i < 3; i++ {
		b, err := gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err)
		require.NoError(t, chain.Store(b, &core.BlockCommitments{}, su, nil))
	}

	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		return calculateBlockCommitments(txn, &utils.Mainnet)
	}))
	for i := uint64(0); i < 3; i++ {
		b, err := chain.BlockCommitmentsByNumber(i)
		require.NoError(t, err)
		assert.NotNil(t, b.TransactionCommitment)
	}
}

func TestL1HandlerTxns(t *testing.T) {
	testdb := pebble.NewMemTest(t)
	chain := blockchain.New(testdb, &utils.Sepolia)
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

func TestMigrateTrieRootKeysFromBitsetToTrieKeys(t *testing.T) {
	memTxn := db.NewMemTransaction()

	bs := bitset.New(251)
	bsBytes, err := bs.MarshalBinary()
	require.NoError(t, err)

	key := []byte{0}
	err = memTxn.Set(key, bsBytes)
	require.NoError(t, err)

	require.NoError(t, migrateTrieRootKeysFromBitsetToTrieKeys(memTxn, key, bsBytes, &utils.Mainnet))

	var trieKey trie.Key
	err = memTxn.Get(key, trieKey.UnmarshalBinary)
	require.NoError(t, err)
	require.Equal(t, bs.Len(), uint(trieKey.Len()))
	require.Equal(t, felt.Zero, trieKey.Felt())
}

func TestMigrateCairo1CompiledClass(t *testing.T) {
	blockchain.RegisterCoreTypesToEncoder()
	txn := db.NewMemTransaction()

	key := []byte("key")
	class := oldCairo1Class{
		Abi:     "some cairo abi",
		AbiHash: randFelt(t),
		EntryPoints: struct {
			Constructor []core.SierraEntryPoint
			External    []core.SierraEntryPoint
			L1Handler   []core.SierraEntryPoint
		}{
			Constructor: []core.SierraEntryPoint{
				{
					Index:    0,
					Selector: randFelt(t),
				},
			},
			External: []core.SierraEntryPoint{
				{
					Index:    0,
					Selector: randFelt(t),
				},
			},
			L1Handler: []core.SierraEntryPoint{
				{
					Index:    0,
					Selector: randFelt(t),
				},
			},
		},
		Program:         randSlice(t),
		ProgramHash:     randFelt(t),
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
		err = txn.Set(key, classBytes)
		require.NoError(t, err)

		require.NoError(t, migrateCairo1CompiledClass(txn, key, classBytes, &utils.Mainnet))

		var actualDeclared core.DeclaredClass
		err = txn.Get(key, func(bytes []byte) error {
			return encoder.Unmarshal(bytes, &actualDeclared)
		})
		require.NoError(t, err)

		assert.Equal(t, actualDeclared.At, expectedDeclared.At)

		actualClass := actualDeclared.Class.(*core.Cairo1Class)
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

func TestMigrateTrieNodesFromBitsetToTrieKey(t *testing.T) {
	migrator := migrateTrieNodesFromBitsetToTrieKey(db.ClassesTrie)
	memTxn := db.NewMemTransaction()

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
	err = memTxn.Set(nodeKey, nodeBytes.Bytes())
	require.NoError(t, err)

	require.NoError(t, migrator(memTxn, nodeKey, nodeBytes.Bytes(), &utils.Mainnet))

	err = memTxn.Get(db.ClassesTrie.Key(bsBytes), func(b []byte) error {
		return nil
	})
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	var nodeKeyBuf bytes.Buffer
	newNodeKey := bitset2Key(bs)
	wrote, err = newNodeKey.WriteTo(&nodeKeyBuf)
	require.True(t, wrote > 0)
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

func TestChangeStateDiffStructEmptyDB(t *testing.T) {
	testdb := pebble.NewMemTest(t)
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		migrator := NewBucketMigrator(db.StateUpdatesByBlockNumber, changeStateDiffStruct)
		require.NoError(t, migrator.Before(nil))
		intermediateState, err := migrator.Migrate(context.Background(), txn, &utils.Mainnet, nil)
		require.NoError(t, err)
		require.Nil(t, intermediateState)

		// DB is still empty.
		iter, err := txn.NewIterator()
		defer func() {
			require.NoError(t, iter.Close())
		}()
		require.NoError(t, err)
		require.False(t, iter.Valid())

		return nil
	}))
}

func TestChangeStateDiffStruct(t *testing.T) {
	testdb := pebble.NewMemTest(t)

	// Initialise DB with two state diffs.
	zero := make([]byte, 8)
	binary.BigEndian.PutUint64(zero, 0)
	su0Key := db.StateUpdatesByBlockNumber.Key(zero)
	one := make([]byte, 8)
	binary.BigEndian.PutUint64(one, 1)
	su1Key := db.StateUpdatesByBlockNumber.Key(one)
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		//nolint: dupl
		su0 := oldStateUpdate{
			BlockHash: utils.HexToFelt(t, "0x0"),
			NewRoot:   utils.HexToFelt(t, "0x1"),
			OldRoot:   utils.HexToFelt(t, "0x2"),
			StateDiff: &oldStateDiff{
				StorageDiffs: map[felt.Felt][]oldStorageDiff{
					*utils.HexToFelt(t, "0x3"): {{Key: utils.HexToFelt(t, "0x4"), Value: utils.HexToFelt(t, "0x5")}},
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x6"): utils.HexToFelt(t, "0x7"),
				},
				DeployedContracts: []oldAddressClassHashPair{{Address: utils.HexToFelt(t, "0x8"), ClassHash: utils.HexToFelt(t, "0x9")}},
				DeclaredV0Classes: []*felt.Felt{utils.HexToFelt(t, "0x10")},
				DeclaredV1Classes: []oldDeclaredV1Class{{ClassHash: utils.HexToFelt(t, "0x11"), CompiledClassHash: utils.HexToFelt(t, "0x12")}},
				ReplacedClasses:   []oldAddressClassHashPair{{Address: utils.HexToFelt(t, "0x13"), ClassHash: utils.HexToFelt(t, "0x14")}},
			},
		}
		su0Bytes, err := encoder.Marshal(su0)
		require.NoError(t, err)
		require.NoError(t, txn.Set(su0Key, su0Bytes))

		//nolint: dupl
		su1 := oldStateUpdate{
			BlockHash: utils.HexToFelt(t, "0x15"),
			NewRoot:   utils.HexToFelt(t, "0x16"),
			OldRoot:   utils.HexToFelt(t, "0x17"),
			StateDiff: &oldStateDiff{
				StorageDiffs: map[felt.Felt][]oldStorageDiff{
					*utils.HexToFelt(t, "0x18"): {{Key: utils.HexToFelt(t, "0x19"), Value: utils.HexToFelt(t, "0x20")}},
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*utils.HexToFelt(t, "0x21"): utils.HexToFelt(t, "0x22"),
				},
				DeployedContracts: []oldAddressClassHashPair{{Address: utils.HexToFelt(t, "0x23"), ClassHash: utils.HexToFelt(t, "0x24")}},
				DeclaredV0Classes: []*felt.Felt{utils.HexToFelt(t, "0x25")},
				DeclaredV1Classes: []oldDeclaredV1Class{{ClassHash: utils.HexToFelt(t, "0x26"), CompiledClassHash: utils.HexToFelt(t, "0x27")}},
				ReplacedClasses:   []oldAddressClassHashPair{{Address: utils.HexToFelt(t, "0x28"), ClassHash: utils.HexToFelt(t, "0x29")}},
			},
		}
		su1Bytes, err := encoder.Marshal(su1)
		require.NoError(t, err)
		require.NoError(t, txn.Set(su1Key, su1Bytes))
		return nil
	}))

	// Migrate.
	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		migrator := NewBucketMigrator(db.StateUpdatesByBlockNumber, changeStateDiffStruct)
		require.NoError(t, migrator.Before(nil))
		intermediateState, err := migrator.Migrate(context.Background(), txn, &utils.Mainnet, nil)
		require.NoError(t, err)
		require.Nil(t, intermediateState)
		return nil
	}))

	// Assert:
	// - Both state diffs have been updated.
	// - There are no extraneous entries in the DB.
	require.NoError(t, testdb.View(func(txn db.Transaction) error {
		iter, err := txn.NewIterator()
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
					BlockHash: utils.HexToFelt(t, "0x0"),
					NewRoot:   utils.HexToFelt(t, "0x1"),
					OldRoot:   utils.HexToFelt(t, "0x2"),
					StateDiff: &core.StateDiff{
						StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x3"): {
								*utils.HexToFelt(t, "0x4"): utils.HexToFelt(t, "0x5"),
							},
						},
						Nonces: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x6"): utils.HexToFelt(t, "0x7"),
						},
						DeployedContracts: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x8"): utils.HexToFelt(t, "0x9"),
						},
						DeclaredV0Classes: []*felt.Felt{utils.HexToFelt(t, "0x10")},
						DeclaredV1Classes: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x11"): utils.HexToFelt(t, "0x12"),
						},
						ReplacedClasses: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x13"): utils.HexToFelt(t, "0x14"),
						},
					},
				},
			},
			//nolint: dupl
			{
				key: su1Key,
				want: &core.StateUpdate{
					BlockHash: utils.HexToFelt(t, "0x15"),
					NewRoot:   utils.HexToFelt(t, "0x16"),
					OldRoot:   utils.HexToFelt(t, "0x17"),
					StateDiff: &core.StateDiff{
						StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x18"): {
								*utils.HexToFelt(t, "0x19"): utils.HexToFelt(t, "0x20"),
							},
						},
						Nonces: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x21"): utils.HexToFelt(t, "0x22"),
						},
						DeployedContracts: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x23"): utils.HexToFelt(t, "0x24"),
						},
						DeclaredV0Classes: []*felt.Felt{utils.HexToFelt(t, "0x25")},
						DeclaredV1Classes: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x26"): utils.HexToFelt(t, "0x27"),
						},
						ReplacedClasses: map[felt.Felt]*felt.Felt{
							*utils.HexToFelt(t, "0x28"): utils.HexToFelt(t, "0x29"),
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
	n := rand.Intn(10)
	sl := make([]*felt.Felt, n)

	for i := range sl {
		sl[i] = randFelt(t)
	}

	return sl
}

func randFelt(t *testing.T) *felt.Felt {
	t.Helper()

	f, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return f
}
