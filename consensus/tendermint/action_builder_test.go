package tendermint

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

// actionBuilder is a helper struct to build expected actions as the result of processing messages and timeouts for the state machine.
type actionBuilder struct {
	thisNodeAddr starknet.Address
	actionHeight types.Height
	actionRound  types.Round
}

func (t actionBuilder) buildMessageHeader(sender *starknet.Address) starknet.MessageHeader {
	return starknet.MessageHeader{Height: t.actionHeight, Round: t.actionRound, Sender: *sender}
}

// broadcastProposal builds and returns a BroadcastProposal action.
func (t actionBuilder) broadcastProposal(val starknet.Value, validRound types.Round) starknet.Action {
	return &starknet.BroadcastProposal{
		MessageHeader: t.buildMessageHeader(&t.thisNodeAddr),
		ValidRound:    validRound,
		Value:         &val,
	}
}

// broadcastPrevote builds and returns a BroadcastPrevote action.
func (t actionBuilder) broadcastPrevote(val *starknet.Value) starknet.Action {
	return &starknet.BroadcastPrevote{
		MessageHeader: t.buildMessageHeader(&t.thisNodeAddr),
		ID:            getHash(val),
	}
}

// broadcastPrecommit builds and returns a BroadcastPrecommit action.
func (t actionBuilder) broadcastPrecommit(val *starknet.Value) starknet.Action {
	return &starknet.BroadcastPrecommit{
		MessageHeader: t.buildMessageHeader(&t.thisNodeAddr),
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

// writeWALStart builds and returns a WriteWALStart action.
func (t actionBuilder) writeWALStart() starknet.Action {
	return &starknet.WriteWAL{
		Entry: (*wal.Start)(&t.actionHeight),
	}
}

// writeWALProposal builds and returns a WriteWALProposal action.
func (t actionBuilder) writeWALProposal(
	sender int,
	val starknet.Value,
	validRound types.Round,
) starknet.Action {
	return &starknet.WriteWAL{
		Entry: &starknet.WALProposal{
			MessageHeader: t.buildMessageHeader(getVal(sender)),
			ValidRound:    validRound,
			Value:         &val,
		},
	}
}

// writeWALPrevote builds and returns a WriteWALPrevote action.
func (t actionBuilder) writeWALPrevote(sender int, val *starknet.Value) starknet.Action {
	return &starknet.WriteWAL{
		Entry: &starknet.WALPrevote{
			MessageHeader: t.buildMessageHeader(getVal(sender)),
			ID:            getHash(val),
		},
	}
}

// writeWALPrecommit builds and returns a WriteWALPrecommit action.
func (t actionBuilder) writeWALPrecommit(sender int, val *starknet.Value) starknet.Action {
	return &starknet.WriteWAL{
		Entry: &starknet.WALPrecommit{
			MessageHeader: t.buildMessageHeader(getVal(sender)),
			ID:            getHash(val),
		},
	}
}

// writeWALTimeout builds and returns a WriteWALTimeout action.
func (t actionBuilder) writeWALTimeout(step types.Step) starknet.Action {
	return &starknet.WriteWAL{
		Entry: &starknet.WALTimeout{
			Step:   step,
			Height: t.actionHeight,
			Round:  t.actionRound,
		},
	}
}
