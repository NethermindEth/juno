package driver_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func TestOnCommit_DropsProposalsAtCommittedHeight(t *testing.T) {
	proposalStore := &proposal.ProposalStore[starknet.Hash]{}
	listener := driver.NewCommitListener[starknet.Value](
		log.NewNopZapLogger(),
		proposalStore,
	)

	committedHeight := types.Height(42)

	committedValue := felt.FromUint64[starknet.Value](1)
	losingValue := felt.FromUint64[starknet.Value](2)
	nextHeightValue := felt.FromUint64[starknet.Value](3)

	proposalStore.Store(committedValue.Hash(), buildResultAtHeight(uint64(committedHeight)))
	proposalStore.Store(losingValue.Hash(), buildResultAtHeight(uint64(committedHeight)))
	proposalStore.Store(nextHeightValue.Hash(), buildResultAtHeight(uint64(committedHeight)+1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		listener.OnCommit(ctx, committedHeight, committedValue)
		close(done)
	}()

	block := <-listener.Listen()
	block.Persisted <- nil
	<-done

	require.Nil(t, proposalStore.Get(committedValue.Hash()))
	require.Nil(t, proposalStore.Get(losingValue.Hash()))
	require.NotNil(t, proposalStore.Get(nextHeightValue.Hash()))
}

type testCommitHook struct {
	called chan struct{}
}

func (h *testCommitHook) OnCommit(context.Context, types.Height, starknet.Value) {
	h.called <- struct{}{}
}

func TestCommitListenerOnCommitReturnsFalseWhenPersistenceFails(t *testing.T) {
	committedHeight := types.Height(1)
	value := felt.FromUint64[starknet.Value](1)
	store := &proposal.ProposalStore[starknet.Hash]{}
	store.Store(value.Hash(), buildResultAtHeight(uint64(committedHeight)))

	hook := &testCommitHook{called: make(chan struct{}, 1)}
	listener := driver.NewCommitListener(
		log.NewNopZapLogger(),
		store,
		hook,
	)

	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- listener.OnCommit(context.Background(), committedHeight, value)
	}()

	committedBlock := <-listener.Listen()
	committedBlock.Persisted <- errors.New("store failed")

	require.False(t, <-resultCh)

	select {
	case <-hook.called:
		t.Fatal("commit hook should not run when persistence fails")
	default:
	}
}

func buildResultAtHeight(blockNumber uint64) *builder.BuildResult {
	return &builder.BuildResult{
		PreConfirmed: &pending.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: blockNumber,
				},
			},
			StateUpdate: &core.StateUpdate{},
		},
		SimulateResult: &blockchain.SimulateResult{
			BlockCommitments: &core.BlockCommitments{},
		},
	}
}
