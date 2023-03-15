package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainId(t *testing.T) {
	for _, n := range []utils.Network{utils.MAINNET, utils.GOERLI, utils.GOERLI2, utils.INTEGRATION} {
		t.Run(n.String(), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockReader := mocks.NewMockReader(mockCtrl)
			handler := rpc.New(mockReader, n)

			cId, err := handler.ChainId()
			require.Nil(t, err)
			assert.Equal(t, n.ChainId(), cId)
		})
	}
}

func TestBlockNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.MAINNET)

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
		assert.Nil(t, err)
		assert.Equal(t, expectedHeight, num)
	})
}

func TestBlockNumberAndHash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.MAINNET)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, err := handler.BlockNumberAndHash()
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrNoBlock, err)
	})

	t.Run("blockchain height is 147", func(t *testing.T) {
		client, closeServer := feeder.NewTestClient(utils.MAINNET)
		defer closeServer()
		gw := adaptfeeder.New(client)

		expectedBlock, err := gw.BlockByNumber(context.Background(), 147)
		require.NoError(t, err)

		expectedBlockHashAndNumber := &rpc.BlockNumberAndHash{Number: expectedBlock.Number, Hash: expectedBlock.Hash}

		mockReader.EXPECT().Head().Return(expectedBlock, nil)

		hashAndNum, rpcErr := handler.BlockNumberAndHash()
		assert.Nil(t, rpcErr)
		assert.Equal(t, expectedBlockHashAndNumber, hashAndNum)
	})
}

func TestBlockTransactionCount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.GOERLI)

	client, closeServer := feeder.NewTestClient(utils.GOERLI)
	defer closeServer()
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(485004)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash
	expectedCount := latestBlock.TransactionCount

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		count, rpcErr := handler.BlockTransactionCount(&rpc.BlockId{Latest: true})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		count, rpcErr := handler.BlockTransactionCount(&rpc.BlockId{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		count, rpcErr := handler.BlockTransactionCount(&rpc.BlockId{Number: uint64(328476)})
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockId - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(&rpc.BlockId{Latest: true})
		assert.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockId - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(&rpc.BlockId{Hash: latestBlockHash})
		assert.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockId - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)

		count, rpcErr := handler.BlockTransactionCount(&rpc.BlockId{Number: latestBlockNumber})
		assert.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})
}

func TestBlockWithTxHashes(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.GOERLI)

	client, closeServer := feeder.NewTestClient(utils.GOERLI)
	defer closeServer()
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(485004)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	checkLatestBlock := func(t *testing.T, b *rpc.BlockWithTxHashes) {
		assert.Equal(t, latestBlock.Number, b.Number)
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

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Latest: true})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Number: uint64(328476)})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockId - latest", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(latestBlock, nil)

		block, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Latest: true})
		assert.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockId - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil)

		block, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Hash: latestBlockHash})
		assert.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})

	t.Run("blockId - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil)

		block, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Number: latestBlockNumber})
		assert.Nil(t, rpcErr)

		checkLatestBlock(t, block)
	})
}

func TestBlockWithTxs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.MAINNET)

	client, closeServer := feeder.NewTestClient(utils.MAINNET)
	defer closeServer()
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(16697)
	latestBlock, err := gw.BlockByNumber(context.Background(), latestBlockNumber)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, rpcErr := handler.BlockWithTxs(&rpc.BlockId{Latest: true})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxs(&rpc.BlockId{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		block, rpcErr := handler.BlockWithTxs(&rpc.BlockId{Number: uint64(328476)})
		assert.Nil(t, block)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	checkLatestBlock := func(t *testing.T, blockWithTxHashes *rpc.BlockWithTxHashes, blockWithTxs *rpc.BlockWithTxs) {
		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		for i, txnHash := range blockWithTxHashes.TxnHashes {
			assert.Equal(t, txnHash, blockWithTxs.Transactions[i].Hash)

			txn, err := handler.TransactionByHash(blockWithTxs.Transactions[i].Hash)
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
	}).Times(len(latestBlock.Transactions) * 3)

	t.Run("blockId - latest", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(latestBlock, nil).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Latest: true})
		assert.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&rpc.BlockId{Latest: true})
		assert.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockId - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockByHash(latestBlockHash).Return(latestBlock, nil).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Hash: latestBlockHash})
		assert.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&rpc.BlockId{Hash: latestBlockHash})
		assert.Nil(t, rpcErr)

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})

	t.Run("blockId - number", func(t *testing.T) {
		mockReader.EXPECT().BlockByNumber(latestBlockNumber).Return(latestBlock, nil).Times(2)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&rpc.BlockId{Number: latestBlockNumber})
		assert.Nil(t, rpcErr)

		blockWithTxs, rpcErr := handler.BlockWithTxs(&rpc.BlockId{Number: latestBlockNumber})
		assert.Nil(t, rpcErr)

		assert.Equal(t, blockWithTxHashes.BlockHeader, blockWithTxs.BlockHeader)
		assert.Equal(t, len(blockWithTxHashes.TxnHashes), len(blockWithTxs.Transactions))

		checkLatestBlock(t, blockWithTxHashes, blockWithTxs)
	})
}

func TestTransactionByHash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockReader := mocks.NewMockReader(mockCtrl)

	client, closeServer := feeder.NewTestClient(utils.MAINNET)
	defer closeServer()
	mainnetGw := adaptfeeder.New(client)

	handler := rpc.New(mockReader, utils.MAINNET)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

		tx, rpcErr := handler.TransactionByHash(txHash)
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
				"nonce": "0x0",
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
       "contract_address": "0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3",
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
       "contract_address": "0x611de19d2df80327af36e9530553c38d2a74fbe74711448689391016324090d",
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

			res, rpcErr := handler.TransactionByHash(hash)
			require.Nil(t, rpcErr)

			resJson, err := json.Marshal(res)
			require.NoError(t, err)
			resMap := make(map[string]any)
			require.NoError(t, json.Unmarshal(resJson, &resMap))

			assert.Equal(t, expectedMap, resMap, string(resJson))
		})
	}
}

func TestTransactionByBlockIdAndIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	client, closer := feeder.NewTestClient(utils.MAINNET)
	defer closer()
	mainnetGw := adaptfeeder.New(client)

	latestBlockNumber := 19199
	latestBlock, err := mainnetGw.BlockByNumber(context.Background(), 19199)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	handler := rpc.New(mockReader, utils.MAINNET)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		txn, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Latest: true}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		txn, rpcErr := handler.TransactionByBlockIdAndIndex(
			&rpc.BlockId{Hash: new(felt.Felt).SetBytes([]byte("random"))}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		txn, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Number: rand.Uint64()}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("negative index", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)

		txn, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Latest: true}, -1)
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("invalid index", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(latestBlockNumber),
			latestBlock.TransactionCount).Return(nil, errors.New("invalid index"))

		txn, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Latest: true}, len(latestBlock.Transactions))
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("blockId - latest", func(t *testing.T) {
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

		txn1, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Latest: true}, index)
		assert.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(latestBlock.Transactions[index].Hash())
		assert.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})

	t.Run("blockId - hash", func(t *testing.T) {
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

		txn1, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Hash: latestBlockHash}, index)
		assert.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(latestBlock.Transactions[index].Hash())
		assert.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})

	t.Run("blockId - number", func(t *testing.T) {
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

		txn1, rpcErr := handler.TransactionByBlockIdAndIndex(&rpc.BlockId{Number: uint64(latestBlockNumber)}, index)
		assert.Nil(t, rpcErr)

		txn2, rpcErr := handler.TransactionByHash(latestBlock.Transactions[index].Hash())
		assert.Nil(t, rpcErr)

		assert.Equal(t, txn1, txn2)
	})
}

func TestTransactionReceiptByHash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.MAINNET)

	t.Run("transaction not found", func(t *testing.T) {
		txHash := new(felt.Felt).SetBytes([]byte("random hash"))
		mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

		tx, rpcErr := handler.TransactionReceiptByHash(txHash)
		assert.Nil(t, tx)
		assert.Equal(t, rpc.ErrTxnHashNotFound, rpcErr)
	})

	client, closer := feeder.NewTestClient(utils.MAINNET)
	defer closer()
	mainnetGw := adaptfeeder.New(client)

	block0, err := mainnetGw.BlockByNumber(context.Background(), 0)
	require.NoError(t, err)

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
					"status": "ACCEPTED_ON_L2",
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
					"status": "ACCEPTED_ON_L2",
					"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
					"block_number": 0,
					"messages_sent": [
						{
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

			expectedMap := make(map[string]any)
			require.NoError(t, json.Unmarshal([]byte(test.expected), &expectedMap))

			receipt, err := handler.TransactionReceiptByHash(txHash)
			require.Nil(t, err)

			receiptJson, jsonErr := json.Marshal(receipt)
			require.NoError(t, jsonErr)

			receiptMap := make(map[string]any)
			require.NoError(t, json.Unmarshal(receiptJson, &receiptMap))
			assert.Equal(t, expectedMap, receiptMap)
		})
	}
}

func TestStateUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, utils.MAINNET)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Height().Return(uint64(0), errors.New("empty blockchain"))

		update, rpcErr := handler.StateUpdate(&rpc.BlockId{Latest: true})
		assert.Nil(t, update)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByHash(gomock.Any()).Return(nil, errors.New("block not found"))

		update, rpcErr := handler.StateUpdate(&rpc.BlockId{Hash: new(felt.Felt).SetBytes([]byte("random"))})
		assert.Nil(t, update)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByNumber(gomock.Any()).Return(nil, errors.New("block not found"))

		update, rpcErr := handler.StateUpdate(&rpc.BlockId{Number: uint64(328476)})
		assert.Nil(t, update)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	client, closer := feeder.NewTestClient(utils.MAINNET)
	defer closer()
	mainnetGw := adaptfeeder.New(client)

	update21656, err := mainnetGw.StateUpdate(context.Background(), 21656)
	require.NoError(t, err)

	checkUpdate := func(t *testing.T, coreUpdate *core.StateUpdate, rpcUpdate *rpc.StateUpdate) {
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

		assert.Equal(t, coreUpdate.StateDiff.DeclaredClasses, rpcUpdate.StateDiff.DeclaredClasses)
	}

	t.Run("latest", func(t *testing.T) {
		mockReader.EXPECT().Height().Return(uint64(21656), nil)
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(&rpc.BlockId{Latest: true})
		assert.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("by height", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByNumber(uint64(21656)).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(&rpc.BlockId{Number: uint64(21656)})
		assert.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})

	t.Run("by hash", func(t *testing.T) {
		mockReader.EXPECT().StateUpdateByHash(update21656.BlockHash).Return(update21656, nil)

		update, rpcErr := handler.StateUpdate(&rpc.BlockId{Hash: update21656.BlockHash})
		assert.Nil(t, rpcErr)
		checkUpdate(t, update21656, update)
	})
}
