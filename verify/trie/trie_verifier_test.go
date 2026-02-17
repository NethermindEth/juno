package trie

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrieVerifier_Run_ValidStateTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePedersen(txn, prefix, StarknetTrieHeight)
	require.NoError(t, err)

	key1 := felt.NewFromUint64[felt.Felt](1)
	value1 := felt.NewFromUint64[felt.Felt](100)
	_, err = testTrie.Put(key1, value1)
	require.NoError(t, err)

	key2 := felt.NewFromUint64[felt.Felt](2)
	value2 := felt.NewFromUint64[felt.Felt](200)
	_, err = testTrie.Put(key2, value2)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_ValidClassTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.ClassesTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePoseidon(txn, prefix, StarknetTrieHeight)
	require.NoError(t, err)

	key1 := felt.NewFromUint64[felt.Felt](10)
	value1 := felt.NewFromUint64[felt.Felt](1000)
	_, err = testTrie.Put(key1, value1)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ClassTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_CorruptedTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePedersen(txn, prefix, StarknetTrieHeight)
	require.NoError(t, err)

	key1 := felt.NewFromUint64[felt.Felt](1)
	value1 := felt.NewFromUint64[felt.Felt](100)
	_, err = testTrie.Put(key1, value1)
	require.NoError(t, err)

	key2 := felt.NewFromUint64[felt.Felt](2)
	value2 := felt.NewFromUint64[felt.Felt](200)
	_, err = testTrie.Put(key2, value2)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	var nodeKey trie.BitArray
	nodeKey.SetFelt(StarknetTrieHeight, key1)

	node, err := trieStorage.Get(&nodeKey)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Value)

	assert.True(t, node.Value.Equal(value1), "Expected value1 but got different value")

	corruptedValue := felt.NewFromUint64[felt.Felt](999999)
	node.Value = corruptedValue

	err = trieStorage.Put(&nodeKey, node)
	require.NoError(t, err)

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCorruptionDetected)
}

func TestTrieVerifier_Run_MultipleTrieTypes(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	statePrefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	stateTrie, err := trie.NewTriePedersen(txn, statePrefix, StarknetTrieHeight)
	require.NoError(t, err)

	key1 := felt.NewFromUint64[felt.Felt](1)
	value1 := felt.NewFromUint64[felt.Felt](100)
	_, err = stateTrie.Put(key1, value1)
	require.NoError(t, err)

	err = stateTrie.Commit()
	require.NoError(t, err)

	stateStorage := trie.NewStorage(txn, statePrefix)
	if stateTrie.RootKey() != nil {
		err = stateStorage.PutRootKey(stateTrie.RootKey())
		require.NoError(t, err)
	}

	classPrefix := db.ClassesTrie.Key()
	classTrie, err := trie.NewTriePoseidon(txn, classPrefix, StarknetTrieHeight)
	require.NoError(t, err)

	key2 := felt.NewFromUint64[felt.Felt](2)
	value2 := felt.NewFromUint64[felt.Felt](200)
	_, err = classTrie.Put(key2, value2)
	require.NoError(t, err)

	err = classTrie.Commit()
	require.NoError(t, err)

	classStorage := trie.NewStorage(txn, classPrefix)
	if classTrie.RootKey() != nil {
		err = classStorage.PutRootKey(classTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractTrie, ClassTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_EmptyTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()

	testTrie, err := trie.NewTriePedersen(txn, prefix, StarknetTrieHeight)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	// Empty trie has nil RootKey, so no need to store it

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_ValidContractStorageTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	txn := testDB.NewIndexedBatch()

	// Create a state trie with a contract address
	// The key at height 251 represents a contract address
	statePrefix := db.StateTrie.Key()
	stateTrie, err := trie.NewTriePedersen(txn, statePrefix, StarknetTrieHeight)
	require.NoError(t, err)

	contractAddr := felt.NewFromUint64[felt.Felt](0x1234)
	contractValue := felt.NewFromUint64[felt.Felt](1)
	_, err = stateTrie.Put(contractAddr, contractValue)
	require.NoError(t, err)

	err = stateTrie.Commit()
	require.NoError(t, err)

	stateStorage := trie.NewStorage(txn, statePrefix)
	if stateTrie.RootKey() != nil {
		err = stateStorage.PutRootKey(stateTrie.RootKey())
		require.NoError(t, err)
	}

	// Create a contract storage trie for this contract
	storagePrefix := db.ContractStorage.Key(contractAddr.Marshal())
	storageTrie, err := trie.NewTriePedersen(txn, storagePrefix, StarknetTrieHeight)
	require.NoError(t, err)

	storageKey := felt.NewFromUint64[felt.Felt](1)
	storageValue := felt.NewFromUint64[felt.Felt](100)
	_, err = storageTrie.Put(storageKey, storageValue)
	require.NoError(t, err)

	err = storageTrie.Commit()
	require.NoError(t, err)

	storageTrieStorage := trie.NewStorage(txn, storagePrefix)
	if storageTrie.RootKey() != nil {
		err = storageTrieStorage.PutRootKey(storageTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractStorageTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_ContractStorageWithFilter(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	txn := testDB.NewIndexedBatch()

	// Create state trie with two contracts
	statePrefix := db.StateTrie.Key()
	stateTrie, err := trie.NewTriePedersen(txn, statePrefix, StarknetTrieHeight)
	require.NoError(t, err)

	contractAddr1 := felt.NewFromUint64[felt.Felt](0x1111)
	contractAddr2 := felt.NewFromUint64[felt.Felt](0x2222)

	_, err = stateTrie.Put(contractAddr1, felt.NewFromUint64[felt.Felt](1))
	require.NoError(t, err)
	_, err = stateTrie.Put(contractAddr2, felt.NewFromUint64[felt.Felt](2))
	require.NoError(t, err)

	err = stateTrie.Commit()
	require.NoError(t, err)

	stateStorage := trie.NewStorage(txn, statePrefix)
	if stateTrie.RootKey() != nil {
		err = stateStorage.PutRootKey(stateTrie.RootKey())
		require.NoError(t, err)
	}

	// Create contract storage trie only for contract 1
	storagePrefix1 := db.ContractStorage.Key(contractAddr1.Marshal())
	storageTrie1, err := trie.NewTriePedersen(txn, storagePrefix1, StarknetTrieHeight)
	require.NoError(t, err)

	_, err = storageTrie1.Put(
		felt.NewFromUint64[felt.Felt](1),
		felt.NewFromUint64[felt.Felt](100),
	)
	require.NoError(t, err)

	err = storageTrie1.Commit()
	require.NoError(t, err)

	storage1 := trie.NewStorage(txn, storagePrefix1)
	if storageTrie1.RootKey() != nil {
		err = storage1.PutRootKey(storageTrie1.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	// Verify only contract 1 using the filter - should succeed
	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractStorageTrie}, contractAddr1)

	ctx := context.Background()
	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_ContextCancellation(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePedersen(txn, prefix, StarknetTrieHeight)
	require.NoError(t, err)

	key1 := felt.NewFromUint64[felt.Felt](1)
	value1 := felt.NewFromUint64[felt.Felt](100)
	_, err = testTrie.Put(key1, value1)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractTrie}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = verifier.Run(ctx)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_RootHashMismatch(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePedersen(txn, prefix, StarknetTrieHeight)
	require.NoError(t, err)

	key1 := felt.NewFromUint64[felt.Felt](1)
	value1 := felt.NewFromUint64[felt.Felt](100)
	_, err = testTrie.Put(key1, value1)
	require.NoError(t, err)

	key2 := felt.NewFromUint64[felt.Felt](2)
	value2 := felt.NewFromUint64[felt.Felt](200)
	_, err = testTrie.Put(key2, value2)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	// Get the root key and corrupt the stored root hash (not node values)
	rootKey := testTrie.RootKey()
	require.NotNil(t, rootKey)

	rootNode, err := trieStorage.Get(rootKey)
	require.NoError(t, err)
	require.NotNil(t, rootNode)

	// Corrupt the root node's value (stored hash) while keeping node structure intact
	// This will cause root hash mismatch, not node corruption during traversal
	corruptedHash := felt.NewFromUint64[felt.Felt](999999)
	rootNode.Value = corruptedHash

	err = trieStorage.Put(rootKey, rootNode)
	require.NoError(t, err)

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger, []TrieType{ContractTrie}, nil)

	ctx := context.Background()
	err = verifier.Run(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCorruptionDetected)
}
