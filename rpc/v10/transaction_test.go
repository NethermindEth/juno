package rpcv10_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// loadBlockFromFeederTestdata loads a block from feeder testdata and adapts it to core.Block.
// This allows us to use testdata blocks without fetching from a real gateway.
func loadBlockFromFeederTestdata(
	t *testing.T,
	network *utils.Network,
	blockNumber uint64,
) *core.Block {
	t.Helper()
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	block, err := gw.BlockByNumber(t.Context(), blockNumber)
	require.NoError(t, err)
	return block
}

func TestTransactionByHashNotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	txHash := felt.NewRandom[felt.Felt]()

	mockReader.EXPECT().TransactionByHash(txHash).Return(nil, db.ErrKeyNotFound)
	mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
	mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

	handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

	tx, rpcErr := handler.TransactionByHash(txHash, rpcv10.ResponseFlags{})
	assert.Nil(t, tx)
	assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
}

func TestTransactionByHashNotFoundInPreConfirmedBlock(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	searchTxHash := felt.NewFromUint64[felt.Felt](0x123456)

	otherTxHash := felt.NewFromUint64[felt.Felt](0x789abc)
	preConfirmedTx := &core.InvokeTransaction{
		TransactionHash: otherTxHash,
		Version:         new(core.TransactionVersion).SetUint64(1),
	}

	preConfirmed := core.PreConfirmed{
		Block: &core.Block{
			Transactions: []core.Transaction{preConfirmedTx},
		},
		CandidateTxs: []core.Transaction{},
	}
	mockReader.EXPECT().TransactionByHash(searchTxHash).Return(nil, db.ErrKeyNotFound)
	mockSyncReader.EXPECT().PendingData().Return(&preConfirmed, nil)

	handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

	tx, rpcErr := handler.TransactionByHash(searchTxHash, rpcv10.ResponseFlags{})
	assert.Nil(t, tx)
	assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
}

func TestTransactionByHash(t *testing.T) {
	tests := map[string]struct {
		hash          string
		network       *utils.Network
		expected      string
		responseFlags rpcv10.ResponseFlags
	}{
		"DECLARE v1": {
			hash:    "0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae",
			network: &utils.Mainnet,
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
			hash:    "0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4",
			network: &utils.Mainnet,
			expected: `{
				"transaction_hash": "0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4",
				"type": "DECLARE",
				"max_fee": "0x0",
				"version": "0x0",
				"signature": [],
				"class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0",
				"sender_address": "0x1"
			}`,
		},

		"L1 Handler v0 with nonce": {
			hash:    "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8",
			network: &utils.Mainnet,
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
		"L1 Handler v0 without nonce": {
			hash:    "0x5d50b7020f7cf8033fd7d913e489f47edf74fbf3c8ada85be512c7baa6a2eab",
			network: &utils.Mainnet,
			expected: `{
				"type": "L1_HANDLER",
				"transaction_hash":  "0x5d50b7020f7cf8033fd7d913e489f47edf74fbf3c8ada85be512c7baa6a2eab",
				"version": "0x0",
				"nonce": "0x0",
				"contract_address":  "0x58b43819bb12aba8ab3fb2e997523e507399a3f48a1e2aa20a5fb7734a0449f",
				"entry_point_selector": "0xe3f5e9e1456ffa52a3fbc7e8c296631d4cc2120c0be1e2829301c0d8fa026b",
				"calldata": [
					"0x5474c49483aa09993090979ade8101ebb4cdce4a",
					"0xabf8dd8438d1c21e83a8b5e9c1f9b58aaf3ed360",
					"0x2",
					"0x4c04fac82913f01a8f01f6e15ff7e834ff2d9a9a1d8e9adffc7bd45692f4f9a"
				]
			}`,
		},

		"Invoke v1": {
			hash:    "0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497",
			network: &utils.Mainnet,
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
			hash:    "0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0",
			network: &utils.Mainnet,
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
			hash:    "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2",
			network: &utils.Mainnet,
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
			hash:    "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e",
			network: &utils.Mainnet,
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
		"DECLARE v3": {
			hash:    "0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3",
			network: &utils.Integration,
			expected: `{
		"transaction_hash": "0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3",
		"type": "DECLARE",
		"version": "0x3",
		"nonce": "0x1",
		"sender_address": "0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50",
		"class_hash": "0x5ae9d09292a50ed48c5930904c880dab56e85b825022a7d689cfc9e65e01ee7",
		"compiled_class_hash": "0x1add56d64bebf8140f3b8a38bdf102b7874437f0c861ab4ca7526ec33b4d0f8",
		"signature": [
			"0x29a49dff154fede73dd7b5ca5a0beadf40b4b069f3a850cd8428e54dc809ccc",
			"0x429d142a17223b4f2acde0f5ecb9ad453e188b245003c86fab5c109bad58fc3"
		],
		"resource_bounds": {
			"l1_gas": {
				"max_amount": "0x186a0",
				"max_price_per_unit": "0x2540be400"
			},
			"l1_data_gas": {
				"max_amount": "0x186a0",
				"max_price_per_unit": "0x2540be400"
			},
			"l2_gas": { "max_amount": "0x0", "max_price_per_unit": "0x0" }
		},
		"tip": "0x0",
		"paymaster_data": [],
		"account_deployment_data": [],
		"nonce_data_availability_mode": "L1",
		"fee_data_availability_mode": "L1"
	   }`,
		},
		"INVOKE v3": {
			hash:    "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
			network: &utils.Integration,
			expected: `{
				"type": "INVOKE",
				"transaction_hash": "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
				"version": "0x3",
				"signature": [
					"0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16",
					"0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"
				],
				"nonce": "0xe97",
				"resource_bounds": {
					"l1_gas": {
						"max_amount": "0x186a0",
						"max_price_per_unit": "0x5af3107a4000"
					},
					"l1_data_gas": {
						"max_amount": "0x186a0",
						"max_price_per_unit": "0x5af3107a4000"
					},
					"l2_gas": { "max_amount": "0x0", "max_price_per_unit": "0x0" }
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
				"nonce_data_availability_mode": "L1",
				"fee_data_availability_mode": "L1"
			}`,
		},
		"DEPLOY ACCOUNT v3": {
			hash:    "0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0",
			network: &utils.Integration,
			expected: `{
				"transaction_hash": "0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0",
				"version": "0x3",
				"signature": [
					"0x6d756e754793d828c6c1a89c13f7ec70dbd8837dfeea5028a673b80e0d6b4ec",
					"0x4daebba599f860daee8f6e100601d98873052e1c61530c630cc4375c6bd48e3"
				],
				"nonce": "0x0",
				"resource_bounds": {
					"l1_gas": {
						"max_amount": "0x186a0",
						"max_price_per_unit": "0x5af3107a4000"
					},
					"l1_data_gas": {
						"max_amount": "0x186a0",
						"max_price_per_unit": "0x5af3107a4000"
					},
					"l2_gas": { "max_amount": "0x0", "max_price_per_unit": "0x0" }
				},
				"tip": "0x0",
				"paymaster_data": [],
				"contract_address_salt": "0x0",
				"class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738",
				"constructor_calldata": [
					"0x5cd65f3d7daea6c63939d659b8473ea0c5cd81576035a4d34e52fb06840196c"
				],
				"type": "DEPLOY_ACCOUNT",
				"nonce_data_availability_mode": "L1",
				"fee_data_availability_mode": "L1"
			}`,
		},
		"INVOKE v3 without l1_data_gas": {
			hash:    "0x2db07ed11b1f6c678de9fc19ef0dfb8e71631e1cff236a34e68f51528a21282",
			network: &utils.Integration,
			expected: `{
				"transaction_hash": "0x2db07ed11b1f6c678de9fc19ef0dfb8e71631e1cff236a34e68f51528a21282",
				"version": "0x3",
				"signature": [
					"0x6b67b53231b0ad782b651cb529004258ac79f8bd069042127e0f58edd40fd89",
					"0x36cc9eaeb31b0b3d990624cf105aa0cea86590452e188dadb75edc02ddeea51"
				],
				"nonce": "0x12c0c",
				"resource_bounds": {
					"l1_gas": {
						"max_amount": "0x60",
						"max_price_per_unit": "0x13ac02cbe617"
					},
					"l1_data_gas": {
						"max_amount": "0x0",
						"max_price_per_unit": "0x0"
					},
					"l2_gas": { "max_amount": "0x0", "max_price_per_unit": "0x0" }
				},
				"tip": "0x0",
				"paymaster_data": [],
				"sender_address": "0x573ea9a8602e03417a4a31d55d115748f37a08bbb23adf6347cb699743a998d",
				"calldata": [
					"0x1",
					"0x53d5cb0de4f03f9ac31f83621cef64b9372bf1f690fdfa2ba8a07c316e67817",
					"0xc844fd57777b0cd7e75c8ea68deec0adf964a6308da7a58de32364b7131cc8",
					"0x13",
					"0x4c7fb0cc02a3432253dcc76f8ab04ed11bc804ca36312d9fbd0777541f266",
					"0x192603",
					"0xd34ba8c515a574cd724301a5b50e997abcfd377f705e70a8330c706f3ccf34",
					"0x663c8f59",
					"0x204030100000000000000000000000000000000000000000000000000000000",
					"0x4",
					"0x5444abc7",
					"0x54497b21",
					"0x54497b21",
					"0x54497b21",
					"0xb6d5fbd139a7d956f",
					"0x1",
					"0x2",
					"0x723516b6471960da09efa937c31abd5c46590e7b9df4773a5a6111497541508",
					"0x385effc19083082f432ed7ab855de96b575e14a21b0a6d7a4395ff4d99e30f6",
					"0x2cb74dff29a13dd5d855159349ec92f943bacf0547ff3734e7d84a15d08cbc5",
					"0xb5eb2dec854e82a991956a933cd3b20b888bcb1737427e829efc5cd7e241e7",
					"0xb71581436348419b44d939dc2af69a310c7a4b7d4f16b07f71d1957e9d5ceb",
					"0x4225d1c8ee8e451a25e30c10689ef898e11ccf5c0f68d0fc7876c47b318e946"
				],
				"account_deployment_data": [],
				"type": "INVOKE",
				"nonce_data_availability_mode": "L1",
				"fee_data_availability_mode": "L1"
			}`,
		},
		"INVOKE v3 with response flags": {
			hash:    "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
			network: &utils.Integration,
			expected: `{
				"type": "INVOKE",
				"transaction_hash": "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
				"version": "0x3",
				"signature": [
					"0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16",
					"0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"
				],
				"nonce": "0xe97",
				"resource_bounds": {
					"l1_gas": {
						"max_amount": "0x186a0",
						"max_price_per_unit": "0x5af3107a4000"
					},
					"l1_data_gas": {
						"max_amount": "0x186a0",
						"max_price_per_unit": "0x5af3107a4000"
					},
					"l2_gas": { "max_amount": "0x0", "max_price_per_unit": "0x0" }
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
				"nonce_data_availability_mode": "L1",
				"fee_data_availability_mode": "L1",
				"proof_facts": ["0x64", "0xc8"]
			}`,
			responseFlags: rpcv10.ResponseFlags{IncludeProofFacts: true},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			gw := adaptfeeder.New(feeder.NewTestClient(t, test.network))
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			mockReader.EXPECT().TransactionByHash(
				gomock.Any()).DoAndReturn(func(hash *felt.Felt) (core.Transaction, error) {
				tx, err := gw.Transaction(t.Context(), hash)
				if err != nil {
					return nil, err
				}
				// Mock the gateway to include proof_facts for "INVOKE v3 with response flags" test
				invokeTx, ok := tx.(*core.InvokeTransaction)
				if ok && invokeTx.Version.Is(3) && test.responseFlags.IncludeProofFacts {
					proofFacts := []*felt.Felt{
						felt.NewFromUint64[felt.Felt](100),
						felt.NewFromUint64[felt.Felt](200),
					}
					invokeTx.ProofFacts = proofFacts
				}
				return tx, nil
			}).Times(1)
			mockSyncReader.EXPECT().PendingData().Return(&core.PreConfirmed{
				Block: &core.Block{
					Header: &core.Header{
						Number:           1,
						TransactionCount: 0,
					},
				},
			}, nil)
			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

			hash, err := felt.NewFromString[felt.Felt](test.hash)
			require.NoError(t, err)

			res, rpcErr := handler.TransactionByHash(hash, test.responseFlags)
			require.Nil(t, rpcErr)
			resJSON, err := json.Marshal(res)
			require.NoError(t, err)
			assert.JSONEq(t, test.expected, string(resJSON))
		})
	}
}

func TestTransactionByHash_PreConfirmedBlock(t *testing.T) {
	gw := feeder.NewTestClient(t, &utils.SepoliaIntegration)
	adapterFeeder := adaptfeeder.New(gw)
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	blockNumber := uint64(1204672)
	preConfirmedBlockWithCandidates, err := gw.PreConfirmedBlock(
		t.Context(),
		strconv.FormatUint(blockNumber, 10),
	)
	require.NoError(t, err)

	adaptedPreConfirmed, err := sn2core.AdaptPreConfirmedBlock(
		preConfirmedBlockWithCandidates,
		blockNumber,
	)
	require.NoError(t, err)
	handler := rpcv10.New(nil, mockSyncReader, nil, nil)

	t.Run("Transaction found in pre_confirmed block", func(t *testing.T) {
		searchTxn := adaptedPreConfirmed.Block.Transactions[0]
		mockSyncReader.EXPECT().PendingData().Return(&adaptedPreConfirmed, nil)
		foundTxn, err := handler.TransactionByHash(searchTxn.Hash(), rpcv10.ResponseFlags{})
		require.Nil(t, err)
		require.Equal(t, searchTxn.Hash(), foundTxn.Hash)
	})

	t.Run("Transaction found in pre_confirmed block - Candidate transactions", func(t *testing.T) {
		searchTxn := adaptedPreConfirmed.CandidateTxs[0]
		mockSyncReader.EXPECT().PendingData().Return(&adaptedPreConfirmed, nil)
		foundTxn, err := handler.TransactionByHash(searchTxn.Hash(), rpcv10.ResponseFlags{})
		require.Nil(t, err)
		require.Equal(t, searchTxn.Hash(), foundTxn.Hash)
	})

	t.Run("Transaction found in pre_latest block", func(t *testing.T) {
		arbitraryBlockInTestData := uint64(1164621)
		testBlock, gwErr := adapterFeeder.BlockByNumber(t.Context(), arbitraryBlockInTestData)
		require.NoError(t, gwErr)
		searchTxn := testBlock.Transactions[0]

		preLatest := core.PreLatest{
			Block: testBlock,
		}
		adaptedPreConfirmed.WithPreLatest(&preLatest)

		mockSyncReader.EXPECT().PendingData().Return(&adaptedPreConfirmed, nil)
		foundTxn, err := handler.TransactionByHash(searchTxn.Hash(), rpcv10.ResponseFlags{})
		require.Nil(t, err)
		require.Equal(t, searchTxn.Hash(), foundTxn.Hash)
	})
}

func TestTransactionByBlockIdAndIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	var latestBlockNumber uint64 = 19199
	latestBlock, err := mainnetGw.BlockByNumber(t.Context(), 19199)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("empty blockchain"))

		blockID := rpcv9.BlockIDLatest()
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, rand.Int(), rpcv10.ResponseFlags{})
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockNumberByHash(gomock.Any()).Return(uint64(0), db.ErrKeyNotFound)

		blockID := rpcv9.BlockIDFromHash(
			felt.NewFromBytes[felt.Felt]([]byte("random")),
		)
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, rand.Int(), rpcv10.ResponseFlags{})
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(
			gomock.Any(), gomock.Any()).Return(nil, db.ErrKeyNotFound)
		blockID := rpcv9.BlockIDFromNumber(rand.Uint64())
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, rand.Int(), rpcv10.ResponseFlags{})
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("negative index", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("negative index"))

		blockID := rpcv9.BlockIDLatest()
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, -1, rpcv10.ResponseFlags{})
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrInvalidTxIndex, rpcErr)
	})

	t.Run("invalid index", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)

		blockID := rpcv9.BlockIDLatest()
		txn, rpcErr := handler.TransactionByBlockIDAndIndex(
			&blockID,
			len(latestBlock.Transactions),
			rpcv10.ResponseFlags{},
		)
		assert.Nil(t, txn)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().TransactionByBlockNumberAndIndex(latestBlockNumber,
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		expectedTxn := rpcv10.AdaptTransaction(latestBlock.Transactions[index], false)
		blockID := rpcv9.BlockIDLatest()
		actualTxn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, index, rpcv10.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.Equal(t, &expectedTxn, actualTxn)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().BlockNumberByHash(latestBlockHash).Return(latestBlock.Number, nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(latestBlockNumber,
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		expectedTxn := rpcv10.AdaptTransaction(latestBlock.Transactions[index], false)
		blockID := rpcv9.BlockIDFromHash(latestBlock.Hash)
		actualTxn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, index, rpcv10.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.Equal(t, &expectedTxn, actualTxn)
	})

	t.Run("blockID - number", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().TransactionByBlockNumberAndIndex(latestBlockNumber,
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		expectedTxn := rpcv10.AdaptTransaction(latestBlock.Transactions[index], false)
		blockID := rpcv9.BlockIDFromNumber(latestBlockNumber)
		actualTxn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, index, rpcv10.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.Equal(t, &expectedTxn, actualTxn)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		index := rand.Intn(int(latestBlock.TransactionCount))

		mockReader.EXPECT().L1Head().Return(
			core.L1Head{
				BlockNumber: latestBlockNumber,
				BlockHash:   latestBlockHash,
				StateRoot:   latestBlock.GlobalStateRoot,
			},
			nil,
		)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(latestBlockNumber,
			uint64(index)).DoAndReturn(func(number, index uint64) (core.Transaction, error) {
			return latestBlock.Transactions[index], nil
		})

		expectedTxn := rpcv10.AdaptTransaction(latestBlock.Transactions[index], false)
		blockID := rpcv9.BlockIDL1Accepted()
		actualTxn, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, index, rpcv10.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.Equal(t, &expectedTxn, actualTxn)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		preConfirmed := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PendingData().Return(
			&preConfirmed,
			nil,
		).Times(2)
		blockID := rpcv9.BlockIDPreConfirmed()

		t.Run("invalid index", func(t *testing.T) {
			invalidIndex := len(preConfirmed.Block.Transactions)

			actualTxn, rpcErr := handler.TransactionByBlockIDAndIndex(
				&blockID,
				invalidIndex,
				rpcv10.ResponseFlags{},
			)
			require.Equal(t, rpcErr, rpccore.ErrInvalidTxIndex)
			require.Nil(t, actualTxn)
		})

		t.Run("valid index", func(t *testing.T) {
			index := rand.Intn(int(latestBlock.TransactionCount))
			expectedTxn := rpcv10.AdaptTransaction(latestBlock.Transactions[index], false)

			actualTxn, rpcErr := handler.TransactionByBlockIDAndIndex(
				&blockID,
				index,
				rpcv10.ResponseFlags{},
			)
			require.Nil(t, rpcErr)
			require.Equal(t, &expectedTxn, actualTxn)
		})
	})

	t.Run("response flags", func(t *testing.T) {
		network := &utils.Sepolia
		client := feeder.NewTestClient(t, network)
		gw := adaptfeeder.New(client)
		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x435f87f1eecd5968ba8190744fee1f3ef69f17471f8902ce1e7d444c4e0c8cb",
		)
		invokeTx, err := gw.Transaction(t.Context(), txnHash)
		require.NoError(t, err)
		require.IsType(t, &core.InvokeTransaction{}, invokeTx)
		invokeTxCore := invokeTx.(*core.InvokeTransaction)
		require.NotNil(t, invokeTxCore.ProofFacts)

		mockReader := mocks.NewMockReader(mockCtrl)
		mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
		blockID := rpcv9.BlockIDFromNumber(1)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(uint64(1), uint64(0)).
			Return(invokeTxCore, nil).AnyTimes()
		mockReader.EXPECT().Network().Return(network).AnyTimes()
		h := rpcv10.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

		t.Run("WithResponseFlag", func(t *testing.T) {
			tx, rpcErr := h.TransactionByBlockIDAndIndex(
				&blockID,
				0,
				rpcv10.ResponseFlags{IncludeProofFacts: true},
			)
			require.Nil(t, rpcErr)
			require.NotNil(t, tx)
			require.NotNil(t, tx.ProofFacts)
			require.Equal(t, len(invokeTxCore.ProofFacts), len(tx.ProofFacts))
		})
		t.Run("WithoutResponseFlag", func(t *testing.T) {
			tx, rpcErr := h.TransactionByBlockIDAndIndex(&blockID, 0, rpcv10.ResponseFlags{})
			require.Nil(t, rpcErr)
			require.NotNil(t, tx)
			require.Nil(t, tx.ProofFacts)
		})
	})
}

func TestTransactionReceiptByHash(t *testing.T) {
	type testCase struct {
		description   string
		network       *utils.Network
		expected      *rpcv9.TransactionReceipt
		pendingDataFn func(t *testing.T, block *core.Block) core.PendingData
		l1Head        core.L1Head
	}

	emptyPendingDataFunc := func(t *testing.T, block *core.Block) core.PendingData {
		return &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: block.Number + 1,
				},
			},
		}
	}

	preConfirmedPendingDataFunc := func(t *testing.T, block *core.Block) core.PendingData {
		return &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					TransactionCount: block.TransactionCount,
					EventCount:       block.EventCount,
				},
				Transactions: block.Transactions,
				Receipts:     block.Receipts,
			},
		}
	}

	withPreLatestPendingDataFunc := func(t *testing.T, block *core.Block) core.PendingData {
		preLatest := core.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					ParentHash:       block.ParentHash,
					TransactionCount: block.TransactionCount,
					EventCount:       block.EventCount,
				},
				Transactions: block.Transactions,
				Receipts:     block.Receipts,
			},
		}
		preConfirmed := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: preLatest.Block.Number + 1,
				},
			},
			PreLatest: &preLatest,
		}

		return preConfirmed
	}

	testCases := []testCase{
		{
			description: "receipt accepted on l2",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_accepted_on_l2.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt accepted on l1",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_accepted_on_l1.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: math.MaxUint64},
		},
		{
			description: "receipt pre confirmed",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_pre_confirmed.json",
			),
			pendingDataFn: preConfirmedPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt pre latest",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_pre_latest.json",
			),
			pendingDataFn: withPreLatestPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt reverted",
			network:     &utils.Integration,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_reverted.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt invoke v3",
			network:     &utils.Integration,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_invoke_v3.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt non empty da",
			network:     &utils.SepoliaIntegration,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_non_empty_da.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt deploy",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_deploy.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			expected := test.expected
			require.NotNil(t, expected.BlockNumber)

			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

			loadedBlock := loadBlockFromFeederTestdata(t, test.network, *expected.BlockNumber)
			var transaction core.Transaction
			var transactionIndex int
			// find the transaction in the block
			for i, tx := range loadedBlock.Transactions {
				if tx.Hash().Equal(expected.Hash) {
					transaction = tx
					transactionIndex = i
					break
				}
			}
			require.NotNil(t, transaction, "transaction not found on expected block")

			pendingData := test.pendingDataFn(t, loadedBlock)
			mockSyncReader.EXPECT().PendingData().Return(pendingData, nil)
			_, _, _, err := pendingData.ReceiptByHash(transaction.Hash())
			if err != nil {
				// receipt belong to canonical block mock expectations
				mockReader.EXPECT().BlockNumberAndIndexByTxHash(
					(*felt.TransactionHash)(expected.Hash),
				).Return(*expected.BlockNumber, uint64(transactionIndex), nil)
				mockReader.EXPECT().TransactionByBlockNumberAndIndex(
					*expected.BlockNumber, uint64(transactionIndex),
				).Return(transaction, nil)
				mockReader.EXPECT().ReceiptByBlockNumberAndIndex(
					*expected.BlockNumber, uint64(transactionIndex),
				).Return(*loadedBlock.Receipts[transactionIndex], expected.BlockHash, nil)
				mockReader.EXPECT().L1Head().Return(test.l1Head, nil)
			}

			receipt, rpcErr := handler.TransactionReceiptByHash(expected.Hash)
			require.Nil(t, rpcErr)
			require.Equal(t, expected, receipt)
		})
	}
}

func TestTransactionReceiptByHash_NotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

	txHash := felt.NewFromBytes[felt.Felt]([]byte("random hash"))
	mockReader.EXPECT().BlockNumberAndIndexByTxHash(
		(*felt.TransactionHash)(txHash),
	).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
	mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
	mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

	tx, rpcErr := handler.TransactionReceiptByHash(txHash)
	assert.Nil(t, tx)
	assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
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
			tx := rpcv10.BroadcastedTransaction{}
			require.NoError(t, json.Unmarshal([]byte(txJSON), &tx))
		})
	}
}

func TestAddTransaction(t *testing.T) {
	n := &utils.Integration
	gw := adaptfeeder.New(feeder.NewTestClient(t, n))
	txWithoutClass := func(hash string) rpcv10.BroadcastedTransaction {
		tx, err := gw.Transaction(t.Context(), felt.NewUnsafeFromString[felt.Felt](hash))
		require.NoError(t, err)
		return rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: *rpcv9.AdaptTransaction(tx),
			},
		}
	}
	tests := map[string]struct {
		txn          rpcv10.BroadcastedTransaction
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
					"L1_DATA_GAS": {
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
			txn: func() rpcv10.BroadcastedTransaction {
				tx := txWithoutClass(
					"0x44b971f7eface29b185f86dd7b3b70acb1e48e0ad459e3a41e06fc42937aaa4",
				)
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
			txn: func() rpcv10.BroadcastedTransaction {
				tx := txWithoutClass(
					"0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3",
				)
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
				  "L1_DATA_GAS": {
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
				  "L1_DATA_GAS": {
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
		"invoke v3 with proof_facts": {
			txn: func() rpcv10.BroadcastedTransaction {
				base := createBaseInvokeTransactionV3()
				proofFacts := []felt.Felt{
					felt.FromUint64[felt.Felt](100),
					felt.FromUint64[felt.Felt](200),
				}
				return rpcv10.BroadcastedTransaction{
					BroadcastedTransaction: rpcv9.BroadcastedTransaction{
						Transaction: *rpcv9.AdaptTransaction(&base),
					},
					Proof:      []uint64{1, 2, 3},
					ProofFacts: proofFacts,
				}
			}(),
			expectedJSON: `{
				"transaction_hash": "0x3039",
				"version": "0x3",
				"signature": [
					"0x1",
					"0x1"
				],
				"nonce": "0x1",
				"nonce_data_availability_mode": 0,
				"fee_data_availability_mode": 0,
				"resource_bounds": {
					"L1_GAS": {
						"max_amount": "0x1",
						"max_price_per_unit": "0x1"
					},
					"L1_DATA_GAS": {
						"max_amount": "0x1",
						"max_price_per_unit": "0x1"
					},
					"L2_GAS": {
						"max_amount": "0x0",
						"max_price_per_unit": "0x0"
					}
				},
				"tip": "0x0",
				"paymaster_data": [],
				"sender_address": "0x1",
				"calldata": [],
				"account_deployment_data": [],
				"type": "INVOKE_FUNCTION",
				"proof": [1, 2, 3],
				"proof_facts": ["0x64", "0xc8"]
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
					assert.JSONEq(t, test.expectedJSON, string(txnJSON))
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

			handler := rpcv10.New(nil, nil, nil, utils.NewNopZapLogger())
			_, rpcErr := handler.AddTransaction(t.Context(), utils.HeapPtr(test.txn))
			require.Equal(t, rpcErr.Code, rpccore.ErrInternal.Code)

			handler = handler.WithGateway(mockGateway)
			got, rpcErr := handler.AddTransaction(t.Context(), utils.HeapPtr(test.txn))
			require.Nil(t, rpcErr)
			require.Equal(t, rpcv10.AddTxResponse{
				TransactionHash: felt.FromUint64[felt.TransactionHash](0x1),
				ContractAddress: felt.NewFromUint64[felt.Address](0x2),
				ClassHash:       felt.NewFromUint64[felt.ClassHash](0x3),
			}, got)
		})
	}

	t.Run("gateway returns expected errors", func(t *testing.T) {
		errorTests := []struct {
			name          string
			gatewayError  *gateway.Error
			expectedError *jsonrpc.Error
		}{
			{
				name:          "InsufficientResourcesForValidate error",
				gatewayError:  &gateway.Error{Code: gateway.InsufficientResourcesForValidate},
				expectedError: rpccore.ErrInsufficientResourcesForValidate,
			},
			{
				name:          "InsufficientAccountBalance error",
				gatewayError:  &gateway.Error{Code: gateway.InsufficientAccountBalance},
				expectedError: rpccore.ErrInsufficientAccountBalanceV0_8,
			},
			{
				name:          "FeeBelowMinimum error",
				gatewayError:  &gateway.Error{Code: gateway.FeeBelowMinimum},
				expectedError: rpccore.ErrFeeBelowMinimum,
			},
			{
				name:          "ReplacementTransactionUnderPriced error",
				gatewayError:  &gateway.Error{Code: gateway.ReplacementTransactionUnderPriced},
				expectedError: rpccore.ErrReplacementTransactionUnderPriced,
			},
			{
				name: "InvalidTransactionNonce error",
				gatewayError: &gateway.Error{
					Code:    gateway.InvalidTransactionNonce,
					Message: "Expected: 2176, got: 845.",
				},
				expectedError: rpccore.ErrInvalidTransactionNonce.
					CloneWithData("Expected: 2176, got: 845."),
			},
			{
				name: "InvalidTransactionNonce error as ErrValidationFailure",
				gatewayError: &gateway.Error{
					Code: gateway.ValidateFailure,
					Message: "StarknetError { code: KnownErrorCode(InvalidTransactionNonce)," +
						" message: 'Invalid transaction nonce. Expected: 2176, got: 845.' }",
				},
				expectedError: rpccore.ErrInvalidTransactionNonce.
					CloneWithData(
						"StarknetError { code: KnownErrorCode(InvalidTransactionNonce)," +
							" message: 'Invalid transaction nonce. Expected: 2176, got: 845.' }",
					),
			},
		}

		for _, tc := range errorTests {
			t.Run(tc.name, func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				t.Cleanup(mockCtrl.Finish)

				mockGateway := mocks.NewMockGateway(mockCtrl)
				mockGateway.
					EXPECT().
					AddTransaction(gomock.Any(), gomock.Any()).
					Return(nil, tc.gatewayError)

				handler := rpcv10.New(nil, nil, nil, utils.NewNopZapLogger()).WithGateway(mockGateway)
				addTxRes, rpcErr := handler.AddTransaction(
					t.Context(),
					utils.HeapPtr(tests["invoke v0"].txn),
				)

				require.Equal(t, tc.expectedError, rpcErr)
				require.Zero(t, addTxRes)
			})
		}
	})
}

func TestTransactionStatus(t *testing.T) {
	mainnetClient := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(mainnetClient)

	block, err := gw.BlockLatest(t.Context())
	require.NoError(t, err)
	tx := block.Transactions[0]
	targetTxnHash := tx.Hash()
	type testCase struct {
		description    string
		network        *utils.Network
		txHash         *felt.Felt
		expectedStatus rpcv9.TransactionStatus
		expectedErr    *jsonrpc.Error
		setupMocks     func(
			mockReader *mocks.MockReader,
			mockSyncReader *mocks.MockSyncReader,
		)
	}
	preConfirmedPlaceHolder := core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: block.Number + 1,
			},
		},
	}
	mockFoundInDB := func(
		mockReader *mocks.MockReader,
		mockSyncReader *mocks.MockSyncReader,
	) {
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(tx.Hash()),
		).Return(block.Number, uint64(0), nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(tx, nil)
		mockReader.EXPECT().ReceiptByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(*block.Receipts[0], block.Hash, nil)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil)
	}

	mockNotFound := func(
		mockReader *mocks.MockReader,
		mockSyncReader *mocks.MockSyncReader,
	) {
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			gomock.Any(),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil).Times(2)
	}

	// TODO(Ege): Add test with failure reason REVERTED
	testCases := []testCase{
		{
			description: "status ACCEPTED_ON_L2",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL2,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockFoundInDB(mockReader, mockSyncReader)
				mockReader.EXPECT().L1Head().Return(core.L1Head{BlockNumber: 0}, nil)
			},
		},
		{
			description: "status ACCEPTED_ON_L1",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL1,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockFoundInDB(mockReader, mockSyncReader)
				mockReader.EXPECT().L1Head().Return(core.L1Head{BlockNumber: math.MaxUint64}, nil)
			},
		},
		{
			description: "status PRE_CONFIRMED",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusPreConfirmed,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockSyncReader.EXPECT().PendingData().Return(
					&core.PreConfirmed{
						Block: &core.Block{
							Header: &core.Header{
								Number: block.Number,
							},
							Transactions: block.Transactions,
							Receipts:     block.Receipts,
						},
					}, nil,
				)
			},
		},
		{
			description: "status CANDIDATE",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusCandidate,
				Execution: rpcv9.UnknownExecution,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockReader.EXPECT().BlockNumberAndIndexByTxHash(
					(*felt.TransactionHash)(targetTxnHash),
				).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
				mockSyncReader.EXPECT().PendingData().Return(
					&core.PreConfirmed{
						Block: &core.Block{
							Header: &core.Header{
								Number: block.Number,
							},
						},
						CandidateTxs: []core.Transaction{tx},
					}, nil,
				).Times(2)
			},
		},
		{
			description: "status ACCEPTED_ON_L2 from pre-latest",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL2,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				preLatest := core.PreLatest{
					Block: &core.Block{
						Header: &core.Header{
							ParentHash: block.ParentHash,
							Number:     block.Number,
						},
						Transactions: block.Transactions,
						Receipts:     block.Receipts,
					},
				}

				mockSyncReader.EXPECT().PendingData().Return(
					preConfirmedPlaceHolder.Copy().WithPreLatest(&preLatest),
					nil,
				)
			},
		},
		{
			description: "not found localy - ACCEPTED_ON_L1 from feeder",
			network:     &utils.Mainnet,
			txHash: felt.NewUnsafeFromString[felt.Felt](
				"0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e",
			),
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL1,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: mockNotFound,
		},
		{
			description: "not found localy - ACCEPTED_ON_L2 from feeder",
			network:     &utils.Mainnet,
			txHash: felt.NewUnsafeFromString[felt.Felt](
				"0x6c40890743aa220b10e5ee68cef694c5c23cc2defd0dbdf5546e687f9982ab1",
			),
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL2,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: mockNotFound,
		},
		{
			description: "transaction not found",
			network:     &utils.Mainnet,
			txHash:      felt.NewUnsafeFromString[felt.Felt]("0xFF00FF00"),
			expectedErr: rpccore.ErrTxnHashNotFound,
			setupMocks:  mockNotFound,
		},
		{
			// RPCv10 does not have REJECTED status.
			// For historical queries return not found error instead.
			description: "REJECTED historical txn found in feeder",
			network:     &utils.SepoliaIntegration,
			txHash:      felt.NewUnsafeFromString[felt.Felt]("0x1111"),
			expectedErr: rpccore.ErrTxnHashNotFound,
			setupMocks:  mockNotFound,
		},
	}
	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			client := feeder.NewTestClient(t, test.network)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			test.setupMocks(mockReader, mockSyncReader)

			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil).WithFeeder(client)

			status, rpcErr := handler.TransactionStatus(t.Context(), test.txHash)
			require.Equal(t, test.expectedErr, rpcErr)
			require.Equal(t, test.expectedStatus, status)
		})
	}
}

func TestSubmittedTransactionsCache(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	log := utils.NewNopZapLogger()
	network := utils.Integration

	client := feeder.NewTestClient(t, &network)

	cacheSize := uint(5)
	cacheEntryTimeOut := time.Second

	txnToAdd := createBaseInvokeTransactionV3()

	broadcastedTxn := &rpcv10.BroadcastedTransaction{
		BroadcastedTransaction: rpcv9.BroadcastedTransaction{
			Transaction: *rpcv9.AdaptTransaction(&txnToAdd),
		},
	}

	var gatewayResponse struct {
		TransactionHash *felt.Felt `json:"transaction_hash"`
	}

	gatewayResponse.TransactionHash = txnToAdd.TransactionHash
	rawGatewayResponse, err := json.Marshal(gatewayResponse)
	require.NoError(t, err)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockGateway := mocks.NewMockGateway(mockCtrl)
	mockGateway.
		EXPECT().
		AddTransaction(gomock.Any(), gomock.Any()).
		Return(rawGatewayResponse, nil).
		Times(2)

	preConfirmedPlaceHolder := core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 1,
			},
		},
	}
	t.Run("transaction not found in db and feeder but found in cache", func(t *testing.T) {
		submittedTransactionCache := rpccore.NewTransactionCache(cacheEntryTimeOut, cacheSize)
		fakeClock := make(chan time.Time, 1)
		defer close(fakeClock)
		submittedTransactionCache.WithTicker(fakeClock)
		ctx := t.Context()
		go func() {
			err := submittedTransactionCache.Run(ctx)
			require.NoError(t, err)
		}()

		handler := rpcv10.New(mockReader, mockSyncReader, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(submittedTransactionCache)

		res, err := handler.AddTransaction(ctx, broadcastedTxn)
		require.Nil(t, err)
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			&res.TransactionHash,
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil).Times(2)

		status, err := handler.TransactionStatus(ctx, (*felt.Felt)(&res.TransactionHash))
		require.Nil(t, err)
		require.Equal(t, rpcv9.TxnStatusReceived, status.Finality)
		require.Equal(t, rpcv9.UnknownExecution, status.Execution)
	})

	t.Run("transaction not found in db and feeder, found in cache but expired", func(t *testing.T) {
		submittedTransactionCache := rpccore.NewTransactionCache(cacheEntryTimeOut, cacheSize)
		fakeClock := make(chan time.Time, 1)
		defer close(fakeClock)
		submittedTransactionCache.WithTicker(fakeClock)
		ctx := t.Context()
		go func() {
			err := submittedTransactionCache.Run(ctx)
			require.NoError(t, err)
		}()

		handler := rpcv10.New(mockReader, mockSyncReader, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(submittedTransactionCache)

		res, err := handler.AddTransaction(ctx, broadcastedTxn)
		require.Nil(t, err)
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			&res.TransactionHash,
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil).Times(2)

		// Expire cache entry
		for range rpccore.NumTimeBuckets {
			fakeClock <- time.Now()
		}
		status, err := handler.TransactionStatus(ctx, (*felt.Felt)(&res.TransactionHash))
		require.Equal(t, rpccore.ErrTxnHashNotFound, err)
		require.Empty(t, status)
	})
}

func TestAdaptBroadcastedTransactionValidation(t *testing.T) {
	network := &utils.Sepolia

	t.Run("RejectProofFactsForNonV3Invoke", func(t *testing.T) {
		proofFact := felt.FromUint64[felt.Felt](100)
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: rpcv9.Transaction{
					Type:    rpcv9.TxnInvoke,
					Version: felt.NewFromUint64[felt.Felt](1),
					Signature: &[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](0x1),
					},
					Nonce:         felt.NewFromUint64[felt.Felt](0x1),
					SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
					CallData:      &[]*felt.Felt{},
				},
			},
			ProofFacts: []felt.Felt{proofFact},
		}

		_, _, _, err := rpcv10.AdaptBroadcastedTransaction(broadcastedTxn, network)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proof_facts can only be included in invoke v3 transactions")
	})

	t.Run("RejectProofForNonV3Invoke", func(t *testing.T) {
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: rpcv9.Transaction{
					Type:    rpcv9.TxnInvoke,
					Version: felt.NewFromUint64[felt.Felt](1),
					Signature: &[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](0x1),
					},
					Nonce:         felt.NewFromUint64[felt.Felt](0x1),
					SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
					CallData:      &[]*felt.Felt{},
				},
			},
			Proof: []uint64{1, 2, 3},
		}

		_, _, _, err := rpcv10.AdaptBroadcastedTransaction(broadcastedTxn, network)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proof can only be included in invoke v3 transactions")
	})

	t.Run("RejectProofFactsForNonInvoke", func(t *testing.T) {
		proofFact := felt.FromUint64[felt.Felt](100)
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: rpcv9.Transaction{
					Type:    rpcv9.TxnDeclare,
					Version: felt.NewFromUint64[felt.Felt](3),
					Signature: &[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](0x1),
					},
					Nonce:         felt.NewFromUint64[felt.Felt](0x1),
					SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
				},
			},
			ProofFacts: []felt.Felt{proofFact},
		}

		_, _, _, err := rpcv10.AdaptBroadcastedTransaction(broadcastedTxn, network)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proof_facts can only be included in invoke v3 transactions")
	})
}

func createBaseInvokeTransactionV3() core.InvokeTransaction {
	return core.InvokeTransaction{
		TransactionHash: felt.NewFromUint64[felt.Felt](12345),
		Version:         new(core.TransactionVersion).SetUint64(3),
		TransactionSignature: []*felt.Felt{
			felt.NewFromUint64[felt.Felt](0x1),
			felt.NewFromUint64[felt.Felt](0x1),
		},
		Nonce:       felt.NewFromUint64[felt.Felt](0x1),
		NonceDAMode: core.DAModeL1,
		FeeDAMode:   core.DAModeL1,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: {
				MaxAmount:       0x1,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](0x1),
			},
			core.ResourceL1DataGas: {
				MaxAmount:       0x1,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](0x1),
			},
			core.ResourceL2Gas: {
				MaxAmount:       0,
				MaxPricePerUnit: &felt.Zero,
			},
		},
		Tip:                   0,
		PaymasterData:         []*felt.Felt{},
		SenderAddress:         felt.NewFromUint64[felt.Felt](0x1),
		CallData:              []*felt.Felt{},
		AccountDeploymentData: []*felt.Felt{},
	}
}
