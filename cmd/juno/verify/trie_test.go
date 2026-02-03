package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/pebblev2"
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

	testTrie, err := trie.NewTriePedersen(txn, prefix, starknetTrieHeight)
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

	rootHash, err := testTrie.Hash()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger)
	cfg := &TrieConfig{
		Tries: []TrieType{ContractTrieType},
	}

	ctx := context.Background()
	err = verifier.Run(ctx, cfg)
	assert.NoError(t, err)

	reader, err := trie.NewTrieReaderPedersen(testDB, prefix, starknetTrieHeight)
	require.NoError(t, err)
	storedHash, err := reader.Hash()
	require.NoError(t, err)
	assert.True(t, rootHash.Equal(&storedHash))
}

func TestTrieVerifier_Run_ValidClassTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.ClassesTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePoseidon(txn, prefix, starknetTrieHeight)
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

	verifier := NewTrieVerifier(testDB, logger)
	cfg := &TrieConfig{
		Tries: []TrieType{ClassTrieType},
	}

	ctx := context.Background()
	err = verifier.Run(ctx, cfg)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_CorruptedTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePedersen(txn, prefix, starknetTrieHeight)
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
	nodeKey.SetFelt(starknetTrieHeight, key1)

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

	verifier := NewTrieVerifier(testDB, logger)
	cfg := &TrieConfig{
		Tries: []TrieType{ContractTrieType},
	}

	ctx := context.Background()
	err = verifier.Run(ctx, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node corruption detected")
}

func TestTrieVerifier_Run_MultipleTrieTypes(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	statePrefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	stateTrie, err := trie.NewTriePedersen(txn, statePrefix, starknetTrieHeight)
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
	classTrie, err := trie.NewTriePoseidon(txn, classPrefix, starknetTrieHeight)
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

	verifier := NewTrieVerifier(testDB, logger)
	cfg := &TrieConfig{
		Tries: []TrieType{ContractTrieType, ClassTrieType},
	}

	ctx := context.Background()
	err = verifier.Run(ctx, cfg)
	assert.NoError(t, err)
}

func TestTrieVerifier_Run_EmptyTrie(t *testing.T) {
	logger := utils.NewNopZapLogger()
	testDB := memory.New()
	defer testDB.Close()

	prefix := db.StateTrie.Key()
	txn := testDB.NewIndexedBatch()
	trieStorage := trie.NewStorage(txn, prefix)

	testTrie, err := trie.NewTriePedersen(txn, prefix, starknetTrieHeight)
	require.NoError(t, err)

	err = testTrie.Commit()
	require.NoError(t, err)

	if testTrie.RootKey() != nil {
		err = trieStorage.PutRootKey(testTrie.RootKey())
		require.NoError(t, err)
	}

	err = txn.Write()
	require.NoError(t, err)

	verifier := NewTrieVerifier(testDB, logger)
	cfg := &TrieConfig{
		Tries: []TrieType{ContractTrieType},
	}

	ctx := context.Background()
	err = verifier.Run(ctx, cfg)
	assert.NoError(t, err)
}

func TestRunTrieVerify_AddressFlagValidation(t *testing.T) {
	tests := []struct {
		name           string
		trieTypes      []string
		address        string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:        "address with contract-storage type should succeed",
			trieTypes:   []string{"contract-storage"},
			address:     "0x123",
			expectError: false,
		},
		{
			name:           "address with contract and class types should fail",
			trieTypes:      []string{"contract", "class"},
			address:        "0x123",
			expectError:    true,
			expectedErrMsg: "--address flag can only be used with --type contract-storage",
		},
		{
			name:        "address with no type specified should succeed (default includes contract-storage)",
			trieTypes:   []string{},
			address:     "0x123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "test.db")

			testDB, err := pebblev2.New(dbPath)
			require.NoError(t, err)
			testDB.Close()

			parentCmd := VerifyCmd("")
			args := []string{"--db-path", dbPath, "trie"}

			for _, trieType := range tt.trieTypes {
				args = append(args, "--type", trieType)
			}

			if tt.address != "" {
				args = append(args, "--address", tt.address)
			}

			parentCmd.SetArgs(args)
			parentCmd.SetOut(os.Stderr)
			parentCmd.SetErr(os.Stderr)

			err = parentCmd.ExecuteContext(context.Background())

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else if err != nil {
				addrFlagErr := "--address flag can only be used with --type contract-storage"
				assert.NotContains(t, err.Error(), addrFlagErr)
			}
		})
	}
}
