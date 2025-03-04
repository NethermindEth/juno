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
	}
	for str, expected := range cases {
		quotedStr := `"` + str + `"`
		require.NoError(t, json.Unmarshal([]byte(quotedStr), fs))
		assert.Equal(t, expected, *fs)
	}

	require.ErrorContains(t, json.Unmarshal([]byte(`"ABC"`), fs), "unknown FinalityStatus")
}

func TestResourceMarshalText(t *testing.T) {
	tests := []struct {
		name     string
		resource starknet.Resource
		want     []byte
		err      bool
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
			name:     "error",
			resource: starknet.Resource(0),
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resource.MarshalText()
			if tt.err {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResourceUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want starknet.Resource
		err  bool
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
			name: "error",
			data: []byte("unknown"),
			err:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got starknet.Resource
			err := got.UnmarshalJSON(tt.data)
			if tt.err {
				require.Error(t, err)
				require.Zero(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
