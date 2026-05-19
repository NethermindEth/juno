package statehistory

import (
	"context"
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
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedContract(
	t *testing.T,
	memDB db.KeyValueStore,
	addr felt.Felt,
	nonce, classHash felt.Felt,
	deployHeight uint64,
) {
	t.Helper()
	require.NoError(t, state.WriteContract(memDB, &addr, nonce, classHash, deployHeight))
}

func seedDeprecatedClassHashHistory(
	t *testing.T,
	w db.KeyValueWriter,
	addr felt.Felt,
	block uint64,
	oldValue felt.Felt,
) {
	t.Helper()
	require.NoError(t, core.WriteDeprecatedContractClassHashHistory(w, &addr, &oldValue, block))
}

func seedDeprecatedNonceHistory(
	t *testing.T,
	w db.KeyValueWriter,
	addr felt.Felt,
	block uint64,
	oldValue felt.Felt,
) {
	t.Helper()
	require.NoError(t, core.WriteDeprecatedContractNonceHistory(w, &addr, &oldValue, block))
}

func seedDeprecatedStorageHistory(
	t *testing.T,
	w db.KeyValueWriter,
	addr, slot felt.Felt,
	block uint64,
	oldValue felt.Felt,
) {
	t.Helper()
	require.NoError(t, core.WriteDeprecatedContractStorageHistory(w, &addr, &slot, &oldValue, block))
}

// seedDeprecatedStorageTrie populates the deprecated ContractStorage trie for
// `addr` with the given (slot -> value) leaves, so the storage phase can read
// head values.
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

// ----- Tests -----

func TestMigrate_EmptyDB(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)
}

// Class-hash phase: a contract that was never reclassed has no deprecated
// entries. After migration, history has exactly one entry:
// (addr, deploy_height) -> ClassHash.
func TestMigrate_ClassHash_DeployOnly(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	classHash := felt.FromUint64[felt.Felt](170)
	seedContract(t, memDB, addr, felt.Zero, classHash, 100)

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)
	got, err := reader.ContractClassHashAt(&addr, 100)
	require.NoError(t, err)
	assert.Equal(t, classHash, got)

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractClassHashHistory),
		"deprecated must be empty",
	)
}

// Class-hash phase: a contract reclassed once. The deprecated bucket has one
// entry holding the deploy class hash (the value before the replace). After
// migration, history has: (addr, deploy_height) -> deploy class hash, and
// (addr, replace_block) -> replaced class hash.
func TestMigrate_ClassHash_Reclassed(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	deployClass := felt.FromUint64[felt.Felt](170)
	replacedClass := felt.FromUint64[felt.Felt](187)
	deployHeight := uint64(100)
	replaceBlock := uint64(300)

	seedContract(t, memDB, addr, felt.Zero, replacedClass, deployHeight)
	seedDeprecatedClassHashHistory(t, memDB, addr, replaceBlock, deployClass)

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	got, err := reader.ContractClassHashAt(&addr, deployHeight)
	require.NoError(t, err)
	assert.Equal(t, deployClass, got, "deploy entry preserved")

	got, err = reader.ContractClassHashAt(&addr, replaceBlock)
	require.NoError(t, err)
	assert.Equal(t, replacedClass, got, "replace block has post-update value (head)")

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractClassHashHistory),
		"deprecated must be empty",
	)
}

// Nonce phase: contract whose nonce was updated multiple times.
// Old: (B_1, 0), (B_2, n_1). New: (B_1, n_1), (B_2, head).
func TestMigrate_Nonce_Updated(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	classHash := felt.FromUint64[felt.Felt](170)
	headNonce := felt.FromUint64[felt.Felt](66)
	deployHeight := uint64(100)

	seedContract(t, memDB, addr, headNonce, classHash, deployHeight)
	seedDeprecatedNonceHistory(t, memDB, addr, 200, felt.Zero)
	seedDeprecatedNonceHistory(t, memDB, addr, 300, felt.FromUint64[felt.Felt](1))

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	got, err := reader.ContractNonceAt(&addr, 200)
	require.NoError(t, err)
	assert.Equal(
		t,
		felt.FromUint64[felt.Felt](1),
		got,
		"value installed at 200 = next entry's old value",
	)

	got, err = reader.ContractNonceAt(&addr, 300)
	require.NoError(t, err)
	assert.Equal(t, headNonce, got, "value installed at 300 = head")

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractNonceHistory),
		"deprecated must be empty",
	)
}

// Nonce phase: deploy-only contract (nonce never updated). No deprecated
// entries. Migration is a no-op for this address; history stays empty for it.
func TestMigrate_Nonce_DeployOnly(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](170), 100)

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.ContractNonceHistory),
		"no nonce entry expected when never updated",
	)
}

// Storage phase: a slot with multi-write history.
// Old has (B_1, 0), (B_2, firstVal), (B_3, secondVal).
// Head from old trie = headVal. New has (B_1, firstVal), (B_2, secondVal), (B_3, headVal).
func TestMigrate_Storage_MultiWrite(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	slot := felt.FromUint64[felt.Felt](170)
	firstVal := felt.FromUint64[felt.Felt](5)
	secondVal := felt.FromUint64[felt.Felt](12)
	headVal := felt.FromUint64[felt.Felt](7)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187), 100)

	seedDeprecatedStorageHistory(t, memDB, addr, slot, 100, felt.Zero)
	seedDeprecatedStorageHistory(t, memDB, addr, slot, 200, firstVal)
	seedDeprecatedStorageHistory(t, memDB, addr, slot, 300, secondVal)

	seedDeprecatedStorageTrie(t, memDB, addr, map[felt.Felt]felt.Felt{slot: headVal})

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	got, err := reader.ContractStorageAt(&addr, &slot, 100)
	require.NoError(t, err)
	assert.Equal(t, firstVal, got, "value installed at 100")

	got, err = reader.ContractStorageAt(&addr, &slot, 200)
	require.NoError(t, err)
	assert.Equal(t, secondVal, got, "value installed at 200")

	got, err = reader.ContractStorageAt(&addr, &slot, 300)
	require.NoError(t, err)
	assert.Equal(t, headVal, got, "value installed at 300 = head from trie")

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory),
		"deprecated must be empty",
	)
}

// Storage phase: a slot with one write at deploy block. Old: (B_1, 0). Head from trie = v.
// New: (B_1, v).
func TestMigrate_Storage_SingleWrite(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	slot := felt.FromUint64[felt.Felt](170)
	v := felt.FromUint64[felt.Felt](9)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187), 100)
	seedDeprecatedStorageHistory(t, memDB, addr, slot, 100, felt.Zero)
	seedDeprecatedStorageTrie(t, memDB, addr, map[felt.Felt]felt.Felt{slot: v})

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	got, err := reader.ContractStorageAt(&addr, &slot, 100)
	require.NoError(t, err)
	assert.Equal(t, v, got)

	assert.Equal(t, 0, bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory))
}

// Idempotency: running the migration twice produces the same history result
// and doesn't corrupt anything.
func TestMigrate_Idempotent(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	deployClass := felt.FromUint64[felt.Felt](170)
	replacedClass := felt.FromUint64[felt.Felt](187)

	seedContract(t, memDB, addr, felt.FromUint64[felt.Felt](66), replacedClass, 100)
	seedDeprecatedClassHashHistory(t, memDB, addr, 300, deployClass)
	seedDeprecatedNonceHistory(t, memDB, addr, 200, felt.Zero)

	for i := 0; i < 3; i++ {
		res, err := Migrator{}.Migrate(
			context.Background(),
			memDB,
			&networks.Sepolia,
			log.NewNopZapLogger(),
		)
		require.NoError(t, err)
		require.Nil(t, res)
	}

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	got, err := reader.ContractClassHashAt(&addr, 100)
	require.NoError(t, err)
	assert.Equal(t, deployClass, got)

	got, err = reader.ContractClassHashAt(&addr, 300)
	require.NoError(t, err)
	assert.Equal(t, replacedClass, got)

	got, err = reader.ContractNonceAt(&addr, 200)
	require.NoError(t, err)
	assert.Equal(t, felt.FromUint64[felt.Felt](66), got)
}

// Models a crash between writing some history entries and the final
// DeleteRange of the deprecated bucket for class-hash: history has the deploy
// entry written, the deprecated bucket is still fully intact. The migration
// must reach the same final state as a clean run.
func TestMigrate_ClassHash_ResumeFromPartial(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	deployClass := felt.FromUint64[felt.Felt](170)
	replacedClass := felt.FromUint64[felt.Felt](187)
	deployHeight := uint64(100)
	replaceBlock := uint64(300)

	seedContract(t, memDB, addr, felt.Zero, replacedClass, deployHeight)
	seedDeprecatedClassHashHistory(t, memDB, addr, replaceBlock, deployClass)

	// Simulate a prior partial run: deploy entry already written to history,
	// deprecated bucket still fully populated, DeleteRange not yet executed.
	require.NoError(t, state.WriteClassHashHistory(memDB, &addr, deployHeight, &deployClass))

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	got, err := reader.ContractClassHashAt(&addr, deployHeight)
	require.NoError(t, err)
	assert.Equal(t, deployClass, got, "deploy entry must survive partial-resume re-run")

	got, err = reader.ContractClassHashAt(&addr, replaceBlock)
	require.NoError(t, err)
	assert.Equal(t, replacedClass, got, "shifted entry at replace block")

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractClassHashHistory),
		"deprecated must be empty after migration completes",
	)
}

// Pedersen storage trie omits zero-valued leaves. A slot that has deprecated
// history (was once written) but whose current value is zero has no leaf in
// the trie. The lockstep iteration must surface head=Zero for such slots.
func TestMigrate_Storage_ZeroedSlotHasNoLeaf(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	zeroedSlot := felt.FromUint64[felt.Felt](170)
	keptSlot := felt.FromUint64[felt.Felt](187)
	keptHead := felt.FromUint64[felt.Felt](9)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](204), 100)

	// Both slots have deprecated entries...
	seedDeprecatedStorageHistory(t, memDB, addr, zeroedSlot, 100, felt.Zero)
	seedDeprecatedStorageHistory(t, memDB, addr, zeroedSlot, 200, felt.FromUint64[felt.Felt](5))
	seedDeprecatedStorageHistory(t, memDB, addr, keptSlot, 100, felt.Zero)

	// ...but only keptSlot has a leaf in the trie. zeroedSlot's current value
	// is 0 → Pedersen trie didn't store a leaf for it.
	seedDeprecatedStorageTrie(t, memDB, addr, map[felt.Felt]felt.Felt{keptSlot: keptHead})

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	// keptSlot's last entry should be the head from the trie.
	got, err := reader.ContractStorageAt(&addr, &keptSlot, 100)
	require.NoError(t, err)
	assert.Equal(t, keptHead, got, "kept slot last entry = head")

	// zeroedSlot's last entry should be 0 (no leaf in trie).
	got, err = reader.ContractStorageAt(&addr, &zeroedSlot, 200)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, got, "zeroed slot last entry = Zero (no leaf in trie)")

	// First entry of zeroedSlot's deprecated history was at block 100 with
	// v_1=0; new layout at block 100 should be v_2 = 0x5 (next entry's value).
	got, err = reader.ContractStorageAt(&addr, &zeroedSlot, 100)
	require.NoError(t, err)
	assert.Equal(t, felt.FromUint64[felt.Felt](5), got, "shift-up at block 100 = next-entry's value")

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory),
		"deprecated fully drained",
	)
}

// Storage at scale: many slots × many writes per slot for a single address.
// Verifies the per-slot grouping plus per-address DeleteRange handles
// non-trivial volumes correctly. Not large enough to hit the 96MB batch
// threshold (memory test runtimes would be silly), but exercises the
// grouping + cleanup logic across hundreds of slot boundaries.
func TestMigrate_Storage_ManyEntries(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	headValues := map[felt.Felt]felt.Felt{}

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187), 100)

	const (
		numSlots          = 50
		numEntriesPerSlot = 20
		startBlock        = uint64(100)
	)

	for s := uint64(1); s <= numSlots; s++ {
		slot := felt.NewFromUint64[felt.Felt](s)
		headVal := felt.NewFromUint64[felt.Felt](1000000 + s)
		headValues[*slot] = *headVal

		for b := uint64(0); b < numEntriesPerSlot; b++ {
			block := startBlock + b
			oldVal := felt.Zero
			if b > 0 {
				oldVal = *felt.NewFromUint64[felt.Felt](s*10000 + b)
			}
			seedDeprecatedStorageHistory(t, memDB, addr, *slot, block, oldVal)
		}
	}
	seedDeprecatedStorageTrie(t, memDB, addr, headValues)

	res, err := Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	assert.Equal(
		t,
		0,
		bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory),
		"deprecated storage history must be fully drained",
	)

	reader, err := state.NewStateReader(&felt.Zero, state.NewStateDB(memDB, triedb.New(memDB, nil)))
	require.NoError(t, err)

	// Sample a few slots: the last block per slot must equal head; intermediate
	// blocks must equal the next deprecated entry's old value.
	for slot, head := range headValues {
		lastBlock := startBlock + numEntriesPerSlot - 1
		got, err := reader.ContractStorageAt(&addr, &slot, lastBlock)
		require.NoErrorf(t, err, "read storage failed for slot %v", &slot)
		assert.Equalf(t, head, got, "last entry must equal head for slot %v", &slot)
	}
}
