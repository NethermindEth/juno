package driver_test

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/stretchr/testify/require"
)

type mockBlockchain struct {
	t              *testing.T
	expectedCommit *starknet.Commit
}

func (m *mockBlockchain) Commit(height types.Height, value starknet.Value) {
	require.Equal(m.t, m.expectedCommit.Value, &value)
	require.Equal(m.t, m.expectedCommit.Height, height)
}

func newMockBlockchain(t *testing.T, expectedCommit *starknet.Commit) blockchain {
	return &mockBlockchain{
		t:              t,
		expectedCommit: expectedCommit,
	}
}
