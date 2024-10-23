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
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(nil, errors.New("non-existent contract"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	t.Run("non-existent key", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(&felt.Zero, errors.New("non-existent key"))

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, storage)
		assert.Equal(t, rpc.ErrContractNotFound, rpcErr)
	})

	expectedStorage := new(felt.Felt).SetUint64(1)

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Latest: true})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockHash(&felt.Zero).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Hash: &felt.Zero})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().StateAtBlockNumber(uint64(0)).Return(mockState, nopCloser, nil)
		mockState.EXPECT().ContractStorage(gomock.Any(), gomock.Any()).Return(expectedStorage, nil)

		storage, rpcErr := handler.StorageAt(felt.Zero, felt.Zero, rpc.BlockID{Number: 0})
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedStorage, storage)
	})
}

func TestStorageProof(t *testing.T) {
	// dummy values
	var (
		blkHash     = utils.HexToFelt(t, "0x11ead")
		clsRoot     = utils.HexToFelt(t, "0xc1a55")
		stgRoot     = utils.HexToFelt(t, "0xc0ffee")
		key         = new(felt.Felt).SetUint64(1)
		noSuchKey   = new(felt.Felt).SetUint64(0)
		value       = new(felt.Felt).SetUint64(51)
		blockLatest = rpc.BlockID{Latest: true}
		blockNumber = uint64(1313)
		nopCloser   = func() error {
			return nil
		}
	)

	tempTrie := emptyTrie(t)

	_, err := tempTrie.Put(key, value)
	require.NoError(t, err)
	_, err = tempTrie.Put(new(felt.Felt).SetUint64(8), new(felt.Felt).SetUint64(59))
	require.NoError(t, err)
	require.NoError(t, tempTrie.Commit())

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockTrie := mocks.NewMockTrieReader(mockCtrl)

	mockReader.EXPECT().HeadState().Return(mockState, func() error {
		return nil
	}, nil).AnyTimes()
	mockReader.EXPECT().HeadTrie().Return(mockTrie, func() error { return nil }, nil).AnyTimes()
	mockReader.EXPECT().Head().Return(&core.Block{Header: &core.Header{Hash: blkHash, Number: blockNumber}}, nil).AnyTimes()
	mockTrie.EXPECT().StateAndClassRoot().Return(stgRoot, clsRoot, nil).AnyTimes()
	mockTrie.EXPECT().ClassTrie().Return(tempTrie, nopCloser, nil).AnyTimes()
	mockTrie.EXPECT().StorageTrie().Return(tempTrie, nopCloser, nil).AnyTimes()

	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, nil, nil, "", log)

	verifyIf := func(proof []*rpc.HashToNode, key *felt.Felt, value *felt.Felt) {
		root, err := tempTrie.Root()
		require.NoError(t, err)

		pnodes := []trie.ProofNode{}
		for _, hn := range proof {
			pnodes = append(pnodes, hn.Node.AsProofNode())
		}

		kbs := key.Bytes()
		kkey := trie.NewKey(251, kbs[:])
		require.True(t, trie.VerifyProof(root, &kkey, value, pnodes, tempTrie.HashFunc()))
	}

	t.Run("Trie proofs sanity check", func(t *testing.T) {
		kbs := key.Bytes()
		kKey := trie.NewKey(251, kbs[:])
		proof, err := trie.GetProof(&kKey, tempTrie)
		require.NoError(t, err)
		root, err := tempTrie.Root()
		require.NoError(t, err)
		require.True(t, trie.VerifyProof(root, &kKey, value, proof, tempTrie.HashFunc()))

		// non-membership test
		kbs = noSuchKey.Bytes()
		kKey = trie.NewKey(251, kbs[:])
		proof, err = trie.GetProof(&kKey, tempTrie)
		require.NoError(t, err)
		require.True(t, trie.VerifyProof(root, &kKey, nil, proof, tempTrie.HashFunc()))
	})
	t.Run("global roots are filled", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, nil)
		require.Nil(t, rpcErr)

		require.NotNil(t, proof)
		require.NotNil(t, proof.GlobalRoots)
		require.Equal(t, blkHash, proof.GlobalRoots.BlockHash)
		require.Equal(t, clsRoot, proof.GlobalRoots.ClassesTreeRoot)
		require.Equal(t, stgRoot, proof.GlobalRoots.ContractsTreeRoot)
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
		verifyIf(proof.ClassesProof, noSuchKey, nil)
	})
	t.Run("class trie hash exists in a trie", func(t *testing.T) {
		proof, rpcErr := handler.StorageProof(blockLatest, []felt.Felt{*key}, nil, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 3, 0, 0, 0)
		verifyIf(proof.ClassesProof, key, value)
	})
	t.Run("storage trie address does not exist in a trie", func(t *testing.T) {
		mockState.EXPECT().ContractNonce(noSuchKey).Return(nil, db.ErrKeyNotFound).Times(1)
		mockState.EXPECT().ContractClassHash(noSuchKey).Return(nil, db.ErrKeyNotFound).Times(0)

		proof, rpcErr := handler.StorageProof(blockLatest, nil, []felt.Felt{*noSuchKey}, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, proof)
		arityTest(t, proof, 0, 3, 1, 0)
		require.Nil(t, proof.ContractsProof.LeavesData[0])

		verifyIf(proof.ContractsProof.Nodes, noSuchKey, nil)
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

		verifyIf(proof.ContractsProof.Nodes, key, value)
	})
	t.Run("contract storage trie address does not exist in a trie", func(t *testing.T) {
		contract := utils.HexToFelt(t, "0xdead")
		mockTrie.EXPECT().StorageTrieForAddr(contract).Return(emptyTrie(t), nil).Times(1)

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
		mockTrie.EXPECT().StorageTrieForAddr(gomock.Any()).Return(tempTrie, nil).Times(1)

		storageKeys := []rpc.StorageKeys{{Contract: *contract, Keys: []felt.Felt{*noSuchKey}}}
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, storageKeys)
		require.NotNil(t, proof)
		require.Nil(t, rpcErr)
		arityTest(t, proof, 0, 0, 0, 1)
		require.Len(t, proof.ContractsStorageProofs[0], 3)

		verifyIf(proof.ContractsStorageProofs[0], noSuchKey, nil)
	})
	//nolint:dupl
	t.Run("contract storage trie address/key exists in a trie", func(t *testing.T) {
		contract := utils.HexToFelt(t, "0xabcd")
		mockTrie.EXPECT().StorageTrieForAddr(gomock.Any()).Return(tempTrie, nil).Times(1)

		storageKeys := []rpc.StorageKeys{{Contract: *contract, Keys: []felt.Felt{*key}}}
		proof, rpcErr := handler.StorageProof(blockLatest, nil, nil, storageKeys)
		require.NotNil(t, proof)
		require.Nil(t, rpcErr)
		arityTest(t, proof, 0, 0, 0, 1)
		require.Len(t, proof.ContractsStorageProofs[0], 3)

		verifyIf(proof.ContractsStorageProofs[0], key, value)
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

func emptyTrie(t *testing.T) *trie.Trie {
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)
	return tempTrie
}

func TestStorageRoots(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	log := utils.NewNopZapLogger()
	testDB := pebble.NewMemTest(t)
	bc := blockchain.New(testDB, &utils.Mainnet)
	synchronizer := sync.New(bc, gw, log, time.Duration(0), false)
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
		reader, closer, err := bc.HeadTrie()
		assert.NoError(t, err)
		defer func() { _ = closer() }()

		stgRoot, clsRoot, err := reader.StateAndClassRoot()
		assert.NoError(t, err)

		assert.Equal(t, expectedClsRoot, clsRoot, clsRoot.String())
		assert.Equal(t, expectedStgRoot, stgRoot, stgRoot.String())

		verifyGlobalStateRoot(t, expectedGlobalRoot, clsRoot, stgRoot)
	})

	t.Run("check requested contract and storage slot exists", func(t *testing.T) {
		trieReader, closer, err := bc.HeadTrie()
		assert.NoError(t, err)
		defer func() { _ = closer() }()

		sTrie, sCloser, err := trieReader.StorageTrie()
		assert.NoError(t, err)
		defer func() { _ = sCloser() }()

		leaf, err := sTrie.Get(expectedContractAddress)
		assert.NoError(t, err)
		assert.Equal(t, leaf, expectedContractLeaf, leaf.String())

		stateReader, stCloser, err := bc.HeadState()
		assert.NoError(t, err)
		defer func() { _ = stCloser() }()

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
						Nonce:     utils.HexToFelt(t, "0x0"),
						ClassHash: utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"),
					},
				},
				Nodes: []*rpc.HashToNode{
					{
						Hash: utils.HexToFelt(t, "0x3ceee867d50b5926bb88c0ec7e0b9c20ae6b537e74aac44b8fcf6bb6da138d9"),
						Node: &rpc.MerkleBinaryNode{
							Left:  utils.HexToFelt(t, "0x4e1f289e55ac8a821fd463478e6f5543256beb934a871be91d00a0d3f2e7964"),
							Right: utils.HexToFelt(t, "0x67d9833b51e7bf1cab0e71e68477bf7f0b704391d753f9d793008e4f6587c53"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x4e1f289e55ac8a821fd463478e6f5543256beb934a871be91d00a0d3f2e7964"),
						Node: &rpc.MerkleBinaryNode{
							Left:  utils.HexToFelt(t, "0x1ef87d62309ff1cad58d39e8f5480f9caa9acd78a43f139d87220a1babe38a4"),
							Right: utils.HexToFelt(t, "0x9a258d24b3aeb7e263e910d68a18d85305703a2f20df2e806ecbb1fb28760f"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x9a258d24b3aeb7e263e910d68a18d85305703a2f20df2e806ecbb1fb28760f"),
						Node: &rpc.MerkleBinaryNode{
							Left:  utils.HexToFelt(t, "0x53f61d0cb8099e2e7ffc214c4ef7ac8520abb5327510f84affe90b1890d314c"),
							Right: utils.HexToFelt(t, "0x45ca67f381dcd01fec774743a4aaed6b36e1bda979185cf5dce538ad0007914"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x53f61d0cb8099e2e7ffc214c4ef7ac8520abb5327510f84affe90b1890d314c"),
						Node: &rpc.MerkleBinaryNode{
							Left:  utils.HexToFelt(t, "0x17d6fc8431c48e41222a3ede441d1e2d91c31eb67a8aa9c030c99c510e9f34c"),
							Right: utils.HexToFelt(t, "0x1cf95259ae39c038e87224fa5fdb7c7eeba6dd4263e05e80c9a8e27c3240f2c"),
						},
					},
					{
						Hash: utils.HexToFelt(t, "0x1cf95259ae39c038e87224fa5fdb7c7eeba6dd4263e05e80c9a8e27c3240f2c"),
						Node: &rpc.MerkleEdgeNode{
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

func verifyGlobalStateRoot(t *testing.T, globalStateRoot, classRoot, storageRoot *felt.Felt) {
	stateVersion := new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	if classRoot.IsZero() {
		assert.Equal(t, globalStateRoot, storageRoot)
	} else {
		assert.Equal(t, globalStateRoot, crypto.PoseidonArray(stateVersion, storageRoot, classRoot))
	}
}
