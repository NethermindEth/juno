package rpc_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
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
		stgRoor     = utils.HexToFelt(t, "0xc0ffee")
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
	mockTrie.EXPECT().StateAndClassRoot().Return(stgRoor, clsRoot, nil).AnyTimes()
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
		require.Equal(t, stgRoor, proof.GlobalRoots.ContractsTreeRoot)
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
