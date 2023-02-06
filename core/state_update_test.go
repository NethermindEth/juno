package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func TestClassStorage(t *testing.T) {
	testDb := db.NewTestDb()
	dbTxn := testDb.NewTransaction(true)
	classStorage := NewClassStorage(dbTxn)

	class := new(ClassDefinition)
	blockNumber := uint64(1)
	classHash := new(felt.Felt)
	assert.NoError(t, classStorage.Put(blockNumber, classHash, class))

	gotClass, err := classStorage.GetClass(blockNumber, classHash)
	assert.NoError(t, err)
	assert.Equal(t, class, gotClass)
}
