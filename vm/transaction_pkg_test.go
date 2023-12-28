package vm

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feeder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionMarshal(t *testing.T) {
	client := feeder.NewTestClient(t, utils.Integration)
	gw := adaptfeeder.New(client)

	tests := map[string]struct {
		Hash     *felt.Felt
		Expected string
	}{
		"invoke v0": {
			Hash: utils.HexToFelt(t, "0x5e91283c1c04c3f88e4a98070df71227fb44dea04ce349c7eb379f85a10d1c3"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Invoke": {
                        "V0": {
                            "version": "0x0",
                            "contract_address": "0x2cbc1f6e80a024900dc949914c7692f802ba90012cda39115db5640f5eca847",
                            "max_fee": "0x0",
                            "signature": [],
                            "calldata": [
                                "0x79631f37538379fc32739605910733219b836b050766a2349e93ec375e62885",
                                "0x0"
                            ],
                            "entry_point_selector": "0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64"                        }
                    }
                },
                "txn_hash": "0x5e91283c1c04c3f88e4a98070df71227fb44dea04ce349c7eb379f85a10d1c3"
            }`,
		},
		"invoke v1": {
			Hash: utils.HexToFelt(t, "0x45d9c2c8e01bacae6dec3438874576a4a1ce65f1d4247f4e9748f0e7216838"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Invoke": {
                        "V1": {
                            "version": "0x1",
                            "sender_address": "0x219937256cd88844f9fdc9c33a2d6d492e253ae13814c2dc0ecab7f26919d46",
                            "max_fee": "0x2386f26fc10000",
                            "signature": [
                                "0x89aa2f42e07913b6dee313c3ef680efb99892feb3e2d08287e01e63418da7a",
                                "0x458fb4c942d5407d8c1ef1557d29487ab8217842d28a907d75ee0828243361"
                            ],
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
                            "nonce": "0x99d"
                        }
                    }
                },
                "txn_hash": "0x45d9c2c8e01bacae6dec3438874576a4a1ce65f1d4247f4e9748f0e7216838"
            }`,
		},
		"invoke v3": {
			Hash: utils.HexToFelt(t, "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd"),
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
		"deploy v0": {
			Hash: utils.HexToFelt(t, "0x2e3106421d38175020cd23a6f1bff87989a64cae6a679c54c7710a033d88faa"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Deploy": {
                        "version": "0x0",
                        "contract_address": "0x18c17afbe50afac8aa6e25bb95a6d7983f0d8a7557fbf98bdad3ccf531a9b60",
                        "contract_address_salt": "0x5de1c0a37865820ce4896872e78da6877b0a8eede3d363131734556a8815d52",
                        "class_hash": "0x71468bd837666b3a05cca1a5363b0d9e15cacafd6eeaddfbc4f00d5c7b9a51d",
                        "constructor_calldata": []
                    }
                },
                "txn_hash": "0x2e3106421d38175020cd23a6f1bff87989a64cae6a679c54c7710a033d88faa"
            }`,
		},
		"declare v1": {
			Hash: utils.HexToFelt(t, "0x2d667ed0aa3a8faef96b466972079826e592ec0aebefafd77a39f2ed06486b4"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Declare": {
                        "V1": {
                            "version": "0x1",
                            "class_hash": "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
                            "sender_address": "0x52125c1e043126c637d1436d9551ef6c4f6e3e36945676bbd716a56e3a41b7a",
                            "max_fee": "0x2386f26fc10000",
                            "signature": [
                                "0x17872d12092aa60331394f514de908309fdba185997fd3d0be1e2896cd1e053",
                                "0x66124ebfe1a34809b2223a9707ac796dc6f4b6310cb002bda1e4c062a4b2867"
                            ],
                            "nonce": "0x1078"
                        }
                    }
                },
                "txn_hash": "0x2d667ed0aa3a8faef96b466972079826e592ec0aebefafd77a39f2ed06486b4"
            }`,
		},
		"declare v2": {
			Hash: utils.HexToFelt(t, "0x44b971f7eface29b185f86dd7b3b70acb1e48e0ad459e3a41e06fc42937aaa4"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Declare": {
                        "V2": {
                            "version": "0x2",
                            "class_hash": "0x7cb013a4139335cefce52adc2ac342c0110811353e7992baefbe547200223c7",
                            "sender_address": "0x3bb81d22ecd0e0a6f3138bdc5c072ff5726c5add02bcfd5b81cd657a6ae10a8",
                            "max_fee": "0x50c8f30c048",
                            "signature": [
                                "0x42a40a113a4381e5f304fd28a707ba4182609db42062a7f36b9291bf8ae8ae7",
                                "0x6035bcf022f887c80dbc2b615e927d662637d2213335ee657893dce8ddabe5b"
                            ],
                            "nonce": "0x11",
                            "compiled_class_hash": "0x67f7deab53a3ba70500bdafe66fb3038bbbaadb36a6dd1a7a5fc5b094e9d724"
                        }
                    }
                },
                "txn_hash": "0x44b971f7eface29b185f86dd7b3b70acb1e48e0ad459e3a41e06fc42937aaa4"
            }`,
		},
		"declare v3": {
			Hash: utils.HexToFelt(t, "0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Declare": {
                        "V3": {
                            "version": "0x3",
                            "class_hash": "0x5ae9d09292a50ed48c5930904c880dab56e85b825022a7d689cfc9e65e01ee7",
                            "sender_address": "0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50",
                            "signature": [
                                "0x29a49dff154fede73dd7b5ca5a0beadf40b4b069f3a850cd8428e54dc809ccc",
                                "0x429d142a17223b4f2acde0f5ecb9ad453e188b245003c86fab5c109bad58fc3"
                            ],
                            "nonce": "0x1",
                            "compiled_class_hash": "0x1add56d64bebf8140f3b8a38bdf102b7874437f0c861ab4ca7526ec33b4d0f8",
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
                            "nonce_data_availability_mode": "L1",
                            "fee_data_availability_mode": "L1",
                            "account_deployment_data": [],
                            "paymaster_data": []
                        }
                    }
                },
                "txn_hash": "0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3"
            }`,
		},
		"deploy account v1": {
			Hash: utils.HexToFelt(t, "0x658f1c44ebf6a1540eac0680956c3a9d315f65d2cb3b53593345905fed3982a"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "DeployAccount": {
                        "V1": {
                            "version": "0x1",
                            "contract_address_salt": "0x7b9f4b7d6d49b60686004dd850a4b41c818d6eb69e226b8ea37ea025e6830f5",
                            "class_hash": "0x5a9941d0cc16b8619a3325055472da709a66113afcc6a8ab86055da7d29c5f8",
                            "constructor_calldata": [
                                "0x7b16a9b7bb08d36950aa5d27d4d2c64bfd54f3ae16a0e01f21a6d410cb5179c"
                            ],
                            "max_fee": "0x2386f273b213da",
                            "signature": [
                                "0x7d31509f555031323050ed226012f0c6361b3dc34f0f5d2c65a76870fd8908b",
                                "0x58d64f6d39dfb20586da0c40e3d575cab940009cdee6423b03268fd893bd27a"
                            ],
                            "nonce": "0x0"
                        }
                    }
                },
                "txn_hash": "0x658f1c44ebf6a1540eac0680956c3a9d315f65d2cb3b53593345905fed3982a"
            }`,
		},
		"deploy account v3": {
			Hash: utils.HexToFelt(t, "0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "DeployAccount": {
                        "V3": {
                            "version": "0x3",
                            "contract_address_salt": "0x0",
                            "class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738",
                            "constructor_calldata": [
                                "0x5cd65f3d7daea6c63939d659b8473ea0c5cd81576035a4d34e52fb06840196c"
                            ],
                            "signature": [
                                "0x6d756e754793d828c6c1a89c13f7ec70dbd8837dfeea5028a673b80e0d6b4ec",
                                "0x4daebba599f860daee8f6e100601d98873052e1c61530c630cc4375c6bd48e3"
                            ],
                            "nonce": "0x0",
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
                            "paymaster_data": []
                        }
                    }
                },
                "txn_hash": "0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0"
            }`,
		},
		"declare v0": {
			Hash: utils.HexToFelt(t, "0x6d346ba207eb124355960c19c737698ad37a3c920a588b741e0130ff5bd4d6d"),
			Expected: `{
                "query_bit": false,
                "txn": {
                    "Declare": {
                        "V0": {
                            "version": "0x0",
                            "class_hash": "0x71e6ef53e53e6f5ca792fc4a5799a33e6f4118e4fd1d948dca3a371506f0cc7",
                            "sender_address": "0x1",
                            "max_fee": "0x0",
                            "signature": [],
                            "nonce": "0x0"
                        }
                    }
                },
                "txn_hash": "0x6d346ba207eb124355960c19c737698ad37a3c920a588b741e0130ff5bd4d6d"
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
