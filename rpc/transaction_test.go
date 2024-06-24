package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTransactionByHashNotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)

	n := utils.Ptr(utils.Mainnet)
	txHash := new(felt.Felt).SetBytes([]byte("random hash"))
	mockReader.EXPECT().TransactionByHash(txHash).Return(nil, errors.New("tx not found"))

	handler := rpc.New(mockReader, nil, nil, "", n, nil)

	tx, rpcErr := handler.TransactionByHash(*txHash)
	assert.Nil(t, tx)
	assert.Equal(t, rpc.ErrTxnHashNotFound, rpcErr)
}

func TestTransactionByHash(t *testing.T) {
	tests := map[string]struct {
		hash     string
		network  *utils.Network
		expected string
	}{
		"DECLARE v1": {
			hash:    "0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae",
			network: utils.Ptr(utils.Mainnet),
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
			network: utils.Ptr(utils.Mainnet),
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
			network: utils.Ptr(utils.Mainnet),
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
			network: utils.Ptr(utils.Mainnet),
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
		"INVOKE v1": {
			hash:    "0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497",
			network: utils.Ptr(utils.Mainnet),
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
			network: utils.Ptr(utils.Mainnet),
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
			network: utils.Ptr(utils.Mainnet),
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
			network: utils.Ptr(utils.Mainnet),
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
			hash:    "0x16d437c46683719659cfdb9934a7e394b9e4a8da4b9ed82e928afbd6434017e",
			network: utils.Ptr(utils.Sepolia),
			expected: `{
				  "transaction_hash": "0x16d437c46683719659cfdb9934a7e394b9e4a8da4b9ed82e928afbd6434017e",
				  "type": "DECLARE",
				  "version": "0x3",
				  "nonce": "0x14",
				  "class_hash": "0x186f1ec45ad75cae3ba30c3ada2ef46941a821c9ad7629988641966978025ad",
				  "sender_address": "0x67329667da0cc3e8e89289af4861d2cc84e44c218b04f2515f49b4a7270f285",
				  "signature": [
					"0x3d2904a3aecffd4564ceea5b14705014f3ace1dbe482cf86d14288318dead24",
					"0x485ab8b4b007401030824214c92f45814ffc9c14c3c08718e2f46429dc6ae2a"
				  ],
				  "compiled_class_hash": "0x5853ce65472c3763373d8121d71c9c5e3ca15dd896bd1144d24567aa8cb8b95",
				  "resource_bounds": {
					"l1_gas": {
					  "max_amount": "0x1723",
					  "max_price_per_unit": "0x17fa7f1d5650"
					},
					"l2_gas": {
					  "max_amount": "0x0",
					  "max_price_per_unit": "0x0"
					}
				  },
				  "tip": "0x0",
				  "paymaster_data": [],
				  "account_deployment_data": [],
				  "nonce_data_availability_mode": "L1",
				  "fee_data_availability_mode": "L1"
			}`,
		},
		"INVOKE v3": {
			hash:    "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164",
			network: utils.Ptr(utils.Sepolia),
			expected: `{
			  "transaction_hash": "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164",
			  "type": "INVOKE",
			  "version": "0x3",
			  "nonce": "0x8",
			  "sender_address": "0x6247aaebf5d2ff56c35cce1585bf255963d94dd413a95020606d523c8c7f696",
			  "signature": [
				"0x1",
				"0x4235b7a9cad6024cbb3296325e23b2a03d34a95c3ee3d5c10e2b6076c257d77",
				"0x439de4b0c238f624c14c2619aa9d190c6c1d17f6556af09f1697cfe74f192fc"
			  ],
			  "calldata": [
				"0x1",
				"0x19c92fa87f4d5e3be25c3dd6a284f30282a07e87cd782f5fd387b82c8142017",
				"0x3059098e39fbb607bc96a8075eb4d17197c3a6c797c166470997571e6fa5b17",
				"0x0"
			  ],
			  "resource_bounds": {
				"l1_gas": {
				  "max_amount": "0xa0",
				  "max_price_per_unit": "0xe91444530acc"
				},
				"l2_gas": {
				  "max_amount": "0x0",
				  "max_price_per_unit": "0x0"
				}
			  },
			  "tip": "0x0",
			  "paymaster_data": [],
			  "account_deployment_data": [],
			  "nonce_data_availability_mode": "L1",
			  "fee_data_availability_mode": "L1"
			}`,
		},
		"DEPLOY ACCOUNT v3": {
			hash:    "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f",
			network: utils.Ptr(utils.Sepolia),
			expected: `{
			  "transaction_hash": "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f",
			  "type": "DEPLOY_ACCOUNT",
			  "version": "0x3",
			  "nonce": "0x0",
			  "contract_address_salt": "0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2",
			  "class_hash": "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b",
			  "constructor_calldata": [
				"0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2",
				"0x0"
			  ],
			  "signature": [
				"0x79ec88c0f655e07f49a66bc6d4d9e696cf578addf6a0538f81dc3b47ca66c64",
				"0x78d3f2549f6f5b8533730a0f4f76c4277bc1b358f805d7cf66414289ce0a46d"
			  ],
			  "resource_bounds": {
				"l1_gas": {
				  "max_amount": "0x1b52",
				  "max_price_per_unit": "0x15416c61bfea"
				},
				"l2_gas": {
				  "max_amount": "0x0",
				  "max_price_per_unit": "0x0"
				}
			  },
			  "tip": "0x0",
			  "paymaster_data": [],
			  "nonce_data_availability_mode": "L1",
			  "fee_data_availability_mode": "L1"
			}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			gw := adaptfeeder.New(feeder.NewTestClient(t, test.network))
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockReader.EXPECT().TransactionByHash(gomock.Any()).DoAndReturn(func(hash *felt.Felt) (core.Transaction, error) {
				return gw.Transaction(context.Background(), hash)
			}).Times(1)
			handler := rpc.New(mockReader, nil, nil, "", test.network, nil)

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
	n := utils.Ptr(utils.Mainnet)
	mockReader := mocks.NewMockReader(mockCtrl)
	client := feeder.NewTestClient(t, n)
	mainnetGw := adaptfeeder.New(client)

	latestBlockNumber := 19199
	latestBlock, err := mainnetGw.BlockByNumber(context.Background(), 19199)
	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash

	handler := rpc.New(mockReader, nil, nil, "", n, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(rpc.BlockID{Latest: true}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		txn, rpcErr := handler.TransactionByBlockIDAndIndex(
			rpc.BlockID{Hash: new(felt.Felt).SetBytes([]byte("random"))}, rand.Int())
		assert.Nil(t, txn)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, db.ErrKeyNotFound)

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
	n := utils.Ptr(utils.Sepolia)
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
			txn: txWithoutClass("0x1481f9561ab004a5b15e5a4b2691ddfc89d1a2a10bdb25c57350fa68c936bd2"),
			expectedJSON: `{
			  "transaction_hash": "0x1481f9561ab004a5b15e5a4b2691ddfc89d1a2a10bdb25c57350fa68c936bd2",
			  "version": "0x0",
			  "contract_address": "0x4a5889207f54a646bbc170c177549357105aa79dba9493ec34eea9f73ebc278",
			  "type": "INVOKE_FUNCTION",
			  "max_fee": "0x2386f26fc10000",
			  "signature": [
				"0x668c99e35f5e4bf8709d657c8f0f341770b05427594a9c6e6e564da301303dc",
				"0x668c99e35f5e4bf8709d657c8f0f341770b05427594a9c6e6e564da301303dc"
			  ],
			  "calldata": [
				"0x2",
				"0x30e93180b2e00b12c8c9d26d91ddef36fa36d3d4b346747ee26bff3562474fe",
				"0x27f806b163e00b12dc7f2e54f3865ceba98cadef57cc65c6e10f64195ccd015",
				"0x1",
				"0x0",
				"0x30e93180b2e00b12c8c9d26d91ddef36fa36d3d4b346747ee26bff3562474fe",
				"0x27f806b163e00b12dc7f2e54f3865ceba98cadef57cc65c6e10f64195ccd015",
				"0x1",
				"0x0"
			  ],
			  "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad"
			}`,
		},
		"invoke v1": {
			txn: txWithoutClass("0xacb2e8bdaeb179c9e3cdf5b21c7697ea7cda240a7f59f65fe243bcfd57d60a"),
			expectedJSON: `{
			  "transaction_hash": "0xacb2e8bdaeb179c9e3cdf5b21c7697ea7cda240a7f59f65fe243bcfd57d60a",
			  "version": "0x1",
			  "type": "INVOKE_FUNCTION",
			  "sender_address": "0x164b9e8615a1fe540ebda04ed0f38e945e06d9f892e120d265b856167ec573d",
			  "max_fee": "0x82be30cf82d5",
			  "signature": [
				"0x312aa541c46537e0199955ffa9f2c056c22b7f1cd3fb92f8db10f7e03a0eb6b",
				"0x6738dc3ead88cf3f21be0a22e4c59e7d21d15a657c587b3ddb0e3c5f7bd1721"
			  ],
			  "calldata": [
				"0x3",
				"0x30058f19ed447208015f6430f0102e8ab82d6c291566d7e73fe8e613c3d2ed",
				"0xa72371689866be053cc37a071de4216af73c9ffff96319b2576f7bf1e15290",
				"0x4",
				"0x5acb0547f4b06e5db4f73f5b6ea7425e9b59b6adc885ed8ecc4baeefae8b8d8",
				"0xa9f7640400",
				"0x11429301fb9b6dd9aa913b6bd05fa63a7ce57f7cfd56766c5ce7c9dc27433d",
				"0x517567ac7026ce129c950e6e113e437aa3c83716cd61481c6bb8c5057e6923e",
				"0x517567ac7026ce129c950e6e113e437aa3c83716cd61481c6bb8c5057e6923e",
				"0xcaffbd1bd76bd7f24a3fa1d69d1b2588a86d1f9d2359b13f6a84b7e1cbd126",
				"0x5",
				"0x5265736f6c766552616e646f6d4576656e74",
				"0x3",
				"0x0",
				"0x1",
				"0x101d",
				"0x517567ac7026ce129c950e6e113e437aa3c83716cd61481c6bb8c5057e6923e",
				"0xcaffbd1bd76bd7f24a3fa1d69d1b2588a86d1f9d2359b13f6a84b7e1cbd126",
				"0xa",
				"0x4163636570745072657061696441677265656d656e74",
				"0x8",
				"0x4",
				"0x18650500000001",
				"0x1",
				"0x1",
				"0x101d",
				"0x2819a0",
				"0x1",
				"0x101d"
			  ],
			  "nonce": "0x323"
			}`,
		},
		"invoke v3": {
			txn: txWithoutClass("0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164"),
			expectedJSON: `{
			  "transaction_hash": "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164",
			  "version": "0x3",
			  "type": "INVOKE_FUNCTION",
			  "sender_address": "0x6247aaebf5d2ff56c35cce1585bf255963d94dd413a95020606d523c8c7f696",
			  "signature": [
				"0x1",
				"0x4235b7a9cad6024cbb3296325e23b2a03d34a95c3ee3d5c10e2b6076c257d77",
				"0x439de4b0c238f624c14c2619aa9d190c6c1d17f6556af09f1697cfe74f192fc"
			  ],
			  "calldata": [
				"0x1",
				"0x19c92fa87f4d5e3be25c3dd6a284f30282a07e87cd782f5fd387b82c8142017",
				"0x3059098e39fbb607bc96a8075eb4d17197c3a6c797c166470997571e6fa5b17",
				"0x0"
			  ],
			  "nonce": "0x8",
			  "resource_bounds": {
				"L1_GAS": {
				  "max_amount": "0xa0",
				  "max_price_per_unit": "0xe91444530acc"
				},
				"L2_GAS": {
				  "max_amount": "0x0",
				  "max_price_per_unit": "0x0"
				}
			  },
			  "tip": "0x0",
			  "nonce_data_availability_mode": 0,
			  "fee_data_availability_mode": 0,
			  "account_deployment_data": [],
			  "paymaster_data": []
			}`,
		},
		"declare v1": {
			txn: txWithoutClass("0x1e36b82b3f24251e9ed8e693d5830c64790238a22ec7e46655083231d222df"),
			expectedJSON: `{
			  "transaction_hash": "0x1e36b82b3f24251e9ed8e693d5830c64790238a22ec7e46655083231d222df",
			  "version": "0x1",
			  "class_hash": "0x3ae2f9b340e70e3c6ae2101715ccde645f3766283bd3bfade4b5ce7cd7dc9c6",
			  "type": "DECLARE",
			  "sender_address": "0x472aa8128e01eb0df145810c9511a92852d62a68ba8198ce5fa414e6337a365",
			  "max_fee": "0x3c7ecb3ed13c00",
			  "signature": [
				"0x4bd022ad8f795f651008786e01f5d33e4c93c5453717e5885c36072ccb87ef5",
				"0x6ebc4d2b5ac856fbfaa897435a747c00791f81be51220129e93ba486ba9947f"
			  ],
			  "nonce": "0x9"
			}`,
		},
		"declare v2": {
			txn: func() rpc.BroadcastedTransaction {
				tx := txWithoutClass("0x327bc9c5d2db0759b775762de8345c390bf852d461f38bddc1dc078c2ec95da")
				tx.ContractClass = json.RawMessage([]byte(`{"sierra_program": {}}`))
				return tx
			}(),
			expectedJSON: `{
			  "transaction_hash": "0x327bc9c5d2db0759b775762de8345c390bf852d461f38bddc1dc078c2ec95da",
			  "version": "0x2",
			  "class_hash": "0x994c025e4d34d3629478e44035b87b3c2407e99ef12bde15a1f284fc13b77e",
			  "type": "DECLARE",
			  "sender_address": "0xcee714eaf27390e630c62aa4b51319f9eda813d6ddd12da0ae8ce00453cb4b",
			  "max_fee": "0x1c47ac44660bc60",
			  "signature": [
				"0x34a33226068e03d016de3e687b712914316b4b59e95acc08a13c0ff3c2c5d5f",
				"0x2d57fca03be8419d3c67071e62fd853643cc8bbf3fcf9247441cd1b729b46ac"
			  ],
			  "nonce": "0x10d",
			  "compiled_class_hash": "0x7291bcf3cf0aed566267c74c6bcf48d9701689c12ce0cfc5ecb5156cacf5dee",
			  "contract_class": {
				"sierra_program": "H4sIAAAAAAAA/6quBQQAAP//Q7+mowIAAAA="
			  }
			}`,
		},
		"declare v3": {
			txn: func() rpc.BroadcastedTransaction {
				tx := txWithoutClass("0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491")
				tx.ContractClass = json.RawMessage([]byte(`{"sierra_program": {}}`))
				return tx
			}(),
			expectedJSON: `{
			  "transaction_hash": "0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491",
			  "version": "0x3",
			  "class_hash": "0x2404dffbfe2910bd921f5935e628c01e457629fc779420a03b7e5e507212f36",
			  "type": "DECLARE",
			  "sender_address": "0x6aac79bb6c90e1e41c33cd20c67c0281c4a95f01b4e15ad0c3b53fcc6010cf8",
			  "signature": [
				"0x5be36745b03aaeb76712c68869f944f7c711f9e734763b8d0b4e5b834408ea4",
				"0x66c9dba8bb26ada30cf3a393a6c26bfd3a40538f19b3b4bfb57c7507962ae79"
			  ],
			  "nonce": "0x3",
			  "compiled_class_hash": "0x5047109bf7eb550c5e6b0c37714f6e0db4bb8b5b227869e0797ecfc39240aa7",
			  "resource_bounds": {
				"L1_GAS": {
				  "max_amount": "0x1f40",
				  "max_price_per_unit": "0x5af3107a4000"
				},
				"L2_GAS": {
				  "max_amount": "0x0",
				  "max_price_per_unit": "0x0"
				}
			  },
			  "tip": "0x0",
			  "nonce_data_availability_mode": 0,
			  "fee_data_availability_mode": 0,
			  "account_deployment_data": [],
			  "paymaster_data": [],
			  "contract_class": {
				"sierra_program": "H4sIAAAAAAAA/6quBQQAAP//Q7+mowIAAAA="
			  }
			}`,
		},
		"deploy account v1": {
			txn: txWithoutClass("0x2f1ebaeae4cecb1bda1f2e98ff75152c39afe2a71652f3f404e8f1fe21a7e93"),
			expectedJSON: `{
			  "transaction_hash": "0x2f1ebaeae4cecb1bda1f2e98ff75152c39afe2a71652f3f404e8f1fe21a7e93",
			  "version": "0x1",
			  "contract_address_salt": "0x52a760d516f0b5aa0875d018c5331401e5101d97f6ec578071fb9e98df77b86",
			  "class_hash": "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b",
			  "constructor_calldata": [
				"0x52a760d516f0b5aa0875d018c5331401e5101d97f6ec578071fb9e98df77b86",
				"0x0"
			  ],
			  "type": "DEPLOY_ACCOUNT",
			  "max_fee": "0x2865a35b6642",
			  "signature": [
				"0x27451728425e8d2ad924cab10a8a5c052682549e5d660e9b9bde85c87a11d85",
				"0x34ca83ac537208e87e9e02ae63c46a599d80ef3b080a88dbfe58abdd5039307"
			  ],
			  "nonce": "0x0"
			}`,
		},
		"deploy account v3": {
			txn: txWithoutClass("0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f"),
			expectedJSON: `{
			  "transaction_hash": "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f",
			  "version": "0x3",
			  "contract_address_salt": "0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2",
			  "class_hash": "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b",
			  "constructor_calldata": [
				"0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2",
				"0x0"
			  ],
			  "type": "DEPLOY_ACCOUNT",
			  "signature": [
				"0x79ec88c0f655e07f49a66bc6d4d9e696cf578addf6a0538f81dc3b47ca66c64",
				"0x78d3f2549f6f5b8533730a0f4f76c4277bc1b358f805d7cf66414289ce0a46d"
			  ],
			  "nonce": "0x0",
			  "resource_bounds": {
				"L1_GAS": {
				  "max_amount": "0x1b52",
				  "max_price_per_unit": "0x15416c61bfea"
				},
				"L2_GAS": {
				  "max_amount": "0x0",
				  "max_price_per_unit": "0x0"
				}
			  },
			  "tip": "0x0",
			  "nonce_data_availability_mode": 0,
			  "fee_data_availability_mode": 0,
			  "paymaster_data": []
			}`,
		},
		"deploy v0": {
			txn: txWithoutClass("0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee"),
			expectedJSON: `{
			  "transaction_hash": "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee",
			  "version": "0x0",
			  "contract_address_salt": "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8",
			  "class_hash": "0x3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e",
			  "constructor_calldata": [
				"0x69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a",
				"0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a",
				"0x1",
				"0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8"
			  ],
			  "type": "DEPLOY"
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
			require.Equal(t, rpcErr.Code, rpc.ErrInternal.Code)

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
			network:           utils.Ptr(utils.Sepolia),
			verifiedTxHash:    utils.HexToFelt(t, "0x1481f9561ab004a5b15e5a4b2691ddfc89d1a2a10bdb25c57350fa68c936bd2"),
			nonVerifiedTxHash: utils.HexToFelt(t, "0x1412f2723569be7f627af887d663b83bfc92e3975bb94848182f755ce9960e8"),
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
						handler := rpc.New(mockReader, nil, nil, "", test.network, nil)
						_, err := handler.TransactionStatus(ctx, *notFoundTest.hash)
						require.Equal(t, rpc.ErrTxnHashNotFound.Code, err.Code)

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
				handler := rpc.New(mockReader, nil, nil, "", test.network, nil).WithFeeder(client)

				_, err := handler.TransactionStatus(ctx, *test.notFoundTxHash)
				require.NotNil(t, err)
				require.Equal(t, err, rpc.ErrTxnHashNotFound)
			})
		})
	}
}
