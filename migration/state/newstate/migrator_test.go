package newstate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/state/newstate"
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

	require.NoError(t, core.WriteContractClassHash(memDB, &fixture.addr, &fixture.headClassHash))
	require.NoError(t, core.WriteContractNonce(memDB, &fixture.addr, &fixture.headNonce))
	require.NoError(t, core.WriteContractDeploymentHeight(memDB, &fixture.addr, deployHeight))

	require.NoError(t, core.WriteDeprecatedContractClassHashHistory(
		memDB, &fixture.addr, &fixture.deployClassHash, replaceHeight))
	require.NoError(t, core.WriteDeprecatedContractNonceHistory(
		memDB, &fixture.addr, &fixture.preNonce, nonceHeight))
	require.NoError(t, core.WriteDeprecatedContractStorageHistory(
		memDB, &fixture.addr, &fixture.slot, &fixture.preSlotValue, storeHeight))

	seedDeprecatedStorageTrie(t, memDB, fixture.addr, map[felt.Felt]felt.Felt{
		fixture.slot: fixture.headSlotValue,
	})
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
		db.ContractStorage.Key(addr.Marshal()),
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

	for _, bucket := range []db.Bucket{
		db.ContractClassHash,
		db.ContractNonce,
		db.ContractDeploymentHeight,
		db.DeprecatedContractClassHashHistory,
		db.DeprecatedContractNonceHistory,
		db.DeprecatedContractStorageHistory,
		db.ClassesTrie,
		db.StateTrie,
		db.ContractStorage,
	} {
		assert.Zerof(t, bucketKeyCount(t, memDB, bucket), "bucket %v must be wiped", bucket)
	}
	assert.NotZero(t, bucketKeyCount(t, memDB, db.ContractTrieStorage),
		"the storage trie must exist in the new layout")
}

func TestMigrateRunsEveryPhase(t *testing.T) {
	memDB := newDB(t)
	seedDeprecated(t, memDB)

	m := newstate.New()
	require.NoError(t, m.Before(nil))
	res, err := m.Migrate(context.Background(), memDB, &networks.Sepolia, log.NewNopZapLogger())

	require.NoError(t, err)
	assert.Nil(t, res, "a completed run must report no intermediate state")
	assertMigrated(t, memDB)
}

func TestMigrateIsIdempotent(t *testing.T) {
	memDB := newDB(t)
	seedDeprecated(t, memDB)

	for range 2 {
		m := newstate.New()
		require.NoError(t, m.Before(nil))
		res, err := m.Migrate(context.Background(), memDB, &networks.Sepolia, log.NewNopZapLogger())
		require.NoError(t, err)
		assert.Nil(t, res)
	}

	assertMigrated(t, memDB)
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
	seedDeprecated(t, memDB)

	m := newstate.New()
	require.NoError(t, m.Before([]byte{1}))
	res, err := m.Migrate(context.Background(), memDB, &networks.Sepolia, log.NewNopZapLogger())

	require.NoError(t, err)
	assert.Nil(t, res)

	assert.NotZero(t, bucketKeyCount(t, memDB, db.ContractClassHash),
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
