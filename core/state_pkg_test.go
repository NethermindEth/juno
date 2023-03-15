package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
)

func TestState_PutNewContract(t *testing.T) {
	testDb := pebble.NewMemTest()
	state := NewState(testDb.NewTransaction(true))

	addr, _ := new(felt.Felt).SetRandom()
	classHash, _ := new(felt.Felt).SetRandom()

	_, err := state.ContractClass(addr)
	assert.EqualError(t, err, db.ErrKeyNotFound.Error())

	assert.Equal(t, nil, state.putNewContract(addr, classHash))
	assert.EqualError(t, state.putNewContract(addr, classHash), "existing contract")

	got, err := state.ContractClass(addr)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, classHash.Equal(got))
}

func TestState_Root(t *testing.T) {
	testDb := pebble.NewMemTest()

	state := NewState(testDb.NewTransaction(true))

	key, _ := new(felt.Felt).SetRandom()
	value, _ := new(felt.Felt).SetRandom()

	// add a value and update db
	storage, err := state.stateStorage()
	assert.Equal(t, nil, err)
	_, err = storage.Put(key, value)
	assert.Equal(t, nil, err)

	err = state.putStateStorage(storage)
	assert.Equal(t, nil, err)

	expectedRootNode := new(trie.Node)
	expectedRootNode.Value = value

	kBits := key.Bits()
	newRootPath := bitset.FromWithLength(stateTrieHeight, kBits[:])
	expectedRoot := expectedRootNode.Hash(newRootPath)

	actualRoot, err := state.Root()
	assert.Equal(t, nil, err)
	assert.Equal(t, true, actualRoot.Equal(expectedRoot))
}
