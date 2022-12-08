package state

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
)

func TestState_PutNewContract(t *testing.T) {
	testDb := db.NewTestDb()
	state := NewState(testDb)

	addr, _ := new(felt.Felt).SetRandom()
	classHash, _ := new(felt.Felt).SetRandom()

	_, err := state.GetContractClass(addr)
	assert.EqualError(t, err, "Key not found")

	testDb.Update(func(txn *badger.Txn) error {
		assert.Equal(t, nil, state.putNewContract(addr, classHash, txn))
		assert.EqualError(t, state.putNewContract(addr, classHash, txn), "existing contract")
		return nil
	})

	got, err := state.GetContractClass(addr)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, classHash.Equal(got))
}

func TestState_Root(t *testing.T) {
	testDb := db.NewTestDb()

	state := NewState(testDb)

	key, _ := new(felt.Felt).SetRandom()
	value, _ := new(felt.Felt).SetRandom()

	var newRootPath *bitset.BitSet

	// add a value and update db
	if err := testDb.Update(func(txn *badger.Txn) error {
		storage, err := state.getStateStorage(txn)
		assert.Equal(t, nil, err)
		assert.Equal(t, nil, storage.Put(key, value))

		err = state.putStateStorage(storage, txn)
		assert.Equal(t, nil, err)
		newRootPath = storage.FeltToBitSet(key)

		return err
	}); err != nil {
		t.Error(err)
	}

	expectedRootNode := new(trie.Node)
	if err := expectedRootNode.UnmarshalBinary(value.Marshal()); err != nil {
		t.Error(err)
	}

	expectedRootNode.UnmarshalBinary(value.Marshal())
	expectedRoot := expectedRootNode.Hash(trie.Path(newRootPath, nil))

	actualRoot, err := state.Root()
	assert.Equal(t, nil, err)
	assert.Equal(t, true, actualRoot.Equal(expectedRoot))
}
