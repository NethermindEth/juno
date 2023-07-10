package migration

import (
	"context"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0000(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	t.Run("empty DB", func(t *testing.T) {
		require.NoError(t, testDB.View(migration0000))
	})

	t.Run("non-empty DB", func(t *testing.T) {
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set([]byte("asd"), []byte("123"))
		}))
		require.EqualError(t, testDB.View(migration0000), "initial DB should be empty")
	})
}

func TestRelocateContractStorageRootKeys(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})

	txn := testDB.NewTransaction(true)

	numberOfContracts := 5

	// Populate the database with entries in the old location.
	for i := 0; i < numberOfContracts; i++ {
		exampleBytes := new(felt.Felt).SetUint64(uint64(i)).Bytes()
		// Use exampleBytes for the key suffix (the contract address) and the value.
		err := txn.Set(db.Unused.Key(exampleBytes[:]), exampleBytes[:])
		require.NoError(t, err)
	}

	require.NoError(t, relocateContractStorageRootKeys(txn))

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
		oldKey := db.Unused.Key(exampleBytes[:])
		err := txn.Get(oldKey, func(val []byte) error { return nil })
		require.ErrorIs(t, db.ErrKeyNotFound, err)
	}
}

func TestRecalculateBloomFilters(t *testing.T) {
	testdb := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testdb.Close())
	})
	chain := blockchain.New(testdb, utils.MAINNET, utils.NewNopZapLogger())
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)

	for i := uint64(0); i < 3; i++ {
		b, err := gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err)

		b.EventsBloom = nil
		require.NoError(t, chain.Store(b, su, nil))
	}

	require.NoError(t, testdb.Update(recalculateBloomFilters))

	for i := uint64(0); i < 3; i++ {
		b, err := chain.BlockByNumber(i)
		require.NoError(t, err)
		assert.Equal(t, core.EventsBloom(b.Receipts), b.EventsBloom)
	}
}

func TestChangeTrieNodeEncoding(t *testing.T) {
	testdb := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testdb.Close())
	})

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
			for i := 0; i < 5; i++ {
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
	m.Before()
	require.NoError(t, testdb.Update(m.Migrate))

	require.NoError(t, testdb.Update(func(txn db.Transaction) error {
		for _, bucket := range buckets {
			for i := 0; i < 5; i++ {
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
