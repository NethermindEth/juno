package blockchain

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBlockchain(t *testing.T) {
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()
	t.Run("empty blockchain's head is nil", func(t *testing.T) {
		chain := NewBlockchain(pebble.NewMemTest(), utils.MAINNET)
		assert.Equal(t, utils.MAINNET, chain.network)
		b, err := chain.Head()
		assert.Nil(t, b)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("non-empty blockchain gets head from db", func(t *testing.T) {
		block0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)

		testDB := pebble.NewMemTest()
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
		chain := NewBlockchain(pebble.NewMemTest(), utils.GOERLI)
		_, err := chain.Height()
		assert.Error(t, err)
	})
	t.Run("return height of the blockchain's head", func(t *testing.T) {
		block0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)

		testDB := pebble.NewMemTest()
		chain := NewBlockchain(testDB, utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))

		chain = NewBlockchain(testDB, utils.MAINNET)
		height, err := chain.Height()
		assert.NoError(t, err)
		assert.Equal(t, block0.Number, height)
	})
}

func TestGetBlockByNumberAndHash(t *testing.T) {
	chain := NewBlockchain(pebble.NewMemTest(), utils.GOERLI)
	t.Run("same block is returned for both by GetBlockByNumber and GetBlockByHash", func(t *testing.T) {
		gw, closer := testsource.NewTestGateway(utils.MAINNET)
		defer closer.Close()

		block, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)
		update, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)

		require.NoError(t, chain.Store(block, update))

		storedByNumber, err := chain.GetBlockByNumber(block.Number)
		require.NoError(t, err)
		assert.Equal(t, block, storedByNumber)

		storedByHash, err := chain.GetBlockByHash(block.Hash)
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

	chain := NewBlockchain(pebble.NewMemTest(), utils.MAINNET)

	t.Run("error if chain is empty and incoming block number is not 0", func(t *testing.T) {
		block := &core.Block{Header: core.Header{Number: 10}}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with number more than 0 in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block), expectedErr.Error())
	})

	t.Run("error if chain is empty and incoming block parent's hash is not 0", func(t *testing.T) {
		block := &core.Block{Header: core.Header{ParentHash: h1}}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with non-zero parent hash in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block), expectedErr.Error())
	})

	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()

	mainnetBlock0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	mainnetStateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	require.NoError(t, chain.Store(mainnetBlock0, mainnetStateUpdate0))

	t.Run("error if difference between incoming block number and head is not 1",
		func(t *testing.T) {
			incomingBlock := &core.Block{Header: core.Header{Number: 10}}

			expectedErr := &ErrIncompatibleBlock{
				"block number difference between head and incoming block is not 1",
			}
			assert.Equal(t, chain.VerifyBlock(incomingBlock), expectedErr)
		})

	t.Run("error when head hash does not match incoming block's parent hash", func(t *testing.T) {
		incomingBlock := &core.Block{Header: core.Header{ParentHash: h1, Number: 1}}

		expectedErr := &ErrIncompatibleBlock{
			"block's parent hash does not match head block hash",
		}
		assert.Equal(t, chain.VerifyBlock(incomingBlock), expectedErr)
	})
}

func TestSanityCheckNewHeight(t *testing.T) {
	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	chain := NewBlockchain(pebble.NewMemTest(), utils.MAINNET)

	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()

	mainnetBlock0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	mainnetStateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	require.NoError(t, chain.Store(mainnetBlock0, mainnetStateUpdate0))

	mainnetBlock1, err := gw.BlockByNumber(context.Background(), 1)
	require.NoError(t, err)
	mainnetStateUpdate1, err := gw.StateUpdate(context.Background(), 1)
	require.NoError(t, err)

	t.Run("error when block hash does not match state update's block hash", func(t *testing.T) {
		stateUpdate := &core.StateUpdate{BlockHash: h1}
		expectedErr := ErrIncompatibleBlockAndStateUpdate{"block hashes do not match"}
		assert.Equal(t, chain.SanityCheckNewHeight(mainnetBlock1, stateUpdate), expectedErr)
	})

	t.Run("error when block global state root does not match state update's new root",
		func(t *testing.T) {
			stateUpdate := &core.StateUpdate{BlockHash: mainnetBlock1.Hash, NewRoot: h1}

			expectedErr := ErrIncompatibleBlockAndStateUpdate{
				"block's GlobalStateRoot does not match state update's NewRoot",
			}
			assert.Equal(t, chain.SanityCheckNewHeight(mainnetBlock1, stateUpdate), expectedErr)
		})
	t.Run("error if block hash has not being calculated properly", func(t *testing.T) {
		wrongHashBlock, wrongHashStateUpdate := new(core.Block), new(core.StateUpdate)

		*wrongHashBlock = *mainnetBlock1
		wrongHashBlock.Hash = h1

		*wrongHashStateUpdate = *mainnetStateUpdate1
		wrongHashStateUpdate.BlockHash = wrongHashBlock.Hash

		assert.EqualError(t, chain.SanityCheckNewHeight(wrongHashBlock, wrongHashStateUpdate), "can not verify hash in block header")
	})
}

func TestStore(t *testing.T) {
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()

	block0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	t.Run("add block to empty blockchain", func(t *testing.T) {
		chain := NewBlockchain(pebble.NewMemTest(), utils.MAINNET)
		require.NoError(t, chain.Store(block0, stateUpdate0))

		headBlock, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, block0, headBlock)

		txn := chain.database.NewTransaction(false)
		defer txn.Discard()

		root, err := core.NewState(txn).Root()
		assert.NoError(t, err)
		assert.Equal(t, stateUpdate0.NewRoot, root)

		got0Block, err := chain.GetBlockByNumber(0)
		assert.NoError(t, err)
		assert.Equal(t, got0Block, block0)
	})
	t.Run("add block to non-empty blockchain", func(t *testing.T) {
		block1, err := gw.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		stateUpdate1, err := gw.StateUpdate(context.Background(), 1)
		require.NoError(t, err)

		chain := NewBlockchain(pebble.NewMemTest(), utils.MAINNET)
		require.NoError(t, chain.Store(block0, stateUpdate0))
		require.NoError(t, chain.Store(block1, stateUpdate1))

		headBlock, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, block1, headBlock)

		txn := chain.database.NewTransaction(false)
		defer txn.Discard()

		root, err := core.NewState(txn).Root()
		assert.NoError(t, err)
		assert.Equal(t, stateUpdate1.NewRoot, root)

		got1Block, err := chain.GetBlockByNumber(1)
		assert.NoError(t, err)
		assert.Equal(t, got1Block, block1)
	})
}

func TestGetTransactionAndReceipt(t *testing.T) {
	chain := NewBlockchain(pebble.NewMemTest(), utils.MAINNET)

	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()

	for i := uint64(0); i < 3; i++ {
		b, err := gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err)

		su, err := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err)

		require.NoError(t, chain.Store(b, su))
	}

	t.Run("GetTransactionByBlockNumberAndIndex returns error if transaction does not exist", func(t *testing.T) {
		tx, err := chain.GetTransactionByBlockNumberAndIndex(32, 20)
		assert.Nil(t, tx)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})

	t.Run("GetTransactionByHash returns error if transaction does not exist", func(t *testing.T) {
		tx, err := chain.GetTransactionByHash(new(felt.Felt).SetUint64(345))
		assert.Nil(t, tx)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})

	t.Run("GetTransactionReceipt returns error if receipt does not exist", func(t *testing.T) {
		r, err := chain.GetReceipt(new(felt.Felt).SetUint64(234))
		assert.Nil(t, r)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})

	t.Run("GetTransactionByHash and GetGetTransactionByBlockNumberAndIndex return same transaction", func(t *testing.T) {
		for i := uint64(0); i < 3; i++ {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				block, err := gw.BlockByNumber(context.Background(), i)
				require.NoError(t, err)

				for j, expectedTx := range block.Transactions {
					gotTx, err := chain.GetTransactionByHash(expectedTx.Hash())
					require.NoError(t, err)
					assert.Equal(t, expectedTx, gotTx)

					gotTx, err = chain.GetTransactionByBlockNumberAndIndex(block.Number, uint64(j))
					require.NoError(t, err)
					assert.Equal(t, expectedTx, gotTx)
				}
			})
		}
	})

	t.Run("GetReceipt returns expected receipt", func(t *testing.T) {
		for i := uint64(0); i < 3; i++ {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				block, err := gw.BlockByNumber(context.Background(), i)
				require.NoError(t, err)

				for _, expectedR := range block.Receipts {
					gotR, err := chain.GetReceipt(expectedR.TransactionHash)
					require.NoError(t, err)
					assert.Equal(t, expectedR, gotR)

				}
			})
		}
	})
}
