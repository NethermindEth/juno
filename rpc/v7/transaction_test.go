package rpcv7_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpc "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTransactionByBlockIdAndIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	latestBlockNumber := 19199
	latestBlock, err := mainnetGw.BlockByNumber(context.Background(), 19199)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	handler := rpc.New(mockReader, mockSyncReader, nil, "", n, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(
			rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Number: rand.Uint64()}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("negative index", func(t *testing.T) {
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, -1)
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("invalid index", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			latestBlock.TransactionCount).Return(nil, errors.New("invalid index"))

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, len(latestBlock.Transactions))
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		// Use v6 handler as v7 `starknet_getTransactionByHash` uses v6 handler
		mockSyncReaderV6 := mocks.NewMockSyncReader(mockCtrl)
		mockReaderV6 := mocks.NewMockReader(mockCtrl)
		mockReaderV6.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, "", n, nil)

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, index)
		require.Nil(t, rpcErr)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, AdaptV6TxToV7(t, txn2))
	})

	t.Run("blockID - hash", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Hash: latestBlockHash}, index)
		require.Nil(t, rpcErr)

		// Use v6 handler as v7 `starknet_getTransactionByHash` uses v6 handler
		mockSyncReaderV6 := mocks.NewMockSyncReader(mockCtrl)
		mockReaderV6 := mocks.NewMockReader(mockCtrl)
		mockReaderV6.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, "", n, nil)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, AdaptV6TxToV7(t, txn2))
	})

	t.Run("blockID - number", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().BlockHeaderByNumber(uint64(latestBlockNumber)).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Number: uint64(latestBlockNumber)}, index)
		require.Nil(t, rpcErr)

		// Use v6 handler as v7 `starknet_getTransactionByHash` uses v6 handler
		mockSyncReaderV6 := mocks.NewMockSyncReader(mockCtrl)
		mockReaderV6 := mocks.NewMockReader(mockCtrl)
		mockReaderV6.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, "", n, nil)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, AdaptV6TxToV7(t, txn2))
	})

	t.Run("blockID - pending", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockSyncReader.EXPECT().Pending().Return(&sync.Pending{
			Block: latestBlock,
		}, nil)

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Pending: true}, index)
		require.Nil(t, rpcErr)

		// Use v6 handler as v7 `starknet_getTransactionByHash` uses v6 handler
		mockSyncReaderV6 := mocks.NewMockSyncReader(mockCtrl)
		mockReaderV6 := mocks.NewMockReader(mockCtrl)
		mockReaderV6.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, "", n, nil)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, AdaptV6TxToV7(t, txn2))
	})
}

// Convert a v6 transaction object to a v7 transaction object
func AdaptV6TxToV7(t *testing.T, tx *rpcv6.Transaction) *rpc.Transaction {
	t.Helper()

	var v7ResourceBounds *map[rpc.Resource]rpc.ResourceBounds
	if tx.ResourceBounds != nil {
		v7ResourceBoundsMap := make(map[rpc.Resource]rpc.ResourceBounds)
		for r, rb := range *tx.ResourceBounds {
			v7ResourceBoundsMap[rpc.Resource(r)] = rpc.ResourceBounds{
				MaxAmount:       rb.MaxAmount,
				MaxPricePerUnit: rb.MaxPricePerUnit,
			}
		}
		v7ResourceBounds = &v7ResourceBoundsMap
	}

	var v7NonceDAMode *rpc.DataAvailabilityMode
	if tx.NonceDAMode != nil {
		v7NonceDAMode = utils.Ptr(rpc.DataAvailabilityMode(*tx.NonceDAMode))
	}

	var v7FeeDAMode *rpc.DataAvailabilityMode
	if tx.FeeDAMode != nil {
		v7FeeDAMode = utils.Ptr(rpc.DataAvailabilityMode(*tx.FeeDAMode))
	}

	return &rpc.Transaction{
		Hash:                  tx.Hash,
		Type:                  rpc.TransactionType(tx.Type),
		Version:               tx.Version,
		Nonce:                 tx.Nonce,
		MaxFee:                tx.MaxFee,
		ContractAddress:       tx.ContractAddress,
		ContractAddressSalt:   tx.ContractAddressSalt,
		ClassHash:             tx.ClassHash,
		ConstructorCallData:   tx.ConstructorCallData,
		SenderAddress:         tx.SenderAddress,
		Signature:             tx.Signature,
		CallData:              tx.CallData,
		EntryPointSelector:    tx.EntryPointSelector,
		CompiledClassHash:     tx.CompiledClassHash,
		ResourceBounds:        v7ResourceBounds,
		Tip:                   tx.Tip,
		PaymasterData:         tx.PaymasterData,
		AccountDeploymentData: tx.AccountDeploymentData,
		NonceDAMode:           v7NonceDAMode,
		FeeDAMode:             v7FeeDAMode,
	}
}

// TODO[Pawel]: The following 2 tests `Test[Legacy]TransactionReceiptByHash` are skipped
// but we still keep them here. I have a doubt whether they test anything useful.
//
//nolint:dupl
func TestTransactionReceiptByHash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncer := mocks.NewMockSyncReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncer, nil, "", n, nil)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		mockSyncer.EXPECT().PendingBlock().Return(nil)

		tx, rpcErr := handler.TransactionReceiptByHash(*txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	block0, err := mainnetGw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	checkTxReceipt := func(t *testing.T, h *felt.Felt, expected string) {
		t.Helper()

		expectedMap := make(map[string]any)
		require.NoError(t, json.Unmarshal([]byte(expected), &expectedMap))

		receipt, err := handler.TransactionReceiptByHash(*h)
		require.Nil(t, err)

		receiptJSON, jsonErr := json.Marshal(receipt)
		require.NoError(t, jsonErr)

		receiptMap := make(map[string]any)
		require.NoError(t, json.Unmarshal(receiptJSON, &receiptMap))
		assert.Equal(t, expectedMap, receiptMap)
	}

	tests := map[string]struct {
		index    int
		expected string
	}{
		"with contract addr": {
			index: 0,
			expected: `{
					"type": "DEPLOY",
					"transaction_hash": "0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75",
					"actual_fee": {"amount": "0x0", "unit": "WEI"},
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [],
					"events": [],
					"contract_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
					"execution_resources":{"data_availability": {"l1_data_gas": 0, "l1_gas": 0}, "steps":29}
				}`,
		},
		"without contract addr": {
			index: 2,
			expected: `{
					"type": "INVOKE",
					"transaction_hash": "0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624",
					"actual_fee": {"amount": "0x0", "unit": "WEI"},
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [
						{
							"from_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
							"to_address": "0xc84dd7fd43a7defb5b7a15c4fbbe11cbba6db1ba",
							"payload": [
								"0xc",
								"0x22"
							]
						}
					],
					"events": [],
					"execution_resources":{"data_availability": {"l1_data_gas": 0, "l1_gas": 0}, "steps":31}
				}`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			txHash := block0.Transactions[test.index].Hash()
			mockReader.EXPECT().TransactionByHash(txHash).Return(block0.Transactions[test.index], nil)
			mockReader.EXPECT().Receipt(txHash).Return(block0.Receipts[test.index], block0.Hash, block0.Number, nil)
			mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

			checkTxReceipt(t, txHash, test.expected)
		})
	}

	t.Run("pending receipt", func(t *testing.T) {
		i := 2
		expected := `{
					"type": "INVOKE",
					"transaction_hash": "0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624",
					"actual_fee": {"amount": "0x0", "unit": "WEI"},
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"messages_sent": [
						{
							"from_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
							"to_address": "0xc84dd7fd43a7defb5b7a15c4fbbe11cbba6db1ba",
							"payload": [
								"0xc",
								"0x22"
							]
						}
					],
					"events": [],
					"execution_resources":{"data_availability": {"l1_data_gas": 0, "l1_gas": 0}, "steps":31}
				}`

		txHash := block0.Transactions[i].Hash()
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		mockSyncer.EXPECT().PendingBlock().Return(block0)

		checkTxReceipt(t, txHash, expected)
	})

	t.Run("accepted on l1 receipt", func(t *testing.T) {
		i := 2
		expected := `{
					"type": "INVOKE",
					"transaction_hash": "0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624",
					"actual_fee": {"amount": "0x0", "unit": "WEI"},
					"finality_status": "ACCEPTED_ON_L1",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [
						{
							"from_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
							"to_address": "0xc84dd7fd43a7defb5b7a15c4fbbe11cbba6db1ba",
							"payload": [
								"0xc",
								"0x22"
							]
						}
					],
					"events": [],
					"execution_resources":{"data_availability": {"l1_data_gas": 0, "l1_gas": 0}, "steps":31}
				}`

		txHash := block0.Transactions[i].Hash()
		mockReader.EXPECT().TransactionByHash(txHash).Return(block0.Transactions[i], nil)
		mockReader.EXPECT().Receipt(txHash).Return(block0.Receipts[i], block0.Hash, block0.Number, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: block0.Number,
			BlockHash:   block0.Hash,
			StateRoot:   block0.GlobalStateRoot,
		}, nil)

		checkTxReceipt(t, txHash, expected)
	})
	t.Run("reverted", func(t *testing.T) {
		expected := `{
			"type": "INVOKE",
			"transaction_hash": "0x19abec18bbacec23c2eee160c70190a48e4b41dd5ff98ad8f247f9393559998",
			"actual_fee": {"amount": "0x247aff6e224", "unit": "WEI"},
			"execution_status": "REVERTED",
			"finality_status": "ACCEPTED_ON_L2",
			"block_hash": "0x76e0229fd0c36dda2ee7905f7e4c9b3ebb78d98c4bfab550bcb3a03bf859a6",
			"block_number": 304740,
			"messages_sent": [],
			"events": [],
			"revert_reason": "Error in the called contract (0x00b1461de04c6a1aa3375bdf9b7723a8779c082ffe21311d683a0b15c078b5dc):\nError at pc=0:25:\nGot an exception while executing a hint.\nCairo traceback (most recent call last):\nUnknown location (pc=0:731)\nUnknown location (pc=0:677)\nUnknown location (pc=0:291)\nUnknown location (pc=0:314)\n\nError in the called contract (0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7):\nError at pc=0:104:\nGot an exception while executing a hint.\nCairo traceback (most recent call last):\nUnknown location (pc=0:1678)\nUnknown location (pc=0:1664)\n\nError in the called contract (0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7):\nError at pc=0:6:\nGot an exception while executing a hint: Assertion failed, 0 % 0x800000000000011000000000000000000000000000000000000000000000001 is equal to 0\nCairo traceback (most recent call last):\nUnknown location (pc=0:1238)\nUnknown location (pc=0:1215)\nUnknown location (pc=0:836)\n",
			"execution_resources":{"data_availability": {"l1_data_gas": 0, "l1_gas": 0}, "steps":0}
		}`

		integClient := feeder.NewTestClient(t, &utils.Integration)
		integGw := adaptfeeder.New(integClient)

		blockWithRevertedTxn, err := integGw.BlockByNumber(context.Background(), 304740)
		require.NoError(t, err)

		revertedTxnIdx := 1
		revertedTxnHash := blockWithRevertedTxn.Transactions[revertedTxnIdx].Hash()

		mockReader.EXPECT().TransactionByHash(revertedTxnHash).Return(blockWithRevertedTxn.Transactions[revertedTxnIdx], nil)
		mockReader.EXPECT().Receipt(revertedTxnHash).Return(blockWithRevertedTxn.Receipts[revertedTxnIdx],
			blockWithRevertedTxn.Hash, blockWithRevertedTxn.Number, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		checkTxReceipt(t, revertedTxnHash, expected)
	})

	t.Run("v3 tx", func(t *testing.T) {
		expected := `{
			"block_hash": "0x50e864db6b81ce69fbeb70e6a7284ee2febbb9a2e707415de7adab83525e9cd",
			"block_number": 319132,
			"execution_status": "SUCCEEDED",
			"finality_status": "ACCEPTED_ON_L2",
			"transaction_hash": "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
			"messages_sent": [],
			"events": [
				{
					"from_address": "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d",
					"keys": [
						"0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"
					],
					"data": [
						"0x3f6f3bc663aedc5285d6013cc3ffcbc4341d86ab488b8b68d297f8258793c41",
						"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
						"0x16d8b4ad4000",
						"0x0"
					]
				},
				{
					"from_address": "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d",
					"keys": [
						"0xa9fa878c35cd3d0191318f89033ca3e5501a3d90e21e3cc9256bdd5cd17fdd"
					],
					"data": [
						"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
						"0x18ad8494375bc00",
						"0x0",
						"0x18aef21f822fc00",
						"0x0"
					]
				}
			],
			"execution_resources": {
				"data_availability": {
					"l1_data_gas": 0,
					"l1_gas": 0
				},
				"steps": 615,
				"range_check_builtin_applications": 19,
				"memory_holes": 4
			},
			"actual_fee": {
				"amount": "0x16d8b4ad4000",
				"unit": "FRI"
			},
			"type": "INVOKE"
		}`

		integClient := feeder.NewTestClient(t, &utils.Integration)
		integGw := adaptfeeder.New(integClient)

		block, err := integGw.BlockByNumber(context.Background(), 319132)
		require.NoError(t, err)

		index := 0
		txnHash := block.Transactions[index].Hash()

		mockReader.EXPECT().TransactionByHash(txnHash).Return(block.Transactions[index], nil)
		mockReader.EXPECT().Receipt(txnHash).Return(block.Receipts[index],
			block.Hash, block.Number, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		checkTxReceipt(t, txnHash, expected)
	})
}

//nolint:dupl
func TestLegacyTransactionReceiptByHash(t *testing.T) {
	t.Skip()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", n, nil)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

		tx, rpcErr := handler.TransactionReceiptByHash(*txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	block0, err := mainnetGw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

	checkTxReceipt := func(t *testing.T, _ *felt.Felt, expected string) {
		t.Helper()

		expectedMap := make(map[string]any)
		require.NoError(t, json.Unmarshal([]byte(expected), &expectedMap))

		//nolint:gocritic
		// receipt, err := handler.LegacyTransactionReceiptByHash(*h)
		// require.Nil(t, err)
		// receiptJSON, jsonErr := json.Marshal(receipt)
		// require.NoError(t, jsonErr)

		receiptMap := make(map[string]any)
		//nolint:gocritic
		// require.NoError(t, json.Unmarshal(receiptJSON, &receiptMap))
		assert.Equal(t, expectedMap, receiptMap)
	}

	tests := map[string]struct {
		index    int
		expected string
	}{
		"with contract addr": {
			index: 0,
			expected: `{
					"type": "DEPLOY",
					"transaction_hash": "0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75",
					"actual_fee": "0x0",
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [],
					"events": [],
					"contract_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
					"execution_resources": {"bitwise_builtin_applications":"0x0", "ec_op_builtin_applications":"0x0", "ecdsa_builtin_applications":"0x0", "keccak_builtin_applications":"0x0", "memory_holes":"0x0", "pedersen_builtin_applications":"0x0", "poseidon_builtin_applications":"0x0", "range_check_builtin_applications":"0x0", "steps":"0x1d"}
				}`,
		},
		"without contract addr": {
			index: 2,
			expected: `{
					"type": "INVOKE",
					"transaction_hash": "0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624",
					"actual_fee": "0x0",
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [
						{
							"from_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
							"to_address": "0xc84dd7fd43a7defb5b7a15c4fbbe11cbba6db1ba",
							"payload": [
								"0xc",
								"0x22"
							]
						}
					],
					"events": [],
					"execution_resources":{"bitwise_builtin_applications":"0x0", "ec_op_builtin_applications":"0x0", "ecdsa_builtin_applications":"0x0", "keccak_builtin_applications":"0x0", "memory_holes":"0x0", "pedersen_builtin_applications":"0x0", "poseidon_builtin_applications":"0x0", "range_check_builtin_applications":"0x0", "steps":"0x1f"}
				}`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			txHash := block0.Transactions[test.index].Hash()
			mockReader.EXPECT().TransactionByHash(txHash).Return(block0.Transactions[test.index], nil)
			mockReader.EXPECT().Receipt(txHash).Return(block0.Receipts[test.index], block0.Hash, block0.Number, nil)
			mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

			checkTxReceipt(t, txHash, test.expected)
		})
	}

	t.Run("pending receipt", func(t *testing.T) {
		i := 2
		expected := `{
					"type": "INVOKE",
					"transaction_hash": "0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624",
					"actual_fee": "0x0",
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"messages_sent": [
						{
							"from_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
							"to_address": "0xc84dd7fd43a7defb5b7a15c4fbbe11cbba6db1ba",
							"payload": [
								"0xc",
								"0x22"
							]
						}
					],
					"events": [],
					"execution_resources":{"bitwise_builtin_applications":"0x0", "ec_op_builtin_applications":"0x0", "ecdsa_builtin_applications":"0x0", "keccak_builtin_applications":"0x0", "memory_holes":"0x0", "pedersen_builtin_applications":"0x0", "poseidon_builtin_applications":"0x0", "range_check_builtin_applications":"0x0", "steps":"0x1f"}
				}`

		txHash := block0.Transactions[i].Hash()
		mockReader.EXPECT().TransactionByHash(txHash).Return(block0.Transactions[i], nil)
		mockReader.EXPECT().Receipt(txHash).Return(block0.Receipts[i], nil, uint64(0), nil)

		checkTxReceipt(t, txHash, expected)
	})

	t.Run("accepted on l1 receipt", func(t *testing.T) {
		i := 2
		expected := `{
					"type": "INVOKE",
					"transaction_hash": "0xce54bbc5647e1c1ea4276c01a708523f740db0ff5474c77734f73beec2624",
					"actual_fee": "0x0",
					"finality_status": "ACCEPTED_ON_L1",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [
						{
							"from_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
							"to_address": "0xc84dd7fd43a7defb5b7a15c4fbbe11cbba6db1ba",
							"payload": [
								"0xc",
								"0x22"
							]
						}
					],
					"events": [],
					"execution_resources":{"bitwise_builtin_applications":"0x0", "ec_op_builtin_applications":"0x0", "ecdsa_builtin_applications":"0x0", "keccak_builtin_applications":"0x0", "memory_holes":"0x0", "pedersen_builtin_applications":"0x0", "poseidon_builtin_applications":"0x0", "range_check_builtin_applications":"0x0", "steps":"0x1f"}
				}`

		txHash := block0.Transactions[i].Hash()
		mockReader.EXPECT().TransactionByHash(txHash).Return(block0.Transactions[i], nil)
		mockReader.EXPECT().Receipt(txHash).Return(block0.Receipts[i], block0.Hash, block0.Number, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: block0.Number,
			BlockHash:   block0.Hash,
			StateRoot:   block0.GlobalStateRoot,
		}, nil)

		checkTxReceipt(t, txHash, expected)
	})
	t.Run("reverted", func(t *testing.T) {
		expected := `{
			"type": "INVOKE",
			"transaction_hash": "0x19abec18bbacec23c2eee160c70190a48e4b41dd5ff98ad8f247f9393559998",
			"actual_fee": "0x247aff6e224",
			"execution_status": "REVERTED",
			"finality_status": "ACCEPTED_ON_L2",
			"block_hash": "0x76e0229fd0c36dda2ee7905f7e4c9b3ebb78d98c4bfab550bcb3a03bf859a6",
			"block_number": 304740,
			"messages_sent": [],
			"events": [],
			"revert_reason": "Error in the called contract (0x00b1461de04c6a1aa3375bdf9b7723a8779c082ffe21311d683a0b15c078b5dc):\nError at pc=0:25:\nGot an exception while executing a hint.\nCairo traceback (most recent call last):\nUnknown location (pc=0:731)\nUnknown location (pc=0:677)\nUnknown location (pc=0:291)\nUnknown location (pc=0:314)\n\nError in the called contract (0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7):\nError at pc=0:104:\nGot an exception while executing a hint.\nCairo traceback (most recent call last):\nUnknown location (pc=0:1678)\nUnknown location (pc=0:1664)\n\nError in the called contract (0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7):\nError at pc=0:6:\nGot an exception while executing a hint: Assertion failed, 0 % 0x800000000000011000000000000000000000000000000000000000000000001 is equal to 0\nCairo traceback (most recent call last):\nUnknown location (pc=0:1238)\nUnknown location (pc=0:1215)\nUnknown location (pc=0:836)\n",
			"execution_resources":{"bitwise_builtin_applications":"0x0", "ec_op_builtin_applications":"0x0", "ecdsa_builtin_applications":"0x0", "keccak_builtin_applications":"0x0", "memory_holes":"0x0", "pedersen_builtin_applications":"0x0", "poseidon_builtin_applications":"0x0", "range_check_builtin_applications":"0x0","steps":"0x0"}
		}`

		integClient := feeder.NewTestClient(t, &utils.Integration)
		integGw := adaptfeeder.New(integClient)

		blockWithRevertedTxn, err := integGw.BlockByNumber(context.Background(), 304740)
		require.NoError(t, err)

		revertedTxnIdx := 1
		revertedTxnHash := blockWithRevertedTxn.Transactions[revertedTxnIdx].Hash()

		mockReader.EXPECT().TransactionByHash(revertedTxnHash).Return(blockWithRevertedTxn.Transactions[revertedTxnIdx], nil)
		mockReader.EXPECT().Receipt(revertedTxnHash).Return(blockWithRevertedTxn.Receipts[revertedTxnIdx],
			blockWithRevertedTxn.Hash, blockWithRevertedTxn.Number, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		checkTxReceipt(t, revertedTxnHash, expected)
	})

	t.Run("v3 tx", func(t *testing.T) {
		expected := `{
			"block_hash": "0x50e864db6b81ce69fbeb70e6a7284ee2febbb9a2e707415de7adab83525e9cd",
			"block_number": 319132,
			"execution_status": "SUCCEEDED",
			"finality_status": "ACCEPTED_ON_L2",
			"transaction_hash": "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
			"messages_sent": [],
			"events": [
				{
					"from_address": "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d",
					"keys": [
						"0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"
					],
					"data": [
						"0x3f6f3bc663aedc5285d6013cc3ffcbc4341d86ab488b8b68d297f8258793c41",
						"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
						"0x16d8b4ad4000",
						"0x0"
					]
				},
				{
					"from_address": "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d",
					"keys": [
						"0xa9fa878c35cd3d0191318f89033ca3e5501a3d90e21e3cc9256bdd5cd17fdd"
					],
					"data": [
						"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
						"0x18ad8494375bc00",
						"0x0",
						"0x18aef21f822fc00",
						"0x0"
					]
				}
			],
			"execution_resources": {"bitwise_builtin_applications":"0x0", "ec_op_builtin_applications":"0x0", "ecdsa_builtin_applications":"0x0", "keccak_builtin_applications":"0x0", "memory_holes":"0x4", "pedersen_builtin_applications":"0x0", "poseidon_builtin_applications":"0x0", "range_check_builtin_applications":"0x13", "steps":"0x267"},
			"actual_fee": "0x16d8b4ad4000",
			"type": "INVOKE"
		}`

		integClient := feeder.NewTestClient(t, &utils.Integration)
		integGw := adaptfeeder.New(integClient)

		block, err := integGw.BlockByNumber(context.Background(), 319132)
		require.NoError(t, err)

		index := 0
		txnHash := block.Transactions[index].Hash()

		mockReader.EXPECT().TransactionByHash(txnHash).Return(block.Transactions[index], nil)
		mockReader.EXPECT().Receipt(txnHash).Return(block.Receipts[index],
			block.Hash, block.Number, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		checkTxReceipt(t, txnHash, expected)
	})
}

func TestAddTransactionUnmarshal(t *testing.T) {
	tests := map[string]string{
		"deploy account v3": `{
			"type": "DEPLOY_ACCOUNT",
			"version": "0x3",
			"signature": [
				"0x73c0e0fe22d6e82187b84e06f33644f7dc6edce494a317bfcdd0bb57ab862fa",
				"0x6119aa7d091eac96f07d7d195f12eff9a8786af85ddf41028428ee8f510e75e"
			],
			"nonce": "0x0",
			"contract_address_salt": "0x510b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb",
			"constructor_calldata": [
				"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2",
				"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463",
				"0x2",
				"0x510b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb",
				"0x0"
			],
			"class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918",
			"resource_bounds": {
				"l1_gas": {
					"max_amount": "0x6fde2b4eb000",
					"max_price_per_unit": "0x6fde2b4eb000"
				},
				"l2_gas": {
					"max_amount": "0x6fde2b4eb000",
					"max_price_per_unit": "0x6fde2b4eb000"
				}
			},
			"tip": "0x0",
			"paymaster_data": [],
			"nonce_data_availability_mode": "L1",
			"fee_data_availability_mode": "L2"
		}`,
	}

	for description, txJSON := range tests {
		t.Run(description, func(t *testing.T) {
			tx := rpc.BroadcastedTransaction{}
			require.NoError(t, json.Unmarshal([]byte(txJSON), &tx))
		})
	}
}

func TestAddTransaction(t *testing.T) {
	n := utils.Ptr(utils.Integration)
	gw := adaptfeeder.New(feeder.NewTestClient(t, n))
	txWithoutClass := func(hash string) rpc.BroadcastedTransaction {
		tx, err := gw.Transaction(context.Background(), utils.HexToFelt(t, hash))
		require.NoError(t, err)
		return rpc.BroadcastedTransaction{
			Transaction: *rpc.AdaptTransaction(tx),
		}
	}
	tests := map[string]struct {
		txn          rpc.BroadcastedTransaction
		expectedJSON string
	}{
		"invoke v0": {
			txn: txWithoutClass("0x5e91283c1c04c3f88e4a98070df71227fb44dea04ce349c7eb379f85a10d1c3"),
			expectedJSON: `{
				"transaction_hash": "0x5e91283c1c04c3f88e4a98070df71227fb44dea04ce349c7eb379f85a10d1c3",
				"version": "0x0",
				"max_fee": "0x0",
				"signature": [],
				"entry_point_selector": "0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64",
				"calldata": [
				  "0x79631f37538379fc32739605910733219b836b050766a2349e93ec375e62885",
				  "0x0"
				],
				"contract_address": "0x2cbc1f6e80a024900dc949914c7692f802ba90012cda39115db5640f5eca847",
				"type": "INVOKE_FUNCTION"
			  }`,
		},
		"invoke v1": {
			txn: txWithoutClass("0x45d9c2c8e01bacae6dec3438874576a4a1ce65f1d4247f4e9748f0e7216838"),
			expectedJSON: `{
				"transaction_hash": "0x45d9c2c8e01bacae6dec3438874576a4a1ce65f1d4247f4e9748f0e7216838",
				"version": "0x1",
				"max_fee": "0x2386f26fc10000",
				"signature": [
				  "0x89aa2f42e07913b6dee313c3ef680efb99892feb3e2d08287e01e63418da7a",
				  "0x458fb4c942d5407d8c1ef1557d29487ab8217842d28a907d75ee0828243361"
				],
				"nonce": "0x99d",
				"sender_address": "0x219937256cd88844f9fdc9c33a2d6d492e253ae13814c2dc0ecab7f26919d46",
				"calldata": [
				  "0x1",
				  "0x7812357541c81dd9a320c2339c0c76add710db15f8cc29e8dde8e588cad4455",
				  "0x7772be8b80a8a33dc6c1f9a6ab820c02e537c73e859de67f288c70f92571bb",
				  "0x0",
				  "0x3",
				  "0x3",
				  "0x24b037cd0ffd500467f4cc7d0b9df27abdc8646379e818e3ce3d9925fc9daec",
				  "0x4b7797c3f6a6d9b1a28bbd6645d3f009bd12587581e21011aeb9b176f801ab0",
				  "0xdfeaf5f022324453e6058c00c7d35ee449c1d01bb897ccb5df20f697d98f26"
				],
				"type": "INVOKE_FUNCTION"
			  }`,
		},
		"invoke v3": {
			txn: txWithoutClass("0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd"),
			expectedJSON: `{
				"transaction_hash": "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
				"version": "0x3",
				"signature": [
				  "0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16",
				  "0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"
				],
				"nonce": "0xe97",
				"nonce_data_availability_mode": 0,
				"fee_data_availability_mode": 0,
				"resource_bounds": {
				  "L1_GAS": {
					"max_amount": "0x186a0",
					"max_price_per_unit": "0x5af3107a4000"
				  },
				  "L2_GAS": {
					"max_amount": "0x0",
					"max_price_per_unit": "0x0"
				  }
				},
				"tip": "0x0",
				"paymaster_data": [],
				"sender_address": "0x3f6f3bc663aedc5285d6013cc3ffcbc4341d86ab488b8b68d297f8258793c41",
				"calldata": [
				  "0x2",
				  "0x450703c32370cf7ffff540b9352e7ee4ad583af143a361155f2b485c0c39684",
				  "0x27c3334165536f239cfd400ed956eabff55fc60de4fb56728b6a4f6b87db01c",
				  "0x0",
				  "0x4",
				  "0x4c312760dfd17a954cdd09e76aa9f149f806d88ec3e402ffaf5c4926f568a42",
				  "0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf",
				  "0x4",
				  "0x1",
				  "0x5",
				  "0x450703c32370cf7ffff540b9352e7ee4ad583af143a361155f2b485c0c39684",
				  "0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf",
				  "0x1",
				  "0x7fe4fd616c7fece1244b3616bb516562e230be8c9f29668b46ce0369d5ca829",
				  "0x287acddb27a2f9ba7f2612d72788dc96a5b30e401fc1e8072250940e024a587"
				],
				"account_deployment_data": [],
				"type": "INVOKE_FUNCTION"
			  }`,
		},
		"deploy v0": {
			txn: txWithoutClass("0x2e3106421d38175020cd23a6f1bff87989a64cae6a679c54c7710a033d88faa"),
			expectedJSON: `{
				"transaction_hash": "0x2e3106421d38175020cd23a6f1bff87989a64cae6a679c54c7710a033d88faa",
				"version": "0x0",
				"contract_address_salt": "0x5de1c0a37865820ce4896872e78da6877b0a8eede3d363131734556a8815d52",
				"class_hash": "0x71468bd837666b3a05cca1a5363b0d9e15cacafd6eeaddfbc4f00d5c7b9a51d",
				"constructor_calldata": [],
				"type": "DEPLOY"
			  }`,
		},
		"declare v1": {
			txn: txWithoutClass("0x2d667ed0aa3a8faef96b466972079826e592ec0aebefafd77a39f2ed06486b4"),
			expectedJSON: `{
				"transaction_hash": "0x2d667ed0aa3a8faef96b466972079826e592ec0aebefafd77a39f2ed06486b4",
				"version": "0x1",
				"max_fee": "0x2386f26fc10000",
				"signature": [
				  "0x17872d12092aa60331394f514de908309fdba185997fd3d0be1e2896cd1e053",
				  "0x66124ebfe1a34809b2223a9707ac796dc6f4b6310cb002bda1e4c062a4b2867"
				],
				"nonce": "0x1078",
				"class_hash": "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
				"sender_address": "0x52125c1e043126c637d1436d9551ef6c4f6e3e36945676bbd716a56e3a41b7a",
				"type": "DECLARE"
			  }`,
		},
		"declare v2": {
			txn: func() rpc.BroadcastedTransaction {
				tx := txWithoutClass("0x44b971f7eface29b185f86dd7b3b70acb1e48e0ad459e3a41e06fc42937aaa4")
				tx.ContractClass = json.RawMessage([]byte(`{"sierra_program": {}}`))
				return tx
			}(),
			expectedJSON: `{
				"transaction_hash": "0x44b971f7eface29b185f86dd7b3b70acb1e48e0ad459e3a41e06fc42937aaa4",
				"version": "0x2",
				"max_fee": "0x50c8f30c048",
				"signature": [
				  "0x42a40a113a4381e5f304fd28a707ba4182609db42062a7f36b9291bf8ae8ae7",
				  "0x6035bcf022f887c80dbc2b615e927d662637d2213335ee657893dce8ddabe5b"
				],
				"nonce": "0x11",
				"class_hash": "0x7cb013a4139335cefce52adc2ac342c0110811353e7992baefbe547200223c7",
				"contract_class": {
					"sierra_program": "H4sIAAAAAAAA/6quBQQAAP//Q7+mowIAAAA="
				},
				"compiled_class_hash": "0x67f7deab53a3ba70500bdafe66fb3038bbbaadb36a6dd1a7a5fc5b094e9d724",
				"sender_address": "0x3bb81d22ecd0e0a6f3138bdc5c072ff5726c5add02bcfd5b81cd657a6ae10a8",
				"type": "DECLARE"
			  }`,
		},
		"declare v3": {
			txn: func() rpc.BroadcastedTransaction {
				tx := txWithoutClass("0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3")
				tx.ContractClass = json.RawMessage([]byte(`{"sierra_program": {}}`))
				return tx
			}(),
			expectedJSON: `{
				"transaction_hash": "0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3",
				"version": "0x3",
				"signature": [
				  "0x29a49dff154fede73dd7b5ca5a0beadf40b4b069f3a850cd8428e54dc809ccc",
				  "0x429d142a17223b4f2acde0f5ecb9ad453e188b245003c86fab5c109bad58fc3"
				],
				"nonce": "0x1",
				"nonce_data_availability_mode": 0,
				"fee_data_availability_mode": 0,
				"resource_bounds": {
				  "L1_GAS": {
					"max_amount": "0x186a0",
					"max_price_per_unit": "0x2540be400"
				  },
				  "L2_GAS": {
					"max_amount": "0x0",
					"max_price_per_unit": "0x0"
				  }
				},
				"tip": "0x0",
				"paymaster_data": [],
				"sender_address": "0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50",
				"class_hash": "0x5ae9d09292a50ed48c5930904c880dab56e85b825022a7d689cfc9e65e01ee7",
				"compiled_class_hash": "0x1add56d64bebf8140f3b8a38bdf102b7874437f0c861ab4ca7526ec33b4d0f8",
				"account_deployment_data": [],
				"type": "DECLARE",
				"contract_class": {
					"sierra_program": "H4sIAAAAAAAA/6quBQQAAP//Q7+mowIAAAA="
				}
			  }`,
		},
		"deploy account v1": {
			txn: txWithoutClass("0x658f1c44ebf6a1540eac0680956c3a9d315f65d2cb3b53593345905fed3982a"),
			expectedJSON: `{
				"transaction_hash": "0x658f1c44ebf6a1540eac0680956c3a9d315f65d2cb3b53593345905fed3982a",
				"version": "0x1",
				"max_fee": "0x2386f273b213da",
				"signature": [
				  "0x7d31509f555031323050ed226012f0c6361b3dc34f0f5d2c65a76870fd8908b",
				  "0x58d64f6d39dfb20586da0c40e3d575cab940009cdee6423b03268fd893bd27a"
				],
				"nonce": "0x0",
				"contract_address_salt": "0x7b9f4b7d6d49b60686004dd850a4b41c818d6eb69e226b8ea37ea025e6830f5",
				"class_hash": "0x5a9941d0cc16b8619a3325055472da709a66113afcc6a8ab86055da7d29c5f8",
				"constructor_calldata": [
				  "0x7b16a9b7bb08d36950aa5d27d4d2c64bfd54f3ae16a0e01f21a6d410cb5179c"
				],
				"type": "DEPLOY_ACCOUNT"
			  }`,
		},
		"deploy account v3": {
			txn: txWithoutClass("0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0"),
			expectedJSON: `{
				"transaction_hash": "0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0",
				"version": "0x3",
				"signature": [
				  "0x6d756e754793d828c6c1a89c13f7ec70dbd8837dfeea5028a673b80e0d6b4ec",
				  "0x4daebba599f860daee8f6e100601d98873052e1c61530c630cc4375c6bd48e3"
				],
				"nonce": "0x0",
				"nonce_data_availability_mode": 0,
				"fee_data_availability_mode": 0,
				"resource_bounds": {
				  "L1_GAS": {
					"max_amount": "0x186a0",
					"max_price_per_unit": "0x5af3107a4000"
				  },
				  "L2_GAS": {
					"max_amount": "0x0",
					"max_price_per_unit": "0x0"
				  }
				},
				"tip": "0x0",
				"paymaster_data": [],
				"contract_address_salt": "0x0",
				"class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738",
				"constructor_calldata": [
				  "0x5cd65f3d7daea6c63939d659b8473ea0c5cd81576035a4d34e52fb06840196c"
				],
				"type": "DEPLOY_ACCOUNT"
			  }`,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockGateway := mocks.NewMockGateway(mockCtrl)
			mockGateway.
				EXPECT().
				AddTransaction(gomock.Any(), gomock.Any()).
				Do(func(_ context.Context, txnJSON json.RawMessage) error {
					assert.JSONEq(t, test.expectedJSON, string(txnJSON), string(txnJSON))
					gatewayTx := starknet.Transaction{}
					// Ensure the Starknet transaction can be unmarshaled properly.
					require.NoError(t, json.Unmarshal(txnJSON, &gatewayTx))
					return nil
				}).
				Return(json.RawMessage(`{
					"transaction_hash": "0x1",
					"address": "0x2",
					"class_hash": "0x3"
				}`), nil).
				Times(1)

			handler := rpc.New(nil, nil, nil, "", n, utils.NewNopZapLogger())
			_, rpcErr := handler.AddTransaction(context.Background(), test.txn)
			require.Equal(t, rpcErr.Code, rpccore.ErrInternal.Code)

			handler = handler.WithGateway(mockGateway)
			got, rpcErr := handler.AddTransaction(context.Background(), test.txn)
			require.Nil(t, rpcErr)
			require.Equal(t, &rpc.AddTxResponse{
				TransactionHash: utils.HexToFelt(t, "0x1"),
				ContractAddress: utils.HexToFelt(t, "0x2"),
				ClassHash:       utils.HexToFelt(t, "0x3"),
			}, got)
		})
	}
}

func TestTransactionStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	tests := []struct {
		network           *utils.Network
		verifiedTxHash    *felt.Felt
		nonVerifiedTxHash *felt.Felt
		notFoundTxHash    *felt.Felt
	}{
		{
			network:           utils.Ptr(utils.Mainnet),
			verifiedTxHash:    utils.HexToFelt(t, "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e"),
			nonVerifiedTxHash: utils.HexToFelt(t, "0x6c40890743aa220b10e5ee68cef694c5c23cc2defd0dbdf5546e687f9982ab1"),
			notFoundTxHash:    utils.HexToFelt(t, "0x8c96a2b3d73294667e489bf8904c6aa7c334e38e24ad5a721c7e04439ff9"),
		},
		{
			network:           utils.Ptr(utils.Integration),
			verifiedTxHash:    utils.HexToFelt(t, "0x5e91283c1c04c3f88e4a98070df71227fb44dea04ce349c7eb379f85a10d1c3"),
			nonVerifiedTxHash: utils.HexToFelt(t, "0x45d9c2c8e01bacae6dec3438874576a4a1ce65f1d4247f4e9748f0e7216838"),
			notFoundTxHash:    utils.HexToFelt(t, "0xd7747f3d0ce84b3a19b05b987a782beac22c54e66773303e94ea78cc3c15"),
		},
	}

	ctx := context.Background()

	for _, test := range tests {
		t.Run(test.network.String(), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			client := feeder.NewTestClient(t, test.network)

			t.Run("tx found in db", func(t *testing.T) {
				gw := adaptfeeder.New(client)

				block, err := gw.BlockLatest(context.Background())
				require.NoError(t, err)

				tx := block.Transactions[0]

				t.Run("not verified", func(t *testing.T) {
					mockReader := mocks.NewMockReader(mockCtrl)
					mockReader.EXPECT().TransactionByHash(tx.Hash()).Return(tx, nil)
					mockReader.EXPECT().Receipt(tx.Hash()).Return(block.Receipts[0], block.Hash, block.Number, nil)
					mockReader.EXPECT().L1Head().Return(nil, nil)

					handler := rpc.New(mockReader, nil, nil, "", test.network, nil)

					want := &rpc.TransactionStatus{
						Finality:  rpc.TxnStatusAcceptedOnL2,
						Execution: rpc.TxnSuccess,
					}
					status, rpcErr := handler.TransactionStatus(ctx, *tx.Hash())
					require.Nil(t, rpcErr)
					require.Equal(t, want, status)
				})
				t.Run("verified", func(t *testing.T) {
					mockReader := mocks.NewMockReader(mockCtrl)
					mockReader.EXPECT().TransactionByHash(tx.Hash()).Return(tx, nil)
					mockReader.EXPECT().Receipt(tx.Hash()).Return(block.Receipts[0], block.Hash, block.Number, nil)
					mockReader.EXPECT().L1Head().Return(&core.L1Head{
						BlockNumber: block.Number + 1,
					}, nil)

					handler := rpc.New(mockReader, nil, nil, "", test.network, nil)

					want := &rpc.TransactionStatus{
						Finality:  rpc.TxnStatusAcceptedOnL1,
						Execution: rpc.TxnSuccess,
					}
					status, rpcErr := handler.TransactionStatus(ctx, *tx.Hash())
					require.Nil(t, rpcErr)
					require.Equal(t, want, status)
				})
			})
			t.Run("transaction not found in db", func(t *testing.T) {
				notFoundTests := map[string]struct {
					finality rpc.TxnStatus
					hash     *felt.Felt
				}{
					"verified": {
						finality: rpc.TxnStatusAcceptedOnL1,
						hash:     test.verifiedTxHash,
					},
					"not verified": {
						finality: rpc.TxnStatusAcceptedOnL2,
						hash:     test.nonVerifiedTxHash,
					},
				}

				for description, notFoundTest := range notFoundTests {
					t.Run(description, func(t *testing.T) {
						mockReader := mocks.NewMockReader(mockCtrl)
						mockReader.EXPECT().TransactionByHash(notFoundTest.hash).Return(nil, db.ErrKeyNotFound).Times(2)

						mockSyncer := mocks.NewMockSyncReader(mockCtrl)
						mockSyncer.EXPECT().PendingBlock().Return(nil).Times(2)

						handler := rpc.New(mockReader, mockSyncer, nil, "", test.network, nil)
						_, err := handler.TransactionStatus(ctx, *notFoundTest.hash)
						require.Equal(t, rpccore.ErrTxnHashNotFound.Code, err.Code)

						handler = handler.WithFeeder(client)
						status, err := handler.TransactionStatus(ctx, *notFoundTest.hash)
						require.Nil(t, err)
						require.Equal(t, notFoundTest.finality, status.Finality)
						require.Equal(t, rpc.TxnSuccess, status.Execution)
					})
				}
			})

			// transaction no† found in db and feeder
			t.Run("transaction not found in db and feeder  ", func(t *testing.T) {
				mockReader := mocks.NewMockReader(mockCtrl)
				mockReader.EXPECT().TransactionByHash(test.notFoundTxHash).Return(nil, db.ErrKeyNotFound)

				mockSyncer := mocks.NewMockSyncReader(mockCtrl)
				mockSyncer.EXPECT().PendingBlock().Return(nil)

				handler := rpc.New(mockReader, mockSyncer, nil, "", test.network, nil).WithFeeder(client)

				_, err := handler.TransactionStatus(ctx, *test.notFoundTxHash)
				require.NotNil(t, err)
				require.Equal(t, err, rpccore.ErrTxnHashNotFound)
			})
		})
	}
}
