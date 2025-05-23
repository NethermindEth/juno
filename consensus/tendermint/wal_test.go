package tendermint

import (
	"testing"

	db "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/mock/gomock"
)

func getPrevote(idx int) starknet.Prevote {
	return starknet.Prevote{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(0),
			Round:  types.Round(0),
			Sender: *getVal(idx),
		},
		ID: utils.HeapPtr(hash.Hash(felt.FromUint64(1))),
	}
}

func getPrecommit(idx int) starknet.Precommit {
	return starknet.Precommit{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(0),
			Round:  types.Round(0),
			Sender: *getVal(idx),
		},
		ID: utils.HeapPtr(hash.Hash(felt.FromUint64(1))),
	}
}

func TestReplayWAL(t *testing.T) {
	proposerAddr := getVal(0)
	nonProposerAddr := getVal(3)
	proposedValue := starknet.Value(1)
	app, chain, vals := newApp(), newChain(), newVals()
	for i := range 4 {
		vals.addValidator(*getVal(i))
	}

	ctrl := gomock.NewController(t)
	proposalMessage := starknet.Proposal{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(0),
			Round:  types.Round(0),
			Sender: *proposerAddr,
		},
		ValidRound: types.Round(-1),
		Value:      &proposedValue,
	}

	t.Run("ReplayWAL: replay on empty db", func(t *testing.T) {
		mockDB := mocks.NewMockTendermintDB[starknet.Value, hash.Hash, address.Address](ctrl)
		stateMachine := New(mockDB, utils.NewNopZapLogger(), *getVal(0), app, chain, vals).(*testStateMachine)
		emptyList := []db.WalEntry[starknet.Value, hash.Hash, address.Address]{}
		mockDB.EXPECT().GetWALEntries(types.Height(0)).Return(emptyList, nil)
		stateMachine.ReplayWAL() // ReplayWAL will panic if anything goes wrong
	})

	t.Run("ReplayWAL: proposer crashes right after proposing", func(t *testing.T) {
		mockDB := mocks.NewMockTendermintDB[starknet.Value, hash.Hash, address.Address](ctrl)
		sMachine := New(mockDB, utils.NewNopZapLogger(), *proposerAddr, app, chain, vals).(*testStateMachine)

		// Start, Propose a block, Progress to Prevote step, assert state
		mockDB.EXPECT().Flush().Return(nil).Times(2)
		mockDB.EXPECT().SetWALEntry(proposalMessage).Return(nil)
		sMachine.ProcessStart(0)
		assertState(t, sMachine, types.Height(0), types.Round(0), types.StepPrevote)

		// Crash the node, replay wal, and assert we get to the expected state
		sMachineRecoverd := New(mockDB, utils.NewNopZapLogger(), *proposerAddr, app, chain, vals).(*testStateMachine)
		assertState(t, sMachineRecoverd, types.Height(0), types.Round(0), types.StepPropose)
		walEntries := []db.WalEntry[starknet.Value, hash.Hash, address.Address]{{Entry: proposalMessage, Type: types.MessageTypeProposal}}
		mockDB.EXPECT().GetWALEntries(types.Height(0)).Return(walEntries, nil)
		sMachineRecoverd.ReplayWAL() // Should not panic
		assertState(t, sMachine, types.Height(0), types.Round(0), types.StepPrevote)
	})

	t.Run("ReplayWAL: non proposer crashes right before commit", func(t *testing.T) {
		// Setup
		mockDB := mocks.NewMockTendermintDB[starknet.Value, hash.Hash, address.Address](ctrl)
		sMachine := New(mockDB, utils.NewNopZapLogger(), *nonProposerAddr, app, chain, vals).(*testStateMachine)

		prevote0 := getPrevote(0)
		prevote1 := getPrevote(1)
		prevote2 := getPrevote(2)
		precommit0 := getPrecommit(0)
		precommit1 := getPrecommit(1)

		// Start
		mockDB.EXPECT().Flush().Return(nil)
		sMachine.ProcessStart(0)

		// Receive proposal
		mockDB.EXPECT().SetWALEntry(proposalMessage).Return(nil)
		sMachine.ProcessProposal(proposalMessage)

		// Receive quorum of prevotes
		mockDB.EXPECT().SetWALEntry(prevote0).Return(nil)
		mockDB.EXPECT().SetWALEntry(prevote1).Return(nil)
		mockDB.EXPECT().SetWALEntry(prevote2).Return(nil)
		sMachine.ProcessPrevote(prevote0)
		sMachine.ProcessPrevote(prevote1)
		sMachine.ProcessPrevote(prevote2)

		// Receive precommit
		mockDB.EXPECT().SetWALEntry(precommit0).Return(nil)
		sMachine.ProcessPrecommit(precommit0)

		assertState(t, sMachine, types.Height(0), types.Round(0), types.StepPrecommit)

		// Crash, recover, and assert we arrive at previous state
		sMachineRecoverd := New(mockDB, utils.NewNopZapLogger(), *nonProposerAddr, app, chain, vals).(*testStateMachine)
		assertState(t, sMachineRecoverd, types.Height(0), types.Round(0), types.StepPropose)
		walEntries := []db.WalEntry[starknet.Value, hash.Hash, address.Address]{
			{Entry: proposalMessage, Type: types.MessageTypeProposal},
			{Entry: prevote0, Type: types.MessageTypePrevote},
			{Entry: prevote1, Type: types.MessageTypePrevote},
			{Entry: prevote2, Type: types.MessageTypePrevote},
			{Entry: precommit0, Type: types.MessageTypePrecommit},
		}
		mockDB.EXPECT().GetWALEntries(types.Height(0)).Return(walEntries, nil)
		sMachineRecoverd.ReplayWAL()
		assertState(t, sMachineRecoverd, types.Height(0), types.Round(0), types.StepPrecommit)

		// Start
		mockDB.EXPECT().Flush().Return(nil).Times(1)
		sMachineRecoverd.ProcessStart(0)

		// Todo: why do we only need two precommits for quorum.....
		// Receive final precommit, now we reach quorum
		mockDB.EXPECT().SetWALEntry(precommit1).Return(nil)
		mockDB.EXPECT().Flush().Return(nil).Times(2) // Once in doCommitValue, once in t.startRound(0)
		mockDB.EXPECT().DeleteWALEntries(types.Height(0)).Return(nil)
		sMachineRecoverd.ProcessPrecommit(precommit1)

		// Progress to new height
		assertState(t, sMachineRecoverd, types.Height(1), types.Round(0), types.StepPropose)
	})

	t.Run("ReplayWAL: a test that requires timeouts", func(t *testing.T) {
		// This test highlights why we must store timeouts in the WAL.
		// Imagine a node starts, and isn't the proposer. It schedules a
		// timeout (line 21), which when triggered, results in a
		// nil vote being cast (line 59).
		// Now imagine it crashes, but this time receives a valid proposal
		// before the new timeout was triggered. The node will send another
		// vote for the same height and round. This is not good!
		// To prevent this from happening, we store a triggered timeout in the WAL.
		// When replaying the WAL msgs, the recovering node will move to the
		// prevote step, which will prevent the node from vasting another prevote
		// for this height and round.

		// Setup
		chain := newChain()
		mockDB := mocks.NewMockTendermintDB[starknet.Value, hash.Hash, address.Address](ctrl)
		sMachine := New(mockDB, utils.NewNopZapLogger(), *nonProposerAddr, app, chain, vals).(*testStateMachine)

		timeout := types.Timeout{
			Step:   types.StepPropose,
			Height: types.Height(0),
			Round:  types.Round(0),
		}

		// Start
		mockDB.EXPECT().Flush().Return(nil)
		sMachine.ProcessStart(0) // scheduleTimeout
		mockDB.EXPECT().SetWALEntry(timeout).Return(nil)
		sMachine.ProcessTimeout(timeout) // trigger timeout, and cast vote (!)
		assertState(t, sMachine, types.Height(0), types.Round(0), types.StepPrevote)

		// Crash and recover
		sMachineRecoverd := New(mockDB, utils.NewNopZapLogger(), *nonProposerAddr, app, chain, vals).(*testStateMachine)
		assertState(t, sMachineRecoverd, types.Height(0), types.Round(0), types.StepPropose)
		walEntries := []db.WalEntry[starknet.Value, hash.Hash, address.Address]{
			{Entry: timeout, Type: types.MessageTypeTimeout},
		}
		mockDB.EXPECT().GetWALEntries(types.Height(0)).Return(walEntries, nil)
		sMachineRecoverd.ReplayWAL()
		assertState(t, sMachineRecoverd, types.Height(0), types.Round(0), types.StepPrevote)
	})
}
