package headstate_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/headstate"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contractData struct {
	addr      felt.Felt
	classHash felt.Felt
	nonce     felt.Felt // felt.Zero means "do not write nonce entry"
	height    uint64
}

func seedDeprecated(t *testing.T, memDB db.KeyValueStore, seeds []contractData) {
	t.Helper()
	for i := range seeds {
		s := &seeds[i]
		require.NoError(t, core.WriteContractClassHash(memDB, &s.addr, &s.classHash))
		if !s.nonce.IsZero() {
			require.NoError(t, core.WriteContractNonce(memDB, &s.addr, &s.nonce))
		}
		require.NoError(t, core.WriteContractDeploymentHeight(memDB, &s.addr, s.height))
	}
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

	res, err := headstate.Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	assert.Nil(t, res, "empty DB must complete with no intermediate state")
}

func TestMigrate_ConsolidatesAddresses(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	seeds := []contractData{
		{
			addr:      felt.FromUint64[felt.Felt](1),
			classHash: felt.FromUint64[felt.Felt](170),
			nonce:     felt.FromUint64[felt.Felt](7),
			height:    100,
		},
		{
			addr:      felt.FromUint64[felt.Felt](2),
			classHash: felt.FromUint64[felt.Felt](187),
			nonce:     felt.Zero, // never updated
			height:    200,
		},
		{
			addr:      felt.FromUint64[felt.Felt](3),
			classHash: felt.FromUint64[felt.Felt](204),
			nonce:     felt.FromUint64[felt.Felt](66),
			height:    300,
		},
	}
	seedDeprecated(t, memDB, seeds)

	res, err := headstate.Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	assert.Nil(t, res)

	for i := range seeds {
		s := &seeds[i]
		got, err := state.GetContract(memDB, &s.addr)
		require.NoErrorf(t, err, "missing Contract for %s", &s.addr)
		assert.Equal(t, s.classHash, got.ClassHash, "ClassHash for %s", &s.addr)
		assert.Equal(t, s.nonce, got.Nonce, "Nonce for %s", &s.addr)
		assert.Equal(t, s.height, got.DeployedHeight, "DeployedHeight for %s", &s.addr)
		assert.True(t, got.StorageRoot.IsZero(), "StorageRoot must be zero (lazy)")
	}

	for _, bucket := range []db.Bucket{
		db.ContractClassHash,
		db.ContractNonce,
		db.ContractDeploymentHeight,
	} {
		assert.Equal(t, 0, bucketKeyCount(t, memDB, bucket), "old bucket %v must be empty", bucket)
	}
}

func TestMigrate_SkipsAlreadyMigrated(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	// addrDone has been migrated already (Contract record exists). Its old
	// keys are still around (simulating a previous partial run that wrote
	// the Contract record but didn't reach wipeDeprecatedBuckets).
	addrDone := felt.FromUint64[felt.Felt](1)
	doneClassHash := felt.FromUint64[felt.Felt](170)
	doneNonce := felt.FromUint64[felt.Felt](57005)

	require.NoError(t, state.WriteContract(memDB, &addrDone, doneNonce, doneClassHash, 111))
	require.NoError(t, core.WriteContractClassHash(memDB, &addrDone, &doneClassHash))
	require.NoError(t, core.WriteContractDeploymentHeight(memDB, &addrDone, 111))

	addrPending := felt.FromUint64[felt.Felt](2)
	pendingClassHash := felt.FromUint64[felt.Felt](187)
	pendingNonce := felt.FromUint64[felt.Felt](9)

	require.NoError(t, core.WriteContractClassHash(memDB, &addrPending, &pendingClassHash))
	require.NoError(t, core.WriteContractNonce(memDB, &addrPending, &pendingNonce))
	require.NoError(t, core.WriteContractDeploymentHeight(memDB, &addrPending, 222))

	res, err := headstate.Migrator{}.Migrate(
		context.Background(),
		memDB,
		&networks.Sepolia,
		log.NewNopZapLogger(),
	)
	require.NoError(t, err)
	assert.Nil(t, res)

	// addrDone's pre-existing Contract was preserved (not overwritten).
	done, err := state.GetContract(memDB, &addrDone)
	require.NoError(t, err)
	assert.Equal(t, doneNonce, done.Nonce, "addrDone's pre-existing Contract must not be overwritten")
	assert.Equal(t, uint64(111), done.DeployedHeight)

	// addrPending got migrated.
	pending, err := state.GetContract(memDB, &addrPending)
	require.NoError(t, err)
	assert.Equal(t, pendingClassHash, pending.ClassHash)
	assert.Equal(t, pendingNonce, pending.Nonce)
	assert.Equal(t, uint64(222), pending.DeployedHeight)

	for _, bucket := range []db.Bucket{
		db.ContractClassHash,
		db.ContractNonce,
		db.ContractDeploymentHeight,
	} {
		assert.Equal(t, 0, bucketKeyCount(t, memDB, bucket), "old bucket %v must be empty", bucket)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	seeds := []contractData{
		{
			addr:      felt.FromUint64[felt.Felt](1),
			classHash: felt.FromUint64[felt.Felt](170),
			nonce:     felt.FromUint64[felt.Felt](7),
			height:    100,
		},
		{
			addr:      felt.FromUint64[felt.Felt](2),
			classHash: felt.FromUint64[felt.Felt](187),
			nonce:     felt.Zero,
			height:    200,
		},
	}
	seedDeprecated(t, memDB, seeds)

	for i := range 3 {
		res, err := headstate.Migrator{}.Migrate(
			context.Background(),
			memDB,
			&networks.Sepolia,
			log.NewNopZapLogger(),
		)
		require.NoErrorf(t, err, "run %d", i)
		assert.Nilf(t, res, "run %d intermediate state", i)
	}

	for i := range seeds {
		s := &seeds[i]
		got, err := state.GetContract(memDB, &s.addr)
		require.NoError(t, err)
		assert.Equal(t, s.classHash, got.ClassHash)
	}
}
