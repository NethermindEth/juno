package tendermint

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
)

// actionBuilder is a helper struct to build expected actions as the result of processing messages and timeouts for the state machine.
type actionBuilder struct {
	thisNodeAddr starknet.Address
	actionHeight types.Height
	actionRound  types.Round
}

func (t actionBuilder) buildMessageHeader() starknet.MessageHeader {
	return starknet.MessageHeader{Height: t.actionHeight, Round: t.actionRound, Sender: t.thisNodeAddr}
}

// broadcastProposal builds and returns a BroadcastProposal action.
func (t actionBuilder) broadcastProposal(val starknet.Value, validRound types.Round) starknet.Action {
	return &starknet.BroadcastProposal{
		MessageHeader: t.buildMessageHeader(),
		ValidRound:    validRound,
		Value:         &val,
	}
}

// broadcastPrevote builds and returns a BroadcastPrevote action.
func (t actionBuilder) broadcastPrevote(val *starknet.Value) starknet.Action {
	return &starknet.BroadcastPrevote{
		MessageHeader: t.buildMessageHeader(),
		ID:            getHash(val),
	}
}

// broadcastPrecommit builds and returns a BroadcastPrecommit action.
func (t actionBuilder) broadcastPrecommit(val *starknet.Value) starknet.Action {
	return &starknet.BroadcastPrecommit{
		MessageHeader: t.buildMessageHeader(),
		ID:            getHash(val),
	}
}

// scheduleTimeout builds and returns a ScheduleTimeout action.
func (t actionBuilder) scheduleTimeout(s types.Step) starknet.Action {
	return &actions.ScheduleTimeout{
		Step:   s,
		Height: t.actionHeight,
		Round:  t.actionRound,
	}
}

// commit returns a commit action.
func (t actionBuilder) commit(val starknet.Value, validRound types.Round, proposer int) starknet.Action {
	return &starknet.Commit{
		MessageHeader: starknet.MessageHeader{
			Height: t.actionHeight,
			Round:  t.actionRound,
			Sender: *getVal(proposer),
		},
		Value:      &val,
		ValidRound: validRound,
	}
}
