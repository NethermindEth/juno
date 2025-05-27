package tendermint

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

// actionBuilder is a helper struct to build expected actions as the result of processing messages and timeouts for the state machine.
type actionBuilder struct {
	thisNodeAddr types.Addr
	actionHeight types.Height
	actionRound  types.Round
}

func (t actionBuilder) buildMessageHeader() types.MessageHeader {
	return types.MessageHeader{Height: t.actionHeight, Round: t.actionRound, Sender: t.thisNodeAddr}
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
	return &types.BroadcastPrevote{
		MessageHeader: t.buildMessageHeader(),
		ID:            getHash(val),
	}
}

// broadcastPrecommit builds and returns a BroadcastPrecommit action.
func (t actionBuilder) broadcastPrecommit(val *starknet.Value) starknet.Action {
	return &types.BroadcastPrecommit{
		MessageHeader: t.buildMessageHeader(),
		ID:            getHash(val),
	}
}

// scheduleTimeout builds and returns a ScheduleTimeout action.
func (t actionBuilder) scheduleTimeout(s types.Step) starknet.Action {
	return &types.ScheduleTimeout{
		Step:   s,
		Height: t.actionHeight,
		Round:  t.actionRound,
	}
}
