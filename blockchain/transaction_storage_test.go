package blockchain

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func TestTransactionStorage(t *testing.T) {
	testDb := db.NewTestDb()
	dbTxn := testDb.NewTransaction(true)
	txnStorage := NewTransactionStorage(dbTxn)

	transaction := new(core.Transaction)
	txnHash := new(felt.Felt)
	storedBlockNumber := uint64(1)
	storedIndex := uint64(2)
	assert.NoError(t, txnStorage.PutTransaction(storedBlockNumber, storedIndex, txnHash, transaction))

	gotBlockNumber, gotIndex, err := txnStorage.GetBlockNumberAndIndex(txnHash)
	assert.NoError(t, err)
	assert.Equal(t, storedBlockNumber, *gotBlockNumber)
	assert.Equal(t, storedIndex, *gotIndex)

	gotTransaction, err := txnStorage.GetTransaction(*gotBlockNumber, *gotIndex)
	assert.NoError(t, err)
	fmt.Println(transaction, gotTransaction)
	// TODO: Fix this once Transaction type has been updated.
	// assert.Equal(t, transaction, gotTransaction)
}

func TestTransactionReceiptStorage(t *testing.T) {
	testDb := db.NewTestDb()
	dbTxn := testDb.NewTransaction(true)
	txnStorage := NewTransactionStorage(dbTxn)

	transactionReceipt := new(core.TransactionReceipt)
	storedBlockNumber := uint64(1)
	storedIndex := uint64(2)
	assert.NoError(t, txnStorage.PutTransactionReceipt(storedBlockNumber, storedIndex, transactionReceipt))

	gotTransactionReceipt, err := txnStorage.GetTransactionReceipt(storedBlockNumber, storedIndex)
	assert.NoError(t, err)
	assert.Equal(t, transactionReceipt, gotTransactionReceipt)
}
