package starknet_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalExecutionStatus(t *testing.T) {
	es := new(starknet.ExecutionStatus)

	cases := map[string]starknet.ExecutionStatus{
		"SUCCEEDED": starknet.Succeeded,
		"REVERTED":  starknet.Reverted,
		"REJECTED":  starknet.Rejected,
	}
	for str, expected := range cases {
		quotedStr := `"` + str + `"`
		require.NoError(t, json.Unmarshal([]byte(quotedStr), es))
		assert.Equal(t, expected, *es)
	}

	require.ErrorContains(t, json.Unmarshal([]byte(`"ABC"`), es), "unknown ExecutionStatus")
}

func TestUnmarshalFinalityStatus(t *testing.T) {
	fs := new(starknet.FinalityStatus)

	cases := map[string]starknet.FinalityStatus{
		"ACCEPTED_ON_L2": starknet.AcceptedOnL2,
		"ACCEPTED_ON_L1": starknet.AcceptedOnL1,
		"NOT_RECEIVED":   starknet.NotReceived,
		"RECEIVED":       starknet.Received,
		"CANDIDATE":      starknet.Candidate,
		"PRE_CONFIRMED":  starknet.PreConfirmed,
	}
	for str, expected := range cases {
		quotedStr := `"` + str + `"`
		require.NoError(t, json.Unmarshal([]byte(quotedStr), fs))
		assert.Equal(t, expected, *fs)
	}

	require.ErrorContains(t, json.Unmarshal([]byte(`"ABC"`), fs), "unknown FinalityStatus")
}

func TestTransactionFailureReasonString(t *testing.T) {
	tests := []struct {
		name   string
		reason starknet.TransactionFailureReason
		want   string
	}{
		{
			name:   "both empty",
			reason: starknet.TransactionFailureReason{},
			want:   "",
		},
		{
			name: "code and message",
			reason: starknet.TransactionFailureReason{
				Code:    "ERROR_CODE",
				Message: "something went wrong",
			},
			want: "ERROR_CODE: something went wrong",
		},
		{
			name: "only code",
			reason: starknet.TransactionFailureReason{
				Code: "ERROR_CODE",
			},
			want: "ERROR_CODE: ",
		},
		{
			name: "only message",
			reason: starknet.TransactionFailureReason{
				Message: "something went wrong",
			},
			want: ": something went wrong",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.reason.String())
		})
	}

	t.Run("should be omitted in TransactionStatus if empty", func(t *testing.T) {
		var status starknet.TransactionStatus
		raw, err := json.Marshal(status)
		require.NoError(t, err)

		var tempMap map[string]json.RawMessage
		err = json.Unmarshal(raw, &tempMap)
		require.NoError(t, err)

		_, ok := tempMap["tx_failure_reason"]
		assert.False(t, ok)
	})
}

func TestResourceMarshalText(t *testing.T) {
	tests := []struct {
		name        string
		resource    starknet.Resource
		want        []byte
		expectedErr string
	}{
		{
			name:     "l1 gas",
			resource: starknet.ResourceL1Gas,
			want:     []byte("L1_GAS"),
		},
		{
			name:     "l2 gas",
			resource: starknet.ResourceL2Gas,
			want:     []byte("L2_GAS"),
		},
		{
			name:     "l1 data gas",
			resource: starknet.ResourceL1DataGas,
			want:     []byte("L1_DATA_GAS"),
		},
		{
			name:        "error",
			resource:    starknet.Resource(0),
			expectedErr: "unknown resource",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resource.MarshalText()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResourceUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		want        starknet.Resource
		expectedErr string
	}{
		{
			name: "l1 gas",
			data: []byte("L1_GAS"),
			want: starknet.ResourceL1Gas,
		},
		{
			name: "l2 gas",
			data: []byte("L2_GAS"),
			want: starknet.ResourceL2Gas,
		},
		{
			name: "l1 data gas",
			data: []byte("L1_DATA_GAS"),
			want: starknet.ResourceL1DataGas,
		},
		{
			name:        "error",
			data:        []byte("unknown"),
			expectedErr: "unknown resource",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got starknet.Resource
			err := got.UnmarshalJSON(tt.data)
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Zero(t, got)
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
