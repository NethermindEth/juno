package rpcv6_test

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
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
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
				wantTrace:   `[ { "trace_root": { "type": "DEPLOY", "constructor_invocation": { "contract_address": "0x7b196a359045d4d0c10f73bdf244a9e1205a615dbb754b8df40173364288534", "entry_point_selector": null, "calldata": [ "0x187d50a5cf3ebd6d4d6fa8e29e4cad0a237759c6416304a25c4ea792ed4bba4", "0x42f5af30d6693674296ad87301935d0c159036c3b24af4042ff0270913bf6c6" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 29 } } }, "transaction_hash": "0x3fa1bff0c86f34b2eb32c26d12208b6bdb4a5f6a434ac1d4f0e2d1db71bd711" }, { "trace_root": { "type": "DEPLOY", "constructor_invocation": { "contract_address": "0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6", "entry_point_selector": null, "calldata": [ "0x5cedec15acd969b0fba39fec9e7d9bd4d0b33f100969ad3a4543039a6f696d4", "0xce9801d27b02543f4d88b60aa456860f94ee9f612fc56464abfbdeedc1ab72" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 29 } } }, "transaction_hash": "0x154c02cc3165cceadaa32e7238a67061b3a1eac414138c4ebe1408f37fd93eb" }, { "trace_root": { "type": "INVOKE", "execute_invocation": { "contract_address": "0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6", "entry_point_selector": null, "calldata": [ "0x17d9c35a8b9a0d4512fa05eafec01c2758a7a5b7ec7b47408a24a4b33124d9b", "0x2", "0x7f800b5bf79637f8f83f47a8fc4d368b43695c781b22a899f11b5f2faba874a", "0x3a7a40d383612b0ad167aec8d90fb07e576e017d07948f63ac318b52511ae93" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 165, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7 } } }, "transaction_hash": "0x7893675c16da857b7c4229cda449e08a4fe13b07ca817e79d1db02e8a046047" }, { "trace_root": { "type": "INVOKE", "execute_invocation": { "contract_address": "0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6", "entry_point_selector": null, "calldata": [ "0x17d9c35a8b9a0d4512fa05eafec01c2758a7a5b7ec7b47408a24a4b33124d9b", "0x2", "0x7f800b5bf79637f8f83f47a8fc4d368b43695c781b22a899f11b5f2faba874a", "0xf140b304e9266c72f1054116dd06d9c1c8e981db7bf34e3c6da99640e9a7c8" ], "caller_address": "0x0", "class_hash": null, "entry_point_type": "", "call_type": "", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 165, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7 } } }, "transaction_hash": "0x4a277d67e3f42c4a343854081d1e2e9e425f1323255e4486d2badb37a1d8630" } ]`,
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0xe3828bd9154ab385e2cbb95b3b650365fb3c6a4321660d98ce8b0a9194f9a3",
				blockNumber: 300000,
				wantTrace:   `[ { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325", "0x0", "0x4", "0x4", "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "0x2", "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325", "0x0", "0x4", "0x4", "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "0x2", "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [ { "contract_address": "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "entry_point_selector": "0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325", "calldata": [ "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "0x2", "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x165e7db96ab97a63c621229617a6d49633737238673477a54720e4c952f2c7e", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [ { "order": 0, "from_address": "0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2", "to_address": "0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9", "payload": [ "0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228", "0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e" ] } ], "execution_resources": { "steps": 233, "memory_holes": 1, "range_check_builtin_applications": 5 } } ], "events": [], "messages": [], "execution_resources": { "steps": 374, "memory_holes": 4, "range_check_builtin_applications": 7 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x127089df3a1984", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [ { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x127089df3a1984", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x28d7d394810ad8c52741ad8f7564717fd02c10ced68657a81d0b6710ce22079", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [ "0x1" ], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 488, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } } ], "events": [], "messages": [], "execution_resources": { "steps": 548, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } } }, "transaction_hash": "0x2a648ab1aa6847eb38507fc842e050f256562bf87b26083c332f3f21318c2c3" }, { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1", "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f", "0x0", "0x4", "0x4", "0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb", "0x2", "0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e", "0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1", "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f", "0x0", "0x4", "0x4", "0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb", "0x2", "0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e", "0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336" ], "caller_address": "0x0", "class_hash": "0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [ { "contract_address": "0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1", "entry_point_selector": "0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f", "calldata": [ "0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb", "0x2", "0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e", "0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x13abfd2f333f9c69f690f1569140cdae25f6f66e3f371c9cbb998b65f664a85", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 166, "memory_holes": 22, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 7 } } ], "events": [], "messages": [], "execution_resources": { "steps": 307, "memory_holes": 25, "pedersen_builtin_applications": 2, "range_check_builtin_applications": 9 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x3b2d25cd7bccc", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [ { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x3b2d25cd7bccc", "0x0" ], "caller_address": "0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7", "class_hash": "0x28d7d394810ad8c52741ad8f7564717fd02c10ced68657a81d0b6710ce22079", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [ "0x1" ], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 488, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } } ], "events": [], "messages": [], "execution_resources": { "steps": 548, "memory_holes": 40, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 21 } } }, "transaction_hash": "0xbc984e8e1fe594dd518a3a51db4f338437a5d2fbdda772d4426b532a67ffff" } ]`,
			},
		}

		AssertTracedBlockTransactions(t, &utils.Integration, tests)
	})

	t.Run("Sepolia", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			"old block": {
				blockHash:   "0x37644818236ee05b7e3b180bed64ea70ee3dd1553ca334a5c2a290ee276f380",
				blockNumber: 3,
				wantTrace:   `[ { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5" ], "calls": [ { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "calldata": [ "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "0x0", "0x0", "0x1" ], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5" ], "calls": [ { "contract_address": "0x23be95f90bf41685e18a4356e57b0cfdc1da22bf382ead8b64108353915c1e5", "entry_point_selector": "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", "calldata": [], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1", "entry_point_type": "CONSTRUCTOR", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 0 } } ], "events": [], "messages": [], "execution_resources": { "steps": 69, "memory_holes": 2, "range_check_builtin_applications": 1 } } ], "events": [], "messages": [], "execution_resources": { "steps": 218, "memory_holes": 5, "range_check_builtin_applications": 3 } } }, "transaction_hash": "0x3f786ecc4955a2602c91a291328518ef866cb7f3d50e4b16fd42282952623aa" }, { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 89, "range_check_builtin_applications": 2, "ecdsa_builtin_applications": 1 } }, "execute_invocation": { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x1", "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "0x0", "0x4", "0x4", "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "0x0", "0x0", "0x1" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x6d8ff7b212b08760c82e4a8f354f6ebc69d748290fa38e92eb859726a88f379" ], "calls": [ { "contract_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "entry_point_selector": "0x2730079d734ee55315f4f141eaed376bddd8c2133523d223a344c5604e0f7f8", "calldata": [ "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "0x0", "0x0", "0x1" ], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x6d8ff7b212b08760c82e4a8f354f6ebc69d748290fa38e92eb859726a88f379" ], "calls": [ { "contract_address": "0x6d8ff7b212b08760c82e4a8f354f6ebc69d748290fa38e92eb859726a88f379", "entry_point_selector": "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", "calldata": [], "caller_address": "0x43abaa073c768ebf039c0c4f46db9acc39e9ec165690418060a652aab39e7d8", "class_hash": "0x4f23a756b221f8ce46b72e6a6b10ee7ee6cf3b59790e76e02433104f9a8c5d1", "entry_point_type": "CONSTRUCTOR", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 0 } } ], "events": [], "messages": [], "execution_resources": { "steps": 69, "memory_holes": 2, "range_check_builtin_applications": 1 } } ], "events": [], "messages": [], "execution_resources": { "steps": 218, "memory_holes": 5, "range_check_builtin_applications": 3 } } }, "transaction_hash": "0x4010bd7b00e591c163729aa501691e89784c2afe77d71f7b27613e377738843" } ]`,
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0x733495d0744edd9785b400408fa87c8ad599f81859df544897f80a3fceab422",
				blockNumber: 40000,
				wantTrace:   `[ { "trace_root": { "type": "INVOKE", "validate_invocation": { "contract_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": [ "0x2", "0x4d0b88ace5705bb7825f91ee95557d906600b7e7762f5615e6a4f407185a43a", "0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3", "0x2", "0x4e946d49fca553930846e35533342f88e59a841c24d9cf507ef28dd6b67cb9b", "0x3ea9c575cfdaa875f3fecaf7db4acdb536ee6b38b8d8a4c769c63d044f942dc", "0x6359ed638df79b82f2f9dbf92abbcb41b57f9dd91ead86b1c85d2dee192c", "0x1a8e87e9d2008fcd3ce423ae5219c21e49be18d05d72825feb7e2bb687ba35c", "0x2", "0x44cd44ad7abf35b9dbe1e17de3610d21", "0x9f806c191aa2a3d47f2b8efc4c412d2f" ], "caller_address": "0x0", "class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x56414c4944" ], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 990, "memory_holes": 63, "range_check_builtin_applications": 25, "ec_op_builtin_applications": 3 } }, "execute_invocation": { "contract_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": [ "0x2", "0x4d0b88ace5705bb7825f91ee95557d906600b7e7762f5615e6a4f407185a43a", "0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3", "0x2", "0x4e946d49fca553930846e35533342f88e59a841c24d9cf507ef28dd6b67cb9b", "0x3ea9c575cfdaa875f3fecaf7db4acdb536ee6b38b8d8a4c769c63d044f942dc", "0x6359ed638df79b82f2f9dbf92abbcb41b57f9dd91ead86b1c85d2dee192c", "0x1a8e87e9d2008fcd3ce423ae5219c21e49be18d05d72825feb7e2bb687ba35c", "0x2", "0x44cd44ad7abf35b9dbe1e17de3610d21", "0x9f806c191aa2a3d47f2b8efc4c412d2f" ], "caller_address": "0x0", "class_hash": "0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x2", "0x0", "0x0" ], "calls": [ { "contract_address": "0x4d0b88ace5705bb7825f91ee95557d906600b7e7762f5615e6a4f407185a43a", "entry_point_selector": "0x3d7905601c217734671143d457f0db37f7f8883112abd34b92c4abfeafde0c3", "calldata": [ "0x4e946d49fca553930846e35533342f88e59a841c24d9cf507ef28dd6b67cb9b", "0x3ea9c575cfdaa875f3fecaf7db4acdb536ee6b38b8d8a4c769c63d044f942dc" ], "caller_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "class_hash": "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 26 } }, { "contract_address": "0x6359ed638df79b82f2f9dbf92abbcb41b57f9dd91ead86b1c85d2dee192c", "entry_point_selector": "0x1a8e87e9d2008fcd3ce423ae5219c21e49be18d05d72825feb7e2bb687ba35c", "calldata": [ "0x44cd44ad7abf35b9dbe1e17de3610d21", "0x9f806c191aa2a3d47f2b8efc4c412d2f" ], "caller_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "class_hash": "0x5a1a156fd2af56bb992ce31fd2a4765e9b65b84efce45f3063974decaa339a2", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 308, "memory_holes": 14, "range_check_builtin_applications": 6 } } ], "events": [], "messages": [], "execution_resources": { "steps": 1419, "memory_holes": 16, "range_check_builtin_applications": 31 } }, "fee_transfer_invocation": { "contract_address": "0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x11ecef7f251258", "0x0" ], "caller_address": "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "class_hash": "0x5327164fa21dca89a92e8eae8a5b7ab90f58373e71f0a16d285e5a4abe5a3cf", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [], "events": [ { "order": 0, "keys": [ "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9" ], "data": [ "0x35acd6dd6c5045d18ca6d0192af46b335a5402c02d41f46e4e77ea2c951d9a3", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x11ecef7f251258", "0x0" ] } ], "messages": [], "execution_resources": { "steps": 876, "memory_holes": 56, "range_check_builtin_applications": 27, "pedersen_builtin_applications": 4 } } }, "transaction_hash": "0x6aa7ec89f36e918c9a168ebc9818e9dd19515a2a4bef87d73e1decbd8a7d131" }, { "trace_root": { "type": "DEPLOY_ACCOUNT", "validate_invocation": { "contract_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "entry_point_selector": "0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895", "calldata": [ "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "0x13e91b7ca4192672", "0x1a3bd006d99712e91bd3fd2eb5fafb0f379d9d594125bb527ec7fc5e133122a" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 75, "ecdsa_builtin_applications": 1 } }, "constructor_invocation": { "contract_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "entry_point_selector": "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", "calldata": [ "0x1a3bd006d99712e91bd3fd2eb5fafb0f379d9d594125bb527ec7fc5e133122a" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "CONSTRUCTOR", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 41 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xe5e432c83b4f", "0x0" ], "caller_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "class_hash": "0x5ffbcfeb50d200a0677c48a129a11245a3fc519d1d98d76882d1c9a1b19c6ed", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [], "events": [ { "order": 0, "keys": [ "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9" ], "data": [ "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xe5e432c83b4f", "0x0" ] } ], "messages": [], "execution_resources": { "steps": 876, "memory_holes": 56, "range_check_builtin_applications": 27, "pedersen_builtin_applications": 4 } } }, "transaction_hash": "0x97468f6928d72808b23fe775e7c71893087600792fb36e0d62ec191363bd34" }, { "trace_root": { "type": "DECLARE", "validate_invocation": { "contract_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "entry_point_selector": "0x289da278a8dc833409cabfdad1581e8e7d40e42dcaed693fa4008dcdb4963b3", "calldata": [ "0x1e7c85ba9d58309d1f257ba201523e1a7b695bfeb6523759da24effd8dc6c0f" ], "caller_address": "0x0", "class_hash": "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [], "events": [], "messages": [], "execution_resources": { "steps": 73, "ecdsa_builtin_applications": 1 } }, "fee_transfer_invocation": { "contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": [ "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xbf730c7e8f2b", "0x0" ], "caller_address": "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "class_hash": "0x5ffbcfeb50d200a0677c48a129a11245a3fc519d1d98d76882d1c9a1b19c6ed", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [ "0x1" ], "calls": [], "events": [ { "order": 0, "keys": [ "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9" ], "data": [ "0x5f7a835be8a4f03c5d98287713e20e4cc5697fd03552493dfbc38430f5ea38a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0xbf730c7e8f2b", "0x0" ] } ], "messages": [], "execution_resources": { "steps": 876, "memory_holes": 56, "pedersen_builtin_applications": 4, "range_check_builtin_applications": 27 } } }, "transaction_hash": "0x6a2df1337b09691711a66fca7e93e9f9fbc04c70dc6a17b9284b7af39c1a6a1" } ]`,
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
		return gateway.BlockByNumber(t.Context(), number)
	}).AnyTimes()
	mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).AnyTimes()

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			mockReader.EXPECT().BlockByHash(utils.HexToFelt(t, test.blockHash)).DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
				return mockReader.BlockByNumber(test.blockNumber)
			})

			handler := rpc.New(mockReader, nil, nil, "", n, nil)
			handler = handler.WithFeeder(client)
			traces, jErr := handler.TraceBlockTransactions(t.Context(), rpc.BlockID{Number: test.blockNumber})
			if n == &utils.Sepolia && description == "newer block" {
				// For Sepolia's newer block test, we test 3 of the block traces (INVOKE, DEPLOY_ACCOUNT, DECLARE)
				traces = []rpc.TracedBlockTransaction{traces[0], traces[7], traces[11]}
			}

			require.Nil(t, jErr)
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
		mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).AnyTimes()

		// No feeder client is set
		handler := rpc.New(mockReader, nil, nil, "", n, nil)

		tracedBlocks, jErr := handler.TraceBlockTransactions(t.Context(), rpc.BlockID{Number: blockNumber})

		require.Nil(t, tracedBlocks)
		require.Equal(t, rpccore.ErrInternal.Code, jErr.Code)
	})
}

func TestTransactionTraceValidation(t *testing.T) {
	validInvokeTransactionTrace := rpc.TransactionTrace{
		Type:              rpc.TxnInvoke,
		ExecuteInvocation: &rpc.ExecuteInvocation{},
	}

	invalidInvokeTransactionTrace := rpc.TransactionTrace{
		Type: rpc.TxnInvoke,
	}

	validDeployAccountTransactionTrace := rpc.TransactionTrace{
		Type:                  rpc.TxnDeployAccount,
		ConstructorInvocation: &rpc.FunctionInvocation{},
	}

	invalidDeployAccountTransactionTrace := rpc.TransactionTrace{
		Type: rpc.TxnDeployAccount,
	}

	validL1HandlerTransactionTrace := rpc.TransactionTrace{
		Type:               rpc.TxnL1Handler,
		FunctionInvocation: &rpc.FunctionInvocation{},
	}

	invalidL1HandlerTransactionTrace := rpc.TransactionTrace{
		Type: rpc.TxnL1Handler,
	}

	tests := []struct {
		name    string
		trace   rpc.TransactionTrace
		wantErr bool
	}{
		{
			name:    "valid INVOKE tx",
			trace:   validInvokeTransactionTrace,
			wantErr: false,
		},
		{
			name:    "invalid INVOKE tx",
			trace:   invalidInvokeTransactionTrace,
			wantErr: true,
		},
		{
			name:    "valid DEPLOY_ACCOUNT tx",
			trace:   validDeployAccountTransactionTrace,
			wantErr: false,
		},
		{
			name:    "invalid DEPLOY_ACCOUNT tx",
			trace:   invalidDeployAccountTransactionTrace,
			wantErr: true,
		},
		{
			name:    "valid L1_HANDLER tx",
			trace:   validL1HandlerTransactionTrace,
			wantErr: false,
		},
		{
			name:    "invalid L1_HANDLER tx",
			trace:   invalidL1HandlerTransactionTrace,
			wantErr: true,
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
			}
		})
	}
}

func TestFunctionInvocationMarshalling(t *testing.T) {
	t.Run("All FunctionInvocation fields must get marshalled", func(t *testing.T) {
		zeroValuedFnInvocation := rpc.FunctionInvocation{}
		expected := `{"contract_address":[0,0,0,0],"entry_point_selector":null,"calldata":null,"caller_address":[0,0,0,0],"class_hash":null,"entry_point_type":"","call_type":"","result":null,"calls":null,"events":null,"messages":null,"execution_resources":null}`

		jsonStr, err := json.Marshal(zeroValuedFnInvocation)

		require.NoError(t, err)
		assert.JSONEq(t, expected, string(jsonStr))
	})
}

func TestTraceTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, mockSyncReader, mockVM, "", &utils.Mainnet, utils.NewNopZapLogger())

	t.Run("not found", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xBBBB")
		// Receipt() returns error related to db
		mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)

		trace, err := handler.TraceTransaction(t.Context(), *hash)
		assert.Nil(t, trace)
		assert.Equal(t, rpccore.ErrTxnHashNotFound, err)
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
			SequencerAddress: utils.HexToFelt(t, "0X111"),
			ProtocolVersion:  "99.12.3",
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
		mockReader.EXPECT().BlockByHash(header.Hash).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTraceJSON := json.RawMessage(`{
		"type": "INVOKE",
		"validate_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [], "events": [], "messages": []}], "events": [], "messages": []},
		"execute_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x38627c278c0b3cb3c84ddee2c783fb22c3c3a3f0e667ea2b82be0ea2253bce4", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x0", "0x0", "0x5"], "calls": [{"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x46668cd07d83af5d7158e7cd62c710f1a7573501bcd4f4092c6a4e1ecd2bf61", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x0", "0x0", "0x5"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1e8480", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1e8480", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2ceb6369dba6af865bca639f9f1342dfb1ae4e5d0d0723de98028b812e7cdd2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "entry_point_selector": "0x2c0f7bf2d6cf5304c29171bf493feb222fef84bdaf17805a6574b0c2e8bcc87", "calldata": ["0x1e8480", "0x0", "0x0", "0x0", "0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x648f780a"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x514718bb56ed2a8607554c7d393c2ffd73cbab971c120b00a2ce27cc58dd1c1", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x2", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x3c388f7eb137a89061c6f0b6e78bae453202258b0b3c419f8dd9814a547d406", "calldata": [], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b60b3a0bcc4aa98", "0xaf07589b7c", "0x648f7422"], "calls": [], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0xaf07771ffc", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0xaf07771ffc", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe14a408baf7f453312eec68e9b7d728ec5337fbdf671f917ee8c80f3255232"], "data": ["0x178b5c9bdd4e74e92b", "0xaf07771ffc"]}, {"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x0", "0x0", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"]}], "messages": []}], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x417c36e4fc16d", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x417c36e4fc16d", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0x5ad857f66a5b55f1301ff1ed7e098ac6d4433148f0b72ebc4a2945ab85ad53"], "data": ["0x2fc5e96de394697c1311606c96ec14840e408493fd42cf0c54b73b39d312b81", "0x2", "0x1", "0x1"]}], "messages": []}], "events": [], "messages": []},
		"fee_transfer_invocation": {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"]}], "messages": []}], "events": [], "messages": []},
		"state_diff": {
			"storage_diffs": [],
			"nonces": [],
			"deployed_contracts": [],
			"deprecated_declared_classes": [],
			"declared_classes": [],
			"replaced_classes": []
		}
	}`)
		vmTrace := new(vm.TransactionTrace)
		require.NoError(t, json.Unmarshal(vmTraceJSON, vmTrace))
		mockVM.EXPECT().Execute([]core.Transaction{tx}, []core.Class{declaredClass.Class}, []*felt.Felt{},
			&vm.BlockInfo{Header: header}, gomock.Any(), &utils.Mainnet, false, false, false, false).
			Return(vm.ExecutionResults{
				DataAvailability: []core.DataAvailability{{L1DataGas: 0}},
				Traces:           []vm.TransactionTrace{*vmTrace},
				NumSteps:         0,
			}, nil)

		trace, err := handler.TraceTransaction(t.Context(), *hash)

		require.Nil(t, err)
		assert.Equal(t, rpc.AdaptVMTransactionTrace(vmTrace), *trace)
	})

	t.Run("pending block", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xceb6a374aff2bbb3537cf35f50df8634b2354a21")
		tx := &core.DeclareTransaction{
			TransactionHash: hash,
			ClassHash:       utils.HexToFelt(t, "0x000000000"),
		}

		header := &core.Header{
			ParentHash:       utils.HexToFelt(t, "0x0"),
			SequencerAddress: utils.HexToFelt(t, "0X111"),
			ProtocolVersion:  "99.12.3",
		}
		require.Nil(t, header.Hash, "hash must be nil for pending block")

		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClass{
			At:    3002,
			Class: &core.Cairo1Class{},
		}

		mockReader.EXPECT().Receipt(hash).Return(nil, header.Hash, header.Number, nil)
		mockSyncReader.EXPECT().Pending().Return(&sync.Pending{
			Block: block,
		}, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockSyncReader.EXPECT().PendingState().Return(headState, nopCloser, nil)

		vmTraceJSON := json.RawMessage(`{
		"type": "INVOKE",
		"validate_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [], "events": [], "messages": []}], "events": [], "messages": []},
		"execute_invocation": {"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "entry_point_selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad", "calldata": ["0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "0x0", "0x3", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "0x3", "0x10", "0x13", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0x0", "class_hash": "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1", "0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x1171593aa5bdadda4d6b0efde6cc94ee7649c3163d5efeb19da6c16d63a2a63", "calldata": ["0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x420eeb770f7a4", "0x0", "0x40139799e37e4", "0x0", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x0", "0x0", "0x1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x64"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x38627c278c0b3cb3c84ddee2c783fb22c3c3a3f0e667ea2b82be0ea2253bce4", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x5ee939756c1a60b029c594da00e637bf5923bf04a86ff163e877e899c0840eb", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x0", "0x0", "0x5"], "calls": [{"contract_address": "0x1ed6790cdca923073adc728080b06c159d9784cc9bf8fb26181acfdbe4256e6", "entry_point_selector": "0x260bb04cf90403013190e77d7e75f3d40d3d307180364da33c63ff53061d4e8", "calldata": [], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x46668cd07d83af5d7158e7cd62c710f1a7573501bcd4f4092c6a4e1ecd2bf61", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x0", "0x0", "0x5"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1e8480", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1e8480", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x0", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2ceb6369dba6af865bca639f9f1342dfb1ae4e5d0d0723de98028b812e7cdd2", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": [], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x219209e083275171774dab1df80982e9df2096516f06319c5c6d71ae0a8480c", "calldata": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x134692b230b9e1ffa39098904722134159652b09c5bc41d88d6698779d228ff"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "entry_point_selector": "0x2c0f7bf2d6cf5304c29171bf493feb222fef84bdaf17805a6574b0c2e8bcc87", "calldata": ["0x1e8480", "0x0", "0x0", "0x0", "0x2", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x648f780a"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x514718bb56ed2a8607554c7d393c2ffd73cbab971c120b00a2ce27cc58dd1c1", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x2", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x3c388f7eb137a89061c6f0b6e78bae453202258b0b3c419f8dd9814a547d406", "calldata": [], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b60b3a0bcc4aa98", "0xaf07589b7c", "0x648f7422"], "calls": [], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x41b033f4a31df8067c24d1e9b550a2ce75fd4a29e1147af9752174f0e6cb20", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x1e8480", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "entry_point_selector": "0x15543c3708653cda9d418b4ccd3be11368e40636c10c44b18cfe756b6d88b29", "calldata": ["0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "class_hash": "0x231adde42526bad434ca2eb983efdd64472638702f87f97e6e3c084f264e06f", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": [], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x178b5c9bdd4e74e92b", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x52c7ba99c77fc38dd3346beea6c0753c3471f2e3135af5bb837d6c9523fff62", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0xaf07771ffc", "0x0"], "calls": [{"contract_address": "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325"], "caller_address": "0x23c72abdf49dffc85ae3ede714f2168ad384cc67d08524732acea90df325", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0xaf07771ffc", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe14a408baf7f453312eec68e9b7d728ec5337fbdf671f917ee8c80f3255232"], "data": ["0x178b5c9bdd4e74e92b", "0xaf07771ffc"]}, {"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0x7a6f98c03379b9513ca84cca1373ff452a7462a3b61598f0af5bb27ad7f76d1", "0x0", "0x0", "0x1e8480", "0x0", "0x417c36e4fc16d", "0x0", "0x0", "0x0", "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"]}], "messages": []}], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x417c36e4fc16d", "0x0"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x2e4263afad30923c891518314c3c95dbe830a16874e8abc5777a9a20b54c76e", "calldata": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x417c36e4fc16d", "0x0"], "calls": [], "events": [], "messages": []}], "events": [], "messages": []}, {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"], "caller_address": "0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0x4270219d365d6b017231b52e92b3fb5d7c8378b05e9abc97724537a80e93b0f", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x417c36e4fc16d", "0x0"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0xe316f0d9d2a3affa97de1d99bb2aac0538e2666d0d8545545ead241ef0ccab"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x53c91253bc9682c04929ca02ed00b3e423f6710d2ee7e0d5ebb06f3ecf368a8", "0x1e8480", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x417c36e4fc16d", "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a"]}], "messages": []}], "events": [], "messages": []}], "events": [{"keys": ["0x5ad857f66a5b55f1301ff1ed7e098ac6d4433148f0b72ebc4a2945ab85ad53"], "data": ["0x2fc5e96de394697c1311606c96ec14840e408493fd42cf0c54b73b39d312b81", "0x2", "0x1", "0x1"]}], "messages": []}], "events": [], "messages": []},
		"fee_transfer_invocation": {"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3", "entry_point_type": "EXTERNAL", "call_type": "CALL", "result": ["0x1"], "calls": [{"contract_address": "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "entry_point_selector": "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e", "calldata": ["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"], "caller_address": "0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "class_hash": "0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0", "entry_point_type": "EXTERNAL", "call_type": "DELEGATE", "result": ["0x1"], "calls": [], "events": [{"keys": ["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"], "data": ["0xd747220b2744d8d8d48c8a52bd3869fb98aea915665ab2485d5eadb49def6a", "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8", "0x2cb6", "0x0"]}], "messages": []}], "events": [], "messages": []},
		"state_diff": {
			"storage_diffs": [],
			"nonces": [],
			"deployed_contracts": [],
			"deprecated_declared_classes": [],
			"declared_classes": [],
			"replaced_classes": []
		}
	}`)
		vmTrace := new(vm.TransactionTrace)
		require.NoError(t, json.Unmarshal(vmTraceJSON, vmTrace))
		mockVM.EXPECT().Execute([]core.Transaction{tx}, []core.Class{declaredClass.Class}, []*felt.Felt{},
			&vm.BlockInfo{Header: header}, gomock.Any(), &utils.Mainnet, false, false, false, false).
			Return(vm.ExecutionResults{
				Traces:   []vm.TransactionTrace{*vmTrace},
				NumSteps: 0,
			}, nil)

		trace, err := handler.TraceTransaction(t.Context(), *hash)
		require.Nil(t, err)
		assert.Equal(t, rpc.AdaptVMTransactionTrace(vmTrace), *trace)
	})

	t.Run("reverted INVOKE tx from feeder", func(t *testing.T) {
		n := &utils.Sepolia

		handler := rpc.New(mockReader, mockSyncReader, mockVM, "", n, utils.NewNopZapLogger())

		client := feeder.NewTestClient(t, n)
		handler.WithFeeder(client)
		gateway := adaptfeeder.New(client)

		// Tx at index 3 in the block
		revertedTxHash := utils.HexToFelt(t, "0x2f00c7f28df2197196440747f97baa63d0851e3b0cfc2efedb6a88a7ef78cb1")

		blockNumber := uint64(18)
		blockHash := utils.HexToFelt(t, "0x5beb56c7d9a9fc066e695c3fc467f45532cace83d9979db4ccfd6b77ca476af")

		mockReader.EXPECT().Receipt(revertedTxHash).Return(nil, blockHash, blockNumber, nil)
		mockReader.EXPECT().BlockByHash(blockHash).DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
			return gateway.BlockByNumber(t.Context(), blockNumber)
		}).Times(2)

		mockReader.EXPECT().L1Head().Return(&core.L1Head{
			BlockNumber: 19, // Doesn't really matter for this test
		}, nil)

		expectedRevertedTrace := rpc.TransactionTrace{
			Type: rpc.TxnInvoke,
			ValidateInvocation: &rpc.FunctionInvocation{
				ContractAddress:    *utils.HexToFelt(t, "0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
				EntryPointSelector: utils.HexToFelt(t, "0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775"),
				Calldata: []felt.Felt{
					*utils.HexToFelt(t, "0x1"),
					*utils.HexToFelt(t, "0x7c687d151607710a7ec82ca5ab0ff2c48f52abd3b4a2773938a0cfef723fe6a"),
					*utils.HexToFelt(t, "0x10b7e63d3ca05c9baffd985d3e1c3858d4dbf0759f066be0eaddc5d71c2cab5"),
					*utils.HexToFelt(t, "0x1"),
					*utils.HexToFelt(t, "0xa"),
				},
				CallerAddress:  *utils.HexToFelt(t, "0x0"),
				ClassHash:      utils.HexToFelt(t, "0x903752516de5c04fe91600ca6891e325278b2dfc54880ae11a809abb364844"),
				EntryPointType: "EXTERNAL",
				CallType:       "CALL",
				Result:         []felt.Felt{*utils.HexToFelt(t, "0x56414c4944")},
				Calls:          []rpc.FunctionInvocation{},
				Events:         []rpc.OrderedEvent{},
				Messages:       []rpc.OrderedL2toL1Message{},
				ExecutionResources: &rpc.ComputationResources{
					Steps:       754,
					MemoryHoles: 5,
					RangeCheck:  17,
					EcOp:        3,
				},
			},
			FeeTransferInvocation: &rpc.FunctionInvocation{
				ContractAddress:    *utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				EntryPointSelector: utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				Calldata: []felt.Felt{
					*utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
					*utils.HexToFelt(t, "0x2847291f968"),
					*utils.HexToFelt(t, "0x0"),
				},
				CallerAddress:  *utils.HexToFelt(t, "0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
				ClassHash:      utils.HexToFelt(t, "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3"),
				EntryPointType: "EXTERNAL",
				CallType:       "CALL",
				Result:         []felt.Felt{*utils.HexToFelt(t, "0x1")},
				Calls: []rpc.FunctionInvocation{
					{
						ContractAddress:    *utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
						EntryPointSelector: utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
						Calldata: []felt.Felt{
							*utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
							*utils.HexToFelt(t, "0x2847291f968"),
							*utils.HexToFelt(t, "0x0"),
						},
						CallerAddress:  *utils.HexToFelt(t, "0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
						ClassHash:      utils.HexToFelt(t, "0x1b661756bf7d16210fc611626e1af4569baa1781ffc964bd018f4585ae241c1"),
						EntryPointType: "EXTERNAL",
						CallType:       "DELEGATE",
						Result:         []felt.Felt{*utils.HexToFelt(t, "0x1")},
						Calls:          []rpc.FunctionInvocation{},
						Events: []rpc.OrderedEvent{
							{
								Order: 0,
								Keys:  []*felt.Felt{utils.HexToFelt(t, "0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9")},
								Data: []*felt.Felt{
									utils.HexToFelt(t, "0x70503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84"),
									utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
									utils.HexToFelt(t, "0x2847291f968"),
									utils.HexToFelt(t, "0x0"),
								},
							},
						},
						Messages: []rpc.OrderedL2toL1Message{},
						ExecutionResources: &rpc.ComputationResources{
							Steps:       529,
							MemoryHoles: 57,
							Pedersen:    4,
							RangeCheck:  21,
						},
					},
				},
				Events:   []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{},
				ExecutionResources: &rpc.ComputationResources{
					Steps:       589,
					MemoryHoles: 57,
					Pedersen:    4,
					RangeCheck:  21,
				},
			},
			ExecuteInvocation: &rpc.ExecuteInvocation{
				RevertReason: "Error in the called contract (0x070503f026c7af73cfd2b007fe650e8c310256e9674ac4e42797c291edca5e84):\nError at pc=0:4288:\nGot an exception while executing a hint: Custom Hint Error: Execution failed. Failure reason: 'Fatal'.\nCairo traceback (most recent call last):\nUnknown location (pc=0:67)\nUnknown location (pc=0:1997)\nUnknown location (pc=0:2729)\nUnknown location (pc=0:3577)\n",
			},
		}

		trace, err := handler.TraceTransaction(t.Context(), *revertedTxHash)

		require.Nil(t, err)
		assert.Equal(t, expectedRevertedTrace, *trace)
	})
}

func TestTraceBlockTransactions(t *testing.T) {
	t.Skip()

	errTests := map[string]rpc.BlockID{
		"latest":  {Latest: true},
		"pending": {Pending: true},
		"hash":    {Hash: new(felt.Felt).SetUint64(1)},
		"number":  {Number: 1},
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(pebble.NewMemTest(t), n)
			handler := rpc.New(chain, nil, nil, "", n, log)

			update, rpcErr := handler.TraceBlockTransactions(t.Context(), id)
			assert.Nil(t, update)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	handler := rpc.New(mockReader, nil, mockVM, "", n, log)

	t.Run("pending block", func(t *testing.T) {
		blockHash := utils.HexToFelt(t, "0x0001")
		header := &core.Header{
			// hash is not set because it's pending block
			ParentHash:      utils.HexToFelt(t, "0x0C3"),
			Number:          0,
			L1GasPriceETH:   utils.HexToFelt(t, "0x777"),
			ProtocolVersion: "99.12.3",
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
		mockSyncReader.EXPECT().PendingState().Return(headState, nopCloser, nil)

		paidL1Fees := []*felt.Felt{(&felt.Felt{}).SetUint64(1)}
		vmTraceJSON := json.RawMessage(`{
			"type": "INVOKE",
			"validate_invocation": {},
			"execute_invocation": {},
			"fee_transfer_invocation": {},
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
		mockVM.EXPECT().Execute(block.Transactions, []core.Class{declaredClass.Class}, paidL1Fees, &vm.BlockInfo{Header: header},
			gomock.Any(), n, false, false, false, false).
			Return(vm.ExecutionResults{
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{vmTrace, vmTrace},
				NumSteps:         0,
			}, nil)

		result, err := handler.TraceBlockTransactions(t.Context(), rpc.BlockID{Hash: blockHash})
		require.Nil(t, err)
		assert.Equal(t, &vm.TransactionTrace{
			ValidateInvocation:    &vm.FunctionInvocation{},
			ExecuteInvocation:     &vm.ExecuteInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			StateDiff: &vm.StateDiff{
				StorageDiffs:              []vm.StorageDiff{},
				Nonces:                    []vm.Nonce{},
				DeployedContracts:         []vm.DeployedContract{},
				DeprecatedDeclaredClasses: []*felt.Felt{},
				DeclaredClasses:           []vm.DeclaredClass{},
				ReplacedClasses:           []vm.ReplacedClass{},
			},
		}, result[0].TraceRoot)
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
			L1GasPriceETH:    utils.HexToFelt(t, "0x777"),
			ProtocolVersion:  "99.12.3",
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

		vmTraceJSON := json.RawMessage(`{
			"type": "INVOKE",
			"validate_invocation":{"entry_point_selector":"0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895","calldata":["0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[{"entry_point_selector":"0x36fcbf06cd96843058359e1a75928beacfac10727dab22a3972f0af8aa92895","calldata":["0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":[],"calls":[],"events":[],"messages":[]}],"events":[],"messages":[]},
			"execute_invocation":{"entry_point_selector":"0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194","calldata":["0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","0x2","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918","entry_point_type":"CONSTRUCTOR","call_type":"CALL","result":[],"calls":[{"entry_point_selector":"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463","calldata":["0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"],"caller_address":"0x0","class_hash":"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":[],"calls":[],"events":[{"keys":["0x10c19bef19acd19b2c9f4caa40fd47c9fbe1d9f91324d44dcd36be2dae96784"],"data":["0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","0x322258135d04971e96b747a5551061aa046ad5d8be11a35c67029d96b23f98","0x0"]}],"messages":[]}],"events":[],"messages":[]},
			"fee_transfer_invocation":{"entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"],"caller_address":"0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","class_hash":"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3","entry_point_type":"EXTERNAL","call_type":"CALL","result":["0x1"],"calls":[{"entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"],"caller_address":"0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","class_hash":"0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":["0x1"],"calls":[],"events":[{"keys":["0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"],"data":["0xdac9bcffb3d967f19a7fe21002c98c984d5a9458a88e6fc5d1c478a97ed412","0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9","0x15be","0x0"]}],"messages":[]}],"events":[],"messages":[]},
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
		mockVM.EXPECT().Execute([]core.Transaction{tx}, []core.Class{declaredClass.Class}, []*felt.Felt{}, &vm.BlockInfo{Header: header},
			gomock.Any(), n, false, false, false, false).
			Return(vm.ExecutionResults{
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{vmTrace},
				NumSteps:         0,
			}, nil)

		expectedTrace := rpc.AdaptVMTransactionTrace(&vmTrace)
		expectedResult := []rpc.TracedBlockTransaction{
			{
				TransactionHash: tx.Hash(),
				TraceRoot:       &expectedTrace,
			},
		}

		result, err := handler.TraceBlockTransactions(t.Context(), rpc.BlockID{Hash: blockHash})

		require.Nil(t, err)
		assert.Equal(t, expectedResult, result)
	})
}

func TestAdaptVMTransactionTrace(t *testing.T) {
	t.Run("successfully adapt INVOKE trace from vm", func(t *testing.T) {
		fromAddr, _ := new(felt.Felt).SetString("0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f")
		toAddrStr := "0x540552aae708306346466633036396334303062342d24292eadbdc777db86e5"

		payload0, _ := new(felt.Felt).SetString("0x0")
		payload1, _ := new(felt.Felt).SetString("0x5ba586f822ce9debae27fa04a3e71721fdc90ff")
		payload2, _ := new(felt.Felt).SetString("0x455448")
		payload3, _ := new(felt.Felt).SetString("0x31da07977d000")
		payload4, _ := new(felt.Felt).SetString("0x0")

		vmTrace := vm.TransactionTrace{
			Type: vm.TxnInvoke,
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						To:    toAddrStr,
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
			FunctionInvocation:    &vm.FunctionInvocation{},
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

		toAddr, _ := new(felt.Felt).SetString(toAddrStr)

		expectedAdaptedTrace := rpc.TransactionTrace{
			Type: rpc.TxnInvoke,
			ValidateInvocation: &rpc.FunctionInvocation{
				Calls:  []rpc.FunctionInvocation{},
				Events: []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{
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
				ExecutionResources: &rpc.ComputationResources{
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
			FeeTransferInvocation: &rpc.FunctionInvocation{
				Calls:    []rpc.FunctionInvocation{},
				Events:   []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{},
			},
			ExecuteInvocation: &rpc.ExecuteInvocation{
				RevertReason: "",
				FunctionInvocation: &rpc.FunctionInvocation{
					Calls:    []rpc.FunctionInvocation{},
					Events:   []rpc.OrderedEvent{},
					Messages: []rpc.OrderedL2toL1Message{},
				},
			},
			StateDiff: &rpc.StateDiff{ //nolint:dupl
				StorageDiffs: []rpc.StorageDiff{
					{
						Address: felt.Zero,
						StorageEntries: []rpc.Entry{
							{
								Key:   felt.Zero,
								Value: felt.Zero,
							},
						},
					},
				},
				Nonces: []rpc.Nonce{
					{
						ContractAddress: felt.Zero,
						Nonce:           felt.Zero,
					},
				},
				DeployedContracts: []rpc.DeployedContract{
					{
						Address:   felt.Zero,
						ClassHash: felt.Zero,
					},
				},
				DeprecatedDeclaredClasses: []*felt.Felt{
					&felt.Zero,
				},
				DeclaredClasses: []rpc.DeclaredClass{
					{
						ClassHash:         felt.Zero,
						CompiledClassHash: felt.Zero,
					},
				},
				ReplacedClasses: []rpc.ReplacedClass{
					{
						ContractAddress: felt.Zero,
						ClassHash:       felt.Zero,
					},
				},
			},
		}

		assert.Equal(t, expectedAdaptedTrace, rpc.AdaptVMTransactionTrace(&vmTrace))
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
			FunctionInvocation:    &vm.FunctionInvocation{},
		}

		expectedAdaptedTrace := rpc.TransactionTrace{
			Type: rpc.TxnDeployAccount,
			ValidateInvocation: &rpc.FunctionInvocation{
				Calls:    []rpc.FunctionInvocation{},
				Events:   []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{},
			},
			FeeTransferInvocation: &rpc.FunctionInvocation{
				Calls:    []rpc.FunctionInvocation{},
				Events:   []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{},
			},
			ConstructorInvocation: &rpc.FunctionInvocation{
				Calls:    []rpc.FunctionInvocation{},
				Events:   []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{},
			},
		}

		adaptedTrace := rpc.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})

	t.Run("successfully adapt L1_HANDLER tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnL1Handler,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.FunctionInvocation{},
		}

		expectedAdaptedTrace := rpc.TransactionTrace{
			Type: rpc.TxnL1Handler,
			FunctionInvocation: &rpc.FunctionInvocation{
				Calls:    []rpc.FunctionInvocation{},
				Events:   []rpc.OrderedEvent{},
				Messages: []rpc.OrderedL2toL1Message{},
			},
		}

		adaptedTrace := rpc.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})
}

func TestAdaptFeederBlockTrace(t *testing.T) {
	t.Run("nil block trace", func(t *testing.T) {
		block := &rpc.BlockWithTxs{}

		res, err := rpc.AdaptFeederBlockTrace(block, nil)
		require.Nil(t, res)
		require.Nil(t, err)
	})

	t.Run("inconsistent blockWithTxs and blockTrace", func(t *testing.T) {
		blockWithTxs := &rpc.BlockWithTxs{
			Transactions: []*rpc.Transaction{
				{},
			},
		}
		blockTrace := &starknet.BlockTrace{}

		res, err := rpc.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, res)
		require.Equal(t, errors.New("mismatched number of txs and traces"), err)
	})

	t.Run("L1_HANDLER tx gets successfully adapted", func(t *testing.T) {
		blockWithTxs := &rpc.BlockWithTxs{
			Transactions: []*rpc.Transaction{
				{
					Type: rpc.TxnL1Handler,
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

		expectedAdaptedTrace := []rpc.TracedBlockTransaction{
			{
				TransactionHash: new(felt.Felt).SetUint64(1),
				TraceRoot: &rpc.TransactionTrace{
					Type: rpc.TxnL1Handler,
					FunctionInvocation: &rpc.FunctionInvocation{
						Calls: []rpc.FunctionInvocation{},
						Events: []rpc.OrderedEvent{{
							Order: 1,
							Keys:  []*felt.Felt{new(felt.Felt).SetUint64(2)},
							Data:  []*felt.Felt{new(felt.Felt).SetUint64(3)},
						}},
						Messages:           []rpc.OrderedL2toL1Message{},
						ExecutionResources: &rpc.ComputationResources{},
					},
				},
			},
		}

		res, err := rpc.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})

	t.Run("INVOKE tx gets successfully adapted (with revert error)", func(t *testing.T) {
		blockWithTxs := &rpc.BlockWithTxs{
			Transactions: []*rpc.Transaction{
				{
					Type: rpc.TxnInvoke,
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

		expectedAdaptedTrace := []rpc.TracedBlockTransaction{
			{
				TransactionHash: new(felt.Felt).SetUint64(1),
				TraceRoot: &rpc.TransactionTrace{
					Type: rpc.TxnInvoke,
					FeeTransferInvocation: &rpc.FunctionInvocation{
						Calls:              []rpc.FunctionInvocation{},
						Events:             []rpc.OrderedEvent{},
						Messages:           []rpc.OrderedL2toL1Message{},
						ExecutionResources: &rpc.ComputationResources{},
					},
					ValidateInvocation: &rpc.FunctionInvocation{
						Calls:              []rpc.FunctionInvocation{},
						Events:             []rpc.OrderedEvent{},
						Messages:           []rpc.OrderedL2toL1Message{},
						ExecutionResources: &rpc.ComputationResources{},
					},
					ExecuteInvocation: &rpc.ExecuteInvocation{
						RevertReason: "some error",
					},
				},
			},
		}

		res, err := rpc.AdaptFeederBlockTrace(blockWithTxs, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})
}

func TestCall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, "", n, utils.NewNopZapLogger())

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(&rpc.FunctionCall{}, &rpc.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(&rpc.FunctionCall{}, &rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(&rpc.FunctionCall{}, &rpc.BlockID{Number: 0})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("call - unknown contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(new(core.Header), nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(nil, errors.New("unknown contract"))

		res, rpcErr := handler.Call(&rpc.FunctionCall{}, &rpc.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpccore.ErrContractNotFound, rpcErr)
	})

	t.Run("ok", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{
			*new(felt.Felt).SetUint64(4),
			*new(felt.Felt).SetUint64(5),
		}
		expectedRes := vm.CallResult{Result: []*felt.Felt{
			new(felt.Felt).SetUint64(6),
			new(felt.Felt).SetUint64(7),
		}}

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		cairoClass := core.Cairo1Class{
			Program: []*felt.Felt{
				new(felt.Felt).SetUint64(3),
				new(felt.Felt),
				new(felt.Felt),
			},
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(classHash, nil)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClass{Class: &cairoClass}, nil)
		mockReader.EXPECT().Network().Return(n)
		mockVM.EXPECT().Call(&vm.CallInfo{
			ContractAddress: contractAddr,
			ClassHash:       classHash,
			Selector:        selector,
			Calldata:        calldata,
		}, &vm.BlockInfo{Header: headsHeader}, gomock.Any(), &utils.Mainnet, uint64(1337), cairoClass.SierraVersion(), false, false).Return(expectedRes, nil)

		res, rpcErr := handler.Call(&rpc.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           calldata,
		}, &rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		require.Equal(t, expectedRes.Result, res)
	})

	t.Run("unknown entrypoint blockifier 0.14.0", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{*new(felt.Felt).SetUint64(4)}
		expectedRes := vm.CallResult{
			Result:          []*felt.Felt{utils.HexToFelt(t, rpccore.EntrypointNotFoundFelt)},
			ExecutionFailed: true,
		}
		expectedErr := rpccore.ErrContractError
		expectedErr.Data = rpc.ContractErrorData{
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

		cairoClass := core.Cairo1Class{
			Program: []*felt.Felt{
				new(felt.Felt).SetUint64(3),
				new(felt.Felt),
				new(felt.Felt),
			},
		}
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(classHash, nil)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClass{Class: &cairoClass}, nil)
		mockReader.EXPECT().Network().Return(n)
		mockVM.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(expectedRes, nil)

		res, rpcErr := handler.Call(&rpc.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           calldata,
		}, &rpc.BlockID{Latest: true})
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})

	t.Run("execution failed with execution failure", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{*new(felt.Felt).SetUint64(4)}
		expectedRes := vm.CallResult{
			Result:          []*felt.Felt{utils.HexToFelt(t, "0xdeadbeef")},
			ExecutionFailed: true,
		}
		expectedErr := rpc.MakeContractError(json.RawMessage(`"0xdeadbeef"`))

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		cairoClass := core.Cairo1Class{
			Program: []*felt.Felt{
				new(felt.Felt).SetUint64(3),
				new(felt.Felt),
				new(felt.Felt),
			},
		}
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(classHash, nil)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClass{Class: &cairoClass}, nil)
		mockReader.EXPECT().Network().Return(n)
		mockVM.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedRes, nil)

		res, rpcErr := handler.Call(&rpc.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           calldata,
		}, &rpc.BlockID{Latest: true})
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})

	t.Run("execution failed with execution failure and empty result", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{*new(felt.Felt).SetUint64(4)}
		expectedRes := vm.CallResult{
			ExecutionFailed: true,
		}
		expectedErr := rpc.MakeContractError(json.RawMessage(""))

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		cairoClass := core.Cairo1Class{
			Program: []*felt.Felt{
				new(felt.Felt).SetUint64(3),
				new(felt.Felt),
				new(felt.Felt),
			},
		}
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(classHash, nil)
		mockState.EXPECT().Class(classHash).Return(&core.DeclaredClass{Class: &cairoClass}, nil)
		mockReader.EXPECT().Network().Return(n)
		mockVM.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedRes, nil)

		res, rpcErr := handler.Call(&rpc.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           calldata,
		}, &rpc.BlockID{Latest: true})
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})
}
