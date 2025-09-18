package state

import (
	"testing"

	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateContract_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		contract *stateContract
	}{
		{
			name: "Full contract",
			contract: &stateContract{
				Nonce:          newFelt(123),
				ClassHash:      newFelt(456),
				StorageRoot:    newFelt(789),
				DeployedHeight: 1000,
			},
		},
		{
			name: "Empty root contract",
			contract: &stateContract{
				Nonce:          newFelt(123),
				ClassHash:      newFelt(456),
				StorageRoot:    felt.Zero,
				DeployedHeight: 1000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := tt.contract.MarshalBinary()
			require.NoError(t, err)

			// Verify length
			if tt.contract.StorageRoot.IsZero() {
				assert.Equal(t, contractEmptyRootEncSize, len(data))
			} else {
				assert.Equal(t, contractEncSize, len(data))
			}

			// Unmarshal into new contract
			newContract := &stateContract{}
			err = newContract.UnmarshalBinary(data)
			require.NoError(t, err)

			// Verify fields match
			assert.Equal(t, tt.contract.Nonce, newContract.Nonce)
			assert.Equal(t, tt.contract.ClassHash, newContract.ClassHash)
			assert.Equal(t, tt.contract.StorageRoot, newContract.StorageRoot)
			assert.Equal(t, tt.contract.DeployedHeight, newContract.DeployedHeight)
		})
	}
}

func TestStateContract_UnmarshalInvalidLength(t *testing.T) {
	contract := &stateContract{}
	err := contract.UnmarshalBinary([]byte{1, 2, 3}) // Invalid length
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid length for state contract")
}

func newFelt(n int) felt.Felt {
	var f felt.Felt
	f.SetUint64(uint64(n))
	return f
}
