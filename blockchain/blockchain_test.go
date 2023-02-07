package blockchain

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBlockchain(t *testing.T) {
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()
	t.Run("empty blockchain's head is nil", func(t *testing.T) {
		chain := NewBlockchain(db.NewTestDb(), utils.MAINNET)
		assert.Equal(t, utils.MAINNET, chain.network)
		b, err := chain.Head()
		assert.Nil(t, b)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("non-empty blockchain gets head from db", func(t *testing.T) {
		block0, err := gw.BlockByNumber(0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(0)
		require.NoError(t, err)

		testDB := db.NewTestDb()
		chain := NewBlockchain(testDB, utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))

		chain = NewBlockchain(testDB, utils.MAINNET)
		b, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, block0, b)
	})
}

func TestHeight(t *testing.T) {
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()
	t.Run("return nil if blockchain is empty", func(t *testing.T) {
		chain := NewBlockchain(db.NewTestDb(), utils.GOERLI)
		_, err := chain.Height()
		assert.Error(t, err)
	})
	t.Run("return height of the blockchain's head", func(t *testing.T) {
		block0, err := gw.BlockByNumber(0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(0)
		require.NoError(t, err)

		testDB := db.NewTestDb()
		chain := NewBlockchain(testDB, utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))

		chain = NewBlockchain(testDB, utils.MAINNET)
		height, err := chain.Height()
		assert.NoError(t, err)
		assert.Equal(t, block0.Number, height)
	})
}

func TestGetBlockByNumberAndHash(t *testing.T) {
	chain := NewBlockchain(db.NewTestDb(), utils.GOERLI)
	t.Run("same block is returned for both by GetBlockByNumber and GetBlockByHash", func(t *testing.T) {
		txn := chain.database.NewTransaction(true)
		defer txn.Discard()

		var err error
		block := new(core.Block)
		block.Number = 0xDEADBEEF
		block.Hash, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.ParentHash, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.GlobalStateRoot, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.SequencerAddress, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.Timestamp, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.TransactionCount, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.TransactionCommitment, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.EventCount, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.EventCommitment, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.ProtocolVersion, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)
		block.ExtraData, err = new(felt.Felt).SetRandom()
		require.NoError(t, err)

		require.NoError(t, putBlock(txn, block))

		storedByNumber, err := getBlockByNumber(txn, block.Number)
		require.NoError(t, err)
		assert.Equal(t, block, storedByNumber)

		storedByHash, err := getBlockByHash(txn, block.Hash)
		require.NoError(t, err)
		assert.Equal(t, block, storedByHash)
	})
	t.Run("GetBlockByNumber returns error if block doesn't exist", func(t *testing.T) {
		_, err := chain.GetBlockByNumber(42)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("GetBlockByHash returns error if block doesn't exist", func(t *testing.T) {
		f, err := new(felt.Felt).SetRandom()
		require.NoError(t, err)
		_, err = chain.GetBlockByHash(f)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
}

func TestVerifyBlock(t *testing.T) {
	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	h2, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	chain := NewBlockchain(db.NewTestDb(), utils.UNITTEST)

	t.Run("error if chain is empty and incoming block number is not 0", func(t *testing.T) {
		block := &core.Block{Number: 10}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with number more than 0 in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block, nil), expectedErr.Error())
	})

	t.Run("error if chain is empty and incoming block parent's hash is not 0", func(t *testing.T) {
		block := &core.Block{ParentHash: h1}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with non-zero parent hash in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block, nil), expectedErr.Error())
	})

	headBlock := &core.Block{
		Number:                0,
		ParentHash:            &felt.Zero,
		GlobalStateRoot:       &felt.Zero,
		TransactionCount:      &felt.Zero,
		TransactionCommitment: &felt.Zero,
	}
	headBlock.Hash, err = core.BlockHash(headBlock, utils.UNITTEST)
	require.NoError(t, err)

	headStateUpdate := &core.StateUpdate{
		BlockHash: headBlock.Hash,
		OldRoot:   &felt.Zero,
		NewRoot:   &felt.Zero,
		StateDiff: new(core.StateDiff),
	}
	require.NoError(t, chain.Store(headBlock, headStateUpdate))

	t.Run("error if difference between incoming block number and head is not 1",
		func(t *testing.T) {
			incomingBlock := &core.Block{Number: 10, ParentHash: headBlock.Hash}

			expectedErr := &ErrIncompatibleBlock{
				"block number difference between head and incoming block is not 1",
			}
			assert.Equal(t, chain.VerifyBlock(incomingBlock, nil), expectedErr)
		})

	t.Run("error when head hash does not match incoming block's parent hash", func(t *testing.T) {
		incomingBlock := &core.Block{ParentHash: h1, Number: 1}

		expectedErr := &ErrIncompatibleBlock{
			"block's parent hash does not match head block hash",
		}
		assert.Equal(t, chain.VerifyBlock(incomingBlock, nil), expectedErr)
	})

	validNextBlock := *headBlock
	validNextBlock.Number = 1
	validNextBlock.ParentHash = headBlock.Hash
	validNextBlock.Hash, err = core.BlockHash(&validNextBlock, utils.UNITTEST)
	assert.NoError(t, err)

	t.Run("error when block hash does not match state update's block hash", func(t *testing.T) {
		stateUpdate := &core.StateUpdate{BlockHash: h1}
		expectedErr := ErrIncompatibleBlockAndStateUpdate{"block hashes do not match"}
		assert.Equal(t, chain.VerifyBlock(&validNextBlock, stateUpdate), expectedErr)
	})

	t.Run("error when block global state root does not match state update's new root",
		func(t *testing.T) {
			block := validNextBlock
			block.GlobalStateRoot = h1
			stateUpdate := &core.StateUpdate{BlockHash: block.Hash, NewRoot: h2}

			expectedErr := ErrIncompatibleBlockAndStateUpdate{
				"block's GlobalStateRoot does not match state update's NewRoot",
			}
			assert.Equal(t, chain.VerifyBlock(&block, stateUpdate), expectedErr)
		})

	validNextStateUpdate := *headStateUpdate
	validNextStateUpdate.BlockHash = validNextBlock.Hash

	t.Run("error if block hash has not being calculated properly", func(t *testing.T) {
		wrongHashBlock := validNextBlock
		wrongHashBlock.Hash = h1

		wrongHashStateUpdate := validNextStateUpdate
		wrongHashStateUpdate.BlockHash = wrongHashBlock.Hash

		expectedErr := &ErrIncompatibleBlock{fmt.Sprintf(
			"incorrect block hash: block.Hash = %v and BlockHash(block) = %v",
			wrongHashBlock.Hash.Text(16), validNextBlock.Hash.Text(16))}
		assert.Equal(t, chain.VerifyBlock(&wrongHashBlock, &wrongHashStateUpdate), expectedErr)
	})

	require.NoError(t, chain.Store(&validNextBlock, &validNextStateUpdate))

	t.Run("no error if block is unverifiable", func(t *testing.T) {
		unverifiedBlock := validNextBlock
		unverifiedBlock.Number = 2
		unverifiedBlock.ParentHash = validNextBlock.Hash
		unverifiedBlock.Hash = h1

		unverifiedStateUpdate := validNextStateUpdate
		unverifiedStateUpdate.BlockHash = unverifiedBlock.Hash

		assert.NoError(t, chain.VerifyBlock(&unverifiedBlock, &unverifiedStateUpdate))
	})
}

func TestStore(t *testing.T) {
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()

	block0, err := gw.BlockByNumber(0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(0)
	require.NoError(t, err)

	t.Run("add block to empty blockchain", func(t *testing.T) {
		chain := NewBlockchain(db.NewTestDb(), utils.MAINNET)
		require.NoError(t, chain.Store(block0, stateUpdate0))

		headBlock, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, block0, headBlock)

		txn := chain.database.NewTransaction(false)
		defer txn.Discard()

		root, err := state.NewState(txn).Root()
		assert.NoError(t, err)
		assert.Equal(t, stateUpdate0.NewRoot, root)

		got0Block, err := chain.GetBlockByNumber(0)
		assert.NoError(t, err)
		assert.Equal(t, got0Block, block0)
	})
	t.Run("add block to non-empty blockchain", func(t *testing.T) {
		block1, err := gw.BlockByNumber(1)
		require.NoError(t, err)

		stateUpdate1, err := gw.StateUpdate(1)
		require.NoError(t, err)

		chain := NewBlockchain(db.NewTestDb(), utils.MAINNET)
		require.NoError(t, chain.Store(block0, stateUpdate0))
		require.NoError(t, chain.Store(block1, stateUpdate1))

		headBlock, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, block1, headBlock)

		txn := chain.database.NewTransaction(false)
		defer txn.Discard()

		root, err := state.NewState(txn).Root()
		assert.NoError(t, err)
		assert.Equal(t, stateUpdate1.NewRoot, root)

		got1Block, err := chain.GetBlockByNumber(1)
		assert.NoError(t, err)
		assert.Equal(t, got1Block, block1)
	})
}
