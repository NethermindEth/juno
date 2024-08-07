package rpc_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBlockId(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		blockIDJSON     string
		expectedBlockID rpc.BlockID
	}{
		"latest": {
			blockIDJSON: `"latest"`,
			expectedBlockID: rpc.BlockID{
				Latest: true,
			},
		},
		"pending": {
			blockIDJSON: `"pending"`,
			expectedBlockID: rpc.BlockID{
				Pending: true,
			},
		},
		"number": {
			blockIDJSON: `{ "block_number" : 123123 }`,
			expectedBlockID: rpc.BlockID{
				Number: 123123,
			},
		},
		"hash": {
			blockIDJSON: `{ "block_hash" : "0x123" }`,
			expectedBlockID: rpc.BlockID{
				Hash: new(felt.Felt).SetUint64(0x123),
			},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockID rpc.BlockID
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
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var blockID rpc.BlockID
			assert.Error(t, blockID.UnmarshalJSON([]byte(test.blockIDJSON)))
		})
	}
}

func TestBlockNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		expectedHeight := uint64(0)
		mockReader.EXPECT().Height().Return(expectedHeight, errors.New("empty blockchain"))

		num, err := handler.BlockNumber()
		assert.Equal(t, expectedHeight, num)
		assert.Equal(t, rpc.ErrNoBlock, err)
	})

	t.Run("blockchain height is 21", func(t *testing.T) {
		expectedHeight := uint64(21)
		mockReader.EXPECT().Height().Return(expectedHeight, nil)

		num, err := handler.BlockNumber()
		require.Nil(t, err)
		assert.Equal(t, expectedHeight, num)
	})
}

func TestBlockHashAndNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, err := handler.BlockHashAndNumber()
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrNoBlock, err)
	})

	t.Run("blockchain height is 147", func(t *testing.T) {
		client := feeder.NewTestClient(t, n)
		gw := adaptfeeder.New(client)

		expectedBlock, err := gw.BlockByNumber(context.Background(), 147)
		require.NoError(t, err)

		expectedBlockHashAndNumber := &rpc.BlockHashAndNumber{Hash: expectedBlock.Hash, Number: expectedBlock.Number}

		mockReader.EXPECT().Head().Return(expectedBlock, nil)

		hashAndNum, rpcErr := handler.BlockHashAndNumber()
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedBlockHashAndNumber, hashAndNum)
	})
}

func TestBlockTransactionCount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash
	expectedCount := latestBlock.TransactionCount

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Latest: true})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Number: uint64(328476)})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			Block: latestBlock,
		}, nil)

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})
}

type MockTrie struct {
	*trie.Trie
	
	feltToKeyFunc     func(*felt.Felt) *trie.Key
	nodesFromRootFunc func(*trie.Key) ([]trie.StorageNode, error)
}

func (m *MockTrie) FeltToKey(f *felt.Felt) *trie.Key {
	if m.feltToKeyFunc != nil {
		return m.feltToKeyFunc(f)
	}
	return &trie.Key{} // Return a default value or mock value
}
func (m *MockTrie) ToTrie() *trie.Trie {
    return m.Trie
}
// func (m *MockTrie) FeltToKey(f *felt.Felt) trie.Key {
// 	return m.feltToKeyFunc(f)
// }

func (m *MockTrie) NodesFromRoot(k *trie.Key) ([]trie.StorageNode, error) {
	 if m.nodesFromRootFunc != nil {
        return m.nodesFromRootFunc(k)
    }
    return nil, nil
}
// type StorageNode struct {
//     Key  *trie.Key
//     Node *trie.Node
// }

func TestGetNodesFromRoot(t *testing.T) {
	var element fp.Element
	element.SetUint64(123)
	key := felt.NewFelt(&element)
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	// convertedKey := &trie.Key{Bytes: []byte{1, 2, 3}}
	mockTrie := &MockTrie{
		Trie: &trie.Trie{},
        feltToKeyFunc: func(f *felt.Felt) *trie.Key {
            // Return a mock key
            return &trie.Key{}
        },
		nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
            // Return a mock response
            return []trie.StorageNode{}, nil
        },
    }

// 	handler := rpc.New(mockReader, nil, nil, "", nil, mockTrie.Trie)

//  trieInstance := mockTrie.ToTrie()	
	//  handler.*trie.Trie = mockTrie
	convertedKey := mockTrie.feltToKeyFunc(key)
_ = convertedKey
anotherKey := mockTrie.FeltToKey(felt.NewFelt(&fp.Element{}))
 
// 	
	// Define test cases
	tests := []struct {
		name              string
		feltToKeyFunc     func(*felt.Felt) *trie.Key
		nodesFromRootFunc func(*trie.Key) ([]trie.StorageNode, error)
		expectErr         *jsonrpc.Error
	}{
		{
			name: "successful retrieval",
			feltToKeyFunc: func(f *felt.Felt) *trie.Key {
				return convertedKey
			},
			nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
				return []trie.StorageNode{
					*trie.NewStorageNode(convertedKey, &trie.Node{}),
				}, nil
			},
			expectErr: nil,
			
		},
		{
			name: "error in NodesFromRoot",
			feltToKeyFunc: func(f *felt.Felt) *trie.Key {
				return convertedKey
			},
			nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
				return nil, fmt.Errorf("some error")
			},
			expectErr: &jsonrpc.Error{
				Code:    jsonrpc.InternalError,
				Message: "some error",
			},
		},
		{
			name: "empty trie",
			feltToKeyFunc: func(f *felt.Felt) *trie.Key {
				return convertedKey
			},
			nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
				return []trie.StorageNode{}, nil
			},
			expectErr: nil,
		},
		{
			name: "nil key",
			feltToKeyFunc: func(f *felt.Felt) *trie.Key {
				return &trie.Key{}
			},
			nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
				return nil, fmt.Errorf("nil key error")
			},
			expectErr: &jsonrpc.Error{
				Code:    jsonrpc.InternalError,
				Message: "nil key error",
			},
		},
		{
			name: "key not found",
			feltToKeyFunc: func(f *felt.Felt) *trie.Key {
				return convertedKey
			},
			nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
				return nil, fmt.Errorf("key not found")
			},
			expectErr: &jsonrpc.Error{
				Code:    jsonrpc.InternalError,
				Message: "key not found",
			},
		},
		{
			name: "multiple nodes",
			feltToKeyFunc: func(f *felt.Felt) *trie.Key {
				return convertedKey
			},
			nodesFromRootFunc: func(k *trie.Key) ([]trie.StorageNode, error) {
				return []trie.StorageNode{
					*trie.NewStorageNode(convertedKey, &trie.Node{}),
				*trie.NewStorageNode(anotherKey, &trie.Node{}),
				}, nil
			},
			expectErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTrie := &MockTrie{
				feltToKeyFunc:     tt.feltToKeyFunc,
				nodesFromRootFunc: tt.nodesFromRootFunc,
			}

			// handler := &rpc.Handler{
			// 	trie:  mockTrie,
			// }
				handler := rpc.New(nil, nil, nil, "", nil, mockTrie.Trie)

			nodes, err := handler.GetNodesFromRoot(key)
			if tt.expectErr != nil {
				assert.Nil(t, nodes)
				assert.Equal(t, tt.expectErr, err)
			} else {
				assert.NotNil(t, nodes)
				assert.Nil(t, err)
			}
		})
	}
}
func TestBlockWithTxHashes(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := utils.Ptr(utils.Mainnet)
			chain := blockchain.New(pebble.NewMemTest(t), n)
			handler := rpc.New(chain, nil, nil, "", log, nil)

			block, rpcErr := handler.BlockWithTxHashes(id)
			assert.Nil(t, block)
			assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkBlock := func(t *testing.T, b *rpc.BlockWithTxHashes) {
		t.Helper()
		assert.Equal(t, latestBlock.Hash, b.Hash)
		assert.Equal(t, latestBlock.GlobalStateRoot, b.NewRoot)
		assert.Equal(t, latestBlock.ParentHash, b.ParentHash)
		assert.Equal(t, latestBlock.SequencerAddress, b.SequencerAddress)
		assert.Equal(t, latestBlock.Timestamp, b.Timestamp)
		assert.Equal(t, len(latestBlock.Transactions), len(b.TxnHashes))
		for i := 0; i < len(latestBlock.Transactions); i++ {
			assert.Equal(t, latestBlock.Transactions[i].Hash(), b.TxnHashes[i])
		}
	}

	checkLatestBlock := func(t *testing.T, b *rpc.BlockWithTxHashes) {
		t.Helper()
		if latestBlock.Hash != nil {
			assert.Equal(t, latestBlock.Number, *b.Number)
		} else {
			assert.Nil(t, b.Number)
			assert.Equal(t, rpc.BlockPending, b.Status)
		}
		checkBlock(t, b)
	}

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Number: latestBlockNumber})
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

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		assert.Equal(t, rpc.BlockAcceptedL1, block.Status)
		checkBlock(t, block)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			Block: latestBlock,
		}, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		checkLatestBlock(t, block)
	})
}

func TestBlockWithTxs(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := utils.Ptr(utils.Mainnet)
			chain := blockchain.New(pebble.NewMemTest(t), n)
			handler := rpc.New(chain, nil, nil, "", log, nil)

			block, rpcErr := handler.BlockWithTxs(id)
			assert.Nil(t, block)
			assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(16697)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkLatestBlock := func(t *testing.T, blockWithTxHashes *rpc.BlockWithTxHashes, blockWithTxs *rpc.BlockWithTxs) {
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

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpc.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpc.BlockID{Number: latestBlockNumber})
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

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpc.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			Block: latestBlock,
		}, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})
}

func TestBlockWithTxHashesV013(t *testing.T) {
	n := utils.Ptr(utils.SepoliaIntegration)
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	blockNumber := uint64(16350)
	gw := adaptfeeder.New(feeder.NewTestClient(t, n))
	coreBlock, err := gw.BlockByNumber(context.Background(), blockNumber)
	require.NoError(t, err)
	tx, ok := coreBlock.Transactions[0].(*core.InvokeTransaction)
	require.True(t, ok)

	mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(coreBlock, nil)
	mockReader.EXPECT().L1Head().Return(&core.L1Head{}, nil)
	got, rpcErr := handler.BlockWithTxsV0_6(rpc.BlockID{Number: blockNumber})
	require.Nil(t, rpcErr)
	got.Transactions = got.Transactions[:1]

	require.Equal(t, &rpc.BlockWithTxs{
		BlockHeader: rpc.BlockHeader{
			Hash:            coreBlock.Hash,
			StarknetVersion: coreBlock.ProtocolVersion,
			NewRoot:         coreBlock.GlobalStateRoot,
			Number:          &coreBlock.Number,
			ParentHash:      coreBlock.ParentHash,
			L1GasPrice: &rpc.ResourcePrice{
				InFri: utils.HexToFelt(t, "0x17882b6aa74"),
				InWei: utils.HexToFelt(t, "0x3b9aca10"),
			},
			SequencerAddress: coreBlock.SequencerAddress,
			Timestamp:        coreBlock.Timestamp,
		},
		Status: rpc.BlockAcceptedL2,
		Transactions: []*rpc.Transaction{
			{
				Hash:               tx.Hash(),
				Type:               rpc.TxnInvoke,
				Version:            tx.Version.AsFelt(),
				Nonce:              tx.Nonce,
				MaxFee:             tx.MaxFee,
				ContractAddress:    tx.ContractAddress,
				SenderAddress:      tx.SenderAddress,
				Signature:          &tx.TransactionSignature,
				CallData:           &tx.CallData,
				EntryPointSelector: tx.EntryPointSelector,
				ResourceBounds: &map[rpc.Resource]rpc.ResourceBounds{
					rpc.ResourceL1Gas: {
						MaxAmount:       new(felt.Felt).SetUint64(tx.ResourceBounds[core.ResourceL1Gas].MaxAmount),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL1Gas].MaxPricePerUnit,
					},
					rpc.ResourceL2Gas: {
						MaxAmount:       new(felt.Felt).SetUint64(tx.ResourceBounds[core.ResourceL2Gas].MaxAmount),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL2Gas].MaxPricePerUnit,
					},
				},
				Tip:                   new(felt.Felt).SetUint64(tx.Tip),
				PaymasterData:         &tx.PaymasterData,
				AccountDeploymentData: &tx.AccountDeploymentData,
				NonceDAMode:           utils.Ptr(rpc.DataAvailabilityMode(tx.NonceDAMode)),
				FeeDAMode:             utils.Ptr(rpc.DataAvailabilityMode(tx.FeeDAMode)),
			},
		},
	}, got)
}

func TestBlockWithReceipts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	t.Run("transaction not found", func(t *testing.T) {
		blockID := rpc.BlockID{Number: 777}

		mockReader.EXPECT().BlockByNumber(blockID.Number).Return(nil, db.ErrKeyNotFound)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		assert.Nil(t, resp)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})
	t.Run("l1head failure", func(t *testing.T) {
		blockID := rpc.BlockID{Number: 777}
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(nil, err)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		assert.Nil(t, resp)
		assert.Equal(t, rpc.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})

	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	t.Run("pending block", func(t *testing.T) {
		t.Skip()
		block0, err := mainnetGw.BlockByNumber(context.Background(), 0)
		require.NoError(t, err)

		blockID := rpc.BlockID{Pending: true}

		mockReader.EXPECT().Pending().Return(blockchain.Pending{Block: block0}, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{}, nil)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		header := resp.BlockHeader

		var txsWithReceipt []rpc.TransactionWithReceipt
		for i, tx := range block0.Transactions {
			receipt := block0.Receipts[i]
			adaptedTx := rpc.AdaptTransaction(tx)

			txsWithReceipt = append(txsWithReceipt, rpc.TransactionWithReceipt{
				Transaction: adaptedTx,
				Receipt:     rpc.AdaptReceipt(receipt, tx, rpc.TxnAcceptedOnL2, nil, 0, true),
			})
		}

		assert.Nil(t, rpcErr)
		assert.Equal(t, &rpc.BlockWithReceipts{
			Status: rpc.BlockPending,
			BlockHeader: rpc.BlockHeader{
				Hash:             header.Hash,
				ParentHash:       header.ParentHash,
				Number:           header.Number,
				NewRoot:          header.NewRoot,
				Timestamp:        header.Timestamp,
				SequencerAddress: header.SequencerAddress,
				L1GasPrice:       header.L1GasPrice,
				StarknetVersion:  header.StarknetVersion,
			},
			Transactions: txsWithReceipt,
		}, resp)
	})

	t.Run("accepted L1 block", func(t *testing.T) {
		t.Skip()
		block1, err := mainnetGw.BlockByNumber(context.Background(), 1)
		require.NoError(t, err)

		blockID := rpc.BlockID{Number: block1.Number}

		mockReader.EXPECT().BlockByNumber(blockID.Number).Return(block1, nil)
		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: block1.Number + 1,
		}, nil)

		resp, rpcErr := handler.BlockWithReceipts(blockID)
		header := resp.BlockHeader

		var transactions []rpc.TransactionWithReceipt
		for i, tx := range block1.Transactions {
			receipt := block1.Receipts[i]
			adaptedTx := rpc.AdaptTransaction(tx)

			transactions = append(transactions, rpc.TransactionWithReceipt{
				Transaction: adaptedTx,
				Receipt:     rpc.AdaptReceipt(receipt, tx, rpc.TxnAcceptedOnL1, nil, 0, true),
			})
		}

		assert.Nil(t, rpcErr)
		assert.Equal(t, &rpc.BlockWithReceipts{
			Status: rpc.BlockAcceptedL1,
			BlockHeader: rpc.BlockHeader{
				Hash:             header.Hash,
				ParentHash:       header.ParentHash,
				Number:           header.Number,
				NewRoot:          header.NewRoot,
				Timestamp:        header.Timestamp,
				SequencerAddress: header.SequencerAddress,
				L1GasPrice:       header.L1GasPrice,
				StarknetVersion:  header.StarknetVersion,
			},
			Transactions: transactions,
		}, resp)
	})
}
func TestBlockWithTxHashesandReceipts(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := utils.Ptr(utils.Mainnet)
			chain := blockchain.New(pebble.NewMemTest(t), n)
			handler := rpc.New(chain, nil, nil, "", log, nil)

			block, rpcErr := handler.BlockWithTxHashesandReceipts(id)
			assert.Nil(t, block)
			assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkBlock := func(t *testing.T, b *rpc.BlockWithTxHashesandReceipts) {
		t.Helper()
		assert.Equal(t, latestBlock.Hash, b.Hash)
		assert.Equal(t, latestBlock.GlobalStateRoot, b.NewRoot)
		assert.Equal(t, latestBlock.ParentHash, b.ParentHash)
		assert.Equal(t, latestBlock.SequencerAddress, b.SequencerAddress)
		assert.Equal(t, latestBlock.Timestamp, b.Timestamp)
		assert.Equal(t, len(latestBlock.Transactions), len(b.TxnHashes))
		for i := 0; i < len(latestBlock.Transactions); i++ {
			assert.Equal(t, latestBlock.Transactions[i].Hash(), b.TxnHashes[i])
		}
	}

	checkLatestBlock := func(t *testing.T, b *rpc.BlockWithTxHashesandReceipts) {
		t.Helper()
		if latestBlock.Hash != nil {
			assert.Equal(t, latestBlock.Number, *b.Number)
		} else {
			assert.Nil(t, b.Number)
			assert.Equal(t, rpc.BlockPending, b.Status)
		}
		checkBlock(t, b)
	}

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashesandReceipts(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashesandReceipts(rpc.BlockID{Hash: latestBlockHash})
		require.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashesandReceipts(rpc.BlockID{Number: latestBlockNumber})
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

		block, rpcErr := handler.BlockWithTxHashesandReceipts(rpc.BlockID{Number: latestBlockNumber})
		require.Nil(t, rpcErr)

		assert.Equal(t, rpc.BlockAcceptedL1, block.Status)
		checkBlock(t, block)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			Block: latestBlock,
		}, nil)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashesandReceipts(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		checkLatestBlock(t, block)
	})
}
func TestRpcBlockAdaptation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.Ptr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil, nil)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)
	latestBlockNumber := uint64(4850)

	t.Run("default sequencer address", func(t *testing.T) {
		latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
		require.NoError(t, err)
		latestBlock.Header.SequencerAddress = nil
		mockReader.EXPECT().Head().Return(latestBlock, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).Times(2)

		block, rpcErr := handler.BlockWithTxs(rpc.BlockID{Latest: true})
		require.NoError(t, err, rpcErr)
		require.Equal(t, &felt.Zero, block.BlockHeader.SequencerAddress)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Latest: true})
		require.NoError(t, err, rpcErr)
		require.Equal(t, &felt.Zero, blockWithTxHashes.BlockHeader.SequencerAddress)
	})
}
