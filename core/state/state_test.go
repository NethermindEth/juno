package state

import (
	"testing"

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
