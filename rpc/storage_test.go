package rpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNonce(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(nil, errors.New("non-existent contract"))

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, nonce)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedNonce := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Latest: true}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Hash: &felt.Zero}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractNonce(&felt.Zero).Return(expectedNonce, nil)

		nonce, rpcErr := handler.Nonce(rpc.BlockID{Number: 0}, felt.Zero)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedNonce, nonce)
	})
}

func TestStorageAt(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, nil, "", log)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(nil, nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(nil, nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrBlockNotFound, rpcErr)
	})

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	t.Run("non-existent contract", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(nil, db.ErrKeyNotFound)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractClassHash(&felt.Zero).Return(nil, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})
}

func TestStorageProof(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	var (
		blkHash     = utils.HexToFelt(t, "0x11ead")
		root        = utils.HexToFelt(t, "0x43f7163af64f9199e7c0bba225c2c3310ee2947be5ec0f03c9fb1551135818b")
		key         = new(felt.Felt).SetUint64(1)
		key2        = new(felt.Felt).SetUint64(8)
		noSuchKey   = new(felt.Felt).SetUint64(0)
		value       = new(felt.Felt).SetUint64(51)
		value2      = new(felt.Felt).SetUint64(58)
		blockLatest = rpc.BlockID{Latest: true}
		blockNumber = uint64(1313)
	)

	tempTrie := emptyTrie(t)
	_, _ = tempTrie.Put(key, value)
	_, _ = tempTrie.Put(key2, value2)
	_ = tempTrie.Commit()
	trieRoot, _ := tempTrie.Root()

	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, func() error { return nil }, nil).AnyTimes()
	mockReader.EXPECT().Head().Return(&core.Block{Header: &core.Header{Hash: blkHash, Number: blockNumber}}, nil).AnyTimes()
	mockState.EXPECT().ClassTrie().Return(tempTrie, nil).AnyTimes()
	mockState.EXPECT().ContractTrie().Return(tempTrie, nil).AnyTimes()

	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, nil, "", log)

	t.Run("global roots are filled", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, rpcErr)

		require.NotNil(t, proof)
		require.NotNil(t, proof.GlobalRoots)
		require.Equal(t, blkHash, proof.GlobalRoots.BlockHash)
		require.Equal(t, root, proof.GlobalRoots.ClassesTreeRoot)
		require.Equal(t, root, proof.GlobalRoots.ContractsTreeRoot)
	})
	t.Run("error is returned whenever not latest block is requested", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(rpc.BlockID{Number: 1}, nil, nil, nil)
		assert.Equal(t, rpc.ErrStorageProofNotSupported, rpcErr)
		require.Nil(t, proof)
	})
	t.Run("error is returned even when blknum matches head", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(rpc.BlockID{Number: blockNumber}, nil, nil, nil)
		assert.Equal(t, rpc.ErrStorageProofNotSupported, rpcErr)
		require.Nil(t, proof)
	})
	t.Run("empty request", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 0, 0, 0, 0)
	})
	t.Run("class trie hash does not exist in a trie", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, []felt.Felt{*noSuchKey}, nil, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 3, 0, 0, 0)
		verifyIf(t, trieRoot, noSuchKey, nil, proof.ClassesProof, tempTrie.HashFn())
	})
	t.Run("class trie hash exists in a trie", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, []felt.Felt{*key}, nil, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 3, 0, 0, 0)
		verifyIf(t, trieRoot, key, value, proof.ClassesProof, tempTrie.HashFn())
	})
	t.Run("only unique proof nodes are returned", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, []felt.Felt{*key, *key2}, nil, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)

		rootNodes := utils.Filter(proof.ClassesProof, func(h *rpc.HashToNode) bool {
			return h.Hash.Equal(trieRoot)
		})
		require.Len(t, rootNodes, 1)

		// verify we can still prove any of the keys in query
		verifyIf(t, trieRoot, key, value, proof.ClassesProof, tempTrie.HashFn())
		verifyIf(t, trieRoot, key2, value2, proof.ClassesProof, tempTrie.HashFn())
	})
	t.Run("storage trie address does not exist in a trie", func(t *testing.T) {
		mockState.EXPECT().ContractNonce(noSuchKey).Return(nil, db.ErrKeyNotFound).Times(1)
		mockState.EXPECT().ContractClassHash(noSuchKey).Return(nil, db.ErrKeyNotFound).Times(0)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, []felt.Felt{*noSuchKey}, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 0, 3, 1, 0)
		require.Nil(t, proof.ContractsProof.LeavesData[0])

		verifyIf(t, trieRoot, noSuchKey, nil, proof.ContractsProof.Nodes, tempTrie.HashFn())
	})
	t.Run("storage trie address exists in a trie", func(t *testing.T) {
		nonce := new(felt.Felt).SetUint64(121)
		mockState.EXPECT().ContractNonce(key).Return(nonce, nil).Times(1)
		classHasah := new(felt.Felt).SetUint64(1234)
		mockState.EXPECT().ContractClassHash(key).Return(classHasah, nil).Times(1)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, []felt.Felt{*key}, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 0, 3, 1, 0)

		require.NotNil(t, proof.ContractsProof.LeavesData[0])
		ld := proof.ContractsProof.LeavesData[0]
		require.Equal(t, nonce, ld.Nonce)
		require.Equal(t, classHasah, ld.ClassHash)

		verifyIf(t, trieRoot, key, value, proof.ContractsProof.Nodes, tempTrie.HashFn())
	})
	t.Run("contract storage trie address does not exist in a trie", func(t *testing.T) {
		contract := utils.HexToFelt(t, "0xdead")
		mockState.EXPECT().ContractStorageTrie(contract).Return(emptyTrie(t), nil).Times(1)

		storageKeys := []rpc.StorageKeys{{Contract: *contract, Keys: []felt.Felt{*key}}}
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, storageKeys)
		require.NotNil(t, proof)
		require.Nil(t, rpcErr)
		arityTest(t, proof, 0, 0, 0, 1)
		require.Len(t, proof.ContractsStorageProofs[0], 0)
	})
	//nolint:dupl
	t.Run("contract storage trie key slot does not exist in a trie", func(t *testing.T) {
		contract := utils.HexToFelt(t, "0xabcd")
		mockState.EXPECT().ContractStorageTrie(contract).Return(tempTrie, nil).Times(1)

		storageKeys := []rpc.StorageKeys{{Contract: *contract, Keys: []felt.Felt{*noSuchKey}}}
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, storageKeys)
		require.NotNil(t, proof)
		require.Nil(t, rpcErr)
		arityTest(t, proof, 0, 0, 0, 1)
		require.Len(t, proof.ContractsStorageProofs[0], 3)

		verifyIf(t, trieRoot, noSuchKey, nil, proof.ContractsStorageProofs[0], tempTrie.HashFn())
	})
	//nolint:dupl
	t.Run("contract storage trie address/key exists in a trie", func(t *testing.T) {
		contract := utils.HexToFelt(t, "0xadd0")
		mockState.EXPECT().ContractStorageTrie(contract).Return(tempTrie, nil).Times(1)

		storageKeys := []rpc.StorageKeys{{Contract: *contract, Keys: []felt.Felt{*key}}}
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, storageKeys)
		require.NotNil(t, proof)
		require.Nil(t, rpcErr)
		arityTest(t, proof, 0, 0, 0, 1)
		require.Len(t, proof.ContractsStorageProofs[0], 3)

		verifyIf(t, trieRoot, key, value, proof.ContractsStorageProofs[0], tempTrie.HashFn())
	})
	t.Run("class & storage tries proofs requested", func(t *testing.T) {
		nonce := new(felt.Felt).SetUint64(121)
		mockState.EXPECT().ContractNonce(key).Return(nonce, nil)
		classHasah := new(felt.Felt).SetUint64(1234)
		mockState.EXPECT().ContractClassHash(key).Return(classHasah, nil)

		proof, rpcErr := handler.StorageProof(blockLatest, []felt.Felt{*key}, []felt.Felt{*key}, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 3, 3, 1, 0)
	})
}

func TestStorageProof_VerifyPathfinderResponse(t *testing.T) {
	t.Parallel()

	// Pathfinder response for query:
	//	"method": "starknet_getStorageProof",
	//	"params": [
	//		"latest",
	//		[],
	//		[
	//		"0x5a03b82d726f9bb31ba41ea3a0c1143f90241e37c9a4a92174d168cda9c716d",
	//		"0x5fbaa249500be29fee38fdd90a7a2651a8d3935c14167570f6863f563d838f0"
	//		]
	//	],
	// Sepolia, at block 10434
	result := rpc.StorageProofResult{
		ClassesProof: []*rpc.HashToNode{},
		ContractsProof: &rpc.ContractProof{
			LeavesData: []*rpc.LeafData{
				{
					Nonce:     utils.HexToFelt(t, "0x0"),
					ClassHash: utils.HexToFelt(t, "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3"),
					// TODO: get the storage root
				},
				{
					Nonce:     utils.HexToFelt(t, "0x0"),
					ClassHash: utils.HexToFelt(t, "0x78401746828463e2c3f92ebb261fc82f7d4d4c8d9a80a356c44580dab124cb0"),
					// TODO: get the storage root
				},
			},
			Nodes: []*rpc.HashToNode{
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x5c6be09d8faaa42a8525898b1047cebdd3526349b48decc2b767a4fa612263d"),
						Right: utils.HexToFelt(t, "0xcd11aa7699c4157a287e5fe574df37e40c8b6a5ed5e1aee658fc2d634398ef"),
					},
					Hash: utils.HexToFelt(t, "0x7884784e689e733c1ea2c4ee3b1f790c4ca4992b26d8aee31abb5d9270d4947"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x1cdf395ebbba2f3a6234ad9827b08453a4a0b7745e2d919fe7b07749efa5325"),
						Right: utils.HexToFelt(t, "0xcdd37cf6cce8bc373e2c9d8d6754b057275ddd910a9d133b4d31086632d0f4"),
					},
					Hash: utils.HexToFelt(t, "0x44fcfce222b7e5a098346615dc838d8ae90ff55da82db7cdce4303f34042ff6"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x2c55bc287a1b31a405c681c2bb720811dd9f33523241561ea4b356f717ff9f6"),
						Right: utils.HexToFelt(t, "0x2012025c00174e3eb72baba21e58a56e5114e571f64cb1040f7de0c8daef618"),
					},
					Hash: utils.HexToFelt(t, "0x7f2b62cf9713a0b635b967c2e2891282631519eebca6ea0bddaa1a1a804919f"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x211a80e63ac0b12b29279c3d57ea5771b5003ea464b055aeb8ad8618ff3cd69"),
						Right: utils.HexToFelt(t, "0x44f55356be17913dcd79e0bb4dbc986d0642bb3f000e540bb54bfa2d4189a74"),
					},
					Hash: utils.HexToFelt(t, "0x69e208899d9deeae0732e95ce9d68d123abd9b59f157435fc3554e1fa3a92a8"),
				},
				{
					Node: &rpc.EdgeNode{
						Child:  utils.HexToFelt(t, "0x6b45780618ce075fb4543396b3a6949915c04962b2e411c4f1b2a6813d540da"),
						Length: 239,
						Path:   "0x3b82d726f9bb31ba41ea3a0c1143f90241e37c9a4a92174d168cda9c716d",
					},
					Hash: utils.HexToFelt(t, "0x2c55bc287a1b31a405c681c2bb720811dd9f33523241561ea4b356f717ff9f6"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x7be97a0f8a99126208712673c69c292a26273707c884e96e17c761ee7097ae5"),
						Right: utils.HexToFelt(t, "0x3ae1731f598d03a9033c6f5d29871cd5a80c4eba36a7a0a73775ea9d8d522f3"),
					},
					Hash: utils.HexToFelt(t, "0xcd11aa7699c4157a287e5fe574df37e40c8b6a5ed5e1aee658fc2d634398ef"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x7f2b62cf9713a0b635b967c2e2891282631519eebca6ea0bddaa1a1a804919f"),
						Right: utils.HexToFelt(t, "0x77f807a73f0e7ccad122cd946d79d8f4ce9e02f01017467e7cf4ad993cfa482"),
					},
					Hash: utils.HexToFelt(t, "0x326e52c7cba85fedb456bb1c25dda2075ebe3367a329eb297144cb7f8d1f7d9"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x35d32a880d122ffc43a46e280c0ff34a9de286c2cb2e3933229f419a6ceed8e"),
						Right: utils.HexToFelt(t, "0x14c9f5368ebbe1cc8d1db2dde1f97d18cabf450bbc23f154985c7e15e15bdcf"),
					},
					Hash: utils.HexToFelt(t, "0x1159575d44f9b716f2cfbb13da873f8e7d9824e6b7b615dac5ce9c7b0e2bffd"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x1e5dfbcf23a5e942208f5ccfa25db1147dbfb2984df32a692102851757998cd"),
						Right: utils.HexToFelt(t, "0x69e208899d9deeae0732e95ce9d68d123abd9b59f157435fc3554e1fa3a92a8"),
					},
					Hash: utils.HexToFelt(t, "0x2722e2a47b3f10db016928bcc7451cd2088a1caea2fbb5f08e1b71dfe1db1c2"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x2634833b52e930231b53d58286647d9818a276dd12ace8286dae63b896c3ba1"),
						Right: utils.HexToFelt(t, "0x1f248a8796f18bc9d116e5f3c3956c47e091c05f1c9596453b2fefa2b725507"),
					},
					Hash: utils.HexToFelt(t, "0x109e30040b25357cc51726d6041ba1f09ec02dd8b3ca2ffa686a858c9293796"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x7884784e689e733c1ea2c4ee3b1f790c4ca4992b26d8aee31abb5d9270d4947"),
						Right: utils.HexToFelt(t, "0x4e354efe4fcc718d3454d532b50cd3c73ac84f05df918981433162c84650f6c"),
					},
					Hash: utils.HexToFelt(t, "0x88648f7a7b355914ed41bb28101110cff8fb68f1a9b39958823c72992d8675"),
				},
				{
					Node: &rpc.EdgeNode{
						Child:  utils.HexToFelt(t, "0x4169679eea4895011fb8e9029b4591a210b3b9e9aa23f12f25cf45cbcaadfe8"),
						Length: 1,
						Path:   "0x1",
					},
					Hash: utils.HexToFelt(t, "0x44f55356be17913dcd79e0bb4dbc986d0642bb3f000e540bb54bfa2d4189a74"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x192804e98b1f3fdad2d8fab79bfb922611edc5fb48dcd1e9db02cd46cfa9763"),
						Right: utils.HexToFelt(t, "0x4717a5dd5048d62401bc7db57594d3bdbfd3c7b99788a83c5e77b6db9822149"),
					},
					Hash: utils.HexToFelt(t, "0x14c9f5368ebbe1cc8d1db2dde1f97d18cabf450bbc23f154985c7e15e15bdcf"),
				},
				{
					Node: &rpc.EdgeNode{
						Child:  utils.HexToFelt(t, "0x25790175fe1fbeed47cbf510a41fba8676bea20a0c8888d4b9090b8f5cf19b8"),
						Length: 238,
						Path:   "0x2a249500be29fee38fdd90a7a2651a8d3935c14167570f6863f563d838f0",
					},
					Hash: utils.HexToFelt(t, "0x331128166378265a07c0be65b242d47d1965e785b6f4f6e1bca3731de5d2d1d"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x331128166378265a07c0be65b242d47d1965e785b6f4f6e1bca3731de5d2d1d"),
						Right: utils.HexToFelt(t, "0x12af5e7e95772777d98792be8ade3b18c06ab21aa492a1821d5be3ac291374a"),
					},
					Hash: utils.HexToFelt(t, "0x4169679eea4895011fb8e9029b4591a210b3b9e9aa23f12f25cf45cbcaadfe8"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x485b298f33aa076113362f82f4bf64f23e2eb5b84209353a630a46cd20fdde5"),
						Right: utils.HexToFelt(t, "0x1159575d44f9b716f2cfbb13da873f8e7d9824e6b7b615dac5ce9c7b0e2bffd"),
					},
					Hash: utils.HexToFelt(t, "0x3ae1731f598d03a9033c6f5d29871cd5a80c4eba36a7a0a73775ea9d8d522f3"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x2358473807e0a43a66b918247c0fb0d0649c72a32f19eee8bcc76c090b37951"),
						Right: utils.HexToFelt(t, "0x109e30040b25357cc51726d6041ba1f09ec02dd8b3ca2ffa686a858c9293796"),
					},
					Hash: utils.HexToFelt(t, "0x485b298f33aa076113362f82f4bf64f23e2eb5b84209353a630a46cd20fdde5"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x326e52c7cba85fedb456bb1c25dda2075ebe3367a329eb297144cb7f8d1f7d9"),
						Right: utils.HexToFelt(t, "0x41149879a9d24ba0a2ccfb56415c04bdabb1c51eb0900a17dee2c715d6b1c70"),
					},
					Hash: utils.HexToFelt(t, "0x1cdf395ebbba2f3a6234ad9827b08453a4a0b7745e2d919fe7b07749efa5325"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x454a8b3fc492869e79b16e87461d0b5101eb5d25389f492039ef6a380878b39"),
						Right: utils.HexToFelt(t, "0x5a99604af4e482d046afe656b6ebe7805c72a1b7979d00608f27b276eb33442"),
					},
					Hash: utils.HexToFelt(t, "0x4717a5dd5048d62401bc7db57594d3bdbfd3c7b99788a83c5e77b6db9822149"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x2f6c0e4b8022b48461e54e4f9358c51d5444ae2e2253a31baa68d4cb0c938de"),
						Right: utils.HexToFelt(t, "0x88648f7a7b355914ed41bb28101110cff8fb68f1a9b39958823c72992d8675"),
					},
					Hash: utils.HexToFelt(t, "0x47182b7d8158a8f80ed15822719aa306af37383a0cf91518d21ba63e73fea13"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x44fcfce222b7e5a098346615dc838d8ae90ff55da82db7cdce4303f34042ff6"),
						Right: utils.HexToFelt(t, "0xc3da9c726d244197963a8a7beb4a3aee353b3b663daf2aa1bcf1c087b5e20d"),
					},
					Hash: utils.HexToFelt(t, "0x2634833b52e930231b53d58286647d9818a276dd12ace8286dae63b896c3ba1"),
				},
				{
					Node: &rpc.BinaryNode{
						Left:  utils.HexToFelt(t, "0x2722e2a47b3f10db016928bcc7451cd2088a1caea2fbb5f08e1b71dfe1db1c2"),
						Right: utils.HexToFelt(t, "0x79c09acd32044c7d455299ca67e2a8fafce25afaf6d5e89ff4632b251dddc8d"),
					},
					Hash: utils.HexToFelt(t, "0x5a99604af4e482d046afe656b6ebe7805c72a1b7979d00608f27b276eb33442"),
				},
			},
		},
		ContractsStorageProofs: [][]*rpc.HashToNode{},
		GlobalRoots: &rpc.GlobalRoots{
			BlockHash:         utils.HexToFelt(t, "0xae4cc763c8b350913e00e12cffd51fb7e3b730e29036864a8afd8ec323ecd6"),
			ClassesTreeRoot:   utils.HexToFelt(t, "0xea1568e1ca4e5b8c19cdf130dc3194f9cb8e5eee2fa5ec54a338a4dccfd6e3"),
			ContractsTreeRoot: utils.HexToFelt(t, "0x47182b7d8158a8f80ed15822719aa306af37383a0cf91518d21ba63e73fea13"),
		},
	}

	root := result.GlobalRoots.ContractsTreeRoot

	t.Run("first contract proof verification", func(t *testing.T) {
		t.Parallel()

		firstContractAddr := utils.HexToFelt(t, "0x5a03b82d726f9bb31ba41ea3a0c1143f90241e37c9a4a92174d168cda9c716d")
		firstContractLeaf := utils.HexToFelt(t, "0x6b45780618ce075fb4543396b3a6949915c04962b2e411c4f1b2a6813d540da")
		verifyIf(t, root, firstContractAddr, firstContractLeaf, result.ContractsProof.Nodes, crypto.Pedersen)
	})

	t.Run("second contract proof verification", func(t *testing.T) {
		t.Parallel()

		secondContractAddr := utils.HexToFelt(t, "0x5fbaa249500be29fee38fdd90a7a2651a8d3935c14167570f6863f563d838f0")
		secondContractLeaf := utils.HexToFelt(t, "0x25790175fe1fbeed47cbf510a41fba8676bea20a0c8888d4b9090b8f5cf19b8")
		verifyIf(t, root, secondContractAddr, secondContractLeaf, result.ContractsProof.Nodes, crypto.Pedersen)
	})
}

func TestStorageProof_StorageRoots(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	log := utils.NewNopZapLogger()
	testDB := pebble.NewMemTest(t)
	bc := blockchain.New(testDB, &utils.Mainnet, nil)
	synchronizer := sync.New(bc, gw, log, time.Duration(0), false, testDB)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	require.NoError(t, synchronizer.Run(ctx))
	cancel()

	var (
		expectedBlockHash       = utils.HexToFelt(t, "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6")
		expectedGlobalRoot      = utils.HexToFelt(t, "0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9")
		expectedClsRoot         = utils.HexToFelt(t, "0x0")
		expectedStgRoot         = utils.HexToFelt(t, "0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9")
		expectedContractAddress = utils.HexToFelt(t, "0x2d6c9569dea5f18628f1ef7c15978ee3093d2d3eec3b893aac08004e678ead3")
		expectedContractLeaf    = utils.HexToFelt(t, "0x7036d8dd68dc9539c6db8c88f72b1ab16e76d62b5f09118eca5ae78276b0ee4")
	)

	t.Run("sanity check - mainnet block 2", func(t *testing.T) {
		expectedBlockNumber := uint64(2)

		blk, err := bc.Head()
		assert.NoError(t, err)
		assert.Equal(t, expectedBlockNumber, blk.Number)
		assert.Equal(t, expectedBlockHash, blk.Hash, blk.Hash.String())
		assert.Equal(t, expectedGlobalRoot, blk.GlobalStateRoot, blk.GlobalStateRoot.String())
	})

	t.Run("check class and storage roots matches the global", func(t *testing.T) {
		reader, closer, err := bc.HeadState()
		assert.NoError(t, err)
		defer func() { _ = closer() }()

		classTrie, err := reader.ClassTrie()
		assert.NoError(t, err)

		contractTrie, err := reader.ContractTrie()
		assert.NoError(t, err)

		clsRoot, err := classTrie.Root()
		assert.NoError(t, err)

		stgRoot, err := contractTrie.Root()
		assert.NoError(t, err)

		assert.Equal(t, expectedClsRoot, clsRoot, clsRoot.String())
		assert.Equal(t, expectedStgRoot, stgRoot, stgRoot.String())

		verifyGlobalStateRoot(t, expectedGlobalRoot, clsRoot, stgRoot)
	})

	t.Run("check requested contract and storage slot exists", func(t *testing.T) {
		stateReader, stCloser, err := bc.HeadState()
		assert.NoError(t, err)
		defer func() { _ = stCloser() }()

		contractTrie, err := stateReader.ContractTrie()
		assert.NoError(t, err)

		leaf, err := contractTrie.Get(expectedContractAddress)
		assert.NoError(t, err)
		assert.Equal(t, leaf, expectedContractLeaf, leaf.String())

		clsHash, err := stateReader.ContractClassHash(expectedContractAddress)
		assert.NoError(t, err)
		assert.Equal(t, clsHash, utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"), clsHash.String())
	})

	t.Run("get contract proof", func(t *testing.T) {
		handler := rpc.New(bc, nil, nil, "", log)
		result, rpcErr := handler.StorageProof(
			rpc.BlockID{Latest: true}, nil, []felt.Felt{*expectedContractAddress}, nil)
		require.Nil(t, rpcErr)

		expectedResult := rpc.StorageProofResult{
			ClassesProof:           []*rpc.HashToNode{},
			ContractsStorageProofs: [][]*rpc.HashToNode{},
			ContractsProof: &rpc.ContractProof{
				LeavesData: []*rpc.LeafData{
					{
						Nonce:       utils.HexToFelt(t, "0x0"),
						ClassHash:   utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"),
						StorageRoot: utils.HexToFelt(t, "0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9"),
					},
				},
				Nodes: []*rpc.HashToNode{
					{
						Hash: utils.HexToFelt(t, "0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9"),
						Node: &rpc.BinaryNode{
							Left:  utils.HexToFelt(t, "0x4e1f289e55ac8a821fd463478e6f5543256beb934a871be91d00a0d3f2e7964"),
							Right: utils.HexToFelt(t, "0x67d9833b51e7bf1cab0e71e68477bf7f0b704391d753f9d793008e4f6587c53"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x4e1f289e55ac8a821fd463478e6f5543256beb934a871be91d00a0d3f2e7964"),
						Node: &rpc.BinaryNode{
							Left:  utils.HexToFelt(t, "0x1ef87d62309ff1cad58d39e8f5480f9caa9acd78a43f139d87220a1babe38a4"),
							Right: utils.HexToFelt(t, "0x9a258d24b3aeb7e263e910d68a18d85305703a2f20df2e806ecbb1fb28760f"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x9a258d24b3aeb7e263e910d68a18d85305703a2f20df2e806ecbb1fb28760f"),
						Node: &rpc.BinaryNode{
							Left:  utils.HexToFelt(t, "0x53f61d0cb8099e2e7ffc214c4ef7ac8520abb5327510f84affe90b1890d314c"),
							Right: utils.HexToFelt(t, "0x45ca67f381dcd01fec774743a4aaed6b36e1bda979185cf5dce538ad0007914"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x53f61d0cb8099e2e7ffc214c4ef7ac8520abb5327510f84affe90b1890d314c"),
						Node: &rpc.BinaryNode{
							Left:  utils.HexToFelt(t, "0x17d6fc8431c48e41222a3ede441d1e2d91c31eb67a8aa9c030c99c510e9f34c"),
							Right: utils.HexToFelt(t, "0x1cf95259ae39c038e87224fa5fdb7c7eeba6dd4263e05e80c9a8e27c3240f2c"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x1cf95259ae39c038e87224fa5fdb7c7eeba6dd4263e05e80c9a8e27c3240f2c"),
						Node: &rpc.EdgeNode{
							Path:   "0x56c9569dea5f18628f1ef7c15978ee3093d2d3eec3b893aac08004e678ead3",
							Length: 247,
							Child:  expectedContractLeaf,
						},
					},
				},
			},
			GlobalRoots: &rpc.GlobalRoots{
				BlockHash:         expectedBlockHash,
				ClassesTreeRoot:   expectedClsRoot,
				ContractsTreeRoot: expectedStgRoot,
			},
		}

		assert.Equal(t, expectedResult, *result)
	})
}

func arityTest(t *testing.T,
	proof *rpc.StorageProofResult,
	classesProofArity int,
	contractsProofNodesArity int,
	contractsProofLeavesArity int,
	contractStorageArity int,
) {
	require.Len(t, proof.ClassesProof, classesProofArity)
	require.Len(t, proof.ContractsStorageProofs, contractStorageArity)
	require.NotNil(t, proof.ContractsProof)
	require.Len(t, proof.ContractsProof.Nodes, contractsProofNodesArity)
	require.Len(t, proof.ContractsProof.LeavesData, contractsProofLeavesArity)
}

func verifyIf(
	t *testing.T,
	root, key, value *felt.Felt,
	proof []*rpc.HashToNode,
	hashF crypto.HashFn,
) {
	t.Helper()

	proofSet := trie.NewProofNodeSet()
	for _, hn := range proof {
		proofSet.Put(*hn.Hash, hn.Node.AsProofNode())
	}

	leaf, err := trie.VerifyProof(root, key, proofSet, hashF)
	require.NoError(t, err)

	// non-membership test
	if value == nil {
		value = felt.Zero.Clone()
	}
	require.Equal(t, leaf, value)
}

func emptyTrie(t *testing.T) *trie.Trie {
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)
	return tempTrie
}

func verifyGlobalStateRoot(t *testing.T, globalStateRoot, classRoot, storageRoot *felt.Felt) {
	stateVersion := new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	if classRoot.IsZero() {
		assert.Equal(t, globalStateRoot, storageRoot)
	} else {
		assert.Equal(t, globalStateRoot, crypto.PoseidonArray(stateVersion, storageRoot, classRoot))
	}
}
