package commonstate_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/state/commonstate"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreStateAdapter_ContractStorageAt(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}
	key := &felt.Felt{}
	blockNumber := uint64(0)

	value, err := coreStateAdapter.ContractStorageAt(addr, key, blockNumber)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, value)
}

func TestCoreStateAdapter_ContractNonceAt(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}
	blockNumber := uint64(0)

	nonce, err := coreStateAdapter.ContractNonceAt(addr, blockNumber)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, nonce)
}

func TestCoreStateAdapter_ContractClassHashAt(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}
	blockNumber := uint64(0)

	classHash, err := coreStateAdapter.ContractClassHashAt(addr, blockNumber)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, classHash)
}

func TestCoreStateAdapter_ContractDeployedAt(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}
	blockNumber := uint64(0)

	deployed, err := coreStateAdapter.ContractDeployedAt(addr, blockNumber)
	require.NoError(t, err)
	assert.False(t, deployed)
}

func TestCoreStateAdapter_ContractClassHash(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}

	classHash, err := coreStateAdapter.ContractClassHash(addr)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, classHash)
}

func TestCoreStateAdapter_ContractNonce(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}

	nonce, err := coreStateAdapter.ContractNonce(addr)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, nonce)
}

func TestCoreStateAdapter_ContractStorage(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}
	key := &felt.Felt{}

	value, err := coreStateAdapter.ContractStorage(addr, key)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, value)
}

func TestCoreStateAdapter_Class(t *testing.T) {
	coreStateAdapter := setupCoreStateAdapter(t)
	classHash := &felt.Felt{}

	class, err := coreStateAdapter.Class(classHash)
	require.NoError(t, err)
	assert.Nil(t, class)
}

func TestStateAdapter_ClassTrie(t *testing.T) {
	stateAdapter := setupStateAdapter(t)

	trie, err := stateAdapter.ClassTrie()
	require.NoError(t, err)
	assert.Nil(t, trie)
}

func TestStateAdapter_ContractTrie(t *testing.T) {
	stateAdapter := setupStateAdapter(t)

	trie, err := stateAdapter.ContractTrie()
	require.NoError(t, err)
	assert.Nil(t, trie)
}

func TestStateAdapter_ContractStorageTrie(t *testing.T) {
	stateAdapter := setupStateAdapter(t)
	addr := &felt.Felt{}

	trie, err := stateAdapter.ContractStorageTrie(addr)
	require.NoError(t, err)
	assert.Nil(t, trie)
}

func TestCoreStateReaderAdapter_Class(t *testing.T) {
	coreStateReaderAdapter := setupStateAdapter(t)
	classHash := &felt.Felt{}

	class, err := coreStateReaderAdapter.Class(classHash)
	require.NoError(t, err)
	assert.Nil(t, class)
}

func TestCoreStateReaderAdapter_ContractClassHash(t *testing.T) {
	coreStateReaderAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}

	classHash, err := coreStateReaderAdapter.ContractClassHash(addr)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, classHash)
}

func TestCoreStateReaderAdapter_ContractNonce(t *testing.T) {
	coreStateReaderAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}

	nonce, err := coreStateReaderAdapter.ContractNonce(addr)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, nonce)
}

func TestCoreStateReaderAdapter_ContractStorage(t *testing.T) {
	coreStateReaderAdapter := setupCoreStateAdapter(t)
	addr := &felt.Felt{}
	key := &felt.Felt{}

	value, err := coreStateReaderAdapter.ContractStorage(addr, key)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, value)
}

func TestStateReaderAdapter_Class(t *testing.T) {
	stateReaderAdapter := setupStateAdapter(t)
	classHash := &felt.Felt{}

	class, err := stateReaderAdapter.Class(classHash)
	require.NoError(t, err)
	assert.Nil(t, class)
}

func TestStateReaderAdapter_ContractClassHash(t *testing.T) {
	stateReaderAdapter := setupStateAdapter(t)
	addr := &felt.Felt{}

	classHash, err := stateReaderAdapter.ContractClassHash(addr)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, classHash)
}

func TestStateReaderAdapter_ContractNonce(t *testing.T) {
	stateReaderAdapter := setupStateAdapter(t)
	addr := &felt.Felt{}

	nonce, err := stateReaderAdapter.ContractNonce(addr)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, nonce)
}

func TestStateReaderAdapter_ContractStorage(t *testing.T) {
	stateReaderAdapter := setupStateAdapter()
	addr := &felt.Felt{}
	key := &felt.Felt{}

	value, err := stateReaderAdapter.ContractStorage(addr, key)
	require.NoError(t, err)
	assert.Equal(t, felt.Zero, value)
}

func setupCoreStateAdapter(t *testing.T) *commonstate.CoreStateAdapter {
	state := setupCoreState()
	return commonstate.NewCoreStateAdapter(state)
}

func setupCoreState() *core.State {
	testDB := memory.New()
	txn := testDB.NewIndexedBatch()

	return core.NewState(txn)
}

func setupStateAdapter() *commonstate.StateAdapter {
	state := setupState([]*core.StateUpdate{}, 0)
	return commonstate.NewStateAdapter(state)
}

func setupState(t *testing.T, stateUpdates []*core.StateUpdate, blocks uint64) *state.State {
	stateDB := newTestStateDB()
	state, err := state.New(&felt.Zero, stateDB)
	require.NoError(t, err)
	return state
}

func newTestStateDB() *state.StateDB {
	memDB := memory.New()
	db, err := triedb.New(memDB, nil)
	if err != nil {
		panic(err)
	}
	return state.NewStateDB(memDB, db)
}
