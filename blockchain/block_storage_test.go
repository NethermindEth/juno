package blockchain

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

func TestBlockStorage(t *testing.T) {
	testDb := db.NewTestDb()
	txn := testDb.NewTransaction(true)
	bs := NewBlockStorage(txn)

	block := new(core.Block)
	block.Number = 0xDEADBEEF
	block.Hash, _ = new(felt.Felt).SetRandom()
	block.ParentHash, _ = new(felt.Felt).SetRandom()
	block.GlobalStateRoot, _ = new(felt.Felt).SetRandom()
	block.SequencerAddress, _ = new(felt.Felt).SetRandom()
	block.Timestamp, _ = new(felt.Felt).SetRandom()
	block.TransactionCount, _ = new(felt.Felt).SetRandom()
	block.TransactionCommitment, _ = new(felt.Felt).SetRandom()
	block.EventCount, _ = new(felt.Felt).SetRandom()
	block.EventCommitment, _ = new(felt.Felt).SetRandom()
	block.ProtocolVersion, _ = new(felt.Felt).SetRandom()
	block.ExtraData, _ = new(felt.Felt).SetRandom()

	assert.NoError(t, bs.Put(block))
	storedByNumber, err := bs.GetByNumber(block.Number)
	assert.NoError(t, err)
	assert.Equal(t, block, storedByNumber)

	storedByHash, err := bs.GetByHash(block.Hash)
	assert.Equal(t, block, storedByHash)

	_, err = bs.GetByNumber(42)
	assert.EqualError(t, err, "Key not found")
	_, err = bs.GetByHash(block.ParentHash)
	assert.EqualError(t, err, "Key not found")
}
