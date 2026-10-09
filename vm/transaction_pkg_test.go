package vm

import (
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/adapters/adaptfeeder/adaptfeedertest"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionMarshal(t *testing.T) {
	txAt := func(network *networks.Network, blockNumber uint64, index int) core.Transaction {
		client := feeder.NewTestClient(t, network)
		return adaptfeedertest.Block(t, client, blockNumber).Transactions[index]
	}

	tests := map[string]struct {
		Txn      core.Transaction
		Expected string
	}{
		"invoke v0": {
			Txn: txAt(&networks.Mainnet, 8, 12),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Invoke": {
						"V0": {
							"version": "0x0",
							"contract_address": "0x43324c97e376d7d164abded1af1e73e9ce8214249f711edb7059c1ca34560e8",
							"max_fee": "0x0",
							"signature": [],
							"calldata": [
								"0x1b654cb59f978da2eee76635158e5ff1399bf607cb2d05e3e3b4e41d7660ca2",
								"0x2",
								"0x5f743efdb29609bfc2002041bdd5c72257c0c6b5c268fc929a3e516c171c731",
								"0x635afb0ea6c4cdddf93f42287b45b67acee4f08c6f6c53589e004e118491546"
							],
							"entry_point_selector": "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f"
						}
					}
				},
				"txn_hash": "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e"
			}`,
		},
		"invoke v1": {
			Txn: txAt(&networks.Sepolia, 469719, 2),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Invoke": {
						"V1": {
							"version": "0x1",
							"sender_address": "0x5598089625602db226a2149c5b2e47d985e56c5201007707b5623c146896295",
							"max_fee": "0x5c1a57b6ea",
							"signature": [
								"0x2b043d7e396849c783cf32c6184ce1505718b0916b5c6a8d052da362000f9d3",
								"0x295d3d0c6819a61e16d26155eed7eb2b9104eee4e92d2d8d2013ef02fb8a14a"
							],
							"calldata": [
								"0x1",
								"0x4715653320bf709d46a964cc5e719285153db9150bef7476a55efc0cffcc05f",
								"0x2214fe6a6e2545aebfe589b84884a2c528416482abec76605b7fdb1c31ce5b2",
								"0x0"
							],
							"nonce": "0xf144"
						}
					}
				},
				"txn_hash": "0x570f295777212b12cf75e681ce0c5d82bfd0088b936a98e2d6c3980fd495a33"
			}`,
		},
		"invoke v3": {
			Txn: txAt(&networks.Integration, 319132, 0),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Invoke": {
                        "V3": {
                            "version": "0x3",
                            "sender_address": "0x3f6f3bc663aedc5285d6013cc3ffcbc4341d86ab488b8b68d297f8258793c41",
                            "signature": [
                                "0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16",
                                "0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5"
                            ],
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
                            "nonce": "0xe97",
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
                            "nonce_data_availability_mode": "L1",
                            "fee_data_availability_mode": "L1",
                            "account_deployment_data": [],
                            "paymaster_data": []
                        }
                    }
                },
                "txn_hash": "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd"
            }`,
		},
		"invoke v3 with proof_facts": {
			// No real on-chain tx carries proof_facts yet, so the transaction is handcrafted.
			Txn: &core.InvokeTransaction{
				TransactionHash:      felt.NewUnsafeFromString[felt.Felt]("0xdeadbeef"),
				Version:              new(core.TransactionVersion).SetUint64(3),
				SenderAddress:        felt.NewUnsafeFromString[felt.Felt]("0x1"),
				TransactionSignature: []felt.Felt{felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2)},
				CallData:             []felt.Felt{felt.FromUint64[felt.Felt](1)},
				Nonce:                felt.NewUnsafeFromString[felt.Felt]("0x1"),
				ResourceBounds: map[core.Resource]core.ResourceBounds{
					core.ResourceL1Gas:     {MaxAmount: 1, MaxPricePerUnit: new(felt.Felt).SetUint64(1)},
					core.ResourceL1DataGas: {MaxAmount: 0, MaxPricePerUnit: new(felt.Felt)},
					core.ResourceL2Gas:     {MaxAmount: 0, MaxPricePerUnit: new(felt.Felt)},
				},
				NonceDAMode:           core.DAModeL1,
				FeeDAMode:             core.DAModeL1,
				AccountDeploymentData: []felt.Felt{},
				PaymasterData:         []felt.Felt{},
				ProofFacts: []felt.Felt{
					felt.FromUint64[felt.Felt](100),
					felt.FromUint64[felt.Felt](200),
				},
			},
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Invoke": {
                        "V3": {
                            "version": "0x3",
                            "sender_address": "0x1",
                            "signature": ["0x1", "0x2"],
                            "calldata": ["0x1"],
                            "nonce": "0x1",
                            "resource_bounds": {
                                "L1_GAS": {
                                    "max_amount": "0x1",
                                    "max_price_per_unit": "0x1"
                                },
                                "L1_DATA": {
                                    "max_amount": "0x0",
                                    "max_price_per_unit": "0x0"
                                },
                                "L2_GAS": {
                                    "max_amount": "0x0",
                                    "max_price_per_unit": "0x0"
                                }
                            },
                            "tip": "0x0",
                            "nonce_data_availability_mode": "L1",
                            "fee_data_availability_mode": "L1",
                            "account_deployment_data": [],
                            "paymaster_data": [],
                            "proof_facts": ["0x64", "0xc8"]
                        }
                    }
                },
                "txn_hash": "0xdeadbeef"
            }`,
		},
		"deploy v0": {
			Txn: txAt(&networks.Mainnet, 2889, 53),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Deploy": {
						"version": "0x0",
						"contract_address": "0x3fe2b97c1fd336e750087d68b9b867997fd64a2661ff3ca5a7c771641e8e7ac",
						"contract_address_salt": "0xc356fd2878d3b7ce9f7ff08aaaad342356d226ab812034e3e8ce5066ecf6",
						"class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62",
						"constructor_calldata": [
							"0x0"
						]
					}
				},
				"txn_hash": "0x260fabbb9a76bc91261cb47eec5ad929a7ed1936e56dd1533356d3b442112fd"
			}`,
		},
		"declare v1": {
			Txn: txAt(&networks.Mainnet, 9306, 46),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Declare": {
						"V1": {
							"version": "0x1",
							"class_hash": "0x2ed6bb4d57ad27a22972b81feb9d09798ff8c273684376ec72c154d90343453",
							"sender_address": "0xb8a60857ed233885155f1d839086ca7ad03e6d4237cc10b085a4652a61a23",
							"max_fee": "0x5af3107a4000",
							"signature": [
								"0x516b5999b47509105675dd4c6ed9c373448038cfd00549fe868695916eee0ff",
								"0x6c0189aaa56bfcb2a3e97198d04bd7a9750a4354b88f4e5edf57cf4d966ddda"
							],
							"nonce": "0x1d"
						}
					}
				},
				"txn_hash": "0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d"
			}`,
		},
		"declare v2": {
			Txn: txAt(&networks.Sepolia, 18, 0),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Declare": {
						"V2": {
							"version": "0x2",
							"class_hash": "0x16342ade8a7cc8296920731bc34b5a6530f5ee1dc1bfd3cc83cb3f519d6530a",
							"sender_address": "0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84",
							"max_fee": "0x58ece00bd5f",
							"signature": [
								"0x25db5938ed86d666ddfdbfe08fdaa1cdcec72911c979304f47851c76afc30ab",
								"0x1c8db05f7fe7aa3d549044c7128b62c9b0f69cdc97f6752ea33a875b6458e8b"
							],
							"nonce": "0x1",
							"compiled_class_hash": "0x7d50adbdf0ac129ba351f21b026e5ccf1741a318c13240e50795f1b7ecde94d"
						}
					}
				},
				"txn_hash": "0x3744af1511b472fa4dac94feefc944ec785c4a380e9b925ea408d4954729453"
			}`,
		},
		"declare v3": {
			Txn: txAt(&networks.Sepolia, 570000, 6),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Declare": {
						"V3": {
							"version": "0x3",
							"class_hash": "0x224518978adb773cfd4862a894e9d333192fbd24bc83841dc7d4167c09b89c5",
							"sender_address": "0x36d67ab362562a97f9fba8a1051cf8e37ff1a1449530fb9f1f0e32ac2da7d06",
							"signature": [
								"0x5c6a94302ef4b6d80a4c6a3eaf5ad30e11fa13aa78f7397a4f69901ceb12b7",
								"0x25bf97f481061f8abf5eb93e67eaebe6bb74dda34d7378a506f5ee2ff1daef1"
							],
							"nonce": "0x2b",
							"compiled_class_hash": "0x6ff9f7df06da94198ee535f41b214dce0b8bafbdb45e6c6b09d4b3b693b1f17",
							"resource_bounds": {
								"L1_GAS": {
									"max_amount": "0x0",
									"max_price_per_unit": "0x10968159929e"
								},
								"L1_DATA": {
									"max_amount": "0x120",
									"max_price_per_unit": "0x99f"
								},
								"L2_GAS": {
									"max_amount": "0x1ff3ec0",
									"max_price_per_unit": "0x197aa1ce3"
								}
							},
							"tip": "0x0",
							"nonce_data_availability_mode": "L1",
							"fee_data_availability_mode": "L1",
							"account_deployment_data": [],
							"paymaster_data": []
						}
					}
				},
				"txn_hash": "0x30c852c522274765e1d681bc8a84ce7c41118370ef2ba7d18a427ed29f5b155"
			}`,
		},
		"deploy account v1": {
			Txn: txAt(&networks.Sepolia, 0, 2),
			Expected: `{
				"query_bit": false,
				"txn": {
					"DeployAccount": {
						"V1": {
							"version": "0x1",
							"contract_address_salt": "0x0",
							"class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
							"constructor_calldata": [
								"0x12c4df40394d06f157edec8d0e64db61fe0c271149ea860c8fe98def29ecf02"
							],
							"max_fee": "0x0",
							"signature": [
								"0x13f82fd9238dfc8d01543f89be2b5d5589b3eb93d9c3b888f1f94b089768771",
								"0x2c279ec310c4dd58a296fab66b2624640780e79a1c5c87388e6150fb5384a9d"
							],
							"nonce": "0x0"
						}
					}
				},
				"txn_hash": "0x144f41e654d0916810a83df0fe8984043671200f28df1206f58566144e302dd"
			}`,
		},
		"deploy account v3": {
			Txn: txAt(&networks.Sepolia, 571531, 6),
			Expected: `{
				"query_bit": false,
				"txn": {
					"DeployAccount": {
						"V3": {
							"version": "0x3",
							"contract_address_salt": "0x2e94ba2293dfa45f86dfcf9952d7a33dc50ce2b00b932999fbe0844772604f3",
							"class_hash": "0x61dac032f228abef9c6626f995015233097ae253a7f72d68552db02f2971b8f",
							"constructor_calldata": [
								"0x2e94ba2293dfa45f86dfcf9952d7a33dc50ce2b00b932999fbe0844772604f3"
							],
							"signature": [
								"0x3ef7f047c95592a04d4d754888dd8f125480a48dee23ee86c115d5da2a86573",
								"0x65e8661ab1526b4f8ea50b76fea1a0e82543de1eb3885e415790d7e1b5a93c7"
							],
							"nonce": "0x0",
							"resource_bounds": {
								"L1_GAS": {
									"max_amount": "0x0",
									"max_price_per_unit": "0x1597b3274d88"
								},
								"L1_DATA": {
									"max_amount": "0x210",
									"max_price_per_unit": "0x97c"
								},
								"L2_GAS": {
									"max_amount": "0xe6fa0",
									"max_price_per_unit": "0x1920d1317"
								}
							},
							"tip": "0x0",
							"nonce_data_availability_mode": "L1",
							"fee_data_availability_mode": "L1",
							"paymaster_data": []
						}
					}
				},
				"txn_hash": "0x32413f8cee053089d6d7026a72e4108262ca3cfe868dd9159bc1dd160aec975"
			}`,
		},
		"declare v0": {
			Txn: txAt(&networks.Sepolia, 0, 1),
			Expected: `{
				"query_bit": false,
				"txn": {
					"Declare": {
						"V0": {
							"version": "0x0",
							"class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3",
							"sender_address": "0x1",
							"max_fee": "0x0",
							"signature": [],
							"nonce": "0x0"
						}
					}
				},
				"txn_hash": "0x32538718071ad83ccd09fca03fe3a17add776ec12002d1c4e16ad4b92ddf752"
			}`,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			jsonB, err := marshalTxn(test.Txn)
			require.NoError(t, err)
			assert.JSONEq(t, test.Expected, string(jsonB))
		})
	}
}

// TestVMTransactionFieldsMatchStarknetTransaction ensures that vm.Transaction stays in sync with
// starknet.Transaction. If a new field is added to starknet.Transaction, this test will fail as a
// reminder to update vm.Transaction (and its adaptTransaction function) accordingly.
//
// The following fields are intentionally absent from vm.Transaction:
//   - Hash: passed separately via the txn_hash field in the marshalTxn wrapper
//   - Type: not needed by the VM; the tx type is encoded in the JSON structure key
func TestVMTransactionFieldsMatchStarknetTransaction(t *testing.T) {
	// Fields in starknet.Transaction that are intentionally absent from vm.Transaction.
	omittedFromVM := map[string]bool{
		"Hash": true,
		"Type": true,
	}

	starknetType := reflect.TypeOf(starknet.Transaction{})
	vmType := reflect.TypeOf(Transaction{})

	vmFields := make(map[string]bool, vmType.NumField())
	for i := range vmType.NumField() {
		vmFields[vmType.Field(i).Name] = true
	}

	starknetFields := make(map[string]bool, starknetType.NumField())
	for i := range starknetType.NumField() {
		starknetFields[starknetType.Field(i).Name] = true
	}

	// Every starknet field (except intentionally omitted ones) must be present in vm.Transaction.
	for i := range starknetType.NumField() {
		name := starknetType.Field(i).Name
		if omittedFromVM[name] {
			continue
		}
		assert.True(t, vmFields[name], "vm.Transaction is missing field %q present in starknet.Transaction", name)
	}

	// Every vm field must be present in starknet.Transaction (no extra fields).
	for i := range vmType.NumField() {
		name := vmType.Field(i).Name
		assert.True(t, starknetFields[name], "vm.Transaction has extra field %q not present in starknet.Transaction", name)
	}
}
