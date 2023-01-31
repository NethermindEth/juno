package blockchain

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func TestTransactionStorage(t *testing.T) {
	testDb := db.NewTestDb()
	txn := testDb.NewTransaction(true)
	txnStorage := NewTransactionStorage(txn)

	txnHash := new(felt.Felt)
	storedBlockNumber := uint64(1)
	storedIndex := uint64(2)
	assert.NoError(t, txnStorage.Put(storedBlockNumber, storedIndex, txnHash))

	gotBlockNumber, gotIndex, err := txnStorage.GetBlockNumberAndIndex(txnHash)
	assert.NoError(t, err)
	assert.Equal(t, storedBlockNumber, *gotBlockNumber)
	assert.Equal(t, storedIndex, *gotIndex)
}
