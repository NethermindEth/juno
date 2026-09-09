package statebackend

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/pruner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOnlyTxn(t *testing.T) {
	store := memory.New()
	require.NoError(t, store.Put([]byte("key"), []byte("value")))
	txn := readOnlyTxn{store}

	assert.ErrorIs(t, txn.Put([]byte("k"), []byte("v")), ErrReadOnlyStateView)
	assert.ErrorIs(t, txn.Delete([]byte("k")), ErrReadOnlyStateView)
	assert.ErrorIs(t, txn.DeleteRange([]byte("a"), []byte("z")), ErrReadOnlyStateView)
	assert.ErrorIs(t, txn.Write(), ErrReadOnlyStateView)
	assert.Zero(t, txn.Size())

	// Close is a no-op: the underlying store must stay open and readable.
	require.NoError(t, txn.Close())
	has, err := store.Has([]byte("key"))
	require.NoError(t, err)
	assert.True(t, has)
}

func TestDeprecatedStateBackendReadOnlyViews(t *testing.T) {
	memDB := memory.New()

	addr := felt.FromUint64[felt.Felt](0xdead)
	classHash := felt.FromUint64[felt.Felt](0xc0ffee)
	nonce := felt.FromUint64[felt.Felt](7)
	su := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{addr: &classHash},
			Nonces:            map[felt.Felt]*felt.Felt{addr: &nonce},
		},
	}

	//nolint:staticcheck,nolintlint // deprecatedstate.New requires an IndexedBatch
	batch := memDB.NewIndexedBatch()
	require.NoError(t, deprecatedstate.New(batch).Update(&core.Header{Number: 0}, su, nil, true))

	blockHash := felt.FromUint64[felt.Felt](0xb10c)
	header := &core.Header{
		Number:          0,
		Hash:            &blockHash,
		ParentHash:      &felt.Zero,
		GlobalStateRoot: &felt.Zero,
	}
	require.NoError(t, core.WriteBlockHeaderByNumber(batch, header))
	require.NoError(t, core.WriteBlockHeaderNumberByHash(batch, &blockHash, 0))
	require.NoError(t, core.WriteChainHeight(batch, 0))
	require.NoError(t, batch.Write())

	backend := &deprecatedStateBackend{baseState{
		database:       memDB,
		retentionFloor: &pruner.RetentionFloor{},
	}}

	views := map[string]func() (core.StateReader, StateCloser, error){
		"HeadState": backend.HeadState,
		"StateAtBlockNumber": func() (core.StateReader, StateCloser, error) {
			return backend.StateAtBlockNumber(0)
		},
		"StateAtBlockHash": func() (core.StateReader, StateCloser, error) {
			return backend.StateAtBlockHash(&blockHash)
		},
	}

	for name, open := range views {
		t.Run(name, func(t *testing.T) {
			state, closer, err := open()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, closer()) })

			gotNonce, err := state.ContractNonce(&addr)
			require.NoError(t, err)
			assert.Equal(t, nonce, gotNonce)

			gotClassHash, err := state.ContractClassHash(&addr)
			require.NoError(t, err)
			assert.Equal(t, classHash, gotClassHash)
		})
	}

	t.Run("zero block hash returns empty usable state", func(t *testing.T) {
		state, closer, err := backend.StateAtBlockHash(&felt.Zero)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, closer()) })

		_, err = state.ContractNonce(&addr)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}
