package tendermint

import consensus "github.com/NethermindEth/juno/consensus/common"

const (
	PROPOSAL  string = "PROPOSAL"
	PREVOTE   string = "PREVOTE"
	PRECOMMIT string = "PRECOMMIT"
)

// Message Todo: locked/validRound can be as large as round so uint vs int might be a bad idea maybe use uint and set to nil for negative value?
type Message struct {
	msgType        string
	height         uint64
	round          uint64
	value          *consensus.Proposable
	lastValidRound int64
}

func (msg *Message) Type() string {
	return msg.msgType
}

func (msg *Message) Height() uint64 {
	return msg.height
}

func (msg *Message) Round() uint64 {
	return msg.round
}

func (msg *Message) Value() *consensus.Proposable {
	return msg.value
}

func (msg *Message) LastValidRound() int64 {
	return msg.lastValidRound
}

func NewMessage() *Message {
	panic("not implemented")
}
