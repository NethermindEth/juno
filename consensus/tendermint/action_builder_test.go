package tendermint

import (
	"github.com/NethermindEth/juno/core/felt"
)

// actionBuilder is a helper struct to build expected actions as the result of processing messages and timeouts for the state machine.
type actionBuilder struct {
	thisNodeAddr felt.Felt
	actionHeight height
	actionRound  round
}

func (t actionBuilder) buildMessageHeader() MessageHeader[felt.Felt] {
	return MessageHeader[felt.Felt]{Height: t.actionHeight, Round: t.actionRound, Sender: t.thisNodeAddr}
}

// broadcastProposal builds and returns a BroadcastProposal action.
func (t actionBuilder) broadcastProposal(val value, validRound round) Action[value, felt.Felt, felt.Felt] {
	return &BroadcastProposal[value, felt.Felt, felt.Felt]{
		MessageHeader: t.buildMessageHeader(),
		ValidRound:    validRound,
		Value:         &val,
	}
}

// broadcastPrevote builds and returns a BroadcastPrevote action.
func (t actionBuilder) broadcastPrevote(val *value) Action[value, felt.Felt, felt.Felt] {
	return &BroadcastPrevote[felt.Felt, felt.Felt]{
		MessageHeader: t.buildMessageHeader(),
		ID:            getHash(val),
	}
}

// broadcastPrecommit builds and returns a BroadcastPrecommit action.
func (t actionBuilder) broadcastPrecommit(val *value) Action[value, felt.Felt, felt.Felt] {
	return &BroadcastPrecommit[felt.Felt, felt.Felt]{
		MessageHeader: t.buildMessageHeader(),
		ID:            getHash(val),
	}
}

// scheduleTimeout builds and returns a ScheduleTimeout action.
func (t actionBuilder) scheduleTimeout(s step) Action[value, felt.Felt, felt.Felt] {
	return &ScheduleTimeout{
		Step:   s,
		Height: t.actionHeight,
		Round:  t.actionRound,
	}
}
