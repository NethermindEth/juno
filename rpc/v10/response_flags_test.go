package rpcv10_test

import (
	"encoding/json"
	"testing"

	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/stretchr/testify/require"
)

func TestResponseFlags_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		json          string
		expected      rpcv10.ResponseFlags
		expectedError string
	}{
		{
			name:     "empty array",
			json:     `[]`,
			expected: rpcv10.ResponseFlags{IncludeProofFacts: false},
		},
		{
			name:     "array with INCLUDE_PROOF_FACTS",
			json:     `["INCLUDE_PROOF_FACTS"]`,
			expected: rpcv10.ResponseFlags{IncludeProofFacts: true},
		},
		{
			name:          "array with unknown flag and valid flag",
			json:          `["INCLUDE_PROOF_FACTS", "UNKNOWN_FLAG"]`,
			expectedError: "unknown flag: UNKNOWN_FLAG",
		},
		{
			name:          "case sensitive",
			json:          `["include_proof_facts"]`,
			expectedError: "unknown flag: include_proof_facts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flags rpcv10.ResponseFlags
			err := json.Unmarshal([]byte(tt.json), &flags)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, flags)
		})
	}
}
