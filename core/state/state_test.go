package state

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bitset"

	"github.com/stretchr/testify/assert"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/dgraph-io/badger/v3"
)

func newTestDb() *badger.DB {
	opt := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opt)
	if err != nil {
		panic(err)
	}
	return db
}

func TestState_PutNewContract(t *testing.T) {
	testDb := newTestDb()

	state := NewState(testDb)

	addr, _ := new(felt.Felt).SetRandom()
	classHash, _ := new(felt.Felt).SetRandom()

	got, err := state.GetContractClass(addr)
	assert.EqualError(t, err, "Key not found")

	testDb.Update(func(txn *badger.Txn) error {
		assert.Equal(t, nil, state.putNewContract(addr, classHash, txn))
		assert.EqualError(t, state.putNewContract(addr, classHash, txn), "existing contract")
		return nil
	})

	got, err = state.GetContractClass(addr)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, classHash.Equal(got))
}

func TestState_Root(t *testing.T) {
	testDb := newTestDb()

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
		newRootPath = storage.PathFromKey(key)

		return err
	}); err != nil {
		t.Error(err)
	}

	expectedRootNode := new(core.TrieNode)
	if err := expectedRootNode.UnmarshalBinary(value.Marshal()); err != nil {
		t.Error(err)
	}

	expectedRootNode.UnmarshalBinary(value.Marshal())
	expectedRoot := expectedRootNode.Hash(core.GetSpecPath(newRootPath, nil))

	actualRoot, err := state.Root()
	assert.Equal(t, nil, err)
	assert.Equal(t, true, actualRoot.Equal(expectedRoot))
}
