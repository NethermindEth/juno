package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTraceFallback(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	network := utils.Integration
	client := feeder.NewTestClient(t, &network)
	mockReader := mocks.NewMockReader(mockCtrl)
	gateway := adaptfeeder.New(client)

	mockReader.EXPECT().BlockByNumber(gomock.Any()).DoAndReturn(func(number uint64) (block *core.Block, err error) {
		return gateway.BlockByNumber(context.Background(), number)
	}).AnyTimes()
	mockReader.EXPECT().L1Head().Return(nil, db.ErrKeyNotFound).AnyTimes()

	tests := map[string]struct {
		hash        string
		blockNumber uint64
		want        string
	}{
		"old block": {
			hash:        "0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
			blockNumber: 0,
			want:        `[{"trace_root":{"type":"DEPLOY","constructor_invocation":{"contract_address":"0x7b196a359045d4d0c10f73bdf244a9e1205a615dbb754b8df40173364288534","calldata":["0x187d50a5cf3ebd6d4d6fa8e29e4cad0a237759c6416304a25c4ea792ed4bba4","0x42f5af30d6693674296ad87301935d0c159036c3b24af4042ff0270913bf6c6"],"caller_address":"0x0","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":29}}},"transaction_hash":"0x3fa1bff0c86f34b2eb32c26d12208b6bdb4a5f6a434ac1d4f0e2d1db71bd711"},{"trace_root":{"type":"DEPLOY","constructor_invocation":{"contract_address":"0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6","calldata":["0x5cedec15acd969b0fba39fec9e7d9bd4d0b33f100969ad3a4543039a6f696d4","0xce9801d27b02543f4d88b60aa456860f94ee9f612fc56464abfbdeedc1ab72"],"caller_address":"0x0","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":29}}},"transaction_hash":"0x154c02cc3165cceadaa32e7238a67061b3a1eac414138c4ebe1408f37fd93eb"},{"trace_root":{"type":"INVOKE","execute_invocation":{"contract_address":"0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6","calldata":["0x17d9c35a8b9a0d4512fa05eafec01c2758a7a5b7ec7b47408a24a4b33124d9b","0x2","0x7f800b5bf79637f8f83f47a8fc4d368b43695c781b22a899f11b5f2faba874a","0x3a7a40d383612b0ad167aec8d90fb07e576e017d07948f63ac318b52511ae93"],"caller_address":"0x0","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":165,"memory_holes":22,"pedersen_builtin_applications":2,"range_check_builtin_applications":7}}},"transaction_hash":"0x7893675c16da857b7c4229cda449e08a4fe13b07ca817e79d1db02e8a046047"},{"trace_root":{"type":"INVOKE","execute_invocation":{"contract_address":"0x64ed79a8ebe97485d3357bbfdf5f6bea0d9db3b5f1feb6e80d564a179122dc6","calldata":["0x17d9c35a8b9a0d4512fa05eafec01c2758a7a5b7ec7b47408a24a4b33124d9b","0x2","0x7f800b5bf79637f8f83f47a8fc4d368b43695c781b22a899f11b5f2faba874a","0xf140b304e9266c72f1054116dd06d9c1c8e981db7bf34e3c6da99640e9a7c8"],"caller_address":"0x0","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":165,"memory_holes":22,"pedersen_builtin_applications":2,"range_check_builtin_applications":7}}},"transaction_hash":"0x4a277d67e3f42c4a343854081d1e2e9e425f1323255e4486d2badb37a1d8630"}]`,
		},
		"newer block": {
			hash:        "0xe3828bd9154ab385e2cbb95b3b650365fb3c6a4321660d98ce8b0a9194f9a3",
			blockNumber: 300000,
			want:        `[{"trace_root":{"type":"INVOKE","validate_invocation":{"contract_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","entry_point_selector":"0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775","calldata":["0x1","0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2","0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325","0x0","0x4","0x4","0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9","0x2","0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228","0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e"],"caller_address":"0x0","class_hash":"0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":89,"range_check_builtin_applications":2,"ecdsa_builtin_applications":1}},"execute_invocation":{"contract_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","entry_point_selector":"0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad","calldata":["0x1","0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2","0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325","0x0","0x4","0x4","0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9","0x2","0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228","0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e"],"caller_address":"0x0","class_hash":"0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[{"contract_address":"0x332299dc083f3778122e5b7762bc9d399da18fefe93769aee67bb49f51c8d2","entry_point_selector":"0x2d7cf5d5a324a320f9f37804b1615a533fde487400b41af80f13f7ac5581325","calldata":["0xaf35ee8ed700ff132c5d1d298a73becda25ccdf9","0x2","0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228","0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e"],"caller_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","class_hash":"0x165e7db96ab97a63c621229617a6d49633737238673477a54720e4c952f2c7e","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[],"events":[],"messages":[{"order":0,"to_address":"0xAf35eE8eD700ff132C5d1d298A73BECdA25ccDF9","payload":["0x6cd852fe1b2bbd8587bb0aaeb09813436c57c8ce21e75651e317273a1f22228","0x58feb991988e53fffcba71f6df23c803fb062f1b3bab126d2c9ce574255b36e"]}],"execution_resources":{"steps":233,"memory_holes":1,"range_check_builtin_applications":5}}],"events":[],"messages":[],"execution_resources":{"steps":374,"memory_holes":4,"range_check_builtin_applications":7}},"fee_transfer_invocation":{"contract_address":"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7","entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8","0x127089df3a1984","0x0"],"caller_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","class_hash":"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3","entry_point_type":"EXTERNAL","call_type":"CALL","result":["0x1"],"calls":[{"contract_address":"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7","entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8","0x127089df3a1984","0x0"],"caller_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","class_hash":"0x28d7d394810ad8c52741ad8f7564717fd02c10ced68657a81d0b6710ce22079","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":["0x1"],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":488,"memory_holes":40,"pedersen_builtin_applications":4,"range_check_builtin_applications":21}}],"events":[],"messages":[],"execution_resources":{"steps":548,"memory_holes":40,"pedersen_builtin_applications":4,"range_check_builtin_applications":21}}},"transaction_hash":"0x2a648ab1aa6847eb38507fc842e050f256562bf87b26083c332f3f21318c2c3"},{"trace_root":{"type":"INVOKE","validate_invocation":{"contract_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","entry_point_selector":"0x162da33a4585851fe8d3af3c2a9c60b557814e221e0d4f30ff0b2189d9c7775","calldata":["0x1","0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1","0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f","0x0","0x4","0x4","0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb","0x2","0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e","0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336"],"caller_address":"0x0","class_hash":"0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":89,"range_check_builtin_applications":2,"ecdsa_builtin_applications":1}},"execute_invocation":{"contract_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","entry_point_selector":"0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad","calldata":["0x1","0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1","0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f","0x0","0x4","0x4","0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb","0x2","0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e","0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336"],"caller_address":"0x0","class_hash":"0x646a72e2aab2fca75d713fbe4a58f2d12cbd64105621b89dc9ce7045b5bf02b","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[{"contract_address":"0x5f9211b05c9609d54a8bf5f9cfa4e2cd5a3cab3b5d79682c585575495a15dd1","entry_point_selector":"0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f","calldata":["0x447379c077035ef4f442411d0407ce9aa66c558f0060137f6455f4f230eabeb","0x2","0x6811b7755a7dd0ec1fb6f51a883e3f255368e2dfd497b5f6480c00cf9cd5a2e","0x23b9e26720dd7aaf98c7cea56499f48f75dc1d4123f7e2d6c23bfc4d5f4a336"],"caller_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","class_hash":"0x13abfd2f333f9c69f690f1569140cdae25f6f66e3f371c9cbb998b65f664a85","entry_point_type":"EXTERNAL","call_type":"CALL","result":[],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":166,"memory_holes":22,"pedersen_builtin_applications":2,"range_check_builtin_applications":7}}],"events":[],"messages":[],"execution_resources":{"steps":307,"memory_holes":25,"pedersen_builtin_applications":2,"range_check_builtin_applications":9}},"fee_transfer_invocation":{"contract_address":"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7","entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8","0x3b2d25cd7bccc","0x0"],"caller_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","class_hash":"0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3","entry_point_type":"EXTERNAL","call_type":"CALL","result":["0x1"],"calls":[{"contract_address":"0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7","entry_point_selector":"0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","calldata":["0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8","0x3b2d25cd7bccc","0x0"],"caller_address":"0x58b7ee817bd2978c7657d05d3131e83e301ed1aa79d5ad16f01925fd52d1da7","class_hash":"0x28d7d394810ad8c52741ad8f7564717fd02c10ced68657a81d0b6710ce22079","entry_point_type":"EXTERNAL","call_type":"DELEGATE","result":["0x1"],"calls":[],"events":[],"messages":[],"execution_resources":{"steps":488,"memory_holes":40,"pedersen_builtin_applications":4,"range_check_builtin_applications":21}}],"events":[],"messages":[],"execution_resources":{"steps":548,"memory_holes":40,"pedersen_builtin_applications":4,"range_check_builtin_applications":21}}},"transaction_hash":"0xbc984e8e1fe594dd518a3a51db4f338437a5d2fbdda772d4426b532a67ffff"}]`,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			mockReader.EXPECT().BlockByHash(utils.HexToFelt(t, test.hash)).DoAndReturn(func(_ *felt.Felt) (block *core.Block, err error) {
				return mockReader.BlockByNumber(test.blockNumber)
			}).Times(2)
			handler := rpc.New(mockReader, nil, nil, "", nil)
			_, jErr := handler.TraceBlockTransactions(context.Background(), rpc.BlockID{Number: test.blockNumber})
			require.Equal(t, rpc.ErrInternal.Code, jErr.Code)

			handler = handler.WithFeeder(client)
			trace, jErr := handler.TraceBlockTransactions(context.Background(), rpc.BlockID{Number: test.blockNumber})
			require.Nil(t, jErr)
			jsonStr, err := json.Marshal(trace)
			require.NoError(t, err)
			assert.JSONEq(t, test.want, string(jsonStr))
		})
	}
}

func TestTraceTransaction(t *testing.T) {
	t.Skip()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&utils.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, "", utils.NewNopZapLogger())

	t.Run("not found", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xBBBB")
		// Receipt() returns error related to db
		mockReader.EXPECT().Receipt(hash).Return(nil, nil, uint64(0), db.ErrKeyNotFound)

		trace, err := handler.TraceTransaction(context.Background(), *hash)
		assert.Nil(t, trace)
		assert.Equal(t, rpc.ErrTxnHashNotFound, err)
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
		mockReader.EXPECT().BlockByNumber(header.Number).Return(block, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateHistoryReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTraceJSON := json.RawMessage(`{
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
			&vm.BlockInfo{Header: header}, gomock.Any(), &utils.Mainnet, false, false, false, false).Return(nil, []vm.TransactionTrace{*vmTrace}, nil)

		trace, err := handler.TraceTransaction(context.Background(), *hash)
		require.Nil(t, err)
		assert.Equal(t, vmTrace, trace)
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
			network := utils.Mainnet
			chain := blockchain.New(pebble.NewMemTest(t), &network)
			handler := rpc.New(chain, nil, nil, "", log)

			update, rpcErr := handler.TraceBlockTransactions(context.Background(), id)
			assert.Nil(t, update)
			assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
		})
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	network := utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&network).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	log := utils.NewNopZapLogger()

	handler := rpc.New(mockReader, nil, mockVM, "", log)

	t.Run("pending block", func(t *testing.T) {
		blockHash := utils.HexToFelt(t, "0x0001")
		header := &core.Header{
			// hash is not set because it's pending block
			ParentHash:      utils.HexToFelt(t, "0x0C3"),
			Number:          0,
			GasPrice:        utils.HexToFelt(t, "0x777"),
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
		mockReader.EXPECT().PendingState().Return(headState, nopCloser, nil)

		paidL1Fees := []*felt.Felt{(&felt.Felt{}).SetUint64(1)}
		vmTraceJSON := json.RawMessage(`{
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
			gomock.Any(), &network, false, false, false, false).Return(nil, []vm.TransactionTrace{vmTrace, vmTrace}, nil)

		result, err := handler.TraceBlockTransactions(context.Background(), rpc.BlockID{Hash: blockHash})
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
			GasPrice:         utils.HexToFelt(t, "0x777"),
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
			gomock.Any(), &network, false, false, false, false).Return(nil, []vm.TransactionTrace{vmTrace}, nil)

		expectedResult := []rpc.TracedBlockTransaction{
			{
				TransactionHash: tx.Hash(),
				TraceRoot:       &vmTrace,
			},
		}
		result, err := handler.TraceBlockTransactions(context.Background(), rpc.BlockID{Hash: blockHash})
		require.Nil(t, err)
		assert.Equal(t, expectedResult, result)
	})
}

func TestCall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpc.New(mockReader, nil, mockVM, "", utils.NewNopZapLogger())

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(rpc.FunctionCall{}, rpc.BlockID{Latest: true})
		require.Nil(t, res)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		res, rpcErr := handler.Call(rpc.FunctionCall{}, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, res)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

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

	t.Run("ok", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337)

		contractAddr := new(felt.Felt).SetUint64(1)
		selector := new(felt.Felt).SetUint64(2)
		classHash := new(felt.Felt).SetUint64(3)
		calldata := []felt.Felt{
			*new(felt.Felt).SetUint64(4),
			*new(felt.Felt).SetUint64(5),
		}
		expectedRes := []*felt.Felt{
			new(felt.Felt).SetUint64(6),
			new(felt.Felt).SetUint64(7),
		}

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(classHash, nil)
		mockReader.EXPECT().Network().Return(&utils.Mainnet)
		mockVM.EXPECT().Call(&vm.CallInfo{
			ContractAddress: contractAddr,
			ClassHash:       classHash,
			Selector:        selector,
			Calldata:        calldata,
		}, &vm.BlockInfo{Header: headsHeader}, gomock.Any(), &utils.Mainnet, uint64(1337), true).Return(expectedRes, nil)

		res, rpcErr := handler.Call(rpc.FunctionCall{
			ContractAddress:    *contractAddr,
			EntryPointSelector: *selector,
			Calldata:           calldata,
		}, rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		require.Equal(t, expectedRes, res)
	})
}
