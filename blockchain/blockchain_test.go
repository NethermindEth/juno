package blockchain_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyCommitments = core.BlockCommitments{}

func TestNew(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	t.Run("empty blockchain's head is nil", func(t *testing.T) {
		chain := blockchain.New(memory.New(), &utils.Mainnet)
		assert.Equal(t, &utils.Mainnet, chain.Network())
		b, err := chain.Head()
		assert.Nil(t, b)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("non-empty blockchain gets head from db", func(t *testing.T) {
		block0, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)

		testDB := memory.New()
		chain := blockchain.New(testDB, &utils.Mainnet)
		assert.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))

		chain = blockchain.New(testDB, &utils.Mainnet)
		b, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, block0, b)
	})
}

func TestHeight(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	t.Run("return nil if blockchain is empty", func(t *testing.T) {
		chain := blockchain.New(memory.New(), &utils.Sepolia)
		_, err := chain.Height()
		assert.Error(t, err)
	})
	t.Run("return height of the blockchain's head", func(t *testing.T) {
		block0, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)

		stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)

		testDB := memory.New()
		chain := blockchain.New(testDB, &utils.Mainnet)
		assert.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))

		chain = blockchain.New(testDB, &utils.Mainnet)
		height, err := chain.Height()
		require.NoError(t, err)
		assert.Equal(t, block0.Number, height)
	})
}

func TestBlockByNumberAndHash(t *testing.T) {
	chain := blockchain.New(memory.New(), &utils.Sepolia)
	t.Run("same block is returned for both core.GetBlockByNumber and GetBlockByHash", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)
		gw := adaptfeeder.New(client)

		block, err := gw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)
		update, err := gw.StateUpdate(t.Context(), 0)
		require.NoError(t, err)

		require.NoError(t, chain.Store(block, &emptyCommitments, update, nil))

		storedByNumber, err := chain.BlockByNumber(block.Number)
		require.NoError(t, err)
		assert.Equal(t, block, storedByNumber)

		storedByHash, err := chain.BlockByHash(block.Hash)
		require.NoError(t, err)
		assert.Equal(t, block, storedByHash)
	})
	t.Run("core.GetBlockByNumber returns error if block doesn't exist", func(t *testing.T) {
		_, err := chain.BlockByNumber(42)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("GetBlockByHash returns error if block doesn't exist", func(t *testing.T) {
		f := felt.NewRandom[felt.Felt]()
		_, err := chain.BlockByHash(f)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
}

func TestVerifyBlock(t *testing.T) {
	h1 := felt.NewRandom[felt.Felt]()

	chain := blockchain.New(memory.New(), &utils.Mainnet)

	t.Run("error if chain is empty and incoming block number is not 0", func(t *testing.T) {
		block := &core.Block{Header: &core.Header{Number: 10}}
		assert.EqualError(t, chain.VerifyBlock(block), "expected block #0, got block #10")
	})

	t.Run("error if chain is empty and incoming block parent's hash is not 0", func(t *testing.T) {
		block := &core.Block{Header: &core.Header{ParentHash: h1}}
		assert.EqualError(
			t, chain.VerifyBlock(block), "block's parent hash does not match head block hash",
		)
	})

	client := feeder.NewTestClient(t, &utils.Mainnet)

	gw := adaptfeeder.New(client)
	mainnetBlock0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	mainnetStateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	t.Run("error if version is invalid", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = "notasemver"
		require.Error(t, chain.Store(mainnetBlock0, &emptyCommitments, mainnetStateUpdate0, nil))
	})

	t.Run("needs padding", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = "99.0" // should be padded to "99.0.0"
		require.ErrorContains(
			t,
			chain.Store(mainnetBlock0, &emptyCommitments, mainnetStateUpdate0, nil),
			"unsupported block version",
		)
	})

	t.Run("needs truncating", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = "99.0.0.0" // last 0 digit should be ignored
		require.ErrorContains(
			t,
			chain.Store(mainnetBlock0, &emptyCommitments, mainnetStateUpdate0, nil),
			"unsupported block version",
		)
	})

	t.Run("greater than supportedStarknetVersion", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = "99.0.0"
		require.ErrorContains(
			t,
			chain.Store(mainnetBlock0, &emptyCommitments, mainnetStateUpdate0, nil),
			"unsupported block version",
		)
	})

	t.Run("mismatch at patch version is ignored", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = core.LatestVer.IncPatch().String()
		assert.NoError(t, chain.VerifyBlock(mainnetBlock0))
	})

	t.Run("error if mismatch at minor version", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = core.LatestVer.IncMinor().String()
		assert.ErrorContains(t, chain.VerifyBlock(mainnetBlock0), "unsupported block version")
	})

	t.Run("error if mismatch at minor version", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = core.LatestVer.IncMajor().String()
		assert.ErrorContains(t, chain.VerifyBlock(mainnetBlock0), "unsupported block version")
	})

	t.Run("no error with no version string", func(t *testing.T) {
		mainnetBlock0.ProtocolVersion = ""
		require.NoError(t, chain.Store(mainnetBlock0, &emptyCommitments, mainnetStateUpdate0, nil))
	})

	t.Run("error if difference between incoming block number and head is not 1",
		func(t *testing.T) {
			incomingBlock := &core.Block{Header: &core.Header{Number: 10}}
			assert.EqualError(t, chain.VerifyBlock(incomingBlock), "expected block #1, got block #10")
		})

	t.Run("error when head hash does not match incoming block's parent hash", func(t *testing.T) {
		incomingBlock := &core.Block{Header: &core.Header{ParentHash: h1, Number: 1}}
		assert.EqualError(t, chain.VerifyBlock(incomingBlock), "block's parent hash does not match head block hash")
	})
}

func TestSanityCheckNewHeight(t *testing.T) {
	h1 := felt.NewRandom[felt.Felt]()

	chain := blockchain.New(memory.New(), &utils.Mainnet)

	client := feeder.NewTestClient(t, &utils.Mainnet)

	gw := adaptfeeder.New(client)

	mainnetBlock0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	mainnetStateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	require.NoError(t, chain.Store(mainnetBlock0, &emptyCommitments, mainnetStateUpdate0, nil))

	t.Run("error when block hash does not match state update's block hash", func(t *testing.T) {
		mainnetBlock1, err := gw.BlockByNumber(t.Context(), 1)
		require.NoError(t, err)

		stateUpdate := &core.StateUpdate{BlockHash: h1}
		_, err = chain.SanityCheckNewHeight(mainnetBlock1, stateUpdate, nil)
		assert.EqualError(t, err, "block hashes do not match")
	})

	t.Run("error when block global state root does not match state update's new root",
		func(t *testing.T) {
			mainnetBlock1, err := gw.BlockByNumber(t.Context(), 1)
			require.NoError(t, err)
			stateUpdate := &core.StateUpdate{BlockHash: mainnetBlock1.Hash, NewRoot: h1}

			_, err = chain.SanityCheckNewHeight(mainnetBlock1, stateUpdate, nil)
			assert.EqualError(t, err, "block's GlobalStateRoot does not match state update's NewRoot")
		})
}

func TestStore(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	block0, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	stateUpdate0, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	t.Run("add block to empty blockchain", func(t *testing.T) {
		chain := blockchain.New(memory.New(), &utils.Mainnet)
		require.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))

		headBlock, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, block0, headBlock)

		root, err := chain.StateCommitment()
		require.NoError(t, err)
		assert.Equal(t, stateUpdate0.NewRoot, &root)

		got0Block, err := chain.BlockByNumber(0)
		require.NoError(t, err)
		assert.Equal(t, block0, got0Block)

		got0Update, err := chain.StateUpdateByHash(block0.Hash)
		require.NoError(t, err)
		assert.Equal(t, stateUpdate0, got0Update)
	})

	t.Run("add block to non-empty blockchain", func(t *testing.T) {
		block1, err := gw.BlockByNumber(t.Context(), 1)
		require.NoError(t, err)

		stateUpdate1, err := gw.StateUpdate(t.Context(), 1)
		require.NoError(t, err)

		chain := blockchain.New(memory.New(), &utils.Mainnet)
		require.NoError(t, chain.Store(block0, &emptyCommitments, stateUpdate0, nil))
		require.NoError(t, chain.Store(block1, &emptyCommitments, stateUpdate1, nil))

		headBlock, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, block1, headBlock)

		root, err := chain.StateCommitment()
		require.NoError(t, err)
		assert.Equal(t, stateUpdate1.NewRoot, &root)

		got1Block, err := chain.BlockByNumber(1)
		require.NoError(t, err)
		assert.Equal(t, block1, got1Block)

		got1Update, err := chain.StateUpdateByNumber(1)
		require.NoError(t, err)
		assert.Equal(t, stateUpdate1, got1Update)
	})
}

func TestStoreL1HandlerTxnHash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)
	chain := blockchain.New(memory.New(), &utils.Sepolia)
	var stateUpdate *core.StateUpdate
	for i := range uint64(7) {
		block, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)
		stateUpdate, err = gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)
		require.NoError(t, chain.Store(block, &emptyCommitments, stateUpdate, nil))
	}
	l1HandlerMsgHash := common.HexToHash("0x42e76df4e3d5255262929c27132bd0d295a8d3db2cfe63d2fcd061c7a7a7ab34")
	l1HandlerTxnHash, err := chain.L1HandlerTxnHash(&l1HandlerMsgHash)
	require.NoError(t, err)
	expectedHash := felt.UnsafeFromString[felt.Felt](
		"0x785c2ada3f53fbc66078d47715c27718f92e6e48b96372b36e5197de69b82b5",
	)
	require.Equal(t, expectedHash, l1HandlerTxnHash)
}

func TestBlockCommitments(t *testing.T) {
	chain := blockchain.New(memory.New(), &utils.Mainnet)
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	b, err := gw.BlockByNumber(t.Context(), 0)
	require.NoError(t, err)

	su, err := gw.StateUpdate(t.Context(), 0)
	require.NoError(t, err)

	expectedCommitments := &core.BlockCommitments{
		TransactionCommitment: new(felt.Felt).SetUint64(1),
		EventCommitment:       new(felt.Felt).SetUint64(2),
		ReceiptCommitment:     new(felt.Felt).SetUint64(3),
		StateDiffCommitment:   new(felt.Felt).SetUint64(4),
	}

	require.NoError(t, chain.Store(b, expectedCommitments, su, nil))

	commitments, err := chain.BlockCommitmentsByNumber(0)
	require.NoError(t, err)
	require.Equal(t, expectedCommitments, commitments)
}

func TestTransactionAndReceipt(t *testing.T) {
	chain := blockchain.New(memory.New(), &utils.Mainnet)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	for i := range uint64(3) {
		b, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)

		su, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)

		require.NoError(t, chain.Store(b, &core.BlockCommitments{
			TransactionCommitment: new(felt.Felt).SetUint64(i),
			EventCommitment:       new(felt.Felt).SetUint64(2 * i),
		}, su, nil))
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
		for i := range uint64(3) {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				block, err := gw.BlockByNumber(t.Context(), i)
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

	t.Run("TransactionsByBlockNumber returns empty for non-existent block", func(t *testing.T) {
		txns, err := chain.TransactionsByBlockNumber(32)
		require.NoError(t, err)
		assert.Empty(t, txns)
	})

	t.Run("TransactionsByBlockNumber returns all transactions for a block", func(t *testing.T) {
		for i := range uint64(3) {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				block, err := gw.BlockByNumber(t.Context(), i)
				require.NoError(t, err)

				txns, err := chain.TransactionsByBlockNumber(i)
				require.NoError(t, err)
				assert.Equal(t, block.Transactions, txns)
			})
		}
	})

	t.Run("GetReceipt returns expected receipt", func(t *testing.T) {
		for i := range uint64(3) {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				block, err := gw.BlockByNumber(t.Context(), i)
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

	t.Run("BlockCommitments returns expected values", func(t *testing.T) {
		for i := range uint64(3) {
			t.Run(fmt.Sprintf("mainnet block %v", i), func(t *testing.T) {
				commitments, err := chain.BlockCommitmentsByNumber(i)
				require.NoError(t, err)
				require.Equal(t, &core.BlockCommitments{
					TransactionCommitment: new(felt.Felt).SetUint64(i),
					EventCommitment:       new(felt.Felt).SetUint64(2 * i),
				}, commitments)
			})
		}
	})
}

func TestState(t *testing.T) {
	testDB := memory.New()
	chain := blockchain.New(testDB, &utils.Mainnet)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	t.Run("head with no blocks", func(t *testing.T) {
		_, _, err := chain.HeadState()
		require.Error(t, err)
	})

	var existingBlockHash *felt.Felt
	for i := range uint64(2) {
		block, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)
		su, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)

		require.NoError(t, chain.Store(block, &emptyCommitments, su, nil))
		existingBlockHash = block.Hash
	}

	t.Run("head with blocks", func(t *testing.T) {
		_, closer, err := chain.HeadState()
		require.NoError(t, err)
		require.NoError(t, closer())
	})

	t.Run("existing height", func(t *testing.T) {
		_, closer, err := chain.StateAtBlockNumber(1)
		require.NoError(t, err)
		require.NoError(t, closer())
	})

	t.Run("non-existent height", func(t *testing.T) {
		_, _, err := chain.StateAtBlockNumber(10)
		require.Error(t, err)
	})

	t.Run("existing hash", func(t *testing.T) {
		_, closer, err := chain.StateAtBlockHash(existingBlockHash)
		require.NoError(t, err)
		require.NoError(t, closer())
	})

	t.Run("non-existent hash", func(t *testing.T) {
		hash := felt.NewRandom[felt.Felt]()
		_, _, err := chain.StateAtBlockHash(hash)
		require.Error(t, err)
	})

	t.Run("zero hash", func(t *testing.T) {
		hash := new(felt.Felt)
		require.True(t, hash.IsZero())

		state, closer, err := chain.StateAtBlockHash(hash)
		require.NoError(t, err)
		require.NotNil(t, state)
		require.NoError(t, closer())
	})
}

func TestEvents(t *testing.T) {
	var pendingB *core.Block
	pendingDataFunc := func() (core.PendingData, error) { //nolint:unparam // used in tests
		preConfirmed := core.NewPreConfirmed(pendingB, nil, nil, nil)
		return &preConfirmed, nil
	}

	testDB := memory.New()
	chain := blockchain.New(testDB, &utils.Goerli2)

	client := feeder.NewTestClient(t, &utils.Goerli2)
	gw := adaptfeeder.New(client)

	const (
		numBlocksToFetch     = 7
		firstPendingBlockNum = 6
	)

	for i := range numBlocksToFetch {
		b, err := gw.BlockByNumber(t.Context(), uint64(i))
		require.NoError(t, err)
		s, err := gw.StateUpdate(t.Context(), uint64(i))
		require.NoError(t, err)

		if b.Number < firstPendingBlockNum {
			require.NoError(t, chain.Store(b, &emptyCommitments, s, nil))
		} else {
			pendingB = b
		}
	}

	t.Run("filter non-existent", func(t *testing.T) {
		filter, err := chain.EventFilter(nil, nil, pendingDataFunc)

		t.Run("block number", func(t *testing.T) {
			err = filter.SetRangeEndBlockByNumber(blockchain.EventFilterTo, uint64(44))
			require.NoError(t, err)
			err = filter.SetRangeEndBlockByNumber(blockchain.EventFilterFrom, uint64(44))
			require.NoError(t, err)
		})

		t.Run("block hash", func(t *testing.T) {
			err = filter.SetRangeEndBlockByHash(blockchain.EventFilterTo, &felt.Zero)
			require.Error(t, err)
			err = filter.SetRangeEndBlockByHash(blockchain.EventFilterFrom, &felt.Zero)
			require.Error(t, err)
		})

		require.NoError(t, filter.Close())
	})

	from := []felt.Address{
		felt.UnsafeFromString[felt.Address](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		),
	}
	t.Run("filter with no keys", func(t *testing.T) {
		filter, err := chain.EventFilter(from, nil, pendingDataFunc)
		require.NoError(t, err)

		require.NoError(t, filter.SetRangeEndBlockByNumber(blockchain.EventFilterFrom, 0))
		require.NoError(t, filter.SetRangeEndBlockByNumber(
			blockchain.EventFilterTo,
			firstPendingBlockNum,
		))

		allEvents := []blockchain.FilteredEvent{}
		t.Run("get all events without pagination", func(t *testing.T) {
			events, cToken, eErr := filter.Events(nil, 10)
			require.Empty(t, cToken)
			require.NoError(t, eErr)
			require.Len(t, events, 3)
			for _, event := range events {
				require.NotNil(t, event.From)
				assert.Contains(
					t,
					from,
					felt.Address(*event.From),
					"event.From should be in the from addresses list",
				)
			}

			allEvents = events
		})

		t.Run("accumulate events with pagination", func(t *testing.T) {
			for _, chunkSize := range []uint64{1, 2} {
				var accEvents []blockchain.FilteredEvent
				var lastToken blockchain.ContinuationToken
				var lastTokenPtr *blockchain.ContinuationToken
				var gotEvents []blockchain.FilteredEvent
				for range len(allEvents) + 1 {
					if !lastToken.IsEmpty() {
						lastTokenPtr = &lastToken
					}
					gotEvents, lastToken, err = filter.Events(lastTokenPtr, chunkSize)
					require.NoError(t, err)
					accEvents = append(accEvents, gotEvents...)
					if lastToken.IsEmpty() {
						break
					}
				}
				assert.Equal(t, allEvents, accEvents)
			}
		})

		require.NoError(t, filter.Close())
	})

	t.Run("filter with keys", func(t *testing.T) {
		key := felt.NewUnsafeFromString[felt.Felt](
			"0x3774b0545aabb37c45c1eddc6a7dae57de498aae6d5e3589e362d4b4323a533",
		)
		filter, err := chain.EventFilter(from, [][]felt.Felt{{*key}}, pendingDataFunc)
		require.NoError(t, err)

		require.NoError(t, filter.SetRangeEndBlockByHash(blockchain.EventFilterFrom,
			felt.NewUnsafeFromString[felt.Felt](
				"0x3b43b334f46b921938854ba85ffc890c1b1321f8fd69e7b2961b18b4260de14",
			)))
		require.NoError(t, filter.SetRangeEndBlockByHash(blockchain.EventFilterTo,
			felt.NewUnsafeFromString[felt.Felt](
				"0x3b43b334f46b921938854ba85ffc890c1b1321f8fd69e7b2961b18b4260de14",
			)))

		events, cToken, err := filter.Events(nil, 10)
		require.Empty(t, cToken)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.NoError(t, filter.Close())
	})

	t.Run("filter with not matching keys", func(t *testing.T) {
		filter, err := chain.EventFilter(
			from,
			[][]felt.Felt{
				{*felt.NewUnsafeFromString[felt.Felt](
					"0x3774b0545aabb37c45c1eddc6a7dae57de498aae6d5e3589e362d4b4323a533",
				)},
				{*felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF")},
			},
			pendingDataFunc,
		)
		require.NoError(t, err)
		require.NoError(t, filter.SetRangeEndBlockByNumber(blockchain.EventFilterFrom, 0))
		require.NoError(t, filter.SetRangeEndBlockByNumber(
			blockchain.EventFilterTo,
			firstPendingBlockNum,
		))
		events, cToken, err := filter.Events(nil, 10)
		require.NoError(t, err)
		require.True(t, cToken.IsEmpty())
		require.Empty(t, events)
		require.NoError(t, filter.Close())
	})

	t.Run("filter with duplicate addresses", func(t *testing.T) {
		address1 := felt.UnsafeFromString[felt.Address](
			"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7",
		)

		addresses := []felt.Address{address1, address1, address1}
		filter, err := chain.EventFilter(addresses, nil, pendingDataFunc)
		require.NoError(t, err)

		require.NoError(t, filter.SetRangeEndBlockByNumber(blockchain.EventFilterFrom, 0))
		require.NoError(t, filter.SetRangeEndBlockByNumber(
			blockchain.EventFilterTo,
			firstPendingBlockNum,
		))

		events, cToken, err := filter.Events(nil, 10)
		require.NoError(t, err)
		require.Empty(t, cToken)
		require.Len(t, events, 3)

		for _, event := range events {
			require.NotNil(t, event.From)
			assert.Equal(t, address1, felt.Address(*event.From))
		}

		require.NoError(t, filter.Close())
	})

	t.Run("filter with no addresses", func(t *testing.T) {
		testCases := []struct {
			addresses []felt.Address
			name      string
		}{
			{nil, "nil"},
			{[]felt.Address{}, "empty slice"},
		}
		for _, testCase := range testCases {
			t.Run("filter with no addresses ("+testCase.name+")", func(t *testing.T) {
				filter, err := chain.EventFilter(testCase.addresses, nil, pendingDataFunc)
				require.NoError(t, err)

				require.NoError(t, filter.SetRangeEndBlockByNumber(blockchain.EventFilterFrom, 0))
				require.NoError(t, filter.SetRangeEndBlockByNumber(
					blockchain.EventFilterTo,
					firstPendingBlockNum,
				))

				events, cToken, err := filter.Events(nil, 10)
				require.NoError(t, err)
				require.Empty(t, cToken)
				require.GreaterOrEqual(t, len(events), 3)

				require.NoError(t, filter.Close())
			})
		}
	})
}

func TestRevert(t *testing.T) {
	testDB := memory.New()
	chain := blockchain.New(testDB, &utils.Mainnet)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	for i := range uint64(3) {
		b, err := gw.BlockByNumber(t.Context(), i)
		require.NoError(t, err)

		su, err := gw.StateUpdate(t.Context(), i)
		require.NoError(t, err)

		require.NoError(t, chain.Store(b, &emptyCommitments, su, nil))
	}

	require.NoError(t, chain.RevertHead())

	t.Run("height should rollback", func(t *testing.T) {
		height, err := chain.Height()
		require.NoError(t, err)
		assert.Equal(t, uint64(1), height)
	})
	t.Run("head should revert", func(t *testing.T) {
		block, err := chain.Head()
		require.NoError(t, err)
		assert.Equal(t, uint64(1), block.Number)
	})
	t.Run("headsheader should revert", func(t *testing.T) {
		header, err := chain.HeadsHeader()
		require.NoError(t, err)
		assert.Equal(t, uint64(1), header.Number)
	})

	revertedHeight := uint64(2)
	t.Run("BlockByNumber should fail with reverted height", func(t *testing.T) {
		_, err := chain.BlockByNumber(revertedHeight)
		require.Error(t, err)
	})
	t.Run("StateUpdateByNumber should fail with reverted height", func(t *testing.T) {
		_, err := chain.StateUpdateByNumber(revertedHeight)
		require.Error(t, err)
	})
	t.Run("BlockHeaderByNumber should fail with reverted height", func(t *testing.T) {
		_, err := chain.BlockHeaderByNumber(revertedHeight)
		require.Error(t, err)
	})
	t.Run("TransactionByBlockNumberAndIndex should fail with reverted height", func(t *testing.T) {
		_, err := chain.TransactionByBlockNumberAndIndex(revertedHeight, 0)
		require.Error(t, err)
	})
	t.Run("TransactionsByBlockNumber should return empty for reverted height", func(t *testing.T) {
		txns, err := chain.TransactionsByBlockNumber(revertedHeight)
		require.NoError(t, err)
		assert.Empty(t, txns)
	})

	require.NoError(t, chain.RevertHead())
	require.NoError(t, chain.RevertHead())

	t.Run("empty blockchain should mean empty db", func(t *testing.T) {
		it, err := testDB.NewIterator(nil, false)
		require.NoError(t, err)
		assert.False(t, it.Next(), it.Key())
		require.NoError(t, it.Close())
	})

	t.Run("cannot revert on empty chain", func(t *testing.T) {
		require.Error(t, chain.RevertHead())
	})
}

// TestRevertHeadMigratedCasmClasses ensures that after storing a block with
// MigratedClasses and then reverting, the classes trie and CASM hash metadata
// are correctly reverted to V1.
func TestRevertHeadMigratedCasmClasses(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)

	sierraClassHashFelt := felt.NewUnsafeFromString[felt.Felt](
		"0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc",
	)
	classDef, err := gw.Class(t.Context(), sierraClassHashFelt)
	require.NoError(t, err)
	sierraClass, ok := classDef.(*core.SierraClass)
	require.True(t, ok, "class must be SierraClass")
	require.NotNil(t, sierraClass.Compiled, "class must have Compiled set")

	sierraHash := felt.SierraClassHash(*sierraClassHashFelt)
	v1CasmHash := felt.CasmClassHash(sierraClass.Compiled.Hash(core.HashVersionV1))
	v2CasmHash := felt.CasmClassHash(sierraClass.Compiled.Hash(core.HashVersionV2))

	newClasses := map[felt.Felt]core.ClassDefinition{
		*sierraClassHashFelt: sierraClass,
	}

	testDB := memory.New()
	chain := blockchain.New(testDB, &utils.Integration)

	receipts0 := make([]*core.TransactionReceipt, 0)
	block0 := &core.Block{
		Header: &core.Header{
			ParentHash:       &felt.Zero,
			Number:           0,
			SequencerAddress: &felt.Zero,
			EventsBloom:      core.EventsBloom(receipts0),
			L1GasPriceETH:    &felt.Zero,
			L1GasPriceSTRK:   &felt.Zero,
			L1DataGasPrice:   &core.GasPrice{PriceInFri: &felt.Zero, PriceInWei: &felt.Zero},
			L2GasPrice:       &core.GasPrice{PriceInFri: &felt.Zero, PriceInWei: &felt.Zero},
			L1DAMode:         core.Calldata,
			// V1 CASM hash is used for classes declared before protocol version 0.14.1
			ProtocolVersion: core.Ver0_14_0.String(),
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts0,
	}

	stateUpdate0 := &core.StateUpdate{
		OldRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{
				*sierraClassHashFelt: (*felt.Felt)(&v1CasmHash),
			},
		},
	}
	require.NoError(t, chain.Finalise(block0, stateUpdate0, newClasses, nil))

	receipts1 := make([]*core.TransactionReceipt, 0)
	block1 := &core.Block{
		Header: &core.Header{
			ParentHash:       block0.Hash,
			Number:           1,
			SequencerAddress: &felt.Zero,
			EventsBloom:      core.EventsBloom(receipts1),
			L1GasPriceETH:    &felt.Zero,
			L1GasPriceSTRK:   &felt.Zero,
			L1DataGasPrice:   &core.GasPrice{PriceInFri: &felt.Zero, PriceInWei: &felt.Zero},
			L2GasPrice:       &core.GasPrice{PriceInFri: &felt.Zero, PriceInWei: &felt.Zero},
			L1DAMode:         core.Calldata,
			// V2 CASM hash is used for classes declared from protocol version 0.14.1 onwards
			ProtocolVersion: core.Ver0_14_1.String(),
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts1,
	}

	stateUpdate1 := &core.StateUpdate{
		StateDiff: &core.StateDiff{
			MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{
				sierraHash: v2CasmHash,
			},
		},
	}
	require.NoError(t, chain.Finalise(block1, stateUpdate1, nil, nil))

	// Revert head should revert the state to casm hash v1
	require.NoError(t, chain.RevertHead())

	state, closer, err := chain.HeadState()
	require.NoError(t, err)
	defer func() { _ = closer() }()

	gotCasmHash, err := state.CompiledClassHash(&sierraHash)
	require.NoError(t, err)
	assert.Equal(t, v1CasmHash, gotCasmHash, "should return V1 after reverting migrated class")

	gotRoot, err := chain.StateCommitment()
	require.NoError(t, err)
	assert.Equal(t, stateUpdate0.NewRoot, &gotRoot, "state root after revert should match block 0")
}

func TestL1Update(t *testing.T) {
	heads := []*core.L1Head{
		{
			BlockNumber: 1,
			StateRoot:   new(felt.Felt).SetUint64(2),
		},
		{
			BlockNumber: 2,
			StateRoot:   new(felt.Felt).SetUint64(3),
		},
	}

	for _, head := range heads {
		t.Run(fmt.Sprintf("update L1 head to block %d", head.BlockNumber), func(t *testing.T) {
			chain := blockchain.New(memory.New(), &utils.Mainnet)
			require.NoError(t, chain.SetL1Head(head))
			got, err := chain.L1Head()
			require.NoError(t, err)
			assert.Equal(t, head, &got)
		})
	}
}

func TestSubscribeL1Head(t *testing.T) {
	l1Head := &core.L1Head{
		BlockNumber: 1,
		StateRoot:   new(felt.Felt).SetUint64(2),
	}

	chain := blockchain.New(memory.New(), &utils.Mainnet)
	sub := chain.SubscribeL1Head()
	t.Cleanup(sub.Unsubscribe)

	require.NoError(t, chain.SetL1Head(l1Head))

	got, ok := <-sub.Recv()
	require.True(t, ok)
	assert.Equal(t, l1Head, got)
}
