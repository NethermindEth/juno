package verify

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
