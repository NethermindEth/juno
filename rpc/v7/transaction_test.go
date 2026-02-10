package rpcv7_test

import (
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
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTransactionByBlockIdAndIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	latestBlockNumber := 19199
	latestBlock, err := mainnetGw.BlockByNumber(t.Context(), 19199)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	handler := rpc.New(mockReader, mockSyncReader, nil, n, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockNumberByHash(gomock.Any()).Return(uint64(0), db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(
			rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))}, rand.Int())

		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(gomock.Any(),
			gomock.Any()).Return(nil, errors.New("invalid index"))

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Number: rand.Uint64()}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrInvalidTxIndex, rpcErr)
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
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, n, nil)

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, index)
		require.Nil(t, rpcErr)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, adaptV6TxToV7(t, txn2))
	})

	t.Run("blockID - hash", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().BlockNumberByHash(latestBlockHash).Return(latestBlock.Number, nil)
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
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, n, nil)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, adaptV6TxToV7(t, txn2))
	})

	t.Run("blockID - number", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

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
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, n, nil)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, adaptV6TxToV7(t, txn2))
	})

	t.Run("blockID - pending", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		pending := core.NewPending(latestBlock, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&pending,
			nil,
		)

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Pending: true}, index)
		require.Nil(t, rpcErr)

		// Use v6 handler as v7 `starknet_getTransactionByHash` uses v6 handler
		mockSyncReaderV6 := mocks.NewMockSyncReader(mockCtrl)
		mockReaderV6 := mocks.NewMockReader(mockCtrl)
		mockReaderV6.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})
		rpcv6Handler := rpcv6.New(mockReaderV6, mockSyncReaderV6, nil, n, nil)

		txn2, rpcErr := rpcv6Handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, adaptV6TxToV7(t, txn2))
	})
}

// Convert a v6 transaction object to a v7 transaction object
func adaptV6TxToV7(t *testing.T, tx *rpcv6.Transaction) *rpc.Transaction {
	t.Helper()

	var v7ResourceBounds *rpcv6.ResourceBoundsMap
	if tx.ResourceBounds != nil {
		v7ResourceBounds = &rpcv6.ResourceBoundsMap{
			L1Gas: &rpcv6.ResourceBounds{
				MaxAmount:       tx.ResourceBounds.L1Gas.MaxAmount,
				MaxPricePerUnit: tx.ResourceBounds.L1Gas.MaxPricePerUnit,
			},
			L2Gas: &rpcv6.ResourceBounds{
				MaxAmount:       tx.ResourceBounds.L2Gas.MaxAmount,
				MaxPricePerUnit: tx.ResourceBounds.L2Gas.MaxPricePerUnit,
			},
		}
	}

	var v7NonceDAMode *rpc.DataAvailabilityMode
	if tx.NonceDAMode != nil {
		v7NonceDAMode = utils.HeapPtr(rpc.DataAvailabilityMode(*tx.NonceDAMode))
	}

	var v7FeeDAMode *rpc.DataAvailabilityMode
	if tx.FeeDAMode != nil {
		v7FeeDAMode = utils.HeapPtr(rpc.DataAvailabilityMode(*tx.FeeDAMode))
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

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncer := mocks.NewMockSyncReader(mockCtrl)
	handler := rpc.New(mockReader, mockSyncer, nil, n, nil)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
		mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)
		tx, rpcErr := handler.TransactionReceiptByHash(*txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	block0, err := mainnetGw.BlockByNumber(t.Context(), 0)
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
			mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

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
		pending := core.NewPending(block0, nil, nil)
		mockSyncer.EXPECT().PendingData().Return(
			&pending,
			nil,
		)

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
		mockReader.EXPECT().L1Head().Return(core.L1Head{
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

		blockWithRevertedTxn, err := integGw.BlockByNumber(t.Context(), 304740)
		require.NoError(t, err)

		revertedTxnIdx := 1
		revertedTxnHash := blockWithRevertedTxn.Transactions[revertedTxnIdx].Hash()

		mockReader.EXPECT().TransactionByHash(revertedTxnHash).Return(blockWithRevertedTxn.Transactions[revertedTxnIdx], nil)
		mockReader.EXPECT().Receipt(revertedTxnHash).Return(blockWithRevertedTxn.Receipts[revertedTxnIdx],
			blockWithRevertedTxn.Hash, blockWithRevertedTxn.Number, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

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

		block, err := integGw.BlockByNumber(t.Context(), 319132)
		require.NoError(t, err)

		index := 0
		txnHash := block.Transactions[index].Hash()

		mockReader.EXPECT().TransactionByHash(txnHash).Return(block.Transactions[index], nil)
		mockReader.EXPECT().Receipt(txnHash).Return(block.Receipts[index],
			block.Hash, block.Number, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

		checkTxReceipt(t, txnHash, expected)
	})
}

//nolint:dupl
func TestLegacyTransactionReceiptByHash(t *testing.T) {
	t.Skip()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, n, nil)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

		tx, rpcErr := handler.TransactionReceiptByHash(*txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	block0, err := mainnetGw.BlockByNumber(t.Context(), 0)
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
			mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

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
		mockReader.EXPECT().L1Head().Return(core.L1Head{
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

		blockWithRevertedTxn, err := integGw.BlockByNumber(t.Context(), 304740)
		require.NoError(t, err)

		revertedTxnIdx := 1
		revertedTxnHash := blockWithRevertedTxn.Transactions[revertedTxnIdx].Hash()

		mockReader.EXPECT().TransactionByHash(revertedTxnHash).Return(blockWithRevertedTxn.Transactions[revertedTxnIdx], nil)
		mockReader.EXPECT().Receipt(revertedTxnHash).Return(blockWithRevertedTxn.Receipts[revertedTxnIdx],
			blockWithRevertedTxn.Hash, blockWithRevertedTxn.Number, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

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

		block, err := integGw.BlockByNumber(t.Context(), 319132)
		require.NoError(t, err)

		index := 0
		txnHash := block.Transactions[index].Hash()

		mockReader.EXPECT().TransactionByHash(txnHash).Return(block.Transactions[index], nil)
		mockReader.EXPECT().Receipt(txnHash).Return(block.Receipts[index],
			block.Hash, block.Number, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)

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

func TestAdaptTransaction(t *testing.T) {
	t.Run("core.Resource `ResourceL1DataGas` should be ignored when converted to v7.Resource", func(t *testing.T) {
		coreTx := core.InvokeTransaction{
			Version: new(core.TransactionVersion).SetUint64(3),
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas:     {MaxAmount: 1, MaxPricePerUnit: new(felt.Felt).SetUint64(2)},
				core.ResourceL2Gas:     {MaxAmount: 3, MaxPricePerUnit: new(felt.Felt).SetUint64(4)},
				core.ResourceL1DataGas: {MaxAmount: 5, MaxPricePerUnit: new(felt.Felt).SetUint64(6)},
			},
		}

		tx := rpc.AdaptTransaction(&coreTx)

		expectedTx := &rpc.Transaction{
			Type:    rpc.TxnInvoke,
			Version: new(felt.Felt).SetUint64(3),
			ResourceBounds: &rpcv6.ResourceBoundsMap{
				L1Gas: &rpcv6.ResourceBounds{MaxAmount: new(felt.Felt).SetUint64(1), MaxPricePerUnit: new(felt.Felt).SetUint64(2)},
				L2Gas: &rpcv6.ResourceBounds{MaxAmount: new(felt.Felt).SetUint64(3), MaxPricePerUnit: new(felt.Felt).SetUint64(4)},
			},
			Tip: new(felt.Felt).SetUint64(0),
			// Those 4 fields are pointers to slice (the SliceHeader is allocated, it just refers to a nil array)
			Signature:             new([]*felt.Felt),
			CallData:              new([]*felt.Felt),
			PaymasterData:         new([]*felt.Felt),
			AccountDeploymentData: new([]*felt.Felt),
			NonceDAMode:           utils.HeapPtr(rpc.DAModeL1),
			FeeDAMode:             utils.HeapPtr(rpc.DAModeL1),
		}

		require.Equal(t, expectedTx, tx)
	})
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
			network:           &utils.Mainnet,
			verifiedTxHash:    felt.NewUnsafeFromString[felt.Felt]("0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e"),
			nonVerifiedTxHash: felt.NewUnsafeFromString[felt.Felt]("0x6c40890743aa220b10e5ee68cef694c5c23cc2defd0dbdf5546e687f9982ab1"),
			notFoundTxHash:    felt.NewUnsafeFromString[felt.Felt]("0x8c96a2b3d73294667e489bf8904c6aa7c334e38e24ad5a721c7e04439ff9"),
		},
		{
			network:           &utils.Integration,
			verifiedTxHash:    felt.NewUnsafeFromString[felt.Felt]("0x5e91283c1c04c3f88e4a98070df71227fb44dea04ce349c7eb379f85a10d1c3"),
			nonVerifiedTxHash: felt.NewUnsafeFromString[felt.Felt]("0x45d9c2c8e01bacae6dec3438874576a4a1ce65f1d4247f4e9748f0e7216838"),
			notFoundTxHash:    felt.NewUnsafeFromString[felt.Felt]("0xd7747f3d0ce84b3a19b05b987a782beac22c54e66773303e94ea78cc3c15"),
		},
	}

	ctx := t.Context()

	for _, test := range tests {
		t.Run(test.network.String(), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			client := feeder.NewTestClient(t, test.network)

			t.Run("tx found in db", func(t *testing.T) {
				gw := adaptfeeder.New(client)

				block, err := gw.BlockLatest(t.Context())
				require.NoError(t, err)

				tx := block.Transactions[0]

				t.Run("not verified", func(t *testing.T) {
					mockReader := mocks.NewMockReader(mockCtrl)
					mockReader.EXPECT().TransactionByHash(tx.Hash()).Return(tx, nil)
					mockReader.EXPECT().Receipt(tx.Hash()).Return(block.Receipts[0], block.Hash, block.Number, nil)
					mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil)

					handler := rpc.New(mockReader, nil, nil, test.network, nil)

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
					mockReader.EXPECT().L1Head().Return(core.L1Head{
						BlockNumber: block.Number + 1,
					}, nil)

					handler := rpc.New(mockReader, nil, nil, test.network, nil)

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
						mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).Times(2)
						mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound).Times(2)
						handler := rpc.New(mockReader, mockSyncer, nil, test.network, nil)
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
				mockSyncer.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
				mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

				handler := rpc.New(mockReader, mockSyncer, nil, test.network, nil).WithFeeder(client)

				_, err := handler.TransactionStatus(ctx, *test.notFoundTxHash)
				require.NotNil(t, err)
				require.Equal(t, err, rpccore.ErrTxnHashNotFound)
			})
		})
	}
}

func TestAdaptBroadcastedTransaction(t *testing.T) {
	txnNonZeroL2GasData := `{
			"type": "DEPLOY_ACCOUNT",
			"version": "0x3",
			"signature": [
				"0x63c0e0fe22d6e82187b84e06f33644f7dc6edce494a317bfcdd0bb57ab862fa",
				"0x6219aa7d091eac96f07d7d195f12eff9a8786af85ddf41028428ee8f510e75e"
			],
			"nonce": "0x1",
			"contract_address_salt": "0x520b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb",
			"constructor_calldata": [
				"0x33444ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2",
				"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463",
				"0x2",
				"0x510b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb",
				"0x0"
			],
			"class_hash": "0x26ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918",
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
			"tip": "0x1",
			"paymaster_data": [],
			"nonce_data_availability_mode": "L1",
			"fee_data_availability_mode": "L2"
		}`
	expectedTxn := core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     felt.NewUnsafeFromString[felt.Felt]("0x7ed2723f72842192aea186a6b6f388fbe7b305e450d3ebd6da6b13ff1b59353"),
			ContractAddressSalt: felt.NewUnsafeFromString[felt.Felt]("0x520b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb"),
			ClassHash:           felt.NewUnsafeFromString[felt.Felt]("0x26ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918"),
			ContractAddress:     felt.NewUnsafeFromString[felt.Felt]("0x55e3ecdbd8f0b537b3cf6c31a77dff63ddfd5bf5dcc5ba7eb4d09e91fbe0f91"),
			ConstructorCallData: []*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x33444ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2"),
				felt.NewUnsafeFromString[felt.Felt]("0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"),
				felt.NewUnsafeFromString[felt.Felt]("0x2"),
				felt.NewUnsafeFromString[felt.Felt]("0x510b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb"),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
			},
			Version: new(core.TransactionVersion).SetUint64(3),
		},
		TransactionSignature: []*felt.Felt{
			felt.NewUnsafeFromString[felt.Felt]("0x63c0e0fe22d6e82187b84e06f33644f7dc6edce494a317bfcdd0bb57ab862fa"),
			felt.NewUnsafeFromString[felt.Felt]("0x6219aa7d091eac96f07d7d195f12eff9a8786af85ddf41028428ee8f510e75e"),
		},
		Nonce: felt.NewUnsafeFromString[felt.Felt]("0x1"),
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: {
				MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x6fde2b4eb000").Uint64(),
				MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x6fde2b4eb000"),
			},
			core.ResourceL2Gas: {
				MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x0").Uint64(), // The original txn should have this field set to zero
				MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x0"),
			},
		},
		Tip:           1, // 0x1
		PaymasterData: []*felt.Felt{},
		NonceDAMode:   core.DAModeL1,
		FeeDAMode:     core.DAModeL2,
	}

	txnNonZeroL2Gas := rpc.BroadcastedTransaction{}
	require.NoError(t, json.Unmarshal([]byte(txnNonZeroL2GasData), &txnNonZeroL2Gas))

	tx, _, _, err := rpc.AdaptBroadcastedTransaction(
		t.Context(),
		nil,
		&txnNonZeroL2Gas,
		&utils.Sepolia,
	)
	require.NoError(t, err)
	resultTxn, ok := (tx).(*core.DeployAccountTransaction)
	require.True(t, ok)
	require.Equal(t, expectedTxn, *resultTxn)
}
