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
