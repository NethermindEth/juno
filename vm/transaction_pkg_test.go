package vm

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionMarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	tests := map[string]struct {
		Hash     *felt.Felt
		Expected string
	}{
		"invoke v0": {
			Hash: utils.HexToFelt(t, "0x1481f9561ab004a5b15e5a4b2691ddfc89d1a2a10bdb25c57350fa68c936bd2"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Invoke": {
				  "V0": {
					"version": "0x0",
					"contract_address": "0x4a5889207f54a646bbc170c177549357105aa79dba9493ec34eea9f73ebc278",
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
				  }
				}
			  },
			  "txn_hash": "0x1481f9561ab004a5b15e5a4b2691ddfc89d1a2a10bdb25c57350fa68c936bd2"
			}`,
		},
		"invoke v1": {
			Hash: utils.HexToFelt(t, "0xacb2e8bdaeb179c9e3cdf5b21c7697ea7cda240a7f59f65fe243bcfd57d60a"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Invoke": {
				  "V1": {
					"version": "0x1",
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
				  }
				}
			  },
			  "txn_hash": "0xacb2e8bdaeb179c9e3cdf5b21c7697ea7cda240a7f59f65fe243bcfd57d60a"
			}`,
		},
		"invoke v3": {
			Hash: utils.HexToFelt(t, "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Invoke": {
				  "V3": {
					"version": "0x3",
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
					"nonce_data_availability_mode": "L1",
					"fee_data_availability_mode": "L1",
					"account_deployment_data": [],
					"paymaster_data": []
				  }
				}
			  },
			  "txn_hash": "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164"
			}`,
		},
		"deploy v0": {
			Hash: utils.HexToFelt(t, "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Deploy": {
				  "version": "0x0",
				  "contract_address": "0x7cc55b21de4b7d6d7389df3b27de950924ac976d263ac8d71022d0b18155fc",
				  "contract_address_salt": "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8",
				  "class_hash": "0x3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e",
				  "constructor_calldata": [
					"0x69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a",
					"0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a",
					"0x1",
					"0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8"
				  ]
				}
			  },
			  "txn_hash": "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee"
			}`,
		},
		"declare v1": {
			Hash: utils.HexToFelt(t, "0x1e36b82b3f24251e9ed8e693d5830c64790238a22ec7e46655083231d222df"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Declare": {
				  "V1": {
					"version": "0x1",
					"class_hash": "0x3ae2f9b340e70e3c6ae2101715ccde645f3766283bd3bfade4b5ce7cd7dc9c6",
					"sender_address": "0x472aa8128e01eb0df145810c9511a92852d62a68ba8198ce5fa414e6337a365",
					"max_fee": "0x3c7ecb3ed13c00",
					"signature": [
					  "0x4bd022ad8f795f651008786e01f5d33e4c93c5453717e5885c36072ccb87ef5",
					  "0x6ebc4d2b5ac856fbfaa897435a747c00791f81be51220129e93ba486ba9947f"
					],
					"nonce": "0x9"
				  }
				}
			  },
			  "txn_hash": "0x1e36b82b3f24251e9ed8e693d5830c64790238a22ec7e46655083231d222df"
			}`,
		},
		"declare v2": {
			Hash: utils.HexToFelt(t, "0x327bc9c5d2db0759b775762de8345c390bf852d461f38bddc1dc078c2ec95da"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Declare": {
				  "V2": {
					"version": "0x2",
					"class_hash": "0x994c025e4d34d3629478e44035b87b3c2407e99ef12bde15a1f284fc13b77e",
					"sender_address": "0xcee714eaf27390e630c62aa4b51319f9eda813d6ddd12da0ae8ce00453cb4b",
					"max_fee": "0x1c47ac44660bc60",
					"signature": [
					  "0x34a33226068e03d016de3e687b712914316b4b59e95acc08a13c0ff3c2c5d5f",
					  "0x2d57fca03be8419d3c67071e62fd853643cc8bbf3fcf9247441cd1b729b46ac"
					],
					"nonce": "0x10d",
					"compiled_class_hash": "0x7291bcf3cf0aed566267c74c6bcf48d9701689c12ce0cfc5ecb5156cacf5dee"
				  }
				}
			  },
			  "txn_hash": "0x327bc9c5d2db0759b775762de8345c390bf852d461f38bddc1dc078c2ec95da"
			}`,
		},
		"declare v3": {
			Hash: utils.HexToFelt(t, "0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Declare": {
				  "V3": {
					"version": "0x3",
					"class_hash": "0x2404dffbfe2910bd921f5935e628c01e457629fc779420a03b7e5e507212f36",
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
					"nonce_data_availability_mode": "L1",
					"fee_data_availability_mode": "L1",
					"account_deployment_data": [],
					"paymaster_data": []
				  }
				}
			  },
			  "txn_hash": "0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491"
			}`,
		},
		"deploy account v1": {
			Hash: utils.HexToFelt(t, "0x2f1ebaeae4cecb1bda1f2e98ff75152c39afe2a71652f3f404e8f1fe21a7e93"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"DeployAccount": {
				  "V1": {
					"version": "0x1",
					"contract_address_salt": "0x52a760d516f0b5aa0875d018c5331401e5101d97f6ec578071fb9e98df77b86",
					"class_hash": "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b",
					"constructor_calldata": [
					  "0x52a760d516f0b5aa0875d018c5331401e5101d97f6ec578071fb9e98df77b86",
					  "0x0"
					],
					"max_fee": "0x2865a35b6642",
					"signature": [
					  "0x27451728425e8d2ad924cab10a8a5c052682549e5d660e9b9bde85c87a11d85",
					  "0x34ca83ac537208e87e9e02ae63c46a599d80ef3b080a88dbfe58abdd5039307"
					],
					"nonce": "0x0"
				  }
				}
			  },
			  "txn_hash": "0x2f1ebaeae4cecb1bda1f2e98ff75152c39afe2a71652f3f404e8f1fe21a7e93"
			}`,
		},
		"deploy account v3": {
			Hash: utils.HexToFelt(t, "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"DeployAccount": {
				  "V3": {
					"version": "0x3",
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
					"nonce_data_availability_mode": "L1",
					"fee_data_availability_mode": "L1",
					"paymaster_data": []
				  }
				}
			  },
			  "txn_hash": "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f"
			}`,
		},
		"declare v0": {
			Hash: utils.HexToFelt(t, "0x7e66506abf3baf370f94a5df8ba32cd70219d8058994fcafdc69854b0881bf2"),
			Expected: `{
			  "query_bit": false,
			  "txn": {
				"Declare": {
				  "V0": {
					"version": "0x0",
					"class_hash": "0x69ecab4645830a7c8be160e69d8014ab8c9c5bb30943438133aa6db8af3aaf2",
					"sender_address": "0x1",
					"max_fee": "0x0",
					"signature": [],
					"nonce": "0x0"
				  }
				}
			  },
			  "txn_hash": "0x7e66506abf3baf370f94a5df8ba32cd70219d8058994fcafdc69854b0881bf2"
			}`,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			txn, err := gw.Transaction(context.Background(), test.Hash)
			require.NoError(t, err)

			jsonB, err := marshalTxn(txn)
			require.NoError(t, err)
			assert.JSONEq(t, test.Expected, string(jsonB))
		})
	}
}
