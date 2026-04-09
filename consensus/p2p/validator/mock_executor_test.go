package validator

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

type mockExecutorState struct {
	nextTransactionIndex int
	buildResult          *builder.BuildResult
}

type mockExecutor struct {
	t *testing.T
	// Currently we don't have an ID to track the BuildState instances, so we temporarily use the height and the proposer address
	states map[types.Height]map[starknet.Address]*mockExecutorState
}

func NewMockExecutor(t *testing.T, network *utils.Network) *mockExecutor {
	return &mockExecutor{
		t:      t,
		states: make(map[types.Height]map[starknet.Address]*mockExecutorState),
	}
}

func (m *mockExecutor) RunTxns(state *builder.BuildState, txns []mempool.BroadcastedTransaction) error {
	executorState := m.getState(state)

	for i := range txns {
		require.Less(
			m.t,
			executorState.nextTransactionIndex,
			len(executorState.buildResult.PreConfirmed.Block.Transactions),
		)
		require.Equal(
			m.t,
			executorState.buildResult.
				PreConfirmed.Block.Transactions[executorState.nextTransactionIndex].Hash(),
			txns[i].Transaction.Hash(),
		)
		executorState.nextTransactionIndex++
	}

	return nil
}

func (m *mockExecutor) Finish(state *builder.BuildState) (blockchain.SimulateResult, error) {
	executorState := m.getState(state)

	require.Equal(
		m.t,
		executorState.nextTransactionIndex,
		len(executorState.buildResult.PreConfirmed.Block.Transactions),
	)
	*state.PreConfirmed = *executorState.buildResult.PreConfirmed
	state.L2GasConsumed = executorState.buildResult.L2GasConsumed
	return *executorState.buildResult.SimulateResult, nil
}

func (m *mockExecutor) RegisterBuildResult(buildResult *builder.BuildResult) {
	height := types.Height(buildResult.PreConfirmed.Block.Header.Number)
	proposer := starknet.Address(*buildResult.PreConfirmed.Block.Header.SequencerAddress)

	addressMap, ok := m.states[height]
	if !ok {
		addressMap = make(map[starknet.Address]*mockExecutorState)
		m.states[height] = addressMap
	}

	addressMap[proposer] = &mockExecutorState{
		nextTransactionIndex: 0,
		buildResult:          buildResult,
	}
}

func (m *mockExecutor) getState(state *builder.BuildState) *mockExecutorState {
	addressMap, ok := m.states[types.Height(state.PreConfirmed.Block.Header.Number)]
	require.True(m.t, ok)

	sequencerAddress := starknet.Address(*state.PreConfirmed.Block.Header.SequencerAddress)
	executorState, ok := addressMap[sequencerAddress]
	require.True(m.t, ok)

	return executorState
}
