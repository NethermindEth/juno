package rpcv7_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type expectedBlockTrace struct {
	blockHash   string
	blockNumber uint64
	wantTrace   string
}

func TestTraceFallback(t *testing.T) {
	t.Run("Goerli Integration", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			"old block": {
				blockHash:   "0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
				blockNumber: 0,
				wantTrace:   `[ { "trace_root": { "type": "DEPLOY", "constructor_invocation": { "contract_address": "0x7b196a359045d4d0c10f73bdf244a9e1205a615dbb754b8df40173364288534", "entry_point_selector": null, "calldata": [ "0x187d50a5cf3ebd6d4d6fa8e29e4cad0a237759c6416304a25c4ea792ed4bba4", "0x42f5af30d6693674296ad87301935d0c159036c3b24af4042ff0270913bf6c6" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 29 } }, "execution_resources": { "steps": 29, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x3fa1bff0c86f34b2eb32c26d12208b6bdb4a5f6a434ac1d4f0e2d1db71bd711" }, { "trace_root": { "type": "DEPLOY", "constructor_invocation": { "contract_address": "0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6", "entry_point_selector": null, "calldata": [ "0x5cedec15acd969b0fba39fec9e7d9bd4d0b33f100969ad3a4543039a6f696d4", "0xce9801d27b02543f4d88b60aa456860f94ee9f612fc56464abfbdeedc1ab72" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 29 } }, "execution_resources": { "steps": 29, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x154c02cc3165cceadaa32e7238a67061b3a1eac414138c4ebe1408f37fd93eb" }, { "trace_root": { "type": "INVOKE", "execute_invocation": { "contract_address": "0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6", "entry_point_selector": null, "calldata": [ "0x17d9c35a8b9a0d4512fa05eafec01c2758a7a5b7ec7b47408a24a4b33124d9b", "0x2", "0x7f800b5bf79637f8f83f47a8fc4d368b43695c781b22a899f11b5f2faba874a", "0x3a7a40d383612b0ad167aec8d90fb07e576e017d07948f63ac318b52511ae93" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 165, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7 } }, "execution_resources": { "steps": 165, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x7893675c16da857b7c4229cda449e08a4fe13b07ca817e79d1db02e8a046047" }, { "trace_root": { "type": "INVOKE", "execute_invocation": { "contract_address": "0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6", "entry_point_selector": null, "calldata": [ "0x17d9c35a8b9a0d4512fa05eafec01c2758a7a5b7ec7b47408a24a4b33124d9b", "0x2", "0x7f800b5bf79637f8f83f47a8fc4d368b43695c781b22a899f11b5f2faba874a", "0xf140b304e9266c72f1054116dd06d9c1c8e981db7bf34e3c6da99640e9a7c8" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 165, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7 } }, "execution_resources": { "steps": 165, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x4a277d67e3f42c4a343854081d1e2e9e425f1323255e4486d2badb37a1d8630" } ]`,
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0xe3828bd9154ab385e2cbb95b3b650365fb3c6a4321660d98ce8b0a9194f9a3",
				blockNumber: 300000,
				wantTrace:   `[ { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325", "0x0", "0x4", "0x4", "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "0x2", "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325", "0x0", "0x4", "0x4", "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "0x2", "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [ { "contract_address": "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "entry_point_selector": "0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325", "calldata": [ "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "0x2", "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x165e7db96ab97a63c621229617a6d49633737238673477a54720e4c952f2c7e", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [ { "order": 0, "from_address": "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "to_address": "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "payload": [ "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ] } ], "execution_resources": { "steps": 233, "memory_holes": 1, "range_check_builtin_applications": 5 } } ], "events": [], "messages": [], "execution_resources": { "steps": 374, "memory_holes": 4, "range_check_builtin_applications": 7 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x127089df3a1984", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [ { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x127089df3a1984", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x28d7d394810ad8c52741ad8f7564717fd02c10ced68657a81d0b6710ce22079", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [ "0x1" ], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 488, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } } ], "events": [], "messages": [], "execution_resources": { "steps": 548, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } }, "execution_resources": { "steps": 1011, "memory_holes": 44, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 30, "ecdsa_builtin_applications": 1, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x2a648ab1aa6847eb38507fc842e050f256562bf87b26083c332f3f21318c2c3" }, { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1", "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f", "0x0", "0x4", "0x4", "0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb", "0x2", "0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e", "0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1", "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f", "0x0", "0x4", "0x4", "0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb", "0x2", "0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e", "0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [ { "contract_address": "0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1", "entry_point_selector": "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f", "calldata": [ "0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb", "0x2", "0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e", "0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x13abfd2f333f9c69f690f1569140cdae25f6f66e3f371c9cbb998b65f664a85", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 166, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7 } } ], "events": [], "messages": [], "execution_resources": { "steps": 307, "memory_holes": 25, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 9 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x3b2d25cd7bccc", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [ { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x3b2d25cd7bccc", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x28d7d394810ad8c52741ad8f7564717fd02c10ced68657a81d0b6710ce22079", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [ "0x1" ], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 488, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } } ], "events": [], "messages": [], "execution_resources": { "steps": 548, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } }, "execution_resources": { "steps": 944, "memory_holes": 65, "pedersen_builtin_applications": 6, "range_check_builtin_applications": 32, "ecdsa_builtin_applications": 1, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0xbc984e8e1fe594dd518a3a51db4f338437a5d2fbdda772d4426b532a67ffff" } ]`,
			},
		}

		AssertTracedBlockTransactions(t, &utils.Integration, tests)
	})

	t.Run("Sepolia", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			"old block": {
				blockHash:   "0x37644818236ee05b7e3b180bed64ea70ee3dd1553ca334a5c2a290ee276f380",
				blockNumber: 3,
				wantTrace:   `[ { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5" ], "calls": [ { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "calldata": [ "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "0x0", "0x0", "0x1" ], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5" ], "calls": [ { "contract_address": "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5", "entry_point_selector": "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", "calldata": [], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "entry_point_type": "CONSTRUCTOR", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 0 } } ], "events": [], "messages": [], "execution_resources": { "steps": 69, "memory_holes": 2, "range_check_builtin_applications": 1 } } ], "events": [], "messages": [], "execution_resources": { "steps": 218, "memory_holes": 5, "range_check_builtin_applications": 3 } }, "execution_resources": { "steps": 307, "memory_holes": 5, "range_check_builtin_applications": 5, "ecdsa_builtin_applications": 1, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x3f786ecc4955a2602c91a291328518ef866cb7f3d50e4b16fd42282952623aa" }, { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x6d8ff7b212b08760c82e4a8f354f6ebc69d748290fa38e92eb859726a88f379" ], "calls": [ { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "calldata": [ "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "0x0", "0x0", "0x1" ], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x6d8ff7b212b08760c82e4a8f354f6ebc69d748290fa38e92eb859726a88f379" ], "calls": [ { "contract_address": "0x6d8ff7b212b08760c82e4a8f354f6ebc69d748290fa38e92eb859726a88f379", "entry_point_selector": "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", "calldata": [], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "entry_point_type": "CONSTRUCTOR", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 0 } } ], "events": [], "messages": [], "execution_resources": { "steps": 69, "memory_holes": 2, "range_check_builtin_applications": 1 } } ], "events": [], "messages": [], "execution_resources": { "steps": 218, "memory_holes": 5, "range_check_builtin_applications": 3 } }, "execution_resources": { "steps": 307, "memory_holes": 5, "range_check_builtin_applications": 5, "ecdsa_builtin_applications": 1, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x4010bd7b00e591c163729aa501691e89784c2afe77d71f7b27613e377738843" } ]`,
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0x733495d0744edd9785b400408fa87c8ad599f81859df544897f80a3fceab422",
				blockNumber: 40000,
				wantTrace:   `[ { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x2", "0x4d0b88ace5705bb7825f91ee95557d906600b7e7762f5615e6a4f407185a43a", "0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3", "0x2", "0x4e946d49fca553930846e35533342f88e59a841c24d9cf507ef28dd6b67cb9b", "0x3ea9c575cfdaa875f3fecaf7db4acdb536ee6b38b8d8a4c769c63d044f942dc", "0x6359ed638df79b82f2f9dbf92abbcb41b57f9dd91ead86b1c85d2dee192c", "0x1a8e87e9d2008fcd3ce423ae5219c21e49be18d05d72825feb7e2bb687ba35c", "0x2", "0x44cd44ad7abf35b9dbe1e17de3610d21", "0x9f806c191aa2a3d47f2b8efc4c412d2f" ], "caller_address": "0x0", "class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x56414c4944" ], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 990, "memory_holes": 63, "range_check_builtin_applications": 25, "ec_op_builtin_applications": 3 } }, "execute_invocation": { "contract_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x2", "0x4d0b88ace5705bb7825f91ee95557d906600b7e7762f5615e6a4f407185a43a", "0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3", "0x2", "0x4e946d49fca553930846e35533342f88e59a841c24d9cf507ef28dd6b67cb9b", "0x3ea9c575cfdaa875f3fecaf7db4acdb536ee6b38b8d8a4c769c63d044f942dc", "0x6359ed638df79b82f2f9dbf92abbcb41b57f9dd91ead86b1c85d2dee192c", "0x1a8e87e9d2008fcd3ce423ae5219c21e49be18d05d72825feb7e2bb687ba35c", "0x2", "0x44cd44ad7abf35b9dbe1e17de3610d21", "0x9f806c191aa2a3d47f2b8efc4c412d2f" ], "caller_address": "0x0", "class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x2", "0x0", "0x0" ], "calls": [ { "contract_address": "0x4d0b88ace5705bb7825f91ee95557d906600b7e7762f5615e6a4f407185a43a", "entry_point_selector": "0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3", "calldata": [ "0x4e946d49fca553930846e35533342f88e59a841c24d9cf507ef28dd6b67cb9b", "0x3ea9c575cfdaa875f3fecaf7db4acdb536ee6b38b8d8a4c769c63d044f942dc" ], "caller_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "class_hash": "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 26 } }, { "contract_address": "0x6359ed638df79b82f2f9dbf92abbcb41b57f9dd91ead86b1c85d2dee192c", "entry_point_selector": "0x1a8e87e9d2008fcd3ce423ae5219c21e49be18d05d72825feb7e2bb687ba35c", "calldata": [ "0x44cd44ad7abf35b9dbe1e17de3610d21", "0x9f806c191aa2a3d47f2b8efc4c412d2f" ], "caller_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "class_hash": "0x5a1a156fd2af56bb992ce31fd2a4765e9b65b84efce45f3063974decaa339a2", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 308, "memory_holes": 14, "range_check_builtin_applications": 6 } } ], "events": [], "messages": [], "execution_resources": { "steps": 1419, "memory_holes": 16, "range_check_builtin_applications": 31 } }, "fee_transfer_invocation": { "contract_address": "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x11ecef7f251258", "0x0" ], "caller_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "class_hash": "0x5327164fa21dca89a92e8eae8a5b7ab90f58373e71f0a16d285e5a4abe5a3cf", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [], "events": [ { "order": 0, "keys": [ "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9" ], "data": [ "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x11ecef7f251258", "0x0" ] } ], "messages": [], "execution_resources": { "steps": 876, "memory_holes": 56, "range_check_builtin_applications": 27, "pedersen_builtin_applications": 4 } }, "execution_resources": { "steps": 3285, "memory_holes": 135, "range_check_builtin_applications": 83, "pedersen_builtin_applications": 4, "ec_op_builtin_applications": 3, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x6aa7ec89f36e918c9a168ebc9818e9dd19515a2a4bef87d73e1decbd8a7d131" }, { "trace_root": { "type": "DEPLOY_ACCOUNT", "validate_invocation": { "contract_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "entry_point_selector": "0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895", "calldata": [ "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "0x13e91b7ca4192672", "0x1a3bd006d99712e91bd3fd2eb5fafb0f379d9d594125bb527ec7fc5e133122a" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 75, "ecdsa_builtin_applications": 1 } }, "constructor_invocation": { "contract_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "entry_point_selector": "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", "calldata": [ "0x1a3bd006d99712e91bd3fd2eb5fafb0f379d9d594125bb527ec7fc5e133122a" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "CONSTRUCTOR", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 41 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xe5e432c83b4f", "0x0" ], "caller_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "class_hash": "0x5ffbcfeb50d200a0677c48a129a11245a3fc519d1d98d76882d1c9a1b19c6ed", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [], "events": [ { "order": 0, "keys": [ "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9" ], "data": [ "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xe5e432c83b4f", "0x0" ] } ], "messages": [], "execution_resources": { "steps": 876, "memory_holes": 56, "range_check_builtin_applications": 27, "pedersen_builtin_applications": 4 } }, "execution_resources": { "steps": 992, "memory_holes": 56, "range_check_builtin_applications": 27, "pedersen_builtin_applications": 4, "ecdsa_builtin_applications": 1, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x97468f6928d72808b23fe775e7c71893087600792fb36e0d62ec191363bd34" }, { "trace_root": { "type": "DECLARE", "validate_invocation": { "contract_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "entry_point_selector": "0x289da278a8dc833409cabfdad1581e8e7d40e42dcaed693fa4008dcdb4963b3", "calldata": [ "0x1e7c85ba9d58309d1f257ba201523e1a7b695bfeb6523759da24effd8dc6c0f" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 73, "ecdsa_builtin_applications": 1 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xbf730c7e8f2b", "0x0" ], "caller_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "class_hash": "0x5ffbcfeb50d200a0677c48a129a11245a3fc519d1d98d76882d1c9a1b19c6ed", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [], "events": [ { "order": 0, "keys": [ "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9" ], "data": [ "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xbf730c7e8f2b", "0x0" ] } ], "messages": [], "execution_resources": { "steps": 876, "memory_holes": 56, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 27 } }, "execution_resources": { "steps": 949, "memory_holes": 56, "range_check_builtin_applications": 27, "pedersen_builtin_applications": 4, "ecdsa_builtin_applications": 1, "data_availability": { "l1_gas": 5, "l1_data_gas": 10 } } }, "transaction_hash": "0x6a2df1337b09691711a66fca7e93e9f9fbc04c70dc6a17b9284b7af39c1a6a1" } ]`,
			},
		}

		AssertTracedBlockTransactions(t, &utils.Sepolia, tests)
	})
}

func AssertTracedBlockTransactions(t *testing.T, n *utils.Network, tests map[string]expectedBlockTrace) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, n)
	gateway := adaptfeeder.New(client)

	mockReader := mocks.NewMockReader(mockCtrl)

	mockReader.EXPECT().BlockByNumber(gomock.Any()).DoAndReturn(func(number uint64) (block *core.Block, err error) {
		block, err = gateway.BlockByNumber(t.Context(), number)

		// Simulate L1 data availability to block receipts
		for _, receipt := range block.Receipts {
			receipt.ExecutionResources.DataAvailability = &core.DataAvailability{
				L1Gas:     5,
				L1DataGas: 10,
			}
		}
		return block, err
	}).AnyTimes()

	mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			mockReader.EXPECT().BlockByHash(felt.NewUnsafeFromString[felt.Felt](test.blockHash)).DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
				return mockReader.BlockByNumber(test.blockNumber)
			})

			handler := rpcv7.New(mockReader, nil, nil, n, nil)
			handler = handler.WithFeeder(client)
			traces, httpHeader, jErr := handler.TraceBlockTransactions(t.Context(), rpcv7.BlockID{Number: test.blockNumber})
			if n == &utils.Sepolia && description == "newer block" {
				// For the newer block test, we test 3 of the block traces (INVOKE, DEPLOY_ACCOUNT, DECLARE)
				traces = []rpcv7.TracedBlockTransaction{traces[0], traces[7], traces[11]}
			}

			require.Nil(t, jErr)
			assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")

			jsonStr, err := json.Marshal(traces)
			require.NoError(t, err)
			assert.JSONEq(t, test.wantTrace, string(jsonStr))
		})
	}
}

func TestTraceBlockTransactionsReturnsError(t *testing.T) {
	t.Run("no feeder client set", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockReader := mocks.NewMockReader(mockCtrl)

		n := &utils.Sepolia
		client := feeder.NewTestClient(t, n)
		gateway := adaptfeeder.New(client)

		blockNumber := uint64(40000)

		mockReader.EXPECT().BlockByNumber(gomock.Any()).DoAndReturn(func(number uint64) (block *core.Block, err error) {
			return gateway.BlockByNumber(t.Context(), number)
		}).Times(2)
		mockReader.EXPECT().BlockByHash(gomock.Any()).DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
			return mockReader.BlockByNumber(blockNumber)
		})
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()

		// No feeder client is set
		handler := rpcv7.New(mockReader, nil, nil, n, nil)

		tracedBlocks, httpHeader, jErr := handler.TraceBlockTransactions(t.Context(), rpcv7.BlockID{Number: blockNumber})

		require.Nil(t, tracedBlocks)
		require.Equal(t, rpccore.ErrInternal.Code, jErr.Code)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
	})
}

func TestTransactionTraceValidation(t *testing.T) {
	validInvokeTransactionTrace := rpcv7.TransactionTrace{
		Type:               rpcv7.TxnInvoke,
		ExecuteInvocation:  &rpcv6.ExecuteInvocation{},
		ExecutionResources: &rpcv7.ExecutionResources{},
	}

	invalidInvokeTransactionTrace := rpcv7.TransactionTrace{
		Type: rpcv7.TxnInvoke,
	}

	validDeployAccountTransactionTrace := rpcv7.TransactionTrace{
		Type:                  rpcv7.TxnDeployAccount,
		ConstructorInvocation: &rpcv6.FunctionInvocation{},
	}

	invalidDeployAccountTransactionTrace := rpcv7.TransactionTrace{
		Type: rpcv7.TxnDeployAccount,
	}

	validL1HandlerTransactionTrace := rpcv7.TransactionTrace{
		Type:               rpcv7.TxnL1Handler,
		FunctionInvocation: &rpcv6.FunctionInvocation{},
	}

	invalidL1HandlerTransactionTrace := rpcv7.TransactionTrace{
		Type: rpcv7.TxnL1Handler,
	}

	tests := []struct {
		name     string
		trace    rpcv7.TransactionTrace
		wantErr  bool
		expected string
	}{
		{
			name:     "valid INVOKE tx",
			trace:    validInvokeTransactionTrace,
			wantErr:  false,
			expected: `{"type":"INVOKE","execute_invocation":{"revert_reason":""},"execution_resources":{"steps":0,"data_availability":null}}`,
		},
		{
			name:     "invalid INVOKE tx",
			trace:    invalidInvokeTransactionTrace,
			wantErr:  true,
			expected: ``,
		},
		{
			name:     "valid DEPLOY_ACCOUNT tx",
			trace:    validDeployAccountTransactionTrace,
			wantErr:  false,
			expected: `{"type":"DEPLOY_ACCOUNT","constructor_invocation":{"contract_address":"0x0","entry_point_selector":null,"calldata":null,"caller_address":"0x0","class_hash":null,"entry_point_type":"","call_type":"","result":null,"calls":null,"events":null,"messages":null,"execution_resources":null},"execution_resources":null}`,
		},
		{
			name:     "invalid DEPLOY_ACCOUNT tx",
			trace:    invalidDeployAccountTransactionTrace,
			wantErr:  true,
			expected: ``,
		},
		{
			name:     "valid L1_HANDLER tx",
			trace:    validL1HandlerTransactionTrace,
			wantErr:  false,
			expected: `{"type":"L1_HANDLER","function_invocation":{"contract_address":"0x0","entry_point_selector":null,"calldata":null,"caller_address":"0x0","class_hash":null,"entry_point_type":"","call_type":"","result":null,"calls":null,"events":null,"messages":null,"execution_resources":null},"execution_resources":null}`,
		},
		{
			name:     "invalid L1_HANDLER tx",
			trace:    invalidL1HandlerTransactionTrace,
			wantErr:  true,
			expected: ``,
		},
	}

	validate := validator.Validator()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validate.Struct(test.trace)

			if test.wantErr {
				assert.Error(t, err, "Expected validation to fail, but it passed")
			} else {
				assert.NoError(t, err, "Expected validation to pass, but it failed")

				// Check marshalling (check required fields)
				j, err := json.Marshal(test.trace)
				require.NoError(t, err)
				assert.Equal(t, test.expected, string(j))
			}
		})
	}
}

func TestTraceTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv7.New(mockReader, mockSyncReader, mockVM, &utils.Mainnet, utils.NewNopZapLogger())

	t.Run("not found", func(t *testing.T) {
		t.Run("key not found", func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.Felt]("0xBBBB")
			// Receipt() returns error related to db
			mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)
			pending := core.NewPending(&core.Block{}, nil, nil)
			mockSyncReader.EXPECT().PendingData().Return(
				&pending,
				nil,
			)

			trace, httpHeader, err := handler.TraceTransaction(t.Context(), *hash)
			assert.Nil(t, trace)
			assert.Equal(t, rpccore.ErrTxnHashNotFound, err)
			assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
		})

		t.Run("other error", func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.Felt]("0xBBBB")
			// Receipt() returns some other error
			mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), errors.New("database error"))

			trace, httpHeader, err := handler.TraceTransaction(t.Context(), *hash)
			assert.Nil(t, trace)
			assert.Equal(t, rpccore.ErrTxnHashNotFound, err)
			assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
		})
	})
	t.Run("ok", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0")
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
			Version:         new(core.TransactionVersion).SetUint64(1),
		}

		header := &core.Header{
			Hash:             felt.NewUnsafeFromString[felt.Felt]("0xCAFEBABE"),
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil)
		mockReader.EXPECT().BlockByHash(header.Hash).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		innerExecutionResources := `{
			"pedersen": 0,
			"rangecheck": 0,
			"bitwise": 0,
			"ecdsa": 0,
			"ecop": 0,
			"keccak": 0,
			"poseidon": 0,
			"segmentarena": 0,
			"memoryholes": 0,
			"steps": 1
		}`

		vmTraceJSON := fmt.Sprintf(`{
			"type": "INVOKE",
			"validate_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [], "events": [], "messages": []}], "events": [], "messages": [], "execution_resources": %[1]s},
			"execute_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x38627c278c0b3cb3c84ddee2c783fb22c3c3a3f0e667ea2b82be0ea2253bce4", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x0", "0x0", "0x5"], "calls": [{"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x46668cd07d83af5d7158e7cd62c710f1a7573501bcd4f4092c6a4e1ecd2bf61", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x0", "0x0", "0x5"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1e8480", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1e8480", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2ceb6369dba6af865bca639f9f1342dfb1ae4e5d0d0723de98028b812e7cdd2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "entry_point_selector": "0x2c0f7bf2d6cf5304c29171bf493feb222fef84bdaf17805a6574b0c2e8bcc87", "calldata": ["0x1e8480", "0x0", "0x0", "0x0", "0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x648f780a"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x514718bb56ed2a8607554c7d393c2ffd73cbab971c120b00a2ce27cc58dd1c1", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x2", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x3c388f7eb137a89061c6f0b6e78bae453202258b0b3c419f8dd9814a547d406", "calldata": [], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b60b3a0bcc4aa98", "0xaf07589b7c", "0x648f7422"], "calls": [], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0xaf07771ffc", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0xaf07771ffc", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe14a408baf7f453312eec68e9b7d728ec5337fbdf671f917ee8c80f3255232"], "data": ["0x178b5c9bdd4e74e92b", "0xaf07771ffc"]}, {"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x0", "0x0", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"]}], "messages": []}], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x417c36e4fc16d", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x417c36e4fc16d", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0x5ad857f66a5b55f1301ff1ed7e098ac6d4433148f0b72ebc4a2945ab85ad53"], "data": ["0x2fc5e96de394697c1311606c96ec14840e408493fd42cf0c54b73b39d312b81", "0x2", "0x1", "0x1"]}], "messages": []}], "events": [], "messages": [], "execution_resources": %[1]s},
			"fee_transfer_invocation": {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"]}], "messages": []}], "events": [], "messages": [], "execution_resources": %[1]s},
			"state_diff": {
				"storage_diffs": [],
				"nonces": [],
				"deployed_contracts": [],
				"deprecated_declared_classes": [],
				"declared_classes": [],
				"replaced_classes": []
			}
		}`, innerExecutionResources)

		vmTrace := new(vm.TransactionTrace)
		require.NoError(t, json.Unmarshal(json.RawMessage(vmTraceJSON), vmTrace))

		dataGas := []core.DataAvailability{{L1Gas: 1, L1DataGas: 0}}
		overallFee := []*felt.Felt{new(felt.Felt).SetUint64(1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			false,
			false,
			false,
		).Return(
			vm.ExecutionResults{
				OverallFees:      overallFee,
				DataAvailability: dataGas,
				Traces:           []vm.TransactionTrace{*vmTrace},
				NumSteps:         stepsUsed,
			}, nil)

		trace, httpHeader, err := handler.TraceTransaction(t.Context(), *hash)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), stepsUsedStr)

		// Root level execution resources should be the sum of inner execution resources
		vmTrace.ExecutionResources = &vm.ExecutionResources{
			ComputationResources: vm.ComputationResources{
				Steps: 3,
			},
			DataAvailability: &vm.DataAvailability{
				L1Gas:     1,
				L1DataGas: 0,
			},
		}
		assert.Equal(t, rpcv7.AdaptVMTransactionTrace(vmTrace), *trace)
	})
	t.Run("pending block", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0xceb6a374aff2bbb3537cf35f50df8634b2354a21")
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
			Version:         new(core.TransactionVersion).SetUint64(1),
		}

		header := &core.Header{
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
		}
		require.Nil(t, header.Hash, "hash must be nil for pending block")

		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil)
		pendingStateDiff := core.EmptyStateDiff()
		pending := core.Pending{
			Block: block,
			StateUpdate: &core.StateUpdate{
				StateDiff: &pendingStateDiff,
			},
			NewClasses: map[felt.Felt]core.ClassDefinition{*tx.ClassHash: declaredClass.Class},
		}
		mockSyncReader.EXPECT().PendingData().Return(
			&pending,
			nil,
		).Times(2)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).
			Return(headState, nopCloser, nil).Times(2)

		innerExecutionResources := `{
			"pedersen": 0,
			"rangecheck": 0,
			"bitwise": 0,
			"ecdsa": 0,
			"ecop": 0,
			"keccak": 0,
			"poseidon": 0,
			"segmentarena": 0,
			"memoryholes": 0,
			"steps": 2
		}`

		vmTraceJSON := fmt.Sprintf(`{
			"type": "INVOKE",
			"validate_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [], "events": [], "messages": []}], "events": [], "messages": [], "execution_resources": %[1]s},
			"execute_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x38627c278c0b3cb3c84ddee2c783fb22c3c3a3f0e667ea2b82be0ea2253bce4", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x0", "0x0", "0x5"], "calls": [{"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x46668cd07d83af5d7158e7cd62c710f1a7573501bcd4f4092c6a4e1ecd2bf61", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x0", "0x0", "0x5"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1e8480", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1e8480", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2ceb6369dba6af865bca639f9f1342dfb1ae4e5d0d0723de98028b812e7cdd2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "entry_point_selector": "0x2c0f7bf2d6cf5304c29171bf493feb222fef84bdaf17805a6574b0c2e8bcc87", "calldata": ["0x1e8480", "0x0", "0x0", "0x0", "0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x648f780a"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x514718bb56ed2a8607554c7d393c2ffd73cbab971c120b00a2ce27cc58dd1c1", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x2", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x3c388f7eb137a89061c6f0b6e78bae453202258b0b3c419f8dd9814a547d406", "calldata": [], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b60b3a0bcc4aa98", "0xaf07589b7c", "0x648f7422"], "calls": [], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0xaf07771ffc", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0xaf07771ffc", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe14a408baf7f453312eec68e9b7d728ec5337fbdf671f917ee8c80f3255232"], "data": ["0x178b5c9bdd4e74e92b", "0xaf07771ffc"]}, {"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x0", "0x0", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"]}], "messages": []}], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x417c36e4fc16d", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x417c36e4fc16d", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0x5ad857f66a5b55f1301ff1ed7e098ac6d4433148f0b72ebc4a2945ab85ad53"], "data": ["0x2fc5e96de394697c1311606c96ec14840e408493fd42cf0c54b73b39d312b81", "0x2", "0x1", "0x1"]}], "messages": []}], "events": [], "messages": [], "execution_resources": %[1]s},
			"fee_transfer_invocation": {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"]}], "messages": []}], "events": [], "messages": [], "execution_resources": %[1]s},
			"state_diff": {
				"storage_diffs": [],
				"nonces": [],
				"deployed_contracts": [],
				"deprecated_declared_classes": [],
				"declared_classes": [],
				"replaced_classes": []
			}
		}`, innerExecutionResources)

		vmTrace := new(vm.TransactionTrace)
		require.NoError(t, json.Unmarshal(json.RawMessage(vmTraceJSON), vmTrace))

		consumedGas := []core.DataAvailability{{L1Gas: 1, L1DataGas: 2}}
		overallFee := []*felt.Felt{new(felt.Felt).SetUint64(1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(), false,
			false,
			false,
			false,
			false,
			false,
		).Return(vm.ExecutionResults{
			OverallFees:      overallFee,
			DataAvailability: consumedGas,
			Traces:           []vm.TransactionTrace{*vmTrace},
			NumSteps:         stepsUsed,
		}, nil)

		trace, httpHeader, err := handler.TraceTransaction(t.Context(), *hash)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), stepsUsedStr)

		// Root level execution resources should be the sum of inner execution resources
		vmTrace.ExecutionResources = &vm.ExecutionResources{
			ComputationResources: vm.ComputationResources{
				Steps: 6,
			},
			DataAvailability: &vm.DataAvailability{
				L1Gas:     1,
				L1DataGas: 2,
			},
		}
		assert.Equal(t, rpcv7.AdaptVMTransactionTrace(vmTrace), *trace)
	})

	t.Run("reverted INVOKE tx from feeder", func(t *testing.T) {
		n := &utils.Sepolia

		handler := rpcv7.New(mockReader, mockSyncReader, mockVM, n, utils.NewNopZapLogger())

		client := feeder.NewTestClient(t, n)
		handler.WithFeeder(client)
		gateway := adaptfeeder.New(client)

		// Tx at index 3 in the block
		revertedTxHash := felt.NewUnsafeFromString[felt.Felt]("0x2f00c7f28df2197196440747f97baa63d0851e3b0cfc2efedb6a88a7ef78cb1")

		blockNumber := uint64(18)
		blockHash := felt.NewUnsafeFromString[felt.Felt]("0x5beb56c7d9a9fc066e695c3fc467f45532cace83d9979db4ccfd6b77ca476af")

		mockReader.EXPECT().Receipt(revertedTxHash).Return(nil, blockHash, blockNumber, nil)
		mockReader.EXPECT().BlockByHash(blockHash).DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
			return gateway.BlockByNumber(t.Context(), blockNumber)
		}).Times(2)

		mockReader.EXPECT().L1Head().Return(core.L1Head{
			BlockNumber: 19, // Doesn't really matter for this test
		}, nil)

		expectedRevertedTrace := rpcv7.TransactionTrace{
			Type: rpcv7.TxnInvoke,
			ValidateInvocation: &rpcv6.FunctionInvocation{
				ContractAddress:    *felt.NewUnsafeFromString[felt.Felt]("0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
				EntryPointSelector: felt.NewUnsafeFromString[felt.Felt]("0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775"),
				Calldata: []felt.Felt{
					*felt.NewUnsafeFromString[felt.Felt]("0x1"),
					*felt.NewUnsafeFromString[felt.Felt]("0x7c687d151607710a7ec82ca5ab0ff2c48f52abd3b4a2773938a0cfef723fe6a"),
					*felt.NewUnsafeFromString[felt.Felt]("0x10b7e63d3ca05c9baffd985d3e1c3858d4dbf0759f066be0eaddc5d71c2cab5"),
					*felt.NewUnsafeFromString[felt.Felt]("0x1"),
					*felt.NewUnsafeFromString[felt.Felt]("0xa"),
				},
				CallerAddress:  *felt.NewUnsafeFromString[felt.Felt]("0x0"),
				ClassHash:      felt.NewUnsafeFromString[felt.Felt]("0x903752516de5c04fe91600ca6891e325278b2dfc54880ae11a809abb364844"),
				EntryPointType: "EXTERNAL",
				CallType:       "CALL",
				Result:         []felt.Felt{*felt.NewUnsafeFromString[felt.Felt]("0x56414c4944")},
				Calls:          []rpcv6.FunctionInvocation{},
				Events:         []rpcv6.OrderedEvent{},
				Messages:       []rpcv6.OrderedL2toL1Message{},
				ExecutionResources: &rpcv6.ComputationResources{
					Steps:       754,
					MemoryHoles: 5,
					RangeCheck:  17,
					EcOp:        3,
				},
			},
			FeeTransferInvocation: &rpcv6.FunctionInvocation{
				ContractAddress:    *felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				EntryPointSelector: felt.NewUnsafeFromString[felt.Felt]("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				Calldata: []felt.Felt{
					*felt.NewUnsafeFromString[felt.Felt]("0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
					*felt.NewUnsafeFromString[felt.Felt]("0x2847291f968"),
					*felt.NewUnsafeFromString[felt.Felt]("0x0"),
				},
				CallerAddress:  *felt.NewUnsafeFromString[felt.Felt]("0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
				ClassHash:      felt.NewUnsafeFromString[felt.Felt]("0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3"),
				EntryPointType: "EXTERNAL",
				CallType:       "CALL",
				Result:         []felt.Felt{*felt.NewUnsafeFromString[felt.Felt]("0x1")},
				Calls: []rpcv6.FunctionInvocation{
					{
						ContractAddress:    *felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
						EntryPointSelector: felt.NewUnsafeFromString[felt.Felt]("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
						Calldata: []felt.Felt{
							*felt.NewUnsafeFromString[felt.Felt]("0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
							*felt.NewUnsafeFromString[felt.Felt]("0x2847291f968"),
							*felt.NewUnsafeFromString[felt.Felt]("0x0"),
						},
						CallerAddress:  *felt.NewUnsafeFromString[felt.Felt]("0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
						ClassHash:      felt.NewUnsafeFromString[felt.Felt]("0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1"),
						EntryPointType: "EXTERNAL",
						CallType:       "DELEGATE",
						Result:         []felt.Felt{*felt.NewUnsafeFromString[felt.Felt]("0x1")},
						Calls:          []rpcv6.FunctionInvocation{},
						Events: []rpcv6.OrderedEvent{
							{
								Order: 0,
								Keys:  []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9")},
								Data: []*felt.Felt{
									felt.NewUnsafeFromString[felt.Felt]("0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
									felt.NewUnsafeFromString[felt.Felt]("0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
									felt.NewUnsafeFromString[felt.Felt]("0x2847291f968"),
									felt.NewUnsafeFromString[felt.Felt]("0x0"),
								},
							},
						},
						Messages: []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{
							Steps:       529,
							MemoryHoles: 57,
							Pedersen:    4,
							RangeCheck:  21,
						},
					},
				},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
				ExecutionResources: &rpcv6.ComputationResources{
					Steps:       589,
					MemoryHoles: 57,
					Pedersen:    4,
					RangeCheck:  21,
				},
			},
			ExecuteInvocation: &rpcv6.ExecuteInvocation{
				RevertReason: "Error in the called contract (0x070503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84):\nError at pc=0:4288:\nGot an exception while executing a hint: Custom Hint Error: Execution failed. Failure reason: 'Fatal'.\nCairo traceback (most recent call last):\nUnknown location (pc=0:67)\nUnknown location (pc=0:1997)\nUnknown location (pc=0:2729)\nUnknown location (pc=0:3577)\n",
			},
			ExecutionResources: &rpcv7.ExecutionResources{
				ComputationResources: rpcv7.ComputationResources{
					Steps:       1343,
					MemoryHoles: 62,
					Pedersen:    4,
					RangeCheck:  38,
					EcOp:        3,
				},
				DataAvailability: &rpcv7.DataAvailability{},
			},
		}

		trace, httpHeader, err := handler.TraceTransaction(t.Context(), *revertedTxHash)

		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
		assert.Equal(t, expectedRevertedTrace, *trace)
	})
}

func TestTraceBlockTransactions(t *testing.T) {
	errTests := map[string]rpcv7.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			handler := rpcv7.New(chain, nil, nil, n, log)

			if description == "pending" {
				mockCtrl := gomock.NewController(t)
				t.Cleanup(mockCtrl.Finish)

				mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)

				handler = rpcv7.New(chain, mockSyncReader, nil, n, log)
			}

			update, httpHeader, rpcErr := handler.TraceBlockTransactions(t.Context(), id)
			assert.Nil(t, update)
			assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()

	handler := rpcv7.New(mockReader, mockSyncReader, mockVM, n, log)

	t.Run("pending block", func(t *testing.T) {
		header := &core.Header{
			// hash is not set because it's pending block
			ParentHash:      felt.NewUnsafeFromString[felt.Felt]("0x0C3"),
			Number:          0,
			L1GasPriceETH:   felt.NewUnsafeFromString[felt.Felt]("0x777"),
			ProtocolVersion: "99.12.3",
		}
		l1Tx := &core.L1HandlerTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x000000C"),
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}
		declareTx := &core.DeclareTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x000000001"),
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x00000BC00"),
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{l1Tx, declareTx},
		}

		pendingStateDiff := core.EmptyStateDiff()
		pending := core.Pending{
			Block: block,
			StateUpdate: &core.StateUpdate{
				StateDiff: &pendingStateDiff,
			},
			NewClasses: map[felt.Felt]core.ClassDefinition{*declareTx.ClassHash: declaredClass.Class},
		}
		mockSyncReader.EXPECT().PendingData().Return(&pending, nil).Times(2)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).
			Return(headState, nopCloser, nil).Times(2)

		paidL1Fees := []*felt.Felt{(&felt.Felt{}).SetUint64(1)}
		vmTraceJSON := json.RawMessage(`{
			"type": "INVOKE",
			"validate_invocation": {"execution_resources":{}},
			"execute_invocation": {"execution_resources":{}},
			"fee_transfer_invocation": {"execution_resources":{}},
			"state_diff": {
				"storage_diffs": [],
				"nonces": [],
				"deployed_contracts": [],
				"deprecated_declared_classes": [],
				"declared_classes": [],
				"replaced_classes": []
			}
		}`)

		vmTrace := vm.TransactionTrace{}
		require.NoError(t, json.Unmarshal(vmTraceJSON, &vmTrace))

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Execute(
			block.Transactions,
			[]core.ClassDefinition{declaredClass.Class},
			paidL1Fees,
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			false,
			false,
			false,
		).Return(vm.ExecutionResults{
			DataAvailability: []core.DataAvailability{{}, {}},
			Traces:           []vm.TransactionTrace{vmTrace, vmTrace},
			NumSteps:         stepsUsed,
		}, nil)

		result, httpHeader, err := handler.TraceBlockTransactions(
			t.Context(),
			rpcv7.BlockID{Pending: true},
		)

		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), stepsUsedStr)
		assert.Equal(t, &rpcv7.TransactionTrace{
			Type: rpcv7.TxnInvoke,
			ValidateInvocation: &rpcv6.FunctionInvocation{
				Calls:              []rpcv6.FunctionInvocation{},
				Events:             []rpcv6.OrderedEvent{},
				Messages:           []rpcv6.OrderedL2toL1Message{},
				ExecutionResources: &rpcv6.ComputationResources{},
			},
			ExecuteInvocation: &rpcv6.ExecuteInvocation{
				FunctionInvocation: &rpcv6.FunctionInvocation{
					Calls:              []rpcv6.FunctionInvocation{},
					Events:             []rpcv6.OrderedEvent{},
					Messages:           []rpcv6.OrderedL2toL1Message{},
					ExecutionResources: &rpcv6.ComputationResources{},
				},
			},
			FeeTransferInvocation: &rpcv6.FunctionInvocation{
				Calls:              []rpcv6.FunctionInvocation{},
				Events:             []rpcv6.OrderedEvent{},
				Messages:           []rpcv6.OrderedL2toL1Message{},
				ExecutionResources: &rpcv6.ComputationResources{},
			},
			ExecutionResources: &rpcv7.ExecutionResources{
				DataAvailability: &rpcv7.DataAvailability{},
			},
			StateDiff: &rpcv6.StateDiff{
				StorageDiffs:              []rpcv6.StorageDiff{},
				Nonces:                    []rpcv6.Nonce{},
				DeployedContracts:         []rpcv6.DeployedContract{},
				DeprecatedDeclaredClasses: []*felt.Felt{},
				DeclaredClasses:           []rpcv6.DeclaredClass{},
				ReplacedClasses:           []rpcv6.ReplacedClass{},
			},
		}, result[0].TraceRoot)
		assert.Equal(t, l1Tx.TransactionHash, result[0].TransactionHash)
	})

	t.Run("regular block", func(t *testing.T) {
		blockHash := felt.NewUnsafeFromString[felt.Felt]("0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0")
		tx := &core.DeclareTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x000000001"),
			ClassHash:       felt.NewUnsafeFromString[felt.Felt]("0x000000000"),
		}

		header := &core.Header{
			Hash:             blockHash,
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			Number:           0,
			SequencerAddress: felt.NewUnsafeFromString[felt.Felt]("0X111"),
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x777"),
			ProtocolVersion:  "99.12.3",
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().BlockByHash(blockHash).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTraceJSON := json.RawMessage(`{
			"type":"INVOKE",
			"validate_invocation":{"entry_point_selector":"0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895","calldata":["0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[{"entry_point_selector":"0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895","calldata":["0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":[],"calls":[],"events":[],"messages":[]}],"events":[],"messages":[], "execution_resources":{"steps": 1}},
			"execute_invocation":{"entry_point_selector":"0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194","calldata":["0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","entry_point_type":"CONSTRUCTOR","call_type":"CALL","result":[],"calls":[{"entry_point_selector":"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","calldata":["0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":[],"calls":[],"events":[{"keys":["0x10c19bef19acd19b2c9f4caa40fd47c9fbe1d9f91324d44dcd36be2dae96784"],"data":["0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"]}],"messages":[]}],"events":[],"messages":[], "execution_resources": {"steps": 2}},
			"fee_transfer_invocation":{"entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"],"caller_address":"0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","class_hash":"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3","entry_point_type":"EXTERNAL","call_type":"CALL","result":["0x1"],"calls":[{"entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"],"caller_address":"0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","class_hash":"0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":["0x1"],"calls":[],"events":[{"keys":["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"],"data":["0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"]}],"messages":[]}],"events":[],"messages":[], "execution_resources": {"steps": 3}},
			"execution_resources":{"steps":777,"data_availability":{"l1_gas":555}},
			"state_diff": {
				"storage_diffs": [],
				"nonces": [],
				"deployed_contracts": [],
				"deprecated_declared_classes": [],
				"declared_classes": [],
				"replaced_classes": []
			}
		}`)
		vmTrace := vm.TransactionTrace{}
		require.NoError(t, json.Unmarshal(vmTraceJSON, &vmTrace))
		stepsUsed := uint64(123)
		stepsUsedStr := "123"
		mockVM.EXPECT().Execute(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			false,
			false,
			false,
			false,
			false,
			false).Return(vm.ExecutionResults{
			DataAvailability: []core.DataAvailability{{L1Gas: 123, L1DataGas: 456}},
			Traces:           []vm.TransactionTrace{vmTrace},
			NumSteps:         stepsUsed,
		}, nil)

		result, httpHeader, err := handler.TraceBlockTransactions(t.Context(), rpcv7.BlockID{Hash: blockHash})

		expectedTrace := rpcv7.AdaptVMTransactionTrace(&vmTrace)

		// The VM trace's root-level ExecutionResources get overridden by the sum of inner call's
		// ComputationResources and ExecutionResults.DataAvailability
		expectedTrace.ExecutionResources = &rpcv7.ExecutionResources{
			ComputationResources: rpcv7.ComputationResources{Steps: 6},
			DataAvailability:     &rpcv7.DataAvailability{L1Gas: 123, L1DataGas: 456},
		}

		expectedResult := []rpcv7.TracedBlockTransaction{
			{
				TransactionHash: tx.Hash(),
				TraceRoot:       &expectedTrace,
			},
		}

		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), stepsUsedStr)
		assert.Equal(t, expectedResult, result)
	})
}

func TestAdaptVMTransactionTrace(t *testing.T) {
	t.Run("successfully adapt INVOKE trace from vm", func(t *testing.T) {
		fromAddr := felt.NewUnsafeFromString[felt.Felt](
			"0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f",
		)
		toAddr := felt.NewUnsafeFromString[felt.Address](
			"0x540552aae708306346466633036396334303062342d24292eadbdc777db86e5",
		)

		payload0 := &felt.Zero
		payload1 := felt.NewUnsafeFromString[felt.Felt]("0x5ba586f822ce9debae27fa04a3e71721fdc90ff")
		payload2 := felt.NewFromUint64[felt.Felt](0x455448)
		payload3 := felt.NewFromUint64[felt.Felt](0x31da07977d000)
		payload4 := &felt.Zero

		vmTrace := vm.TransactionTrace{
			Type: vm.TxnInvoke,
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						To:    toAddr,
						Payload: []*felt.Felt{
							payload0,
							payload1,
							payload2,
							payload3,
							payload4,
						},
					},
				},
				ExecutionResources: &vm.ExecutionResources{
					L1Gas:     1,
					L1DataGas: 2,
					L2Gas:     3,
					ComputationResources: vm.ComputationResources{
						Steps:        1,
						MemoryHoles:  2,
						Pedersen:     3,
						RangeCheck:   4,
						Bitwise:      5,
						Ecdsa:        6,
						EcOp:         7,
						Keccak:       8,
						Poseidon:     9,
						SegmentArena: 10,
						AddMod:       11,
						MulMod:       12,
						RangeCheck96: 13,
						Output:       14,
					},
					DataAvailability: &vm.DataAvailability{
						L1Gas:     1,
						L1DataGas: 2,
					},
				},
			},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
			StateDiff: &vm.StateDiff{ //nolint:dupl
				StorageDiffs: []vm.StorageDiff{
					{
						Address: felt.Zero,
						StorageEntries: []vm.Entry{
							{
								Key:   felt.Zero,
								Value: felt.Zero,
							},
						},
					},
				},
				Nonces: []vm.Nonce{
					{
						ContractAddress: felt.Zero,
						Nonce:           felt.Zero,
					},
				},
				DeployedContracts: []vm.DeployedContract{
					{
						Address:   felt.Zero,
						ClassHash: felt.Zero,
					},
				},
				DeprecatedDeclaredClasses: []*felt.Felt{
					&felt.Zero,
				},
				DeclaredClasses: []vm.DeclaredClass{
					{
						ClassHash:         felt.Zero,
						CompiledClassHash: felt.Zero,
					},
				},
				ReplacedClasses: []vm.ReplacedClass{
					{
						ContractAddress: felt.Zero,
						ClassHash:       felt.Zero,
					},
				},
			},
		}

		expectedAdaptedTrace := rpcv7.TransactionTrace{
			Type: rpcv7.TxnInvoke,
			ValidateInvocation: &rpcv6.FunctionInvocation{
				Calls:  []rpcv6.FunctionInvocation{},
				Events: []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						// todo(rdr): this conversion is unnecessary but the right fix it's a huge
						//            refactor
						To: (*felt.Felt)(toAddr),
						Payload: []*felt.Felt{
							payload0,
							payload1,
							payload2,
							payload3,
							payload4,
						},
					},
				},
				ExecutionResources: &rpcv6.ComputationResources{
					Steps:        1,
					MemoryHoles:  2,
					Pedersen:     3,
					RangeCheck:   4,
					Bitwise:      5,
					Ecdsa:        6,
					EcOp:         7,
					Keccak:       8,
					Poseidon:     9,
					SegmentArena: 10,
				},
			},
			FeeTransferInvocation: &rpcv6.FunctionInvocation{
				Calls:    []rpcv6.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
			ExecuteInvocation: &rpcv6.ExecuteInvocation{
				RevertReason: "",
				FunctionInvocation: &rpcv6.FunctionInvocation{
					Calls:    []rpcv6.FunctionInvocation{},
					Events:   []rpcv6.OrderedEvent{},
					Messages: []rpcv6.OrderedL2toL1Message{},
				},
			},
			StateDiff: &rpcv6.StateDiff{ //nolint:dupl
				StorageDiffs: []rpcv6.StorageDiff{
					{
						Address: felt.Zero,
						StorageEntries: []rpcv6.Entry{
							{
								Key:   felt.Zero,
								Value: felt.Zero,
							},
						},
					},
				},
				Nonces: []rpcv6.Nonce{
					{
						ContractAddress: felt.Zero,
						Nonce:           felt.Zero,
					},
				},
				DeployedContracts: []rpcv6.DeployedContract{
					{
						Address:   felt.Zero,
						ClassHash: felt.Zero,
					},
				},
				DeprecatedDeclaredClasses: []*felt.Felt{
					&felt.Zero,
				},
				DeclaredClasses: []rpcv6.DeclaredClass{
					{
						ClassHash:         felt.Zero,
						CompiledClassHash: felt.Zero,
					},
				},
				ReplacedClasses: []rpcv6.ReplacedClass{
					{
						ContractAddress: felt.Zero,
						ClassHash:       felt.Zero,
					},
				},
			},
		}

		assert.Equal(t, expectedAdaptedTrace, rpcv7.AdaptVMTransactionTrace(&vmTrace))
	})

	t.Run("successfully adapt DEPLOY_ACCOUNT tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnDeployAccount,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
		}

		expectedAdaptedTrace := rpcv7.TransactionTrace{
			Type: rpcv7.TxnDeployAccount,
			ValidateInvocation: &rpcv6.FunctionInvocation{
				Calls:    []rpcv6.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
			FeeTransferInvocation: &rpcv6.FunctionInvocation{
				Calls:    []rpcv6.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
			ConstructorInvocation: &rpcv6.FunctionInvocation{
				Calls:    []rpcv6.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
		}

		adaptedTrace := rpcv7.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})

	t.Run("successfully adapt L1_HANDLER tx from vm", func(t *testing.T) {
		t.Run("Execution succeed", func(t *testing.T) {
			vmTrace := vm.TransactionTrace{
				Type:                  vm.TxnL1Handler,
				ValidateInvocation:    &vm.FunctionInvocation{},
				FeeTransferInvocation: &vm.FunctionInvocation{},
				ExecuteInvocation: &vm.ExecuteInvocation{
					RevertReason:       "",
					FunctionInvocation: &vm.FunctionInvocation{},
				},
				ConstructorInvocation: &vm.FunctionInvocation{},
				FunctionInvocation: &vm.ExecuteInvocation{
					FunctionInvocation: &vm.FunctionInvocation{},
				},
			}

			expectedAdaptedTrace := rpcv7.TransactionTrace{
				Type: rpcv7.TxnL1Handler,
				FunctionInvocation: &rpcv6.FunctionInvocation{
					Calls:    []rpcv6.FunctionInvocation{},
					Events:   []rpcv6.OrderedEvent{},
					Messages: []rpcv6.OrderedL2toL1Message{},
				},
			}

			adaptedTrace := rpcv7.AdaptVMTransactionTrace(&vmTrace)

			require.Equal(t, expectedAdaptedTrace, adaptedTrace)
		})

		t.Run("Execution reverted", func(t *testing.T) {
			vmTrace := vm.TransactionTrace{
				Type:                  vm.TxnL1Handler,
				ValidateInvocation:    &vm.FunctionInvocation{},
				FeeTransferInvocation: &vm.FunctionInvocation{},
				ExecuteInvocation: &vm.ExecuteInvocation{
					RevertReason:       "",
					FunctionInvocation: &vm.FunctionInvocation{},
				},
				ConstructorInvocation: &vm.FunctionInvocation{},
				FunctionInvocation: &vm.ExecuteInvocation{
					RevertReason: "Reverted",
				},
			}

			defaultL1HandlerInvocation := rpcv6.DefaultL1HandlerFunctionInvocation()
			expectedAdaptedTrace := rpcv7.TransactionTrace{
				Type:               rpcv7.TxnL1Handler,
				FunctionInvocation: &defaultL1HandlerInvocation,
			}

			adaptedTrace := rpcv7.AdaptVMTransactionTrace(&vmTrace)

			require.Equal(t, expectedAdaptedTrace, adaptedTrace)
		})
	})

	t.Run("successfully adapt DECLARE tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnDeclare,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
		}

		expectedAdaptedTrace := rpcv7.TransactionTrace{
			Type: rpcv7.TxnDeclare,
			ValidateInvocation: &rpcv6.FunctionInvocation{
				Calls:    []rpcv6.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
			FeeTransferInvocation: &rpcv6.FunctionInvocation{
				Calls:    []rpcv6.FunctionInvocation{},
				Events:   []rpcv6.OrderedEvent{},
				Messages: []rpcv6.OrderedL2toL1Message{},
			},
		}

		adaptedTrace := rpcv7.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})
}

func TestAdaptFeederBlockTrace(t *testing.T) {
	t.Run("nil block trace", func(t *testing.T) {
		block := &rpcv7.BlockWithTxs{}

		res, err := rpcv7.AdaptFeederBlockTrace(block, nil)
		require.Nil(t, res)
		require.Nil(t, err)
	})

	t.Run("inconsistent blockWithTxs and blockTrace", func(t *testing.T) {
		blockWithTxs := &rpcv7.BlockWithTxs{
			Transactions: []*rpcv7.Transaction{
				{},
			},
		}
		blockTrace := &starknet.BlockTrace{}

		res, err := rpcv7.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, res)
		require.Equal(t, errors.New("mismatched number of txs and traces"), err)
	})

	t.Run("L1_HANDLER tx gets successfully adapted", func(t *testing.T) {
		blockWithTxs := &rpcv7.BlockWithTxs{
			Transactions: []*rpcv7.Transaction{
				{
					Type: rpcv7.TxnL1Handler,
				},
			},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       *new(felt.Felt).SetUint64(1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					FunctionInvocation: &starknet.FunctionInvocation{
						Events: []starknet.OrderedEvent{{
							Order: 1,
							Keys:  []felt.Felt{*new(felt.Felt).SetUint64(2)},
							Data:  []felt.Felt{*new(felt.Felt).SetUint64(3)},
						}},
					},
				},
			},
		}

		expectedAdaptedTrace := []rpcv7.TracedBlockTransaction{
			{
				TransactionHash: new(felt.Felt).SetUint64(1),
				TraceRoot: &rpcv7.TransactionTrace{
					Type: rpcv7.TxnL1Handler,
					FunctionInvocation: &rpcv6.FunctionInvocation{
						Calls: []rpcv6.FunctionInvocation{},
						Events: []rpcv6.OrderedEvent{{
							Order: 1,
							Keys:  []*felt.Felt{new(felt.Felt).SetUint64(2)},
							Data:  []*felt.Felt{new(felt.Felt).SetUint64(3)},
						}},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
				},
			},
		}

		res, err := rpcv7.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})

	t.Run("INVOKE tx gets successfully adapted (with revert error)", func(t *testing.T) {
		blockWithTxs := &rpcv7.BlockWithTxs{
			Transactions: []*rpcv7.Transaction{
				{
					Type: rpcv7.TxnInvoke,
				},
			},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       *new(felt.Felt).SetUint64(1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					// When revert error, feeder trace has no FunctionInvocation only RevertError is set
					RevertError: "some error",
				},
			},
		}

		expectedAdaptedTrace := []rpcv7.TracedBlockTransaction{
			{
				TransactionHash: new(felt.Felt).SetUint64(1),
				TraceRoot: &rpcv7.TransactionTrace{
					Type: rpcv7.TxnInvoke,
					FeeTransferInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
					ValidateInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
					ExecuteInvocation: &rpcv6.ExecuteInvocation{
						RevertReason: "some error",
					},
				},
			},
		}

		res, err := rpcv7.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})

	t.Run("DEPLOY_ACCOUNT tx gets successfully adapted", func(t *testing.T) {
		blockWithTxs := &rpcv7.BlockWithTxs{
			Transactions: []*rpcv7.Transaction{
				{
					Type: rpcv7.TxnDeployAccount,
				},
			},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       *new(felt.Felt).SetUint64(1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					FunctionInvocation:    &starknet.FunctionInvocation{},
				},
			},
		}

		expectedAdaptedTrace := []rpcv7.TracedBlockTransaction{
			{
				TransactionHash: new(felt.Felt).SetUint64(1),
				TraceRoot: &rpcv7.TransactionTrace{
					Type: rpcv7.TxnDeployAccount,
					FeeTransferInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
					ValidateInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
					ConstructorInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
				},
			},
		}

		res, err := rpcv7.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})

	t.Run("DECLARE tx gets successfully adapted", func(t *testing.T) {
		blockWithTxs := &rpcv7.BlockWithTxs{
			Transactions: []*rpcv7.Transaction{
				{
					Type: rpcv7.TxnDeclare,
				},
			},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       *new(felt.Felt).SetUint64(1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					FunctionInvocation:    &starknet.FunctionInvocation{},
				},
			},
		}

		expectedAdaptedTrace := []rpcv7.TracedBlockTransaction{
			{
				TransactionHash: new(felt.Felt).SetUint64(1),
				TraceRoot: &rpcv7.TransactionTrace{
					Type: rpcv7.TxnDeclare,
					FeeTransferInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
					ValidateInvocation: &rpcv6.FunctionInvocation{
						Calls:              []rpcv6.FunctionInvocation{},
						Events:             []rpcv6.OrderedEvent{},
						Messages:           []rpcv6.OrderedL2toL1Message{},
						ExecutionResources: &rpcv6.ComputationResources{},
					},
				},
			},
		}

		res, err := rpcv7.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})
}

func TestTotalExecutionResources(t *testing.T) {
	resources := &rpcv6.ComputationResources{
		Steps:        1,
		MemoryHoles:  2,
		Pedersen:     3,
		RangeCheck:   4,
		Bitwise:      5,
		Ecdsa:        6,
		EcOp:         7,
		Keccak:       8,
		Poseidon:     9,
		SegmentArena: 10,
	}

	tests := map[string]struct {
		multiplier uint64
		trace      *rpcv7.TransactionTrace
	}{
		"many top-level invocations": {
			multiplier: 5,
			trace: &rpcv7.TransactionTrace{
				ValidateInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
				FunctionInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
				ConstructorInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
				ExecuteInvocation: &rpcv6.ExecuteInvocation{
					FunctionInvocation: &rpcv6.FunctionInvocation{
						ExecutionResources: resources,
					},
				},
				FeeTransferInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
			},
		},
		"only validate invocation": {
			multiplier: 1,
			trace: &rpcv7.TransactionTrace{
				ValidateInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
				},
			},
		},
		"present in some sub-calls": {
			multiplier: 2,
			trace: &rpcv7.TransactionTrace{
				ValidateInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
					Calls: []rpcv6.FunctionInvocation{
						{
							ExecutionResources: resources,
						},
					},
				},
				FunctionInvocation: &rpcv6.FunctionInvocation{
					ExecutionResources: resources,
					Calls: []rpcv6.FunctionInvocation{
						{
							ExecutionResources: resources,
						},
					},
				},
			},
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			require.Equal(t, rpcv7.ComputationResources{
				Steps:        resources.Steps * test.multiplier,
				MemoryHoles:  resources.MemoryHoles * test.multiplier,
				Pedersen:     resources.Pedersen * test.multiplier,
				RangeCheck:   resources.RangeCheck * test.multiplier,
				Bitwise:      resources.Bitwise * test.multiplier,
				Ecdsa:        resources.Ecdsa * test.multiplier,
				EcOp:         resources.EcOp * test.multiplier,
				Keccak:       resources.Keccak * test.multiplier,
				Poseidon:     resources.Poseidon * test.multiplier,
				SegmentArena: resources.SegmentArena * test.multiplier,
			}, test.trace.TotalComputationResources())
		})
	}
}

func TestCall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv7.New(mockReader, nil, mockVM, n, utils.NewNopZapLogger())

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{}, rpcv7.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{}, rpcv7.BlockID{Hash: &felt.Zero})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{}, rpcv7.BlockID{Number: 0})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("call - unknown contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(felt.Zero, errors.New("unknown contract"))

		res, rpcErr := handler.Call(rpcv7.FunctionCall{}, rpcv7.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("ok", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{
			*new(felt.Felt).SetUint64(4),
			*new(felt.Felt).SetUint64(5),
		}
		expectedRes := vm.CallResult{
			Result: []*felt.Felt{
				new(felt.Felt).SetUint64(6),
				new(felt.Felt).SetUint64(7),
			},
			ExecutionFailed: false,
		}
		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			&vm.CallInfo{
				ContractAddress: contractAddr,
				ClassHash:       classHash,
				Selector:        selector,
				Calldata:        calldata,
			},
			&vm.BlockInfo{
				Header: headsHeader,
			},
			gomock.Any(),
			uint64(1337),
			uint64(1338),
			false,
			false,
		).Return(expectedRes, nil)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           rpcv7.CalldataInputs{Data: calldata},
		}, rpcv7.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		require.Equal(t, expectedRes.Result, res)
	})

	t.Run("unknown entrypoint blockifier 0.14.0", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{*new(felt.Felt).SetUint64(4)}
		expectedRes := vm.CallResult{
			Result:          []*felt.Felt{felt.NewUnsafeFromString[felt.Felt](rpccore.EntrypointNotFoundFelt)},
			ExecutionFailed: true,
		}
		expectedErr := rpccore.ErrContractError
		expectedErr.Data = rpcv7.ContractErrorData{
			RevertError: json.RawMessage(`"` +
				fmt.Sprintf(
					rpccore.ErrEPSNotFound,
					selector.String(),
				) +
				`"`,
			),
		}

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			&vm.CallInfo{
				ContractAddress: contractAddr,
				ClassHash:       classHash,
				Selector:        selector,
				Calldata:        calldata,
			},
			&vm.BlockInfo{
				Header: headsHeader,
			},
			gomock.Any(),
			uint64(1337),
			uint64(1338),
			false,
			false,
		).Return(expectedRes, nil)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           rpcv7.CalldataInputs{Data: calldata},
		}, rpcv7.BlockID{Latest: true})
		require.Nil(t, res)
		require.Equal(t, rpcErr, expectedErr)
	})

	t.Run("execution failed with execution failure", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{*new(felt.Felt).SetUint64(4)}
		expectedRes := vm.CallResult{
			Result:          []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0xdeadbeef")},
			ExecutionFailed: true,
		}
		expectedErr := rpcv7.MakeContractError(json.RawMessage(`"0xdeadbeef"`))

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(expectedRes, nil)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           rpcv7.CalldataInputs{Data: calldata},
		}, rpcv7.BlockID{Latest: true})
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})

	t.Run("execution failed with execution failure and empty result", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxSteps(1338)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{*new(felt.Felt).SetUint64(4)}
		expectedRes := vm.CallResult{
			ExecutionFailed: true,
		}
		expectedErr := rpcv7.MakeContractError(json.RawMessage(""))

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(expectedRes, nil)

		res, rpcErr := handler.Call(rpcv7.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           rpcv7.CalldataInputs{Data: calldata},
		}, rpcv7.BlockID{Latest: true})
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})
}
