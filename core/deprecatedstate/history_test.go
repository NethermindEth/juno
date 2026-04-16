package deprecatedstate_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateHistory(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()
	state := deprecatedstate.New(txn)

	addr := felt.FromUint64[felt.Address](1)
	addrFelt := felt.Felt(addr)
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

	require.NoError(t, state.Update(&core.Header{Number: deployedHeight}, &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{addrFelt: initialClassHash},
			Nonces:            map[felt.Felt]*felt.Felt{addrFelt: initialNonce},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				addrFelt: {*storageKey: initialStorage},
			},
		},
	}, map[felt.Felt]core.ClassDefinition{*declaredCH: &core.SierraClass{}}, true))

	root, err := state.Commitment("")
	require.NoError(t, err)

	require.NoError(t, state.Update(&core.Header{Number: changeHeight}, &core.StateUpdate{
		OldRoot: &root,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			ReplacedClasses: map[felt.Felt]*felt.Felt{addrFelt: updatedClassHash},
			Nonces:          map[felt.Felt]*felt.Felt{addrFelt: updatedNonce},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				addrFelt: {*storageKey: updatedStorage},
			},
		},
	}, nil, true))

	snapshotBeforeDeployment := deprecatedstate.NewHistory(state, deployedHeight-1)
	snapshotAtDeployment := deprecatedstate.NewHistory(state, deployedHeight)
	snapshotAfterChange := deprecatedstate.NewHistory(state, changeHeight+1)

	t.Run("contract not deployed", func(t *testing.T) {
		_, err := snapshotBeforeDeployment.ContractClassHash(&addrFelt)
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		_, err = snapshotBeforeDeployment.ContractNonce(&addrFelt)
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		_, err = snapshotBeforeDeployment.ContractStorage(&addrFelt, storageKey)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("historical values at deployment height", func(t *testing.T) {
		classHash, err := snapshotAtDeployment.ContractClassHash(&addrFelt)
		require.NoError(t, err)
		require.Equal(t, *initialClassHash, classHash)

		nonce, err := snapshotAtDeployment.ContractNonce(&addrFelt)
		require.NoError(t, err)
		require.Equal(t, *initialNonce, nonce)

		storage, err := snapshotAtDeployment.ContractStorage(&addrFelt, storageKey)
		require.NoError(t, err)
		require.Equal(t, *initialStorage, storage)
	})

	t.Run("head state values after change", func(t *testing.T) {
		classHash, err := snapshotAfterChange.ContractClassHash(&addrFelt)
		require.NoError(t, err)
		require.Equal(t, *updatedClassHash, classHash)

		nonce, err := snapshotAfterChange.ContractNonce(&addrFelt)
		require.NoError(t, err)
		require.Equal(t, *updatedNonce, nonce)

		storage, err := snapshotAfterChange.ContractStorage(&addrFelt, storageKey)
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

	t.Run("ContractStorageLastUpdatedBlock", func(t *testing.T) {
		t.Run("returns not found before any storage update", func(t *testing.T) {
			blockNum, err := snapshotBeforeDeployment.ContractStorageLastUpdatedBlock(
				&addr, storageKey,
			)
			require.NoError(t, err)
			assert.Equal(t, uint64(0), blockNum)
		})

		t.Run("returns deployment height at deployment snapshot", func(t *testing.T) {
			blockNum, err := snapshotAtDeployment.ContractStorageLastUpdatedBlock(
				&addr, storageKey,
			)
			require.NoError(t, err)
			assert.Equal(t, deployedHeight, blockNum)
		})

		t.Run("returns deployment height between deployment and change", func(t *testing.T) {
			snapshotBetween := deprecatedstate.NewHistory(state, changeHeight-1)
			blockNum, err := snapshotBetween.ContractStorageLastUpdatedBlock(
				&addr, storageKey,
			)
			require.NoError(t, err)
			assert.Equal(t, deployedHeight, blockNum)
		})

		t.Run("returns change height after storage was updated", func(t *testing.T) {
			blockNum, err := snapshotAfterChange.ContractStorageLastUpdatedBlock(
				&addr, storageKey,
			)
			require.NoError(t, err)
			assert.Equal(t, changeHeight, blockNum)
		})

		t.Run("returns not found for a key that was never updated", func(t *testing.T) {
			unknownKey := felt.NewFromUint64[felt.Felt](999)
			blockNum, err := snapshotAtDeployment.ContractStorageLastUpdatedBlock(
				&addr, unknownKey,
			)
			require.NoError(t, err)
			assert.Equal(t, uint64(0), blockNum)
		})
	})

	t.Run(
		"deprecated state history trie methods return ErrHistoricalTrieNotSupported",
		func(t *testing.T) {
			_, err := snapshotAtDeployment.ClassTrie()
			require.ErrorIs(t, err, deprecatedstate.ErrHistoricalTrieNotSupported)

			_, err = snapshotAtDeployment.ContractTrie()
			require.ErrorIs(t, err, deprecatedstate.ErrHistoricalTrieNotSupported)

			_, err = snapshotAtDeployment.ContractStorageTrie(&addrFelt)
			require.ErrorIs(t, err, deprecatedstate.ErrHistoricalTrieNotSupported)
		})
}
