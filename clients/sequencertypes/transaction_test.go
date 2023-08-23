package sequencertypes_test

import (
	"testing"

	"github.com/NethermindEth/juno/clients/sequencertypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalExecutionStatus(t *testing.T) {
	es := new(sequencertypes.ExecutionStatus)
	require.NoError(t, es.UnmarshalJSON([]byte(`"SUCCEEDED"`)))
	assert.Equal(t, sequencertypes.Succeeded, *es)

	require.NoError(t, es.UnmarshalJSON([]byte(`"REVERTED"`)))
	assert.Equal(t, sequencertypes.Reverted, *es)

	require.ErrorContains(t, es.UnmarshalJSON([]byte("ABC")), "unknown ExecutionStatus")
}

func TestUnmarshalFinalityStatus(t *testing.T) {
	fs := new(sequencertypes.FinalityStatus)
	require.NoError(t, fs.UnmarshalJSON([]byte(`"ACCEPTED_ON_L1"`)))
	assert.Equal(t, sequencertypes.AcceptedOnL1, *fs)

	require.NoError(t, fs.UnmarshalJSON([]byte(`"ACCEPTED_ON_L2"`)))
	assert.Equal(t, sequencertypes.AcceptedOnL2, *fs)

	require.ErrorContains(t, fs.UnmarshalJSON([]byte("ABC")), "unknown FinalityStatus")
}
