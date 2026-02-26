package rpcv9_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/statetestutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBlockIDMarshalling(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		blockIDJSON string
		checkFunc   func(*rpcv9.BlockID) bool
	}{
		"latest": {
			blockIDJSON: `"latest"`,
			checkFunc: func(blockID *rpcv9.BlockID) bool {
				return blockID.IsLatest()
			},
		},
		"pre_confirmed": {
			blockIDJSON: `"pre_confirmed"`,
			checkFunc: func(blockID *rpcv9.BlockID) bool {
				return blockID.IsPreConfirmed()
			},
		},
		"number": {
			blockIDJSON: `{ "block_number" : 123123 }`,
			checkFunc: func(blockID *rpcv9.BlockID) bool {
				return blockID.IsNumber() && blockID.Number() == 123123
			},
		},
		"hash": {
			blockIDJSON: `{ "block_hash" : "0x123" }`,
			checkFunc: func(blockID *rpcv9.BlockID) bool {
				return blockID.IsHash() && *blockID.Hash() == felt.FromUint64[felt.Felt](0x123)
			},
		},
		"l1_accepted": {
			blockIDJSON: `"l1_accepted"`,
			checkFunc: func(blockID *rpcv9.BlockID) bool {
				return blockID.IsL1Accepted()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockID rpcv9.BlockID
			require.NoError(t, blockID.UnmarshalJSON([]byte(test.blockIDJSON)))
			assert.True(t, test.checkFunc(&blockID))
		})
	}

	failingTests := map[string]struct {
		blockIDJSON string
	}{
		"unknown tag": {
			blockIDJSON: `"unknown tag"`,
		},
		"an empyt json object": {
			blockIDJSON: "{  }",
		},
		"a json list": {
			blockIDJSON: "[  ]",
		},
		"cannot parse number": {
			blockIDJSON: `{ "block_number" : asd }`,
		},
		"cannot parse hash": {
			blockIDJSON: `{ "block_hash" : asd }`,
		},
	}
	for name, test := range failingTests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockID rpcv9.BlockID
			assert.Error(t, blockID.UnmarshalJSON([]byte(test.blockIDJSON)))
		})
	}
}

func TestBlockTransactionCount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpcv9.New(mockReader, mockSyncReader, nil, log)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)

	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash
	expectedCount := latestBlock.TransactionCount

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)
		latest := blockIDLatest(t)
		count, rpcErr := handler.BlockTransactionCount(&latest)
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)
		hash := blockIDHash(t, felt.NewFromBytes[felt.Felt]([]byte("random")))
		count, rpcErr := handler.BlockTransactionCount(&hash)
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent pre_confirmed block", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)
		preConfirmed := blockIDPreConfirmed(t)
		count, rpcErr := handler.BlockTransactionCount(&preConfirmed)
		require.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		assert.Equal(t, uint64(0), count)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, db.ErrKeyNotFound)
		number := blockIDNumber(t, uint64(328476))
		count, rpcErr := handler.BlockTransactionCount(&number)
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		latest := blockIDLatest(t)
		count, rpcErr := handler.BlockTransactionCount(&latest)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)
		hash := blockIDHash(t, latestBlockHash)
		count, rpcErr := handler.BlockTransactionCount(&hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		number := blockIDNumber(t, latestBlockNumber)
		count, rpcErr := handler.BlockTransactionCount(&number)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		mockReader.EXPECT().L1Head().Return(
			core.L1Head{
				BlockNumber: latestBlock.Number,
				BlockHash:   latestBlock.Hash,
				StateRoot:   latestBlock.GlobalStateRoot,
			},
			nil,
		)
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		l1AcceptedID := blockIDL1Accepted(t)
		count, rpcErr := handler.BlockTransactionCount(&l1AcceptedID)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		preConfirmed := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)

		preConfirmedID := blockIDPreConfirmed(t)
		count, rpcErr := handler.BlockTransactionCount(&preConfirmedID)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})
}

func TestBlockWithTxHashes(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        blockIDLatest(t),
		"pre_confirmed": blockIDPreConfirmed(t),
		"hash":          blockIDHash(t, &felt.One),
		"number":        blockIDNumber(t, 2),
	}
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	var mockSyncReader *mocks.MockSyncReader
	mockReader := mocks.NewMockReader(mockCtrl)

	for description, id := range errTests { //nolint:dupl
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n, statetestutils.UseNewState())

			if description == "pre_confirmed" {
				mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
			}

			handler := rpcv9.New(chain, mockSyncReader, nil, log)

			block, rpcErr := handler.BlockWithTxHashes(&id)
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	n := &utils.Sepolia
	handler := rpcv9.New(mockReader, mockSyncReader, nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkBlock := func(t *testing.T, b *rpcv9.BlockWithTxHashes) {
		t.Helper()
		assert.Equal(t, latestBlock.Hash, b.Hash)
		assert.Equal(t, latestBlock.GlobalStateRoot, b.NewRoot)
		assert.Equal(t, latestBlock.ParentHash, b.ParentHash)
		assert.Equal(t, latestBlock.SequencerAddress, b.SequencerAddress)
		assert.Equal(t, latestBlock.Timestamp, b.Timestamp)
		assert.Equal(t, len(latestBlock.Transactions), len(b.TxnHashes))
		for i := range len(latestBlock.Transactions) {
			assert.Equal(t, latestBlock.Transactions[i].Hash(), b.TxnHashes[i])
		}
	}

	checkLatestBlock := func(t *testing.T, b *rpcv9.BlockWithTxHashes) {
		t.Helper()
		if latestBlock.Hash != nil {
			assert.Equal(t, latestBlock.Number, *b.Number)
		} else {
			assert.Equal(t, latestBlock.Number, *b.Number)
			assert.Equal(t, rpcv9.BlockPreConfirmed, b.Status)
		}
		checkBlock(t, b)
	}

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlock.Number).Return(
			latestBlock.Transactions, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

		latest := blockIDLatest(t)
		block, rpcErr := handler.BlockWithTxHashes(&latest)
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlock.Number).Return(
			latestBlock.Transactions, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

		hash := blockIDHash(t, latestBlockHash)
		block, rpcErr := handler.BlockWithTxHashes(&hash)
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlockNumber).Return(
			latestBlock.Transactions, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

		number := blockIDNumber(t, latestBlockNumber)
		block, rpcErr := handler.BlockWithTxHashes(&number)
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - number accepted on l1", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlockNumber).Return(
			latestBlock.Transactions, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{
			BlockNumber: latestBlockNumber,
			BlockHash:   latestBlockHash,
			StateRoot:   latestBlock.GlobalStateRoot,
		}, nil)

		number := blockIDNumber(t, latestBlockNumber)
		block, rpcErr := handler.BlockWithTxHashes(&number)
		require.Nil(t, rpcErr)

		assert.Equal(t, rpcv9.BlockAcceptedL1, block.Status)
		checkBlock(t, block)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlockNumber).Return(
			latestBlock.Transactions, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{
			BlockNumber: latestBlockNumber,
			BlockHash:   latestBlockHash,
			StateRoot:   latestBlock.GlobalStateRoot,
		}, nil).Times(2)

		l1AcceptedID := blockIDL1Accepted(t)
		block, rpcErr := handler.BlockWithTxHashes(&l1AcceptedID)
		require.Nil(t, rpcErr)

		assert.Equal(t, rpcv9.BlockAcceptedL1, block.Status)
		checkBlock(t, block)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		preConfirmed := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

		preConfirmedID := blockIDPreConfirmed(t)
		block, rpcErr := handler.BlockWithTxHashes(&preConfirmedID)
		require.Nil(t, rpcErr)
		checkLatestBlock(t, block)
	})
}

//nolint:dupl // similar to TestBlockWithTxs_TxnsFetchError
func TestBlockWithTxHashes_TxnsFetchError(t *testing.T) {
	blockNumber := uint64(123)
	header := &core.Header{Number: blockNumber}

	t.Run("TransactionsByBlockNumber returns ErrKeyNotFound", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv9.New(mockReader, nil, nil, nil)

		id := blockIDNumber(t, blockNumber)
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(&id)
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("TransactionsByBlockNumber returns internal error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv9.New(mockReader, nil, nil, nil)

		id := blockIDNumber(t, blockNumber)
		internalErr := errors.New("some internal error")
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, internalErr)

		block, rpcErr := handler.BlockWithTxHashes(&id)
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(internalErr), rpcErr)
	})
}

func TestBlockWithTxs(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        blockIDLatest(t),
		"pre_confirmed": blockIDPreConfirmed(t),
		"hash":          blockIDHash(t, &felt.One),
		"number":        blockIDNumber(t, 1),
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	var mockSyncReader *mocks.MockSyncReader
	mockReader := mocks.NewMockReader(mockCtrl)

	for description, id := range errTests { //nolint:dupl
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n, statetestutils.UseNewState())

			if description == "pre_confirmed" {
				mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
			}

			handler := rpcv9.New(chain, mockSyncReader, nil, log)

			block, rpcErr := handler.BlockWithTxs(&id)
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv9.New(mockReader, mockSyncReader, nil, nil)

	n := &utils.Mainnet
	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(16697)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkLatestBlock := func(t *testing.T, blockWithTxHashes *rpcv9.BlockWithTxHashes, blockWithTxs *rpcv9.BlockWithTxs) {
		t.Helper()
		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		for i, txnHash := range blockWithTxHashes.TxnHashes {
			txn, err := handler.TransactionByHash(txnHash)
			require.Nil(t, err)

			assert.Equal(t, txn, blockWithTxs.Transactions[i])
		}
	}

	latestBlockTxMap := make(map[felt.Felt]core.Transaction)
	for _, tx := range latestBlock.Transactions {
		latestBlockTxMap[*tx.Hash()] = tx
	}

	preConfirmed := &core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: latestBlockNumber + 1,
			},
		},
	}
	mockSyncReader.EXPECT().PendingData().Return(
		preConfirmed,
		nil,
	).Times(len(latestBlock.Transactions) * 5)

	mockReader.EXPECT().TransactionByHash(gomock.Any()).DoAndReturn(
		func(hash *felt.Felt) (core.Transaction, error) {
			if tx, found := latestBlockTxMap[*hash]; found {
				return tx, nil
			}
			return nil, errors.New("txn not found")
		},
	).Times(len(latestBlock.Transactions) * 5)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlock.Number).Return(
			latestBlock.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)

		latest := blockIDLatest(t)
		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&latest)
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&latest)
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlock.Number).Return(
			latestBlock.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)

		hash := blockIDHash(t, latestBlockHash)
		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&hash)
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&hash)
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(
			latestBlock.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlockNumber).Return(
			latestBlock.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)

		number := blockIDNumber(t, latestBlockNumber)
		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&number)
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&number)
		require.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - number accepted on l1", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(
			latestBlock.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlockNumber).Return(
			latestBlock.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{
			BlockNumber: latestBlockNumber,
			BlockHash:   latestBlockHash,
			StateRoot:   latestBlock.GlobalStateRoot,
		}, nil).Times(2)

		number := blockIDNumber(t, latestBlockNumber)
		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&number)
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&number)
		require.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(
			latestBlock.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlockNumber).Return(
			latestBlock.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{
			BlockNumber: latestBlockNumber,
			BlockHash:   latestBlockHash,
			StateRoot:   latestBlock.GlobalStateRoot,
		}, nil).Times(4)

		l1AcceptedID := blockIDL1Accepted(t)
		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&l1AcceptedID)
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&l1AcceptedID)
		require.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		preConfirmed := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		).Times(4 + len(latestBlock.Transactions))
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)

		preConfirmedID := blockIDPreConfirmed(t)
		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&preConfirmedID)
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&preConfirmedID)
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})
}

//nolint:dupl // similar to TestBlockWithTxHashes_TxnsFetchError
func TestBlockWithTxs_TxnsFetchError(t *testing.T) {
	blockNumber := uint64(123)
	header := &core.Header{Number: blockNumber}

	t.Run("TransactionsByBlockNumber returns ErrKeyNotFound", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv9.New(mockReader, nil, nil, nil)

		id := blockIDNumber(t, blockNumber)
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxs(&id)
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("TransactionsByBlockNumber returns internal error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv9.New(mockReader, nil, nil, nil)

		id := blockIDNumber(t, blockNumber)
		internalErr := errors.New("some internal error")
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, internalErr)

		block, rpcErr := handler.BlockWithTxs(&id)
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(internalErr), rpcErr)
	})
}

func TestBlockWithTxHashesV013(t *testing.T) {
	n := utils.HeapPtr(utils.SepoliaIntegration)
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpcv9.New(mockReader, nil, nil, nil)

	blockNumber := uint64(16350)
	gw := adaptfeeder.New(feeder.NewTestClient(t, n))
	coreBlock, err := gw.BlockByNumber(t.Context(), blockNumber)
	require.NoError(t, err)
	tx, ok := coreBlock.Transactions[0].(*core.InvokeTransaction)
	require.True(t, ok)

	mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(coreBlock.Header, nil)
	mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(coreBlock.Transactions, nil)
	mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil)

	blockID := blockIDNumber(t, blockNumber)
	got, rpcErr := handler.BlockWithTxs(&blockID)
	require.Nil(t, rpcErr)
	got.Transactions = got.Transactions[:1]

	require.Equal(t, &rpcv9.BlockWithTxs{
		BlockHeader: rpcv9.BlockHeader{
			Hash:            coreBlock.Hash,
			StarknetVersion: coreBlock.ProtocolVersion,
			NewRoot:         coreBlock.GlobalStateRoot,
			Number:          &coreBlock.Number,
			ParentHash:      coreBlock.ParentHash,
			L1DAMode:        utils.HeapPtr(rpcv6.Blob),
			L1GasPrice: &rpcv6.ResourcePrice{
				InFri: felt.NewUnsafeFromString[felt.Felt]("0x17882b6aa74"),
				InWei: felt.NewUnsafeFromString[felt.Felt]("0x3b9aca10"),
			},
			L1DataGasPrice: &rpcv6.ResourcePrice{
				InFri: felt.NewUnsafeFromString[felt.Felt]("0x2cc6d7f596e1"),
				InWei: felt.NewUnsafeFromString[felt.Felt]("0x716a8f6dd"),
			},
			SequencerAddress: coreBlock.SequencerAddress,
			Timestamp:        coreBlock.Timestamp,
			L2GasPrice: &rpcv6.ResourcePrice{
				InFri: &felt.One,
				InWei: &felt.One,
			},
		},
		Status: rpcv9.BlockAcceptedL2,
		Transactions: []*rpcv9.Transaction{
			{
				Hash:               tx.Hash(),
				Type:               rpcv9.TxnInvoke,
				Version:            tx.Version.AsFelt(),
				Nonce:              tx.Nonce,
				MaxFee:             tx.MaxFee,
				ContractAddress:    tx.ContractAddress,
				SenderAddress:      tx.SenderAddress,
				Signature:          &tx.TransactionSignature,
				CallData:           &tx.CallData,
				EntryPointSelector: tx.EntryPointSelector,
				ResourceBounds: &rpcv9.ResourceBoundsMap{
					L1Gas: &rpcv9.ResourceBounds{
						MaxAmount:       felt.NewFromUint64[felt.Felt](tx.ResourceBounds[core.ResourceL1Gas].MaxAmount),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL1Gas].MaxPricePerUnit,
					},
					L2Gas: &rpcv9.ResourceBounds{
						MaxAmount:       felt.NewFromUint64[felt.Felt](tx.ResourceBounds[core.ResourceL2Gas].MaxAmount),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL2Gas].MaxPricePerUnit,
					},
					L1DataGas: &rpcv9.ResourceBounds{
						MaxAmount:       &felt.Zero,
						MaxPricePerUnit: &felt.Zero,
					},
				},
				Tip:                   felt.NewFromUint64[felt.Felt](tx.Tip),
				PaymasterData:         &tx.PaymasterData,
				AccountDeploymentData: &tx.AccountDeploymentData,
				NonceDAMode:           utils.HeapPtr(rpcv9.DataAvailabilityMode(tx.NonceDAMode)),
				FeeDAMode:             utils.HeapPtr(rpcv9.DataAvailabilityMode(tx.FeeDAMode)),
			},
		},
	}, got)
}

func TestBlockWithReceipts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.HeapPtr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv9.New(mockReader, mockSyncReader, nil, nil)

	t.Run("block not found", func(t *testing.T) {
		blockID := blockIDNumber(t, 777)

		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(nil, db.ErrKeyNotFound)

		resp, rpcErr := handler.BlockWithReceipts(&blockID)
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})
	t.Run("l1head failure", func(t *testing.T) {
		blockID := blockIDNumber(t, 777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID)
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	t.Run("pre_confirmed block", func(t *testing.T) {
		block0, err := mainnetGw.BlockByNumber(t.Context(), 0)
		require.NoError(t, err)

		// pre_confirmed block does not have, hash, parent_hash, global_state_root
		block0.Hash = nil
		block0.ParentHash = nil
		block0.GlobalStateRoot = nil
		preConfirmed := core.NewPreConfirmed(block0, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil)

		blockID := blockIDPreConfirmed(t)
		resp, rpcErr := handler.BlockWithReceipts(&blockID)
		header := resp.BlockHeader

		txsWithReceipt := make([]rpcv9.TransactionWithReceipt, 0, len(block0.Transactions))
		for i, tx := range block0.Transactions {
			receipt := block0.Receipts[i]
			adaptedTx := rpcv9.AdaptTransaction(tx)
			adaptedTx.Hash = nil

			txsWithReceipt = append(txsWithReceipt, rpcv9.TransactionWithReceipt{
				Transaction: adaptedTx,
				Receipt: rpcv9.AdaptReceipt(
					receipt,
					tx,
					rpcv9.TxnPreConfirmed,
				),
			})
		}

		assert.Nil(t, rpcErr)
		assert.Equal(t, &rpcv9.BlockWithReceipts{
			Status: rpcv9.BlockPreConfirmed,
			BlockHeader: rpcv9.BlockHeader{
				Number:           header.Number,
				Timestamp:        header.Timestamp,
				SequencerAddress: header.SequencerAddress,
				L1GasPrice:       header.L1GasPrice,
				L1DataGasPrice:   header.L1DataGasPrice,
				L1DAMode:         header.L1DAMode,
				StarknetVersion:  header.StarknetVersion,
				L2GasPrice:       header.L2GasPrice,
			},
			Transactions: txsWithReceipt,
		}, resp)
	})

	t.Run("accepted L1 block", func(t *testing.T) {
		block1, err := mainnetGw.BlockByNumber(t.Context(), 1)
		require.NoError(t, err)

		blockID := blockIDNumber(t, block1.Number)

		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block1, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{
			BlockNumber: block1.Number + 1,
		}, nil)

		resp, rpcErr := handler.BlockWithReceipts(&blockID)
		header := resp.BlockHeader

		transactions := make([]rpcv9.TransactionWithReceipt, len(block1.Transactions))
		for i, tx := range block1.Transactions {
			receipt := block1.Receipts[i]
			adaptedTx := rpcv9.AdaptTransaction(tx)
			adaptedTx.Hash = nil

			transactions[i] = rpcv9.TransactionWithReceipt{
				Transaction: adaptedTx,
				Receipt: rpcv9.AdaptReceipt(
					receipt,
					tx,
					rpcv9.TxnAcceptedOnL1,
				),
			}
		}

		assert.Nil(t, rpcErr)
		assert.Equal(t, &rpcv9.BlockWithReceipts{
			Status: rpcv9.BlockAcceptedL1,
			BlockHeader: rpcv9.BlockHeader{
				Hash:             header.Hash,
				ParentHash:       header.ParentHash,
				Number:           header.Number,
				NewRoot:          header.NewRoot,
				Timestamp:        header.Timestamp,
				SequencerAddress: header.SequencerAddress,
				L1DAMode:         header.L1DAMode,
				L1GasPrice:       header.L1GasPrice,
				L1DataGasPrice:   header.L1DataGasPrice,
				StarknetVersion:  header.StarknetVersion,
				L2GasPrice:       header.L2GasPrice,
			},
			Transactions: transactions,
		}, resp)
	})
}

func TestRpcBlockAdaptation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpcv9.New(mockReader, nil, nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)
	latestBlockNumber := uint64(4850)

	t.Run("default sequencer address", func(t *testing.T) {
		latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)
		require.NoError(t, err)
		latestBlock.Header.SequencerAddress = nil
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(latestBlock.Number).Return(
			latestBlock.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)

		blockID := blockIDLatest(t)
		block, rpcErr := handler.BlockWithTxs(&blockID)
		require.NoError(t, err, rpcErr)
		require.Equal(t, &felt.Zero, block.BlockHeader.SequencerAddress)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&blockID)
		require.NoError(t, err, rpcErr)
		require.Equal(t, &felt.Zero, blockWithTxHashes.BlockHeader.SequencerAddress)
	})
}

func blockIDPreConfirmed(t *testing.T) rpcv9.BlockID {
	t.Helper()

	blockID := rpcv9.BlockID{}
	require.NoError(t, blockID.UnmarshalJSON([]byte(`"pre_confirmed"`)))
	return blockID
}

func blockIDLatest(t *testing.T) rpcv9.BlockID {
	t.Helper()

	blockID := rpcv9.BlockID{}
	require.NoError(t, blockID.UnmarshalJSON([]byte(`"latest"`)))
	return blockID
}

func blockIDHash(t *testing.T, val *felt.Felt) rpcv9.BlockID {
	t.Helper()

	blockID := rpcv9.BlockID{}
	require.NoError(
		t,
		blockID.UnmarshalJSON(
			[]byte(fmt.Sprintf(`{ "block_hash" : %q }`, val.String())),
		),
	)
	return blockID
}

func blockIDNumber(t *testing.T, val uint64) rpcv9.BlockID {
	t.Helper()

	blockID := rpcv9.BlockID{}
	require.NoError(t,
		blockID.UnmarshalJSON([]byte(fmt.Sprintf(`{ "block_number" : %d}`, val))),
	)
	return blockID
}

func blockIDL1Accepted(t *testing.T) rpcv9.BlockID {
	t.Helper()

	blockID := rpcv9.BlockID{}
	require.NoError(t, blockID.UnmarshalJSON([]byte(`"l1_accepted"`)))
	return blockID
}
