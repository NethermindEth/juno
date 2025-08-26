package driver_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/require"
)

type mockCommitListener struct {
	t              *testing.T
	expectedCommit *starknet.Commit
}

func (m *mockCommitListener) Commit(
	ctx context.Context,
	height types.Height,
	value starknet.Value,
) {
	require.Equal(m.t, m.expectedCommit.Value, &value)
	require.Equal(m.t, m.expectedCommit.Height, height)
}

func newMockCommitListener(t *testing.T, expectedCommit *starknet.Commit) commitListener {
	return &mockCommitListener{
		t:              t,
		expectedCommit: expectedCommit,
	}
}

func (m *mockCommitListener) Listen() <-chan sync.CommittedBlock {
	return nil
}
