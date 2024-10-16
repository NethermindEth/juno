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

	require.NoError(t, json.Unmarshal([]byte(`"SUCCEEDED"`), es))
	assert.Equal(t, starknet.Succeeded, *es)

	require.NoError(t, json.Unmarshal([]byte(`"REVERTED"`), es))
	assert.Equal(t, starknet.Reverted, *es)

	require.ErrorContains(t, json.Unmarshal([]byte(`"ABC"`), es), "unknown ExecutionStatus")
}

func TestUnmarshalFinalityStatus(t *testing.T) {
	fs := new(starknet.FinalityStatus)
	require.NoError(t, json.Unmarshal([]byte(`"ACCEPTED_ON_L1"`), fs))
	assert.Equal(t, starknet.AcceptedOnL1, *fs)

	require.NoError(t, json.Unmarshal([]byte(`"ACCEPTED_ON_L2"`), fs))
	assert.Equal(t, starknet.AcceptedOnL2, *fs)

	require.ErrorContains(t, json.Unmarshal([]byte(`"ABC"`), fs), "unknown FinalityStatus")
}
