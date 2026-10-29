package newstate_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	//nolint:staticcheck,nolintlint // old state layout
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/historyprunner"
	"github.com/NethermindEth/juno/migration/state/newstate"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture is one contract deployed at block 100 whose class hash was
// replaced at 200, whose nonce changed at 300, and whose storage slot changed
// at 400, seeded purely in the deprecated layout:
//
//	head          class hash 0xBB, nonce 7, slot 0x55 = 0x99 (storage trie)
//	deprecated    classHashHistory[200]   = 0xAA  (pre-value)
//	              nonceHistory[300]       = 3     (pre-value)
//	              storageHistory[0x55,400] = 0x11 (pre-value)
//
// After every phase the new layout must read back as:
//
//	classHashAt(100)  = 0xAA   classHashAt(200) = 0xBB
//	nonceAt(300)      = 7
//	storageAt(0x55, 400) = 0x99
//
// That last one is also the ordering assertion: the head value can only come
// from the deprecated storage trie, which the trie phase wipes. If trie ran
// before history, it would read back as zero.
const (
	deployHeight  = uint64(100)
	replaceHeight = uint64(200)
	nonceHeight   = uint64(300)
	storeHeight   = uint64(400)
)

type contractFixture struct {
	addr            felt.Felt
	deployClassHash felt.Felt
	headClassHash   felt.Felt
	preNonce        felt.Felt
	headNonce       felt.Felt
	slot            felt.Felt
	preSlotValue    felt.Felt
	headSlotValue   felt.Felt
}

var fixture = contractFixture{
	addr:            felt.FromUint64[felt.Felt](0xABCDEF),
	deployClassHash: felt.FromUint64[felt.Felt](0xAA),
	headClassHash:   felt.FromUint64[felt.Felt](0xBB),
	preNonce:        felt.FromUint64[felt.Felt](3),
	headNonce:       felt.FromUint64[felt.Felt](7),
	slot:            felt.FromUint64[felt.Felt](0x55),
	preSlotValue:    felt.FromUint64[felt.Felt](0x11),
	headSlotValue:   felt.FromUint64[felt.Felt](0x99),
}

func seedDeprecated(t *testing.T, memDB db.KeyValueStore) {
	t.Helper()
	seedDeprecatedContract(t, memDB)
	seedDeprecatedStorageTrie(t, memDB, fixture.addr, map[felt.Felt]felt.Felt{
		fixture.slot: fixture.headSlotValue,
	})
}

// seedDeprecatedContract omits the storage trie
func seedDeprecatedContract(t *testing.T, memDB db.KeyValueStore) {
	t.Helper()

	require.NoError(t, deprecatedstate.WriteContractClassHash(
		memDB, &fixture.addr, &fixture.headClassHash))
	require.NoError(t, deprecatedstate.WriteContractNonce(memDB, &fixture.addr, &fixture.headNonce))
	require.NoError(t, deprecatedstate.WriteContractDeploymentHeight(
		memDB, &fixture.addr, deployHeight))

	require.NoError(t, deprecatedstate.WriteContractClassHashHistory(
		memDB, &fixture.addr, &fixture.deployClassHash, replaceHeight))
	require.NoError(t, deprecatedstate.WriteContractNonceHistory(
		memDB, &fixture.addr, &fixture.preNonce, nonceHeight))
	require.NoError(t, deprecatedstate.WriteContractStorageHistory(
		memDB, &fixture.addr, &fixture.slot, &fixture.preSlotValue, storeHeight))
}

func seedDeprecatedStorageTrie(
	t *testing.T,
	memDB db.KeyValueStore,
	addr felt.Felt,
	leaves map[felt.Felt]felt.Felt,
) {
	t.Helper()
	//nolint:staticcheck // Necessary for old state
	txn := memDB.NewIndexedBatch()
	tr, err := trie.NewTriePedersen(
		txn,
		//nolint:staticcheck,nolintlint // old state layout
		db.DeprecatedContractStorage.Key(addr.Marshal()),
		deprecatedstate.ContractStorageTrieHeight,
	)
	require.NoError(t, err)
	for k, v := range leaves {
		_, err := tr.Put(&k, &v)
		require.NoError(t, err)
	}
	require.NoError(t, tr.Commit())
	require.NoError(t, txn.Write())
}

func bucketKeyCount(t *testing.T, r db.KeyValueReader, bucket db.Bucket) int {
	t.Helper()
	it, err := r.NewIterator(bucket.Key(), true)
	require.NoError(t, err)
	defer it.Close()
	count := 0
	for valid := it.First(); valid; valid = it.Next() {
		count++
	}
	return count
}

func newDB(t *testing.T) db.KeyValueStore {
	t.Helper()
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })
	return memDB
}

// assertMigrated checks the outcome through the public StateReader, which is
// what the running node uses, rather than through raw bucket reads.
func assertMigrated(t *testing.T, memDB db.KeyValueStore) {
	t.Helper()

	contract, err := state.GetContract(memDB, &fixture.addr)
	require.NoError(t, err)
	assert.Equal(t, fixture.headClassHash, contract.ClassHash)
	assert.Equal(t, fixture.headNonce, contract.Nonce)
	assert.Equal(t, deployHeight, contract.DeployedHeight)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	atDeploy, err := reader.ContractClassHashAt(&fixture.addr, deployHeight)
	require.NoError(t, err)
	assert.Equal(t, fixture.deployClassHash, atDeploy, "class hash at the deploy block")

	atReplace, err := reader.ContractClassHashAt(&fixture.addr, replaceHeight)
	require.NoError(t, err)
	assert.Equal(t, fixture.headClassHash, atReplace, "class hash at the replace block")

	nonce, err := reader.ContractNonceAt(&fixture.addr, nonceHeight)
	require.NoError(t, err)
	assert.Equal(t, fixture.headNonce, nonce, "nonce at the change block")

	slotValue, err := reader.ContractStorageAt(&fixture.addr, &fixture.slot, storeHeight)
	require.NoError(t, err)
	assert.Equal(t, fixture.headSlotValue, slotValue,
		"storage at the change block must come from the deprecated head trie, "+
			"which is only readable while the trie phase has not wiped it")

	assertDeprecatedWiped(t, memDB)
	assert.NotZero(t, bucketKeyCount(t, memDB, db.ContractTrieStorage),
		"the storage trie must exist in the new layout")
}

func assertDeprecatedWiped(t *testing.T, r db.KeyValueReader) {
	t.Helper()
	for _, bucket := range []db.Bucket{
		db.DeprecatedContractClassHash,        //nolint:staticcheck,nolintlint // old state layout
		db.DeprecatedContractNonce,            //nolint:staticcheck,nolintlint // old state layout
		db.DeprecatedContractDeploymentHeight, //nolint:staticcheck,nolintlint // old state layout
		db.DeprecatedContractClassHashHistory,
		db.DeprecatedContractNonceHistory,
		db.DeprecatedContractStorageHistory,
		//nolint:staticcheck // the deprecated buckets are this migration's input
		db.DeprecatedClassesTrie,
		//nolint:staticcheck // the deprecated buckets are this migration's input
		db.DeprecatedStateTrie,
		//nolint:staticcheck // the deprecated buckets are this migration's input
		db.DeprecatedContractStorage,
	} {
		assert.Zerof(t, bucketKeyCount(t, r, bucket), "bucket %v must be wiped", bucket)
	}
}

func TestMigrateOnEmptyDB(t *testing.T) {
	memDB := newDB(t)

	m := newstate.New()
	require.NoError(t, m.Before(nil))
	res, err := m.Migrate(context.Background(), memDB, &networks.Sepolia, log.NewNopZapLogger())

	require.NoError(t, err)
	assert.Nil(t, res)
}

func TestBeforeResumesAtNamedPhase(t *testing.T) {
	memDB := newDB(t)
	seedDeprecatedContract(t, memDB)

	m := newstate.New()
	require.NoError(t, m.Before([]byte{1}))
	res, err := m.Migrate(context.Background(), memDB, &networks.Sepolia, log.NewNopZapLogger())

	require.NoError(t, err)
	assert.Nil(t, res)

	//nolint:staticcheck,nolintlint // old state layout
	assert.NotZero(t, bucketKeyCount(t, memDB, db.DeprecatedContractClassHash),
		"the headstate phase must not have run")
	assert.Zero(t, bucketKeyCount(t, memDB, db.Contract),
		"no Contract record can exist when the headstate phase is skipped")
}

func TestBeforeRejectsUnknownPhase(t *testing.T) {
	m := newstate.New()
	err := m.Before([]byte{200})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phase 200")
}

func TestBeforeTreatsEmptyStateAsFresh(t *testing.T) {
	memDB := newDB(t)
	seedDeprecated(t, memDB)

	m := newstate.New()
	require.NoError(t, m.Before([]byte{}))
	res, err := m.Migrate(context.Background(), memDB, &networks.Sepolia, log.NewNopZapLogger())

	require.NoError(t, err)
	assert.Nil(t, res)
	assertMigrated(t, memDB)
}

func TestMigrateCheckpointsOnCancellation(t *testing.T) {
	memDB := newDB(t)
	seedDeprecated(t, memDB)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := newstate.New()
	require.NoError(t, m.Before(nil))
	res, err := m.Migrate(ctx, memDB, &networks.Sepolia, log.NewNopZapLogger())

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"the error must unwrap to context.Canceled, got %v", err)
	require.NotNil(t, res, "a cancelled run must return a checkpoint")
	assert.Equal(t, uint8(0), res[0], "cancellation happened in the first phase")
}

// TestMigratePrunedHistory runs the migration before and after pruning, with
// cancellations and a restart after a crash, and checks every read from the
// floor up against the deprecated state before pruning.
// retained == blocks means no pruning.
func TestMigratePrunedHistory(t *testing.T) {
	const blocks = 40
	const tip = blocks - 1
	for _, retained := range []uint64{blocks, 25, 5, 0} {
		for seed := range int64(3) {
			t.Run(fmt.Sprintf("retained%d/seed%d", retained, seed), func(t *testing.T) {
				diffs := randomStateDiffs(seed, blocks)
				var floor uint64
				if retained < tip {
					floor = tip - retained - 1
				}

				reference := testutils.NewPebbleTestDB(t)
				storeDeprecatedChain(t, reference, diffs)
				want := stateAnswers(t, reference, floor, true)

				scenarios := []struct {
					name string
					run  func(t *testing.T, database db.KeyValueStore)
				}{
					{"migrated then pruned", func(t *testing.T, database db.KeyValueStore) {
						migrateNewState(t, database, 0)
						pruneHistory(t, database, retained)
					}},
					{"pruned then migrated with cancellations", func(t *testing.T, database db.KeyValueStore) {
						pruneHistory(t, database, retained)
						migrateNewState(t, database, 20)
					}},
					{"restarted after a crash in the trie phase", func(t *testing.T, database db.KeyValueStore) {
						pruneHistory(t, database, retained)
						crashInTriePhase(t, database)
						migrateNewState(t, database, 0)
					}},
				}
				for _, s := range scenarios {
					t.Run(s.name, func(t *testing.T) {
						database := testutils.NewPebbleTestDB(t)
						storeDeprecatedChain(t, database, diffs)
						s.run(t, database)
						assert.Equal(t, want, stateAnswers(t, database, floor, false))
						assertDeprecatedWiped(t, database)
					})
				}
			})
		}
	}
}

// migrateNewState runs the migration to completion, cancelling every run after
// cancelAfter iterators (never when 0) and resuming from its checkpoint.
func migrateNewState(t *testing.T, database db.KeyValueStore, cancelAfter int64) {
	t.Helper()
	var checkpoint []byte
	for range 1000 {
		ctx, cancel := context.WithCancel(t.Context())
		wrapped := &cancelAfterIterators{KeyValueStore: database, cancel: cancel}
		wrapped.left.Store(cancelAfter)
		m := newstate.New()
		require.NoError(t, m.Before(checkpoint))
		var err error
		checkpoint, err = m.Migrate(ctx, wrapped, &networks.Sepolia, log.NewNopZapLogger())
		cancel()
		if checkpoint == nil {
			require.NoError(t, err)
			return
		}
		if err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
	}
	t.Fatal("the migration made no progress")
}

// cancelAfterIterators cancels once left iterators have been opened.
type cancelAfterIterators struct {
	db.KeyValueStore
	cancel context.CancelFunc
	left   atomic.Int64
}

func (c *cancelAfterIterators) NewIterator(prefix []byte, upperBound bool) (db.Iterator, error) {
	if c.left.Add(-1) == 0 {
		c.cancel()
	}
	return c.KeyValueStore.NewIterator(prefix, upperBound)
}

// crashInTriePhase stops the migration once the trie phase starts, when the
// history phases are done but the deprecated tries are still there, and drops
// the checkpoint as a crash would.
func crashInTriePhase(t *testing.T, database db.KeyValueStore) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	checkpoint, err := newstate.New().
		Migrate(ctx, &cancelOnTrieRead{database, cancel}, &networks.Sepolia, log.NewNopZapLogger())
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
	require.NotNil(t, checkpoint)
	require.Equal(t, uint8(2), checkpoint[0], "stopped in the trie phase")
}

// cancelOnTrieRead cancels on the first read of a bucket only the trie phase reads.
type cancelOnTrieRead struct {
	db.KeyValueStore
	cancel context.CancelFunc
}

func (c *cancelOnTrieRead) NewIterator(prefix []byte, upperBound bool) (db.Iterator, error) {
	//nolint:staticcheck,nolintlint // the deprecated tries are this migration's input
	if bytes.HasPrefix(prefix, db.DeprecatedClassesTrie.Key()) ||
		bytes.HasPrefix(prefix, db.DeprecatedStateTrie.Key()) {
		c.cancel()
	}
	return c.KeyValueStore.NewIterator(prefix, upperBound)
}

func pruneHistory(t *testing.T, database db.KeyValueStore, retained uint64) {
	t.Helper()
	r, err := historyprunner.New(retained, 0).
		Migrate(t.Context(), database, &networks.Sepolia, log.NewNopZapLogger())
	require.NoError(t, err)
	require.Nil(t, r)
}
