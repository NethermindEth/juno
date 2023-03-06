package rpc_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	bc := blockchain.New(pebble.NewMemTest(), utils.MAINNET)
	handler := rpc.New(bc, utils.MAINNET.ChainId())

	t.Run("starknet_chainId", func(t *testing.T) {
		cId, err := handler.ChainId()
		require.Nil(t, err)
		assert.Equal(t, utils.MAINNET.ChainId(), cId)
	})

	t.Run("empty bc - starknet_blockNumber", func(t *testing.T) {
		_, err := handler.BlockNumber()
		assert.Equal(t, rpc.ErrNoBlock, err)
	})
	t.Run("empty bc - starknet_blockHashAndNumber", func(t *testing.T) {
		_, err := handler.BlockNumberAndHash()
		assert.Equal(t, rpc.ErrNoBlock, err)
	})
	t.Run("empty bc - starknet_getBlockWithTxHashes", func(t *testing.T) {
		_, err := handler.GetBlockWithTxHashes(&rpc.BlockId{Number: 0})
		assert.Equal(t, rpc.ErrBlockNotFound, err)
	})

	log := utils.NewNopZapLogger()
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer()
	synchronizer := sync.NewSynchronizer(bc, gw, log)
	ctx, canceler := context.WithCancel(context.Background())

	syncNodeChan := make(chan struct{})
	go func() {
		synchronizer.Run(ctx)
		close(syncNodeChan)
	}()

	time.Sleep(time.Second)

	t.Run("starknet_blockNumber", func(t *testing.T) {
		num, err := handler.BlockNumber()
		assert.Nil(t, err)
		assert.Equal(t, true, num > 0)
	})

	t.Run("starknet_blockHashAndNumber", func(t *testing.T) {
		hashAndNum, err := handler.BlockNumberAndHash()
		assert.Nil(t, err)
		assert.Equal(t, true, hashAndNum.Number > 0)
		gwBlock, gwErr := gw.BlockByNumber(ctx, hashAndNum.Number)
		assert.NoError(t, gwErr)
		assert.Equal(t, gwBlock.Hash, hashAndNum.Hash)
	})

	t.Run("starknet_getBlockWithTxHashes", func(t *testing.T) {
		latestRpc, err := handler.GetBlockWithTxHashes(&rpc.BlockId{Latest: true})
		assert.Nil(t, err)
		gwBlock, gwErr := gw.BlockByNumber(ctx, latestRpc.Number)
		assert.NoError(t, gwErr)

		assert.Equal(t, gwBlock.Number, latestRpc.Number)
		assert.Equal(t, gwBlock.Hash, latestRpc.Hash)
		assert.Equal(t, gwBlock.GlobalStateRoot, latestRpc.NewRoot)
		assert.Equal(t, gwBlock.ParentHash, latestRpc.ParentHash)
		assert.Equal(t, gwBlock.SequencerAddress, latestRpc.SequencerAddress)
		assert.Equal(t, gwBlock.Timestamp, latestRpc.Timestamp)
		for i := 0; i < len(gwBlock.Transactions); i++ {
			assert.Equal(t, gwBlock.Transactions[i].Hash(), latestRpc.TxnHashes[i])
		}

		byHash, err := handler.GetBlockWithTxHashes(&rpc.BlockId{Hash: latestRpc.Hash})
		require.Nil(t, err)
		assert.Equal(t, latestRpc, byHash)
		byNumber, err := handler.GetBlockWithTxHashes(&rpc.BlockId{Number: latestRpc.Number})
		require.Nil(t, err)
		assert.Equal(t, latestRpc, byNumber)
	})

	t.Run("starknet_getBlockWithTxs", func(t *testing.T) {
		latestRpcWithTxs, err := handler.GetBlockWithTxs(&rpc.BlockId{Latest: true})
		assert.Nil(t, err)

		rpcWithTxHashes, err := handler.GetBlockWithTxHashes(&rpc.BlockId{Hash: latestRpcWithTxs.Hash})
		assert.Nil(t, err)

		assert.Equal(t, rpcWithTxHashes.BlockHeader, latestRpcWithTxs.BlockHeader)
		for i := 0; i < len(rpcWithTxHashes.TxnHashes); i++ {
			assert.Equal(t, rpcWithTxHashes.TxnHashes[i], latestRpcWithTxs.Transactions[i].Hash)
			txn, err := handler.GetTransactionByHash(latestRpcWithTxs.Transactions[i].Hash)
			require.Nil(t, err)
			assert.Equal(t, txn, latestRpcWithTxs.Transactions[i])
		}
	})

	t.Run("starknet_getBlockTransactionCount", func(t *testing.T) {
		blockTxnCount, err := handler.GetBlockTransactionCount(&rpc.BlockId{Number: 2})
		assert.Nil(t, err)

		gwBlock, gwErr := gw.BlockByNumber(ctx, 2)
		assert.NoError(t, gwErr)

		assert.Equal(t, len(gwBlock.Transactions), blockTxnCount)
	})

	canceler()
	<-syncNodeChan
}

// implements only GetTransactionByHash from BlockchainReader interface
// calling any other function from BlockchainReader will panic
type fakeBcReader struct {
	*blockchain.Blockchain
	gw starknetdata.StarknetData
}

func (r *fakeBcReader) GetTransactionByHash(hash *felt.Felt) (transaction core.Transaction, err error) {
	return r.gw.Transaction(context.Background(), hash)
}

func TestGetTransactionByHash(t *testing.T) {
	mainnetGw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer()

	handler := rpc.New(&fakeBcReader{nil, mainnetGw}, nil)

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

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hash, err := new(felt.Felt).SetString(test.hash)
			require.NoError(t, err)
			res, rpcErr := handler.GetTransactionByHash(hash)
			require.Nil(t, rpcErr)

			expectedMap := make(map[string]any)
			require.NoError(t, json.Unmarshal([]byte(test.expected), &expectedMap))

			resJson, err := json.Marshal(res)
			require.NoError(t, err)
			resMap := make(map[string]any)
			require.NoError(t, json.Unmarshal(resJson, &resMap))
			assert.Equal(t, expectedMap, resMap, string(resJson))
		})
	}
}
