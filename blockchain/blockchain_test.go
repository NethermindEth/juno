package blockchain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()
	t.Run("empty blockchain's head is nil", func(t *testing.T) {
		chain := blockchain.New(pebble.NewMemTest(), utils.MAINNET, log)
		assert.Equal(t, utils.MAINNET, chain.Network())
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
		chain := blockchain.New(testDB, utils.MAINNET, log)
		assert.NoError(t, chain.Store(block0, stateUpdate0, nil))

		chain = blockchain.New(testDB, utils.MAINNET, log)
		b, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, block0, b)
	})
}

func TestHeight(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()
	t.Run("return nil if blockchain is empty", func(t *testing.T) {
		chain := blockchain.New(pebble.NewMemTest(), utils.GOERLI, log)
		_, err := chain.Height()
		assert.Error(t, err)
	})
	t.Run("return height of the blockchain's head", func(t *testing.T) {
		block0, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)

		testDB := pebble.NewMemTest()
		chain := blockchain.New(testDB, utils.MAINNET, log)
		assert.NoError(t, chain.Store(block0, stateUpdate0, nil))

		chain = blockchain.New(testDB, utils.MAINNET, log)
		height, err := chain.Height()
		require.NoError(t, err)
		assert.Equal(t, block0.Number, height)
	})
}

func TestBlockByNumberAndHash(t *testing.T) {
	chain := blockchain.New(pebble.NewMemTest(), utils.GOERLI, utils.NewNopZapLogger())
	t.Run("same block is returned for both GetBlockByNumber and GetBlockByHash", func(t *testing.T) {
		client, closeFn := feeder.NewTestClient(utils.MAINNET)
		t.Cleanup(closeFn)
		gw := adaptfeeder.New(client)

		block, err := gw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)
		update, err := gw.StateUpdate(context.Background(), 0)
		require.NoError(t, err)

		require.NoError(t, chain.Store(block, update, nil))

		storedByNumber, err := chain.BlockByNumber(block.Number)
		require.NoError(t, err)
		assert.Equal(t, block, storedByNumber)

		storedByHash, err := chain.BlockByHash(block.Hash)
		require.NoError(t, err)
		assert.Equal(t, block, storedByHash)
	})
	t.Run("GetBlockByNumber returns error if block doesn't exist", func(t *testing.T) {
		_, err := chain.BlockByNumber(42)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("GetBlockByHash returns error if block doesn't exist", func(t *testing.T) {
		f, err := new(felt.Felt).SetRandom()
		require.NoError(t, err)
		_, err = chain.BlockByHash(f)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
}

func TestVerifyBlock(t *testing.T) {
	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	chain := blockchain.New(pebble.NewMemTest(), utils.MAINNET, utils.NewNopZapLogger())

	t.Run("error if chain is empty and incoming block number is not 0", func(t *testing.T) {
		block := &core.Block{Header: &core.Header{Number: 10}}
		assert.EqualError(t, chain.VerifyBlock(block), "cannot insert a block with number more than 0 in an empty blockchain")
	})

	t.Run("error if chain is empty and incoming block parent's hash is not 0", func(t *testing.T) {
		block := &core.Block{Header: &core.Header{ParentHash: h1}}
		assert.EqualError(t, chain.VerifyBlock(block), "cannot insert a block with non-zero parent hash in an empty blockchain")
	})

	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)

	mainnetBlock0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	mainnetStateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	t.Run("error if version is invalid", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = "notasemver"
		require.Error(t, chain.Store(mainnetBlock0, mainnetStateUpdate0, nil))
	})

	t.Run("error if version is unsupported", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = "99.0.0"
		semver.MustParse(mainnetBlock0.ProtocolVersion)
		require.Error(t, chain.Store(mainnetBlock0, mainnetStateUpdate0, nil))
	})

	t.Run("no error with no version string", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = ""
		require.NoError(t, chain.Store(mainnetBlock0, mainnetStateUpdate0, nil))
	})

	t.Run("error if difference between incoming block number and head is not 1",
		func(t *testing.T) {
			incomingBlock := &core.Block{Header: &core.Header{Number: 10}}
			assert.EqualError(t, chain.VerifyBlock(incomingBlock), "block number difference between head and incoming block is not 1")
		})

	t.Run("error when head hash does not match incoming block's parent hash", func(t *testing.T) {
		incomingBlock := &core.Block{Header: &core.Header{ParentHash: h1, Number: 1}}
		assert.EqualError(t, chain.VerifyBlock(incomingBlock), "block's parent hash does not match head block hash")
	})
}

func TestSanityCheckNewHeight(t *testing.T) {
	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	chain := blockchain.New(pebble.NewMemTest(), utils.MAINNET, utils.NewNopZapLogger())

	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)

	mainnetBlock0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	mainnetStateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	t.Run("builds the expected new classes map", func(t *testing.T) {
		onUnknownClass := func(classHash *felt.Felt) (core.Class, error) {
			return gw.Class(context.Background(), classHash)
		}
		newClasses, err := chain.SanityCheckNewHeight(mainnetBlock0, mainnetStateUpdate0, onUnknownClass)
		require.NoError(t, err)
		require.Equal(t, 1, len(newClasses))
		assert.NotNil(t, newClasses[*mainnetStateUpdate0.StateDiff.DeployedContracts[0].ClassHash])
	})

	require.NoError(t, chain.Store(mainnetBlock0, mainnetStateUpdate0, nil))

	t.Run("error when block hash does not match state update's block hash", func(t *testing.T) {
		mainnetBlock1, err := gw.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		stateUpdate := &core.StateUpdate{BlockHash: h1}
		_, err = chain.SanityCheckNewHeight(mainnetBlock1, stateUpdate, nil)
		assert.EqualError(t, err, "block hashes do not match")
	})

	t.Run("error when block global state root does not match state update's new root",
		func(t *testing.T) {
			mainnetBlock1, err := gw.BlockByNumber(context.Background(), 1)
			require.NoError(t, err)
			stateUpdate := &core.StateUpdate{BlockHash: mainnetBlock1.Hash, NewRoot: h1}

			_, err = chain.SanityCheckNewHeight(mainnetBlock1, stateUpdate, nil)
			assert.EqualError(t, err, "block's GlobalStateRoot does not match state update's NewRoot")
		})
}

func TestStore(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()

	block0, err := gw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(context.Background(), 0)
	require.NoError(t, err)

	t.Run("add block to empty blockchain", func(t *testing.T) {
		chain := blockchain.New(pebble.NewMemTest(), utils.MAINNET, log)
		require.NoError(t, chain.Store(block0, stateUpdate0, nil))

		headBlock, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, block0, headBlock)

		root, err := chain.StateCommitment()
		require.NoError(t, err)
		assert.Equal(t, stateUpdate0.NewRoot, root)

		got0Block, err := chain.BlockByNumber(0)
		require.NoError(t, err)
		assert.Equal(t, block0, got0Block)

		got0Update, err := chain.StateUpdateByHash(block0.Hash)
		require.NoError(t, err)
		assert.Equal(t, stateUpdate0, got0Update)
	})
	t.Run("add block to non-empty blockchain", func(t *testing.T) {
		block1, err := gw.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		stateUpdate1, err := gw.StateUpdate(context.Background(), 1)
		require.NoError(t, err)

		chain := blockchain.New(pebble.NewMemTest(), utils.MAINNET, log)
		require.NoError(t, chain.Store(block0, stateUpdate0, nil))
		require.NoError(t, chain.Store(block1, stateUpdate1, nil))

		headBlock, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, block1, headBlock)

		root, err := chain.StateCommitment()
		require.NoError(t, err)
		assert.Equal(t, stateUpdate1.NewRoot, root)

		got1Block, err := chain.BlockByNumber(1)
		require.NoError(t, err)
		assert.Equal(t, block1, got1Block)

		got1Update, err := chain.StateUpdateByNumber(1)
		require.NoError(t, err)
		assert.Equal(t, stateUpdate1, got1Update)
	})
}

func TestTransactionAndReceipt(t *testing.T) {
	chain := blockchain.New(pebble.NewMemTest(), utils.MAINNET, utils.NewNopZapLogger())

	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)
	gw := adaptfeeder.New(client)

	for i := uint64(0); i < 3; i++ {
		b, err := gw.BlockByNumber(context.Background(), i)
		require.NoError(t, err)

		su, err := gw.StateUpdate(context.Background(), i)
		require.NoError(t, err)

		require.NoError(t, chain.Store(b, su, nil))
	}

	t.Run("GetTransactionByBlockNumberAndIndex returns error if transaction does not exist", func(t *testing.T) {
		tx, err := chain.TransactionByBlockNumberAndIndex(32, 20)
		assert.Nil(t, tx)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})

	t.Run("GetTransactionByHash returns error if transaction does not exist", func(t *testing.T) {
		tx, err := chain.TransactionByHash(new(felt.Felt).SetUint64(345))
		assert.Nil(t, tx)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})

	t.Run("GetTransactionReceipt returns error if receipt does not exist", func(t *testing.T) {
		r, _, _, err := chain.Receipt(new(felt.Felt).SetUint64(234))
		assert.Nil(t, r)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})

	t.Run("GetTransactionByHash and GetGetTransactionByBlockNumberAndIndex return same transaction", func(t *testing.T) {
		for i := uint64(0); i < 3; i++ {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				block, err := gw.BlockByNumber(context.Background(), i)
				require.NoError(t, err)

				for j, expectedTx := range block.Transactions {
					gotTx, err := chain.TransactionByHash(expectedTx.Hash())
					require.NoError(t, err)
					assert.Equal(t, expectedTx, gotTx)

					gotTx, err = chain.TransactionByBlockNumberAndIndex(block.Number, uint64(j))
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
					gotR, hash, number, err := chain.Receipt(expectedR.TransactionHash)
					require.NoError(t, err)
					assert.Equal(t, expectedR, gotR)
					assert.Equal(t, block.Hash, hash)
					assert.Equal(t, block.Number, number)
				}
			})
		}
	})
}
