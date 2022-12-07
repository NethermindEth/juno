package state

import (
	"testing"

	"github.com/NethermindEth/juno/db"

	"github.com/stretchr/testify/assert"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/dgraph-io/badger/v3"
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
