package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nopCloser() error { return nil }

func TestChainId(t *testing.T) {
	for _, n := range []utils.Network{utils.MAINNET, utils.GOERLI, utils.GOERLI2, utils.INTEGRATION} {
		t.Run(n.String(), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockReader := mocks.NewMockReader(mockCtrl)
			handler := rpc.New(mockReader, nil, n, nil, nil, nil, "", nil)

			cID, err := handler.ChainID()
			require.Nil(t, err)
			assert.Equal(t, n.ChainID(), cID)
		})
	}
}

func TestBlockNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

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

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, err := handler.BlockHashAndNumber()
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrNoBlock, err)
	})

	t.Run("blockchain height is 147", func(t *testing.T) {
		client := feeder.NewTestClient(t, utils.MAINNET)
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

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.GOERLI, nil, nil, nil, "", nil)

	client := feeder.NewTestClient(t, utils.GOERLI)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(485004)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash
	expectedCount := latestBlock.TransactionCount

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Latest: true})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		count, rpcErr := handler.BlockTransactionCount(rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

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

func TestBlockWithTxHashes(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.GOERLI, nil, nil, nil, "", nil)

	client := feeder.NewTestClient(t, utils.GOERLI)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(485004)
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

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Latest: true})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxHashes(rpc.BlockID{Number: uint64(328476)})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

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
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

	client := feeder.NewTestClient(t, utils.MAINNET)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(16697)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, rpcErr := handler.BlockWithTxs(rpc.BlockID{Latest: true})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxs(rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxs(rpc.BlockID{Number: uint64(328476)})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	checkLatestBlock := func(t *testing.T, blockWithTxHashes *rpc.BlockWithTxHashes, blockWithTxs *rpc.BlockWithTxs) {
		t.Helper()
		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		for i, txnHash := range blockWithTxHashes.TxnHashes {
			assert.Equal(t, txnHash, blockWithTxs.Transactions[i].Hash)

			txn, err := handler.TransactionByHash(*blockWithTxs.Transactions[i].Hash)
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

func TestTransactionByHash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)

	client := feeder.NewTestClient(t, utils.MAINNET)
	mainnetGw := adaptfeeder.New(client)

	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

		tx, rpcErr := handler.TransactionByHash(*txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpc.ErrTxnHashNotFound, rpcErr)
	})

	tests := map[string]struct {
		hash     string
		expected string
	}{
		"DECLARE v1": {
			hash: "0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae",
			expected: `{
			"type": "DECLARE",
			"transaction_hash": "0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae",
			"max_fee": "0xf6dbd653833",
			"version": "0x1",
			"signature": [
		"0x221b9576c4f7b46d900a331d89146dbb95a7b03d2eb86b4cdcf11331e4df7f2",
		"0x667d8062f3574ba9b4965871eec1444f80dacfa7114e1d9c74662f5672c0620"
		],
		"nonce": "0x5",
		"class_hash": "0x7aed6898458c4ed1d720d43e342381b25668ec7c3e8837f761051bf4d655e54",
		"sender_address": "0x39291faa79897de1fd6fb1a531d144daa1590d058358171b83eadb3ceafed8"
		}`,
		},

		"DECLARE v0": {
			hash: "0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4",
			expected: `{
				"type": "DECLARE",
				"transaction_hash": "0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4",
				"max_fee": "0x0",
				"version": "0x0",
				"signature": [],
				"class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0",
				"sender_address": "0x1"
			}`,
		},

		"L1 Handler v0": {
			hash: "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8",
			expected: `{
       "type": "L1_HANDLER",
       "transaction_hash": "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8",
       "version": "0x0",
       "nonce": "0x2",
       "contract_address": "0xda8054260ec00606197a4103eb2ef08d6c8af0b6a808b610152d1ce498f8c3",
       "entry_point_selector": "0xc73f681176fc7b3f9693986fd7b14581e8d540519e27400e88b8713932be01",
       "calldata": [
           "0x142273bcbfca76512b2a05aed21f134c4495208",
           "0x160c35f9f962e1bc997f9133d9fb231afd5799f7d63dcbcd506af4866b3874",
           "0x16345785d8a0000",
           "0x0",
           "0x3"
       ]
   }`,
		},

		"Invoke v1": {
			hash: "0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497",
			expected: `{
       "type": "INVOKE",
       "transaction_hash": "0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497",
       "max_fee": "0x17f0de82f4be6",
       "version": "0x1",
       "signature": [
           "0x383ba105b6d0f59fab96a412ad267213ddcd899e046278bdba64cd583d680b",
           "0x1896619a17fde468978b8d885ffd6f5c8f4ac1b188233b81b91bcf7dbc56fbd"
       ],
       "nonce": "0x42",
       "sender_address": "0x1fc039de7d864580b57a575e8e6b7114f4d2a954d7d29f876b2eb3dd09394a0",
       "calldata": [
           "0x1",
           "0x727a63f78ee3f1bd18f78009067411ab369c31dece1ae22e16f567906409905",
           "0x22de356837ac200bca613c78bd1fcc962a97770c06625f0c8b3edeb6ae4aa59",
           "0x0",
           "0xb",
           "0xb",
           "0xa",
           "0x6db793d93ce48bc75a5ab02e6a82aad67f01ce52b7b903090725dbc4000eaa2",
           "0x6141eac4031dfb422080ed567fe008fb337b9be2561f479a377aa1de1d1b676",
           "0x27eb1a21fa7593dd12e988c9dd32917a0dea7d77db7e89a809464c09cf951c0",
           "0x400a29400a34d8f69425e1f4335e6a6c24ce1111db3954e4befe4f90ca18eb7",
           "0x599e56821170a12cdcf88fb8714057ce364a8728f738853da61d5b3af08a390",
           "0x46ad66f467df625f3b2dd9d3272e61713e8f74b68adac6718f7497d742cfb17",
           "0x4f348b585e6c1919d524a4bfe6f97230ecb61736fe57534ec42b628f7020849",
           "0x19ae40a095ffe79b0c9fc03df2de0d2ab20f59a2692ed98a8c1062dbf691572",
           "0xe120336994adef6c6e47694f87278686511d4622997d4a6f216bd6e9fa9acc",
           "0x56e6637a4958d062db8c8198e315772819f64d915e5c7a8d58a99fa90ff0742"
       ]
   }`,
		},

		"DEPLOY v0": {
			hash: "0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0",
			expected: `{
       "type": "DEPLOY",
       "transaction_hash": "0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0",
       "version": "0x0",
       "class_hash": "0x46f844ea1a3b3668f81d38b5c1bd55e816e0373802aefe732138628f0133486",
       "contract_address_salt": "0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b",
       "constructor_calldata": [
           "0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4",
           "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463",
           "0x2",
           "0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6",
           "0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328"
       ]
   }`,
		},

		"DEPLOY ACCOUNT v1": {
			hash: "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2",
			expected: `{
       "type": "DEPLOY_ACCOUNT",
       "transaction_hash": "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2",
       "max_fee": "0xb5e620f48000",
       "version": "0x1",
       "signature": [
           "0x41c3543008dd65ed98c767e5d218b0c0ce1bd0cd60877824951a6f87cc1637d",
           "0x7f803845aa7e43d183fd05cd553c64711b1c49af69a155fe8144e8da9a5a50d"
       ],
       "nonce": "0x0",
       "class_hash": "0x1fac3074c9d5282f0acc5c69a4781a1c711efea5e73c550c5d9fb253cf7fd3d",
       "contract_address_salt": "0x14e2ae44cbb50dff0e18140e7c415c1f281207d06fd6a0106caf3ff21e130d8",
       "constructor_calldata": [
           "0x6113c1775f3d0fda0b45efbb69f6e2306da3c174df523ef0acdd372bf0a61cb"
       ]
   }`,
		},

		"INVOKE v0": {
			hash: "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e",
			expected: `{
       "type": "INVOKE",
       "transaction_hash": "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e",
       "max_fee": "0x0",
       "version": "0x0",
       "signature": [],
       "contract_address": "0x43324c97e376d7d164abded1af1e73e9ce8214249f711edb7059c1ca34560e8",
       "entry_point_selector": "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f",
       "calldata": [
           "0x1b654cb59f978da2eee76635158e5ff1399bf607cb2d05e3e3b4e41d7660ca2",
           "0x2",
           "0x5f743efdb29609bfc2002041bdd5c72257c0c6b5c268fc929a3e516c171c731",
           "0x635afb0ea6c4cdddf93f42287b45b67acee4f08c6f6c53589e004e118491546"
       ]
   }`,
		},
	}

	mockReader.EXPECT().TransactionByHash(gomock.Any()).DoAndReturn(func(hash *felt.Felt) (core.Transaction, error) {
		return mainnetGw.Transaction(context.Background(), hash)
	}).Times(len(tests))

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hash, err := new(felt.Felt).SetString(test.hash)
			require.NoError(t, err)

			expectedMap := make(map[string]any)
			require.NoError(t, json.Unmarshal([]byte(test.expected), &expectedMap))

			res, rpcErr := handler.TransactionByHash(*hash)
			require.Nil(t, rpcErr)

			resJSON, err := json.Marshal(res)
			require.NoError(t, err)
			resMap := make(map[string]any)
			require.NoError(t, json.Unmarshal(resJSON, &resMap))

			assert.Equal(t, expectedMap, resMap, string(resJSON))
		})
	}
}

func TestTransactionByBlockIdAndIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	client := feeder.NewTestClient(t, utils.MAINNET)
	mainnetGw := adaptfeeder.New(client)

	latestBlockNumber := 19199
	latestBlock, err := mainnetGw.BlockByNumber(context.Background(), 19199)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(
			rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Number: rand.Uint64()}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("negative index", func(t *testing.T) {
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, -1)
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("invalid index", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			latestBlock.TransactionCount).Return(nil, errors.New("invalid index"))

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, len(latestBlock.Transactions))
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})
		mockReader.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, index)
		require.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})
		mockReader.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Hash: latestBlockHash}, index)
		require.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})

	t.Run("blockID - number", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().BlockHeaderByNumber(uint64(latestBlockNumber)).Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})
		mockReader.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Number: uint64(latestBlockNumber)}, index)
		require.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})

	t.Run("blockID - pending", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			Block: latestBlock,
		}, nil)
		mockReader.EXPECT().TransactionByHash(latestBlock.Transactions[index].Hash()).DoAndReturn(
			func(hash *felt.Felt) (core.Transaction, error) {
				return latestBlock.Transactions[index], nil
			})

		txn1, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Pending: true}, index)
		require.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(*latestBlock.Transactions[index].Hash())
		require.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})
}

func TestTransactionReceiptByHash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

		tx, rpcErr := handler.TransactionReceiptByHash(*txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpc.ErrTxnHashNotFound, rpcErr)
	})

	client := feeder.NewTestClient(t, utils.MAINNET)
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
					"actual_fee": "0x0",
					"finality_status": "ACCEPTED_ON_L2",
					"execution_status": "SUCCEEDED",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [],
					"events": [],
					"contract_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6"
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
					"events": []
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
					"events": []
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
					"events": []
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
			"revert_reason": "Error in the called contract (0x00b1461de04c6a1aa3375bdf9b7723a8779c082ffe21311d683a0b15c078b5dc):\nError at pc=0:25:\nGot an exception while executing a hint.\nCairo traceback (most recent call last):\nUnknown location (pc=0:731)\nUnknown location (pc=0:677)\nUnknown location (pc=0:291)\nUnknown location (pc=0:314)\n\nError in the called contract (0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7):\nError at pc=0:104:\nGot an exception while executing a hint.\nCairo traceback (most recent call last):\nUnknown location (pc=0:1678)\nUnknown location (pc=0:1664)\n\nError in the called contract (0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7):\nError at pc=0:6:\nGot an exception while executing a hint: Assertion failed, 0 % 0x800000000000011000000000000000000000000000000000000000000000001 is equal to 0\nCairo traceback (most recent call last):\nUnknown location (pc=0:1238)\nUnknown location (pc=0:1215)\nUnknown location (pc=0:836)\n"
		}`

		integClient := feeder.NewTestClient(t, utils.INTEGRATION)
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
}

func TestStateUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Height().Return(uint64(0), errors.New("empty blockchain"))

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Latest: true})
		assert.Nil(t, update)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Nil(t, update)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Number: uint64(328476)})
		assert.Nil(t, update)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	client := feeder.NewTestClient(t, utils.MAINNET)
	mainnetGw := adaptfeeder.New(client)

	update21656, err := mainnetGw.StateUpdate(context.Background(), 21656)
	require.NoError(t, err)

	checkUpdate := func(t *testing.T, coreUpdate *core.StateUpdate, rpcUpdate *rpc.StateUpdate) {
		t.Helper()
		assert.Equal(t, coreUpdate.BlockHash, rpcUpdate.BlockHash)
		assert.Equal(t, coreUpdate.NewRoot, rpcUpdate.NewRoot)
		assert.Equal(t, coreUpdate.OldRoot, rpcUpdate.OldRoot)

		assert.Equal(t, len(coreUpdate.StateDiff.StorageDiffs), len(rpcUpdate.StateDiff.StorageDiffs))
		for _, diff := range rpcUpdate.StateDiff.StorageDiffs {
			coreDiffs := coreUpdate.StateDiff.StorageDiffs[*diff.Address]
			assert.Equal(t, len(coreDiffs), len(diff.StorageEntries))
			for index, entry := range diff.StorageEntries {
				assert.Equal(t, entry.Key, coreDiffs[index].Key)
				assert.Equal(t, entry.Value, coreDiffs[index].Value)
			}
		}

		assert.Equal(t, len(coreUpdate.StateDiff.Nonces), len(rpcUpdate.StateDiff.Nonces))
		for _, nonce := range rpcUpdate.StateDiff.Nonces {
			assert.Equal(t, coreUpdate.StateDiff.Nonces[*nonce.ContractAddress], nonce.Nonce)
		}

		assert.Equal(t, len(coreUpdate.StateDiff.DeployedContracts), len(rpcUpdate.StateDiff.DeployedContracts))
		for index := range rpcUpdate.StateDiff.DeployedContracts {
			assert.Equal(t, coreUpdate.StateDiff.DeployedContracts[index].Address,
				rpcUpdate.StateDiff.DeployedContracts[index].Address)
			assert.Equal(t, coreUpdate.StateDiff.DeployedContracts[index].ClassHash,
				rpcUpdate.StateDiff.DeployedContracts[index].ClassHash)
		}

		assert.Equal(t, coreUpdate.StateDiff.DeclaredV0Classes, rpcUpdate.StateDiff.DeprecatedDeclaredClasses)

		assert.Equal(t, len(coreUpdate.StateDiff.ReplacedClasses), len(rpcUpdate.StateDiff.ReplacedClasses))
		for index := range rpcUpdate.StateDiff.ReplacedClasses {
			assert.Equal(t, coreUpdate.StateDiff.ReplacedClasses[index].Address,
				rpcUpdate.StateDiff.ReplacedClasses[index].ContractAddress)
			assert.Equal(t, coreUpdate.StateDiff.ReplacedClasses[index].ClassHash,
				rpcUpdate.StateDiff.ReplacedClasses[index].ClassHash)
		}

		assert.Equal(t, len(coreUpdate.StateDiff.DeclaredV1Classes), len(rpcUpdate.StateDiff.DeclaredClasses))
		for index := range rpcUpdate.StateDiff.DeclaredClasses {
			assert.Equal(t, coreUpdate.StateDiff.DeclaredV1Classes[index].CompiledClassHash,
				rpcUpdate.StateDiff.DeclaredClasses[index].CompiledClassHash)
			assert.Equal(t, coreUpdate.StateDiff.DeclaredV1Classes[index].ClassHash,
				rpcUpdate.StateDiff.DeclaredClasses[index].ClassHash)
		}
	}

	t.Run("latest", func(t *testing.T) {
		mockReader.EXPECT().Height().Return(uint64(21656), nil)
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("by height", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Number: uint64(21656)})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("by hash", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByHash(update21656.BlockHash).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Hash: update21656.BlockHash})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("post v0.11.0", func(t *testing.T) {
		integrationClient := feeder.NewTestClient(t, utils.INTEGRATION)
		integGw := adaptfeeder.New(integrationClient)

		for name, height := range map[string]uint64{
			"declared Cairo0 classes": 283746,
			"declared Cairo1 classes": 283364,
			"replaced classes":        283428,
		} {
			t.Run(name, func(t *testing.T) {
				gwUpdate, err := integGw.StateUpdate(context.Background(), height)
				require.NoError(t, err)

				mockReader.EXPECT().StateUpdateByNumber(height).Return(gwUpdate, nil)

				update, rpcErr := handler.StateUpdate(rpc.BlockID{Number: height})
				require.Nil(t, rpcErr)

				checkUpdate(t, gwUpdate, update)
			})
		}
	})

	t.Run("pending", func(t *testing.T) {
		update21656.BlockHash = nil
		update21656.NewRoot = nil
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			StateUpdate: update21656,
		}, nil)

		update, rpcErr := handler.StateUpdate(rpc.BlockID{Pending: true})
		require.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})
}

func TestSyncing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, utils.MAINNET)
	gw := adaptfeeder.New(client)
	log := utils.NewNopZapLogger()
	synchronizer := sync.New(nil, gw, log, time.Duration(0))

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, synchronizer, utils.MAINNET, nil, nil, nil, "", nil)
	defaultSyncState := false

	t.Run("undefined starting block", func(t *testing.T) {
		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	startingBlock := uint64(0)
	synchronizer.StartingBlockNumber = &startingBlock
	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(startingBlock).Return(nil, errors.New("empty blockchain"))

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})
	t.Run("undefined highest block", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(startingBlock).Return(&core.Header{}, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})
	t.Run("block height is greater than highest block", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(startingBlock).Return(&core.Header{}, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})

	synchronizer.HighestBlockHeader.Store(&core.Header{Number: 2, Hash: new(felt.Felt).SetUint64(2)})
	t.Run("block height is equal to highest block", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(startingBlock).Return(&core.Header{}, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 2}, nil)

		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, &rpc.Sync{Syncing: &defaultSyncState}, syncing)
	})
	t.Run("syncing", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(startingBlock).Return(&core.Header{Hash: &felt.Zero}, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1, Hash: new(felt.Felt).SetUint64(1)}, nil)

		currentBlockNumber := uint64(1)
		highestBlockNumber := uint64(2)
		expectedSyncing := &rpc.Sync{
			StartingBlockHash:   &felt.Zero,
			StartingBlockNumber: &startingBlock,
			CurrentBlockHash:    new(felt.Felt).SetUint64(1),
			CurrentBlockNumber:  &currentBlockNumber,
			HighestBlockHash:    new(felt.Felt).SetUint64(2),
			HighestBlockNumber:  &highestBlockNumber,
		}
		syncing, err := handler.Syncing()
		assert.Nil(t, err)
		assert.Equal(t, expectedSyncing, syncing)
	})
}

func TestNonce(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, errors.New("empty blockchain"))

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, errors.New("non-existent block hash"))

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, errors.New("non-existent block number"))

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(nil, errors.New("non-existent contract"))

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedNonce := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})
}

func TestStorageAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, errors.New("empty blockchain"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, errors.New("non-existent block hash"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, errors.New("non-existent block number"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(nil, errors.New("non-existent contract"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(&felt.Zero, errors.New("non-existent key"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})
}

func TestClassHashAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, errors.New("empty blockchain"))

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, errors.New("non-existent block hash"))

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, errors.New("non-existent block number"))

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, errors.New("non-existent contract"))

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, classHash)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedClassHash := new(felt.Felt).SetUint64(3)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(expectedClassHash, nil)

		classHash, rpcErr := handler.ClassHashAt(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedClassHash, classHash)
	})
}

func assertEqualCairo0Class(t *testing.T, cairo0Class *core.Cairo0Class, class *rpc.Class) {
	assert.Equal(t, cairo0Class.Program, class.Program)
	assert.Equal(t, cairo0Class.Abi, class.Abi.(json.RawMessage))

	require.Equal(t, len(cairo0Class.L1Handlers), len(class.EntryPoints.L1Handler))
	for idx := range cairo0Class.L1Handlers {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Offset, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Constructors), len(class.EntryPoints.Constructor))
	for idx := range cairo0Class.Constructors {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo0Class.Constructors[idx].Offset, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo0Class.Constructors[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Externals), len(class.EntryPoints.External))
	for idx := range cairo0Class.Externals {
		assert.Nil(t, class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo0Class.Externals[idx].Offset, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo0Class.Externals[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}

func assertEqualCairo1Class(t *testing.T, cairo1Class *core.Cairo1Class, class *rpc.Class) {
	assert.Equal(t, cairo1Class.Program, class.SierraProgram)
	assert.Equal(t, cairo1Class.Abi, class.Abi.(string))
	assert.Equal(t, cairo1Class.SemanticVersion, class.ContractClassVersion)

	require.Equal(t, len(cairo1Class.EntryPoints.L1Handler), len(class.EntryPoints.L1Handler))
	for idx := range cairo1Class.EntryPoints.L1Handler {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Index, *class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.Constructor), len(class.EntryPoints.Constructor))
	for idx := range cairo1Class.EntryPoints.Constructor {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Index, *class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.External), len(class.EntryPoints.External))
	for idx := range cairo1Class.EntryPoints.External {
		assert.Nil(t, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Index, *class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}

func TestClass(t *testing.T) {
	integrationClient := feeder.NewTestClient(t, utils.INTEGRATION)
	integGw := adaptfeeder.New(integrationClient)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	mockState.EXPECT().Class(gomock.Any()).DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClass, error) {
		class, err := integGw.Class(context.Background(), classHash)
		return &core.DeclaredClass{Class: class, At: 0}, err
	}).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", utils.NewNopZapLogger())

	latest := rpc.BlockID{Latest: true}

	t.Run("sierra class", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")

		coreClass, err := integGw.Class(context.Background(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(latest, *hash)
		require.Nil(t, rpcErr)
		cairo1Class := coreClass.(*core.Cairo1Class)
		assertEqualCairo1Class(t, cairo1Class, class)
	})

	t.Run("casm class", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")

		coreClass, err := integGw.Class(context.Background(), hash)
		require.NoError(t, err)

		class, rpcErr := handler.Class(latest, *hash)
		require.Nil(t, rpcErr)

		cairo0Class := coreClass.(*core.Cairo0Class)
		assertEqualCairo0Class(t, cairo0Class, class)
	})
}

func TestClassAt(t *testing.T) {
	integrationClient := feeder.NewTestClient(t, utils.INTEGRATION)
	integGw := adaptfeeder.New(integrationClient)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	cairo0ContractAddress, _ := new(felt.Felt).SetRandom()
	cairo0ClassHash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")
	mockState.EXPECT().ContractClassHash(cairo0ContractAddress).Return(cairo0ClassHash, nil)

	cairo1ContractAddress, _ := new(felt.Felt).SetRandom()
	cairo1ClassHash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")
	mockState.EXPECT().ContractClassHash(cairo1ContractAddress).Return(cairo1ClassHash, nil)

	mockState.EXPECT().Class(gomock.Any()).DoAndReturn(func(classHash *felt.Felt) (*core.DeclaredClass, error) {
		class, err := integGw.Class(context.Background(), classHash)
		return &core.DeclaredClass{Class: class, At: 0}, err
	}).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil).AnyTimes()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", utils.NewNopZapLogger())

	latest := rpc.BlockID{Latest: true}

	t.Run("sierra class", func(t *testing.T) {
		coreClass, err := integGw.Class(context.Background(), cairo1ClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(latest, *cairo1ContractAddress)
		require.Nil(t, rpcErr)
		cairo1Class := coreClass.(*core.Cairo1Class)
		assertEqualCairo1Class(t, cairo1Class, class)
	})

	t.Run("casm class", func(t *testing.T) {
		coreClass, err := integGw.Class(context.Background(), cairo0ClassHash)
		require.NoError(t, err)

		class, rpcErr := handler.ClassAt(latest, *cairo0ContractAddress)
		require.Nil(t, rpcErr)

		cairo0Class := coreClass.(*core.Cairo0Class)
		assertEqualCairo0Class(t, cairo0Class, class)
	})
}

func TestEvents(t *testing.T) {
	testDB := pebble.NewMemTest()
	t.Cleanup(func() {
		require.NoError(t, testDB.Close())
	})
	chain := blockchain.New(testDB, utils.GOERLI2, utils.NewNopZapLogger())

	client := feeder.NewTestClient(t, utils.GOERLI2)
	gw := adaptfeeder.New(client)

	for i := 0; i < 7; i++ {
		b, err := gw.BlockByNumber(context.Background(), uint64(i))
		require.NoError(t, err)
		s, err := gw.StateUpdate(context.Background(), uint64(i))
		require.NoError(t, err)

		if b.Number < 6 {
			require.NoError(t, chain.Store(b, &core.BlockCommitments{}, s, nil))
		} else {
			b.Hash = nil
			b.GlobalStateRoot = nil
			require.NoError(t, chain.StorePending(&blockchain.Pending{
				Block:       b,
				StateUpdate: s,
			}))
		}
	}

	handler := rpc.New(chain, nil, utils.GOERLI2, nil, nil, nil, "", utils.NewNopZapLogger())
	from := utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
	args := rpc.EventsArg{
		EventFilter: rpc.EventFilter{
			FromBlock: &rpc.BlockID{Number: 0},
			ToBlock:   &rpc.BlockID{Latest: true},
			Address:   from,
			Keys:      [][]felt.Felt{},
		},
		ResultPageRequest: rpc.ResultPageRequest{
			ChunkSize:         100,
			ContinuationToken: "",
		},
	}

	t.Run("filter non-existent", func(t *testing.T) {
		t.Run("block number", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Number: 55}
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 3)
		})

		t.Run("block hash", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Hash: new(felt.Felt).SetUint64(55)}
			_, err := handler.Events(args)
			require.Equal(t, rpc.ErrBlockNotFound, err)
		})
	})

	t.Run("filter with no from_block", func(t *testing.T) {
		args.FromBlock = nil
		args.ToBlock = &rpc.BlockID{Latest: true}
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no to_block", func(t *testing.T) {
		args.FromBlock = &rpc.BlockID{Number: 0}
		args.ToBlock = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no address", func(t *testing.T) {
		args.ToBlock = &rpc.BlockID{Latest: true}
		args.Address = nil
		_, err := handler.Events(args)
		require.Nil(t, err)
	})

	t.Run("filter with no keys", func(t *testing.T) {
		var allEvents []*rpc.EmittedEvent
		t.Run("get canonical events without pagination", func(t *testing.T) {
			args.ToBlock = &rpc.BlockID{Latest: true}
			args.Address = from
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 2)
			require.Empty(t, events.ContinuationToken)
			allEvents = events.Events
		})

		t.Run("accumulate events with pagination", func(t *testing.T) {
			var accEvents []*rpc.EmittedEvent
			args.ChunkSize = 1

			for i := 0; i < len(allEvents)+1; i++ {
				events, err := handler.Events(args)
				require.Nil(t, err)
				accEvents = append(accEvents, events.Events...)
				args.ContinuationToken = events.ContinuationToken
				if args.ContinuationToken == "" {
					break
				}
			}
			require.Equal(t, allEvents, accEvents)
		})
	})

	t.Run("filter with keys", func(t *testing.T) {
		key := utils.HexToFelt(t, "0x3774b0545aabb37c45c1eddc6a7dae57de498aae6d5e3589e362d4b4323a533")

		t.Run("get all events without pagination", func(t *testing.T) {
			args.ChunkSize = 100
			args.Keys = append(args.Keys, []felt.Felt{*key})
			events, err := handler.Events(args)
			require.Nil(t, err)
			require.Len(t, events.Events, 1)
			require.Empty(t, events.ContinuationToken)

			require.Equal(t, from, events.Events[0].From)
			require.Equal(t, []*felt.Felt{key}, events.Events[0].Keys)
			require.Equal(t, []*felt.Felt{
				utils.HexToFelt(t, "0x2ee9bf3da86f3715e8a20429feed8e37fef58004ee5cf52baf2d8fc0d94c9c8"),
				utils.HexToFelt(t, "0x2ee9bf3da86f3715e8a20429feed8e37fef58004ee5cf52baf2d8fc0d94c9c8"),
			}, events.Events[0].Data)
			require.Equal(t, uint64(5), *events.Events[0].BlockNumber)
			require.Equal(t, utils.HexToFelt(t, "0x3b43b334f46b921938854ba85ffc890c1b1321f8fd69e7b2961b18b4260de14"), events.Events[0].BlockHash)
			require.Equal(t, utils.HexToFelt(t, "0x6d1431d875ba082365b888c1651e026012a94172b04589c91c2adeb6c1b7ace"), events.Events[0].TransactionHash)
		})
	})

	t.Run("large page size", func(t *testing.T) {
		args.ChunkSize = 10240 + 1
		events, err := handler.Events(args)
		require.Equal(t, rpc.ErrPageSizeTooBig, err)
		require.Nil(t, events)
	})

	t.Run("too many keys", func(t *testing.T) {
		args.ChunkSize = 2
		args.Keys = make([][]felt.Felt, 1024+1)
		events, err := handler.Events(args)
		require.Equal(t, rpc.ErrTooManyKeysInFilter, err)
		require.Nil(t, events)
	})

	t.Run("get pending events without pagination", func(t *testing.T) {
		args = rpc.EventsArg{
			EventFilter: rpc.EventFilter{
				FromBlock: &rpc.BlockID{Pending: true},
				ToBlock:   &rpc.BlockID{Pending: true},
			},
			ResultPageRequest: rpc.ResultPageRequest{
				ChunkSize:         100,
				ContinuationToken: "",
			},
		}
		events, err := handler.Events(args)
		require.Nil(t, err)
		require.Len(t, events.Events, 1)
		require.Empty(t, events.ContinuationToken)

		assert.Nil(t, events.Events[0].BlockHash)
		assert.Nil(t, events.Events[0].BlockNumber)
		assert.Equal(t, utils.HexToFelt(t, "0x5fe34d6903420e489b6faa8804c7a1af311446934bac1ba1e79b53cee61756c"), events.Events[0].TransactionHash)
	})
}

func TestAddTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockGateway := mocks.NewMockGateway(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(nil, nil, utils.MAINNET, mockGateway, nil, nil, "", log)

	t.Run("invalid json", func(t *testing.T) {
		_, err := handler.AddTransaction(json.RawMessage(`{]`))
		require.NotNil(t, err)
		assert.Equal(t, jsonrpc.InvalidJSON, err.Code)
	})

	t.Run("ok response", func(t *testing.T) {
		mockGateway.EXPECT().AddTransaction(gomock.Any()).Return(json.RawMessage(`
		{
			"transaction_hash" : "0x1",
			"contract_address" : "0x2",
			"class_hash" : "0x3"
		}
		`), nil)

		response, err := handler.AddTransaction(json.RawMessage(`{}`))
		require.Nil(t, err)
		assert.Equal(t, "0x1", response.TransactionHash.String())
		assert.Equal(t, "0x2", response.ContractAddress.String())
		assert.Equal(t, "0x3", response.ClassHash.String())
	})

	t.Run("compresses sierra program", func(t *testing.T) {
		declareTxV2 := `{"contract_class":{"sierra_program":["0x0","0x0"]},"type":"DECLARE","version":"0x2"}`
		gwDeclareTxV2 := `{"contract_class":{"sierra_program":"H4sIAAAAAAAA/4pWMqgwUNIBk7GAAAAA//9n6XuWDQAAAA=="},"type":"DECLARE","version":"0x2"}`

		mockGateway.EXPECT().AddTransaction(json.RawMessage(gwDeclareTxV2)).Return(json.RawMessage(`{}`), nil)

		_, err := handler.AddTransaction(json.RawMessage(declareTxV2))
		require.Nil(t, err)
	})

	t.Run("changes invoke type", func(t *testing.T) {
		invokeTxn := `{"type":"INVOKE"}`
		gwInvokeTxn := `{"type":"INVOKE_FUNCTION"}`

		mockGateway.EXPECT().AddTransaction(json.RawMessage(gwInvokeTxn)).Return(json.RawMessage(`{}`), nil)

		_, err := handler.AddTransaction(json.RawMessage(invokeTxn))
		require.Nil(t, err)
	})
}

func TestPendingTransactions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", log)

	t.Run("no pending", func(t *testing.T) {
		mockReader.EXPECT().Pending().Return(blockchain.Pending{}, errors.New("no pending"))

		txns, err := handler.PendingTransactions()
		require.Nil(t, err)
		require.Empty(t, txns)
	})

	t.Run("with pending", func(t *testing.T) {
		mockReader.EXPECT().Pending().Return(blockchain.Pending{
			Block: &core.Block{
				Transactions: []core.Transaction{
					&core.InvokeTransaction{
						TransactionHash: utils.HexToFelt(t, "0xdeadbeef"),
						Version:         utils.HexToFelt(t, "0x0"),
					},
				},
			},
		}, nil)

		txns, err := handler.PendingTransactions()
		require.Nil(t, err)
		require.Len(t, txns, 1)
		require.Equal(t, "deadbeef", txns[0].Hash.Text(16))
	})
}

func TestVersion(t *testing.T) {
	const version = "1.2.3-rc1"

	handler := rpc.New(nil, nil, utils.MAINNET, nil, nil, nil, version, nil)
	ver, err := handler.Version()
	require.Nil(t, err)
	assert.Equal(t, version, ver)
}

func TestTransactionStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	tests := []struct {
		network           utils.Network
		verifiedTxHash    *felt.Felt
		nonVerifiedTxHash *felt.Felt
		notFoundTxHash    *felt.Felt
	}{
		{
			network:           utils.MAINNET,
			verifiedTxHash:    utils.HexToFelt(t, "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e"),
			nonVerifiedTxHash: utils.HexToFelt(t, "0x6c40890743aa220b10e5ee68cef694c5c23cc2defd0dbdf5546e687f9982ab1"),
			notFoundTxHash:    utils.HexToFelt(t, "0x8c96a2b3d73294667e489bf8904c6aa7c334e38e24ad5a721c7e04439ff9"),
		},
		{
			network:           utils.INTEGRATION,
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

					handler := rpc.New(mockReader, nil, test.network, nil, nil, nil, "", nil)

					want := &rpc.TransactionStatus{
						Finality:  rpc.TxnAcceptedOnL2,
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

					handler := rpc.New(mockReader, nil, test.network, nil, nil, nil, "", nil)

					want := &rpc.TransactionStatus{
						Finality:  rpc.TxnAcceptedOnL1,
						Execution: rpc.TxnSuccess,
					}
					status, rpcErr := handler.TransactionStatus(ctx, *tx.Hash())
					require.Nil(t, rpcErr)
					require.Equal(t, want, status)
				})
			})
			t.Run("transaction not found in db", func(t *testing.T) {
				notFoundTests := map[string]struct {
					finality rpc.TxnFinalityStatus
					hash     *felt.Felt
				}{
					"verified": {
						finality: rpc.TxnAcceptedOnL1,
						hash:     test.verifiedTxHash,
					},
					"not verified": {
						finality: rpc.TxnAcceptedOnL2,
						hash:     test.nonVerifiedTxHash,
					},
				}

				for description, notFoundTest := range notFoundTests {
					t.Run(description, func(t *testing.T) {
						mockReader := mocks.NewMockReader(mockCtrl)
						mockReader.EXPECT().TransactionByHash(notFoundTest.hash).Return(nil, db.ErrKeyNotFound)
						handler := rpc.New(mockReader, nil, test.network, nil, client, nil, "", nil)

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
				handler := rpc.New(mockReader, nil, test.network, nil, client, nil, "", nil)

				_, err := handler.TransactionStatus(ctx, *test.notFoundTxHash)
				require.NotNil(t, err)
				require.Equal(t, err, rpc.ErrTxnHashNotFound)
			})
		})
	}
}

func TestCall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, errors.New("empty blockchain"))

		res, rpcErr := handler.Call(rpc.FunctionCall{}, rpc.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, errors.New("non-existent block hash"))

		res, rpcErr := handler.Call(rpc.FunctionCall{}, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, res)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, errors.New("non-existent block number"))

		res, rpcErr := handler.Call(rpc.FunctionCall{}, rpc.BlockID{Number: 0})
		require.Nil(t, res)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("call - unknown contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(nil, errors.New("unknown contract"))

		res, rpcErr := handler.Call(rpc.FunctionCall{}, rpc.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})
}

func TestEstimateMessageFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()

	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, mockVM, "", log)
	msg := rpc.MsgFromL1{
		From:     common.HexToAddress("0xDEADBEEF"),
		To:       *new(felt.Felt).SetUint64(1337),
		Payload:  []felt.Felt{*new(felt.Felt).SetUint64(1), *new(felt.Felt).SetUint64(2)},
		Selector: *new(felt.Felt).SetUint64(44),
	}

	t.Run("block not found", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, errors.New("not found"))
		_, err := handler.EstimateMessageFee(msg, rpc.BlockID{Latest: true})
		require.Equal(t, rpc.ErrBlockNotFound, err)
	})

	latestHeader := &core.Header{
		Number:    123,
		Timestamp: 456,
		GasPrice:  new(felt.Felt).SetUint64(42),
	}
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)

	expectedGasConsumed := new(felt.Felt).SetUint64(37)
	mockVM.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), utils.MAINNET, gomock.Any(), gomock.Any(), latestHeader.GasPrice).DoAndReturn(
		func(txns []core.Transaction, declaredClasses []core.Class, blockNumber, blockTimestamp uint64,
			sequencerAddress *felt.Felt, state core.StateReader, network utils.Network, paidFeesOnL1 []*felt.Felt,
			skipChargeFee bool, gasPrice *felt.Felt,
		) ([]*felt.Felt, []json.RawMessage, error) {
			require.Len(t, txns, 1)
			assert.NotNil(t, txns[0].(*core.L1HandlerTransaction))

			assert.Empty(t, declaredClasses)
			assert.Equal(t, latestHeader.Number, blockNumber)
			assert.Equal(t, latestHeader.Timestamp, blockTimestamp)
			assert.NotNil(t, sequencerAddress)
			assert.Len(t, paidFeesOnL1, 1)

			actualFee := new(felt.Felt).Mul(expectedGasConsumed, gasPrice)
			return []*felt.Felt{actualFee}, []json.RawMessage{{}}, nil
		},
	)

	estimateFee, err := handler.EstimateMessageFee(msg, rpc.BlockID{Latest: true})
	require.Nil(t, err)
	require.Equal(t, rpc.FeeEstimate{
		GasConsumed: expectedGasConsumed,
		GasPrice:    latestHeader.GasPrice,
		OverallFee:  new(felt.Felt).Mul(expectedGasConsumed, latestHeader.GasPrice),
	}, *estimateFee)
}

func TestTraceTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, utils.MAINNET, nil, nil, mockVM, "", log)

	t.Run("not found", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xBBBB")
		// Receipt() returns error related to db
		mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)

		trace, err := handler.TraceTransaction(*hash)
		assert.Nil(t, trace)
		assert.Equal(t, rpc.ErrInvalidTxHash, err)
	})
	t.Run("ok", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0")
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       utils.HexToFelt(t, "0x000000000"),
		}

		header := &core.Header{
			Hash:             utils.HexToFelt(t, "0xCAFEBABE"),
			ParentHash:       utils.HexToFelt(t, "0x0"),
			Number:           0,
			SequencerAddress: utils.HexToFelt(t, "0X111"),
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClass{
			At:    3002,
			Class: &core.Cairo1Class{},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil)
		mockReader.EXPECT().BlockByNumber(header.Number).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTrace := json.RawMessage(`{
		"validate_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": [], "calls": [], "events": [], "messages": []}], "events": [], "messages": []},
		"execute_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x38627c278c0b3cb3c84ddee2c783fb22c3c3a3f0e667ea2b82be0ea2253bce4", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x0", "0x0", "0x5"], "calls": [{"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x46668cd07d83af5d7158e7cd62c710f1a7573501bcd4f4092c6a4e1ecd2bf61", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x0", "0x0", "0x5"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1e8480", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1e8480", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2ceb6369dba6af865bca639f9f1342dfb1ae4e5d0d0723de98028b812e7cdd2", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": [], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "entry_point_selector": "0x2c0f7bf2d6cf5304c29171bf493feb222fef84bdaf17805a6574b0c2e8bcc87", "calldata": ["0x1e8480", "0x0", "0x0", "0x0", "0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x648f780a"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x514718bb56ed2a8607554c7d393c2ffd73cbab971c120b00a2ce27cc58dd1c1", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x2", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x3c388f7eb137a89061c6f0b6e78bae453202258b0b3c419f8dd9814a547d406", "calldata": [], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b60b3a0bcc4aa98", "0xaf07589b7c", "0x648f7422"], "calls": [], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0xaf07771ffc", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0xaf07771ffc", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe14a408baf7f453312eec68e9b7d728ec5337fbdf671f917ee8c80f3255232"], "data": ["0x178b5c9bdd4e74e92b", "0xaf07771ffc"]}, {"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x0", "0x0", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"]}], "messages": []}], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x417c36e4fc16d", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x417c36e4fc16d", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0x5ad857f66a5b55f1301ff1ed7e098ac6d4433148f0b72ebc4a2945ab85ad53"], "data": ["0x2fc5e96de394697c1311606c96ec14840e408493fd42cf0c54b73b39d312b81", "0x2", "0x1", "0x1"]}], "messages": []}], "events": [], "messages": []},
		"fee_transfer_invocation": {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "LIBRARY_CALL", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"]}], "messages": []}], "events": [], "messages": []}
	}`)
		mockVM.EXPECT().Execute([]core.Transaction{tx}, []core.Class{declaredClass.Class}, header.Number, header.Timestamp, header.SequencerAddress,
			nil, utils.MAINNET, []*felt.Felt{}, false, gomock.Any()).Return(nil, []json.RawMessage{vmTrace}, nil)

		trace, err := handler.TraceTransaction(*hash)
		require.Nil(t, err)
		assert.Equal(t, vmTrace, trace)
	})
}

func TestSimulateTransactions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	const network = utils.MAINNET

	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, network, nil, nil, mockVM, "", log)

	t.Run("failure if skip validate provided", func(t *testing.T) {
		_, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipValidateFlag})
		require.NotNil(t, err)
	})
	t.Run("ok with zero values", func(t *testing.T) {
		mockState := mocks.NewMockStateHistoryReader(mockCtrl)

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)

		sequencerAddress := core.NetworkBlockHashMetaInfo(network).FallBackSequencerAddress
		mockVM.EXPECT().Execute(nil, nil, uint64(0), uint64(0), sequencerAddress, mockState, network, []*felt.Felt{}, true, nil).
			Return([]*felt.Felt{}, []json.RawMessage{}, nil)

		_, err := handler.SimulateTransactions(rpc.BlockID{Latest: true}, []rpc.BroadcastedTransaction{}, []rpc.SimulationFlag{rpc.SkipFeeChargeFlag})
		require.Nil(t, err)
	})
}

func TestTraceBlockTransactions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()

	const network = utils.MAINNET
	handler := rpc.New(mockReader, nil, network, nil, nil, mockVM, "", log)

	t.Run("block not found", func(t *testing.T) {
		blockHash := utils.HexToFelt(t, "0x0001")
		mockReader.EXPECT().BlockByHash(blockHash).Return(nil, errors.New("some new err"))

		result, err := handler.TraceBlockTransactions(*blockHash)
		require.Equal(t, rpc.ErrInvalidBlockHash, err)
		assert.Nil(t, result)
	})
	t.Run("pending block", func(t *testing.T) {
		blockHash := utils.HexToFelt(t, "0x0001")
		header := &core.Header{
			// hash is not set because it's pending block
			ParentHash: utils.HexToFelt(t, "0x0C3"),
			Number:     0,
			GasPrice:   utils.HexToFelt(t, "0x777"),
		}
		l1Tx := &core.L1HandlerTransaction{
			TransactionHash: utils.HexToFelt(t, "0x000000C"),
		}
		declaredClass := &core.DeclaredClass{
			At:    3002,
			Class: &core.Cairo1Class{},
		}
		declareTx := &core.DeclareTransaction{
			TransactionHash: utils.HexToFelt(t, "0x000000001"),
			ClassHash:       utils.HexToFelt(t, "0x00000BC00"),
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{l1Tx, declareTx},
		}

		mockReader.EXPECT().BlockByHash(blockHash).Return(block, nil)
		state := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(state, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(declareTx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().PendingState().Return(headState, nopCloser, nil)
		const height uint64 = 8
		mockReader.EXPECT().Height().Return(height, nil)

		sequencerAddress := core.NetworkBlockHashMetaInfo(network).FallBackSequencerAddress
		paidL1Fees := []*felt.Felt{(&felt.Felt{}).SetUint64(1)}
		vmTrace := json.RawMessage(`{
			"validate_invocation": {},
			"execute_invocation": {},
			"fee_transfer_invocation": {}
		}`)
		mockVM.EXPECT().Execute(block.Transactions, []core.Class{declaredClass.Class}, height+1, header.Timestamp, sequencerAddress,
			state, network, paidL1Fees, false, header.GasPrice).Return(nil, []json.RawMessage{vmTrace, vmTrace}, nil)

		result, err := handler.TraceBlockTransactions(*blockHash)
		require.Nil(t, err)
		assert.Equal(t, vmTrace, result[0].TraceRoot)
		assert.Equal(t, l1Tx.TransactionHash, result[0].TransactionHash)
	})
	t.Run("regular block", func(t *testing.T) {
		blockHash := utils.HexToFelt(t, "0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0")
		tx := &core.DeclareTransaction{
			TransactionHash: utils.HexToFelt(t, "0x000000001"),
			ClassHash:       utils.HexToFelt(t, "0x000000000"),
		}

		header := &core.Header{
			Hash:             blockHash,
			ParentHash:       utils.HexToFelt(t, "0x0"),
			Number:           0,
			SequencerAddress: utils.HexToFelt(t, "0X111"),
			GasPrice:         utils.HexToFelt(t, "0x777"),
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClass{
			At:    3002,
			Class: &core.Cairo1Class{},
		}

		mockReader.EXPECT().BlockByHash(blockHash).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTrace := json.RawMessage(`{
			"validate_invocation":{"entry_point_selector":"0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895","calldata":["0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[{"entry_point_selector":"0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895","calldata":["0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","entry_point_type":"EXTERNAL","call_type":"LIBRARY_CALL","result":[],"calls":[],"events":[],"messages":[]}],"events":[],"messages":[]},
			"execute_invocation":{"entry_point_selector":"0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194","calldata":["0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","entry_point_type":"CONSTRUCTOR","call_type":"CALL","result":[],"calls":[{"entry_point_selector":"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","calldata":["0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","entry_point_type":"EXTERNAL","call_type":"LIBRARY_CALL","result":[],"calls":[],"events":[{"keys":["0x10c19bef19acd19b2c9f4caa40fd47c9fbe1d9f91324d44dcd36be2dae96784"],"data":["0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"]}],"messages":[]}],"events":[],"messages":[]},
			"fee_transfer_invocation":{"entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"],"caller_address":"0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","class_hash":"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3","entry_point_type":"EXTERNAL","call_type":"CALL","result":["0x1"],"calls":[{"entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"],"caller_address":"0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","class_hash":"0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0","entry_point_type":"EXTERNAL","call_type":"LIBRARY_CALL","result":["0x1"],"calls":[],"events":[{"keys":["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"],"data":["0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"]}],"messages":[]}],"events":[],"messages":[]}}
		}`)
		mockVM.EXPECT().Execute([]core.Transaction{tx}, []core.Class{declaredClass.Class}, header.Number, header.Timestamp, header.SequencerAddress,
			nil, network, []*felt.Felt{}, false, header.GasPrice).Return(nil, []json.RawMessage{vmTrace}, nil)

		expectedResult := []rpc.TracedBlockTransaction{
			{
				TransactionHash: tx.Hash(),
				TraceRoot:       vmTrace,
			},
		}
		result, err := handler.TraceBlockTransactions(*blockHash)
		require.Nil(t, err)
		assert.Equal(t, expectedResult, result)
	})
}

func TestRpcBlockAdaptation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, utils.GOERLI, nil, nil, nil, "", nil)

	client := feeder.NewTestClient(t, utils.GOERLI)
	gw := adaptfeeder.New(client)
	latestBlockNumber := uint64(485004)

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
