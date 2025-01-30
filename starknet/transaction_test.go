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
		resource starknet.Resource
		text     string
		hasError bool
	}{
		{starknet.ResourceL1Gas, "L1_GAS", false},
		{starknet.ResourceL2Gas, "L2_GAS", false},
		{starknet.ResourceL1DataGas, "L1_DATA_GAS", false},
		{starknet.Resource(999), "error", true},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			b, err := test.resource.MarshalText()
			if test.hasError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.text, string(b))
		})
	}
}

func TestResourceUnmarshalText(t *testing.T) {
	tests := []struct {
		resource starknet.Resource
		text     string
		hasError bool
	}{
		{starknet.ResourceL1Gas, "L1_GAS", false},
		{starknet.ResourceL2Gas, "L2_GAS", false},
		{starknet.ResourceL1DataGas, "L1_DATA_GAS", false},
		{starknet.Resource(999), "error", true},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			var r starknet.Resource
			err := r.UnmarshalText([]byte(test.text))
			if test.hasError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.resource, r)
		})
	}
}
