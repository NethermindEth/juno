package rpcv7_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBlockId(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		blockIDJSON     string
		expectedBlockID rpcv7.BlockID
	}{
		"latest": {
			blockIDJSON: `"latest"`,
			expectedBlockID: rpcv7.BlockID{
				Latest: true,
			},
		},
		"pending": {
			blockIDJSON: `"pending"`,
			expectedBlockID: rpcv7.BlockID{
				Pending: true,
			},
		},
		"number": {
			blockIDJSON: `{ "block_number" : 123123 }`,
			expectedBlockID: rpcv7.BlockID{
				Number: 123123,
			},
		},
		"hash": {
			blockIDJSON: `{ "block_hash" : "0x123" }`,
			expectedBlockID: rpcv7.BlockID{
				Hash: new(felt.Felt).SetUint64(0x123),
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockID rpcv7.BlockID
			require.NoError(t, blockID.UnmarshalJSON([]byte(test.blockIDJSON)))
			assert.Equal(t, test.expectedBlockID, blockID)
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
			var blockID rpcv7.BlockID
			assert.Error(t, blockID.UnmarshalJSON([]byte(test.blockIDJSON)))
		})
	}
}

func TestBlockTransactionCount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv7.New(mockReader, mockSyncReader, nil, "", n, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash
	expectedCount := latestBlock.TransactionCount

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Latest: true})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Number: uint64(328476)})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockSyncReader.EXPECT().Pending().Return(&sync.Pending{
			Block: latestBlock,
		}, nil)

		count, rpcErr := handler.BlockTransactionCount(rpcv7.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})
}

func TestBlockWithTxHashes(t *testing.T) {
	errTests := map[string]rpcv7.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	var mockSyncReader *mocks.MockSyncReader
	mockReader := mocks.NewMockReader(mockCtrl)

	for description, id := range errTests { //nolint:dupl
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := utils.Ptr(utils.Mainnet)
			chain := blockchain.New(pebble.NewMemTest(t), n, nil)

			if description == "pending" { //nolint:goconst
				mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().Pending().Return(nil, sync.ErrPendingBlockNotFound)
			}

			handler := rpcv7.New(chain, mockSyncReader, nil, "", n, log)

			block, rpcErr := handler.BlockWithTxHashes(id)
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	n := utils.Ptr(utils.Sepolia)
	handler := rpcv7.New(mockReader, mockSyncReader, nil, "", n, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkBlock := func(t *testing.T, b *rpcv7.BlockWithTxHashes) {
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

	checkLatestBlock := func(t *testing.T, b *rpcv7.BlockWithTxHashes) {
		t.Helper()
		if latestBlock.Hash != nil {
			assert.Equal(t, latestBlock.Number, *b.Number)
		} else {
			assert.Nil(t, b.Number)
			assert.Equal(t, rpcv6.BlockPending, b.Status)
		}
		checkBlock(t, b)
	}

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - number accepted on l1", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: latestBlockNumber,
			BlockHash:   latestBlockHash,
			StateRoot:   latestBlock.GlobalStateRoot,
		}, nil)

		block, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		assert.Equal(t, rpcv6.BlockAcceptedL1, block.Status)
		checkBlock(t, block)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockSyncReader.EXPECT().Pending().Return(&sync.Pending{
			Block: latestBlock,
		}, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		checkLatestBlock(t, block)
	})
}

func TestBlockWithTxs(t *testing.T) {
	errTests := map[string]rpcv7.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	var mockSyncReader *mocks.MockSyncReader
	mockReader := mocks.NewMockReader(mockCtrl)

	for description, id := range errTests { //nolint:dupl
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := utils.Ptr(utils.Mainnet)
			chain := blockchain.New(pebble.NewMemTest(t), n, nil)

			if description == "pending" {
				mockSyncReader = mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().Pending().Return(nil, sync.ErrPendingBlockNotFound)
			}

			handler := rpcv7.New(chain, mockSyncReader, nil, "", n, log)

			block, rpcErr := handler.BlockWithTxs(id)
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	n := utils.Ptr(utils.Mainnet)
	handler := rpcv7.New(mockReader, mockSyncReader, nil, "", n, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(16697)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkLatestBlock := func(t *testing.T, blockWithTxHashes *rpcv7.BlockWithTxHashes, blockWithTxs *rpcv7.BlockWithTxs) {
		t.Helper()
		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		for i, txnHash := range blockWithTxHashes.TxnHashes {
			txn, err := handler.TransactionByHash(*txnHash)
			require.Nil(t, err)

			assert.Equal(t, txn, blockWithTxs.Transactions[i])
		}
	}

	latestBlockTxMap := make(map[felt.Felt]core.Transaction)
	for _, tx := range latestBlock.Transactions {
		latestBlockTxMap[*tx.Hash()] = tx
	}

	mockReader.EXPECT().TransactionByHash(gomock.Any()).DoAndReturn(func(hash *felt.Felt) (core.Transaction, error) {
		if tx, found := latestBlockTxMap[*hash]; found {
			return tx, nil
		}
		return nil, errors.New("txn not found")
	}).Times(len(latestBlock.Transactions) * 5)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - number accepted on l1", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: latestBlockNumber,
			BlockHash:   latestBlockHash,
			StateRoot:   latestBlock.GlobalStateRoot,
		}, nil).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockSyncReader.EXPECT().Pending().Return(&sync.Pending{
			Block: latestBlock,
		}, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Pending: true})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Pending: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})
}

func TestBlockWithTxHashesV013(t *testing.T) {
	n := utils.Ptr(utils.SepoliaIntegration)
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpcv7.New(mockReader, nil, nil, "", n, nil)

	blockNumber := uint64(16350)
	gw := adaptfeeder.New(feeder.NewTestClient(t, n))
	coreBlock, err := gw.BlockByNumber(context.Background(), blockNumber)
	require.NoError(t, err)
	tx, ok := coreBlock.Transactions[0].(*core.InvokeTransaction)
	require.True(t, ok)

	mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(coreBlock, nil)
	mockReader.EXPECT().L1Head().Return(&core.L1Head{}, nil)
	got, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Number: blockNumber})
	require.Nil(t, rpcErr)
	got.Transactions = got.Transactions[:1]

	require.Equal(t, &rpcv7.BlockWithTxs{
		BlockHeader: rpcv6.BlockHeader{
			Hash:            coreBlock.Hash,
			StarknetVersion: coreBlock.ProtocolVersion,
			NewRoot:         coreBlock.GlobalStateRoot,
			Number:          &coreBlock.Number,
			ParentHash:      coreBlock.ParentHash,
			L1DAMode:        utils.Ptr(rpcv6.Blob),
			L1GasPrice: &rpcv6.ResourcePrice{
				InFri: utils.HexToFelt(t, "0x17882b6aa74"),
				InWei: utils.HexToFelt(t, "0x3b9aca10"),
			},
			L1DataGasPrice: &rpcv6.ResourcePrice{
				InFri: utils.HexToFelt(t, "0x2cc6d7f596e1"),
				InWei: utils.HexToFelt(t, "0x716a8f6dd"),
			},
			SequencerAddress: coreBlock.SequencerAddress,
			Timestamp:        coreBlock.Timestamp,
		},
		Status: rpcv6.BlockAcceptedL2,
		Transactions: []*rpcv7.Transaction{
			{
				Hash:               tx.Hash(),
				Type:               rpcv7.TxnInvoke,
				Version:            tx.Version.AsFelt(),
				Nonce:              tx.Nonce,
				MaxFee:             tx.MaxFee,
				ContractAddress:    tx.ContractAddress,
				SenderAddress:      tx.SenderAddress,
				Signature:          &tx.TransactionSignature,
				CallData:           &tx.CallData,
				EntryPointSelector: tx.EntryPointSelector,
				ResourceBounds: map[rpcv7.Resource]rpcv7.ResourceBounds{
					rpcv7.ResourceL1Gas: {
						MaxAmount:       new(felt.Felt).SetUint64(tx.ResourceBounds[core.ResourceL1Gas].MaxAmount),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL1Gas].MaxPricePerUnit,
					},
					rpcv7.ResourceL2Gas: {
						MaxAmount:       new(felt.Felt).SetUint64(tx.ResourceBounds[core.ResourceL2Gas].MaxAmount),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL2Gas].MaxPricePerUnit,
					},
				},
				Tip:                   new(felt.Felt).SetUint64(tx.Tip),
				PaymasterData:         &tx.PaymasterData,
				AccountDeploymentData: &tx.AccountDeploymentData,
				NonceDAMode:           utils.Ptr(rpcv7.DataAvailabilityMode(tx.NonceDAMode)),
				FeeDAMode:             utils.Ptr(rpcv7.DataAvailabilityMode(tx.FeeDAMode)),
			},
		},
	}, got)
}

func TestBlockWithReceipts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv7.New(mockReader, mockSyncReader, nil, "", n, nil)

	t.Run("transaction not found", func(t *testing.T) {
		blockID := rpcv7.BlockID{Number: 777}

		mockReader.EXPECT().BlockByNumber(blockID.Number).Return(nil, db.ErrKeyNotFound)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})
	t.Run("l1head failure", func(t *testing.T) {
		blockID := rpcv7.BlockID{Number: 777}
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(nil, err)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	t.Run("pending block", func(t *testing.T) {
		block0, err := mainnetGw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)

		mockSyncReader.EXPECT().Pending().Return(&sync.Pending{Block: block0}, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{}, nil)

		resp, rpcErr := handler.BlockWithReceipts(rpcv7.BlockID{Pending: true})
		header := resp.BlockHeader

		var txsWithReceipt []rpcv7.TransactionWithReceipt
		for i, tx := range block0.Transactions {
			receipt := block0.Receipts[i]
			adaptedTx := rpcv7.AdaptTransaction(tx)
			adaptedTx.Hash = nil

			txsWithReceipt = append(txsWithReceipt, rpcv7.TransactionWithReceipt{
				Transaction: adaptedTx,
				Receipt:     rpcv7.AdaptReceipt(receipt, tx, rpcv7.TxnAcceptedOnL2, nil, 0),
			})
		}

		assert.Nil(t, rpcErr)
		assert.Equal(t, &rpcv7.BlockWithReceipts{
			Status: rpcv6.BlockPending,
			BlockHeader: rpcv6.BlockHeader{
				Hash:             header.Hash,
				ParentHash:       header.ParentHash,
				Number:           header.Number,
				NewRoot:          header.NewRoot,
				Timestamp:        header.Timestamp,
				SequencerAddress: header.SequencerAddress,
				L1GasPrice:       header.L1GasPrice,
				L1DataGasPrice:   header.L1DataGasPrice,
				L1DAMode:         header.L1DAMode,
				StarknetVersion:  header.StarknetVersion,
			},
			Transactions: txsWithReceipt,
		}, resp)
	})

	t.Run("accepted L1 block", func(t *testing.T) {
		block1, err := mainnetGw.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		blockID := rpcv7.BlockID{Number: block1.Number}

		mockReader.EXPECT().BlockByNumber(blockID.Number).Return(block1, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: block1.Number + 1,
		}, nil)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		header := resp.BlockHeader

		var transactions []rpcv7.TransactionWithReceipt
		for i, tx := range block1.Transactions {
			receipt := block1.Receipts[i]
			adaptedTx := rpcv7.AdaptTransaction(tx)
			adaptedTx.Hash = nil

			transactions = append(transactions, rpcv7.TransactionWithReceipt{
				Transaction: adaptedTx,
				Receipt:     rpcv7.AdaptReceipt(receipt, tx, rpcv7.TxnAcceptedOnL1, nil, 0),
			})
		}

		assert.Nil(t, rpcErr)
		assert.Equal(t, &rpcv7.BlockWithReceipts{
			Status: rpcv6.BlockAcceptedL1,
			BlockHeader: rpcv6.BlockHeader{
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
			},
			Transactions: transactions,
		}, resp)
	})
}

func TestRpcBlockAdaptation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpcv7.New(mockReader, nil, nil, "", n, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)
	latestBlockNumber := uint64(4850)

	t.Run("default sequencer address", func(t *testing.T) {
		latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
		require.NoError(t, err)
		latestBlock.Header.SequencerAddress = nil
		mockReader.EXPECT().Head().Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		block, rpcErr := handler.BlockWithTxs(rpcv7.BlockID{Latest: true})
		require.NoError(t, err, rpcErr)
		require.Equal(t, &felt.Zero, block.BlockHeader.SequencerAddress)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpcv7.BlockID{Latest: true})
		require.NoError(t, err, rpcErr)
		require.Equal(t, &felt.Zero, blockWithTxHashes.BlockHeader.SequencerAddress)
	})
}
