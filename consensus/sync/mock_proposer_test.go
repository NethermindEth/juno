package sync_test

import (
	"context"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
)

// Driver only calls `OnCommit`, so we don't have to mock the other methods
type mockProposer struct{}

func newMockProposer() *mockProposer {
	return &mockProposer{}
}

func (m *mockProposer) Run(ctx context.Context) error {
	return nil
}

func (m *mockProposer) OnCommit(ctx context.Context, height types.Height, value starknet.Value) {
}

func (m *mockProposer) Value() starknet.Value {
	return starknet.Value(felt.Zero)
}

func (m *mockProposer) Valid(value starknet.Value) bool {
	return true
}

func (m *mockProposer) Submit(ctx context.Context, transactions []mempool.BroadcastedTransaction) {
}

func (m *mockProposer) Push(ctx context.Context, transaction *mempool.BroadcastedTransaction) error {
	return nil
}

func (m *mockProposer) Preconfirmed() *core.PreConfirmed {
	return nil
}
