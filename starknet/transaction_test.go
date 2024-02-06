package starknet_test

import (
	"testing"

	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalExecutionStatus(t *testing.T) {
	es := new(starknet.ExecutionStatus)
	require.NoError(t, es.UnmarshalJSON([]byte(`"SUCCEEDED"`)))
	assert.Equal(t, starknet.Succeeded, *es)

	require.NoError(t, es.UnmarshalJSON([]byte(`"REVERTED"`)))
	assert.Equal(t, starknet.Reverted, *es)

	require.ErrorContains(t, es.UnmarshalJSON([]byte("ABC")), "unknown ExecutionStatus")
}

func TestUnmarshalFinalityStatus(t *testing.T) {
	fs := new(starknet.FinalityStatus)
	require.NoError(t, fs.UnmarshalJSON([]byte(`"ACCEPTED_ON_L1"`)))
	assert.Equal(t, starknet.AcceptedOnL1, *fs)

	require.NoError(t, fs.UnmarshalJSON([]byte(`"ACCEPTED_ON_L2"`)))
	assert.Equal(t, starknet.AcceptedOnL2, *fs)

	require.ErrorContains(t, fs.UnmarshalJSON([]byte("ABC")), "unknown FinalityStatus")
}
