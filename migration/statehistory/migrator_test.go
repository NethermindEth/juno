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
) {
	t.Helper()
	require.NoError(t, state.WriteContract(memDB, &addr, nonce, classHash, 100))
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

func TestMigrate_ClassHash_DeployOnly(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	classHash := felt.FromUint64[felt.Felt](170)
	seedContract(t, memDB, addr, felt.Zero, classHash)

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

func TestMigrate_ClassHash_Reclassed(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	deployClass := felt.FromUint64[felt.Felt](170)
	replacedClass := felt.FromUint64[felt.Felt](187)
	deployHeight := uint64(100)
	replaceBlock := uint64(300)

	seedContract(t, memDB, addr, felt.Zero, replacedClass)
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

func TestMigrate_Nonce_Updated(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	classHash := felt.FromUint64[felt.Felt](170)
	headNonce := felt.FromUint64[felt.Felt](66)

	seedContract(t, memDB, addr, headNonce, classHash)
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

func TestMigrate_Nonce_DeployOnly(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](170))

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

func TestMigrate_Storage_MultiWrite(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	slot := felt.FromUint64[felt.Felt](170)
	firstVal := felt.FromUint64[felt.Felt](5)
	secondVal := felt.FromUint64[felt.Felt](12)
	headVal := felt.FromUint64[felt.Felt](7)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187))

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

func TestMigrate_Storage_SingleWrite(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	slot := felt.FromUint64[felt.Felt](170)
	v := felt.FromUint64[felt.Felt](9)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187))
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

func TestMigrate_Idempotent(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	deployClass := felt.FromUint64[felt.Felt](170)
	replacedClass := felt.FromUint64[felt.Felt](187)

	seedContract(t, memDB, addr, felt.FromUint64[felt.Felt](66), replacedClass)
	seedDeprecatedClassHashHistory(t, memDB, addr, 300, deployClass)
	seedDeprecatedNonceHistory(t, memDB, addr, 200, felt.Zero)

	for range 3 {
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

func TestMigrate_ClassHash_ResumeFromPartial(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	deployClass := felt.FromUint64[felt.Felt](170)
	replacedClass := felt.FromUint64[felt.Felt](187)
	deployHeight := uint64(100)
	replaceBlock := uint64(300)

	seedContract(t, memDB, addr, felt.Zero, replacedClass)
	seedDeprecatedClassHashHistory(t, memDB, addr, replaceBlock, deployClass)

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

func TestMigrate_Storage_ZeroedSlotHasNoLeaf(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	zeroedSlot := felt.FromUint64[felt.Felt](170)
	keptSlot := felt.FromUint64[felt.Felt](187)
	keptHead := felt.FromUint64[felt.Felt](9)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](204))

	seedDeprecatedStorageHistory(t, memDB, addr, zeroedSlot, 100, felt.Zero)
	seedDeprecatedStorageHistory(t, memDB, addr, zeroedSlot, 200, felt.FromUint64[felt.Felt](5))
	seedDeprecatedStorageHistory(t, memDB, addr, keptSlot, 100, felt.Zero)

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

	got, err := reader.ContractStorageAt(&addr, &keptSlot, 100)
	require.NoError(t, err)
	assert.Equal(t, keptHead, got, "kept slot last entry = head")

	got, err = reader.ContractStorageAt(&addr, &zeroedSlot, 200)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, got, "zeroed slot last entry = Zero (no leaf in trie)")

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

func TestMigrate_Storage_ManyEntries(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	headValues := map[felt.Felt]felt.Felt{}

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187))

	const (
		numSlots          = 50
		numEntriesPerSlot = 20
		startBlock        = uint64(100)
	)

	for s := uint64(1); s <= numSlots; s++ {
		slot := felt.NewFromUint64[felt.Felt](s)
		headVal := felt.NewFromUint64[felt.Felt](1000000 + s)
		headValues[*slot] = *headVal

		for b := range uint64(numEntriesPerSlot) {
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

	for slot, head := range headValues {
		lastBlock := startBlock + numEntriesPerSlot - 1
		got, err := reader.ContractStorageAt(&addr, &slot, lastBlock)
		require.NoErrorf(t, err, "read storage failed for slot %v", &slot)
		assert.Equalf(t, head, got, "last entry must equal head for slot %v", &slot)
	}
}

func TestMigrate_Storage_MultiAddress(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addrs := []felt.Felt{
		felt.FromUint64[felt.Felt](1),
		felt.FromUint64[felt.Felt](2),
		felt.FromUint64[felt.Felt](3),
	}
	slots := []felt.Felt{
		felt.FromUint64[felt.Felt](100),
		felt.FromUint64[felt.Felt](200),
	}

	for i := range addrs {
		seedContract(t, memDB, addrs[i], felt.Zero, felt.FromUint64[felt.Felt](uint64(170+i)))
		for _, slot := range slots {
			seedDeprecatedStorageHistory(t, memDB, addrs[i], slot, 100, felt.Zero)
			seedDeprecatedStorageHistory(
				t, memDB, addrs[i], slot, 200,
				felt.FromUint64[felt.Felt](uint64(10+i)),
			)
		}
		seedDeprecatedStorageTrie(t, memDB, addrs[i], map[felt.Felt]felt.Felt{
			slots[0]: felt.FromUint64[felt.Felt](uint64(1000 + i*10)),
			slots[1]: felt.FromUint64[felt.Felt](uint64(1000 + i*10 + 1)),
		})
	}

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

	for i := range addrs {
		for j, slot := range slots {
			got, err := reader.ContractStorageAt(&addrs[i], &slot, 100)
			require.NoErrorf(t, err, "addr %d slot %d block 100", i, j)
			assert.Equalf(
				t, felt.FromUint64[felt.Felt](uint64(10+i)), got,
				"addr %d slot %d block 100 = next-entry's value", i, j,
			)
			got, err = reader.ContractStorageAt(&addrs[i], &slot, 200)
			require.NoErrorf(t, err, "addr %d slot %d block 200", i, j)
			assert.Equalf(
				t, felt.FromUint64[felt.Felt](uint64(1000+i*10+j)), got,
				"addr %d slot %d block 200 = head from trie", i, j,
			)
		}
	}

	assert.Zero(
		t,
		bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory),
		"deprecated must be empty across all addresses",
	)
}

func TestMigrate_CancelledContext_ResumesCleanly(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](170))
	seedDeprecatedClassHashHistory(t, memDB, addr, 200, felt.FromUint64[felt.Felt](42))
	seedDeprecatedNonceHistory(t, memDB, addr, 200, felt.Zero)
	slot := felt.FromUint64[felt.Felt](5)
	seedDeprecatedStorageHistory(t, memDB, addr, slot, 200, felt.Zero)
	seedDeprecatedStorageTrie(t, memDB, addr, map[felt.Felt]felt.Felt{
		slot: felt.FromUint64[felt.Felt](9),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := Migrator{}.Migrate(ctx, memDB, &networks.Sepolia, log.NewNopZapLogger())
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, res, "shouldRerun sentinel must not be nil")
	require.Empty(t, res, "shouldRerun is a non-nil empty slice")

	res, err = Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	require.Nil(t, res)

	assert.Zero(t, bucketKeyCount(t, memDB, db.DeprecatedContractClassHashHistory))
	assert.Zero(t, bucketKeyCount(t, memDB, db.DeprecatedContractNonceHistory))
	assert.Zero(t, bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory))
}

func TestMigrate_Storage_ResumeFromPartial(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	slot := felt.FromUint64[felt.Felt](170)
	firstVal := felt.FromUint64[felt.Felt](5)
	secondVal := felt.FromUint64[felt.Felt](12)
	headVal := felt.FromUint64[felt.Felt](7)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](187))

	seedDeprecatedStorageHistory(t, memDB, addr, slot, 100, felt.Zero)
	seedDeprecatedStorageHistory(t, memDB, addr, slot, 200, firstVal)
	seedDeprecatedStorageHistory(t, memDB, addr, slot, 300, secondVal)
	seedDeprecatedStorageTrie(t, memDB, addr, map[felt.Felt]felt.Felt{slot: headVal})

	require.NoError(t, state.WriteStorageHistory(memDB, &addr, &slot, 100, &firstVal))

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
	assert.Equal(t, firstVal, got, "preserved deploy entry after partial-resume")

	got, err = reader.ContractStorageAt(&addr, &slot, 200)
	require.NoError(t, err)
	assert.Equal(t, secondVal, got)

	got, err = reader.ContractStorageAt(&addr, &slot, 300)
	require.NoError(t, err)
	assert.Equal(t, headVal, got)

	assert.Zero(t, bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory))
}

func TestMigrate_AddressWithEmptyHistoryForOnePhase(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	classHash := felt.FromUint64[felt.Felt](170)
	deployClass := felt.FromUint64[felt.Felt](42)

	seedContract(t, memDB, addr, felt.Zero, classHash)
	seedDeprecatedClassHashHistory(t, memDB, addr, 300, deployClass)

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
	assert.Equal(t, deployClass, got)

	got, err = reader.ContractClassHashAt(&addr, 300)
	require.NoError(t, err)
	assert.Equal(t, classHash, got)

	assert.Zero(
		t, bucketKeyCount(t, memDB, db.ContractNonceHistory),
		"nonce history empty when phase no-ops",
	)
	assert.Zero(
		t, bucketKeyCount(t, memDB, db.ContractStorageHistory),
		"storage history empty when phase no-ops",
	)
	assert.Zero(t, bucketKeyCount(t, memDB, db.DeprecatedContractClassHashHistory))
}

func TestMigrate_Storage_InterleavedZeroedSlots(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addr := felt.FromUint64[felt.Felt](1)
	slot1 := felt.FromUint64[felt.Felt](100) // zeroed
	slot2 := felt.FromUint64[felt.Felt](200) // kept
	slot3 := felt.FromUint64[felt.Felt](300) // zeroed
	slot4 := felt.FromUint64[felt.Felt](400) // kept
	head2 := felt.FromUint64[felt.Felt](22)
	head4 := felt.FromUint64[felt.Felt](44)

	seedContract(t, memDB, addr, felt.Zero, felt.FromUint64[felt.Felt](204))

	for _, slot := range []felt.Felt{slot1, slot2, slot3, slot4} {
		seedDeprecatedStorageHistory(t, memDB, addr, slot, 100, felt.Zero)
	}
	seedDeprecatedStorageTrie(t, memDB, addr, map[felt.Felt]felt.Felt{
		slot2: head2,
		slot4: head4,
	})

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

	for _, slot := range []felt.Felt{slot1, slot3} {
		got, err := reader.ContractStorageAt(&addr, &slot, 100)
		require.NoErrorf(t, err, "zeroed slot %v", &slot)
		assert.Equalf(t, felt.Zero, got, "zeroed slot %v has no trie leaf", &slot)
	}
	got, err := reader.ContractStorageAt(&addr, &slot2, 100)
	require.NoError(t, err)
	assert.Equal(t, head2, got, "slot2 kept its head value")
	got, err = reader.ContractStorageAt(&addr, &slot4, 100)
	require.NoError(t, err)
	assert.Equal(t, head4, got, "slot4 kept its head value")

	assert.Zero(t, bucketKeyCount(t, memDB, db.DeprecatedContractStorageHistory))
}
