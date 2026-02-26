package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/require"
)

func TestStateHistory(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()
	state := core.NewState(txn)

	addr := felt.NewFromUint64[felt.Felt](1)
	storageKey := felt.NewFromUint64[felt.Felt](2)
	declaredCH := felt.NewFromUint64[felt.Felt](3)

	initialClassHash := felt.NewFromUint64[felt.Felt](10)
	updatedClassHash := felt.NewFromUint64[felt.Felt](20)
	initialNonce := felt.NewFromUint64[felt.Felt](5)
	updatedNonce := felt.NewFromUint64[felt.Felt](9)
	initialStorage := felt.NewFromUint64[felt.Felt](100)
	updatedStorage := felt.NewFromUint64[felt.Felt](200)

	deployedHeight := uint64(3)
	changeHeight := uint64(10)

	require.NoError(t, state.Update(deployedHeight, &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{*addr: initialClassHash},
			Nonces:            map[felt.Felt]*felt.Felt{*addr: initialNonce},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*addr: {*storageKey: initialStorage},
			},
		},
	}, map[felt.Felt]core.ClassDefinition{*declaredCH: &core.SierraClass{}}, true))

	root, err := state.Commitment()
	require.NoError(t, err)

	require.NoError(t, state.Update(changeHeight, &core.StateUpdate{
		OldRoot: &root,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			ReplacedClasses: map[felt.Felt]*felt.Felt{*addr: updatedClassHash},
			Nonces:          map[felt.Felt]*felt.Felt{*addr: updatedNonce},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*addr: {*storageKey: updatedStorage},
			},
		},
	}, nil, true))

	snapshotBeforeDeployment := core.NewDeprecatedStateHistory(state, deployedHeight-1)
	snapshotAtDeployment := core.NewDeprecatedStateHistory(state, deployedHeight)
	snapshotAfterChange := core.NewDeprecatedStateHistory(state, changeHeight+1)

	t.Run("contract not deployed", func(t *testing.T) {
		_, err := snapshotBeforeDeployment.ContractClassHash(addr)
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		_, err = snapshotBeforeDeployment.ContractNonce(addr)
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		_, err = snapshotBeforeDeployment.ContractStorage(addr, storageKey)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("historical values at deployment height", func(t *testing.T) {
		classHash, err := snapshotAtDeployment.ContractClassHash(addr)
		require.NoError(t, err)
		require.Equal(t, *initialClassHash, classHash)

		nonce, err := snapshotAtDeployment.ContractNonce(addr)
		require.NoError(t, err)
		require.Equal(t, *initialNonce, nonce)

		storage, err := snapshotAtDeployment.ContractStorage(addr, storageKey)
		require.NoError(t, err)
		require.Equal(t, *initialStorage, storage)
	})

	t.Run("head state values after change", func(t *testing.T) {
		classHash, err := snapshotAfterChange.ContractClassHash(addr)
		require.NoError(t, err)
		require.Equal(t, *updatedClassHash, classHash)

		nonce, err := snapshotAfterChange.ContractNonce(addr)
		require.NoError(t, err)
		require.Equal(t, *updatedNonce, nonce)

		storage, err := snapshotAfterChange.ContractStorage(addr, storageKey)
		require.NoError(t, err)
		require.Equal(t, *updatedStorage, storage)
	})

	t.Run("non-deployed address returns error", func(t *testing.T) {
		nonDeployedAddr := felt.NewFromUint64[felt.Felt](999)
		_, err := snapshotAtDeployment.ContractClassHash(nonDeployedAddr)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("Class", func(t *testing.T) {
		t.Run("before declaration", func(t *testing.T) {
			_, err := snapshotBeforeDeployment.Class(declaredCH)
			require.ErrorIs(t, err, db.ErrKeyNotFound)
		})

		t.Run("at declaration height", func(t *testing.T) {
			declared, err := snapshotAtDeployment.Class(declaredCH)
			require.NoError(t, err)
			require.Equal(t, deployedHeight, declared.At)
		})

		t.Run("after declaration", func(t *testing.T) {
			declared, err := snapshotAfterChange.Class(declaredCH)
			require.NoError(t, err)
			require.Equal(t, deployedHeight, declared.At)
		})
	})

	t.Run("CompiledClassHash", func(t *testing.T) {
		sierraClassHash := felt.NewFromUint64[felt.SierraClassHash](123)
		casmHashV1 := felt.NewFromUint64[felt.CasmClassHash](456)
		casmHashV2 := felt.NewFromUint64[felt.CasmClassHash](789)

		record := core.NewCasmHashMetadataDeclaredV1(deployedHeight, casmHashV1, casmHashV2)
		require.NoError(t, record.Migrate(changeHeight))
		require.NoError(t, core.WriteClassCasmHashMetadata(txn, sierraClassHash, &record))

		t.Run("before migration", func(t *testing.T) {
			got, err := snapshotAtDeployment.CompiledClassHash(sierraClassHash)
			require.NoError(t, err)
			require.Equal(t, *casmHashV1, got)
		})

		t.Run("after migration", func(t *testing.T) {
			got, err := snapshotAfterChange.CompiledClassHash(sierraClassHash)
			require.NoError(t, err)
			require.Equal(t, *casmHashV2, got)
		})
	})

	t.Run(
		"deprecated state history trie methods return ErrHistoricalTrieNotSupported",
		func(t *testing.T) {
			_, err := snapshotAtDeployment.ClassTrie()
			require.ErrorIs(t, err, core.ErrHistoricalTrieNotSupported)

			_, err = snapshotAtDeployment.ContractTrie()
			require.ErrorIs(t, err, core.ErrHistoricalTrieNotSupported)

			_, err = snapshotAtDeployment.ContractStorageTrie(addr)
			require.ErrorIs(t, err, core.ErrHistoricalTrieNotSupported)
		})
}
