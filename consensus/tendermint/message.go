package tendermint

import consensus "github.com/NethermindEth/juno/consensus/common"

type MsgType = int8
type VoteLevel = int8

const (
	// todo: remove step prefix
	MSG_PROPOSAL     MsgType = 0
	MSG_PREVOTE      MsgType = 1
	MSG_PRECOMMIT    MsgType = 2
	MSG_UNIQUE_VOTES MsgType = 3
	MSG_EMPTY        MsgType = -1

	VOTE_LEVEL_MAJORITY           VoteLevel = 3
	VOTE_LEVEL_MINORITY           VoteLevel = 2
	VOTE_LEVEL_LESS_THAN_MINORITY VoteLevel = 1
	VOTE_LEVEL_EMPTY              VoteLevel = -1

	ROUND_EMPTY int64 = -9999
)

// Message Todo: locked/validRound can be as large as round so uint vs int might be a bad idea maybe use uint and set to nil for negative value?
type Message struct {
	msgType        MsgType
	height         uint64
	round          uint64
	value          *consensus.Proposable
	lastValidRound int64
	voteLevel      VoteLevel
	sender         interface{} // change to match id type
}

func (msg *Message) Type() MsgType {
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

func (msg *Message) VoteLevel() VoteLevel {
	return msg.voteLevel
}

func (msg *Message) Sender() interface{} {
	return msg.sender
}

//func (msg *Message) ParamsMatch(params map[string]interface{}) bool {
//	for k, v := range params {
//		switch k {
//		case "height":
//			val, ok := v.(uint64)
//			// panic if error?
//			if !ok {
//				return false
//			}
//			if val != msg.height {
//				return false
//			}
//		case "round":
//			val, ok := v.(uint64)
//			if !ok {
//				return false
//			}
//			if val != msg.round {
//				return false
//			}
//		case "sender":
//			val, ok := v.(string)
//			if !ok {
//				return false
//			}
//			if msg.sender != val {
//				return false
//			}
//		case "last_valid_round":
//			val, ok := v.(int64)
//			if !ok {
//				return false
//			}
//			if msg.lastValidRound != val {
//				return false
//			}
//		default:
//			panic("unknown param type: " + k)
//		}
//	}
//
//	return true
//
//}

func newMessage(msgType MsgType,
	height uint64,
	round uint64,
	value *consensus.Proposable,
	validRound int64,
	voteLevel VoteLevel) *Message {
	return &Message{
		msgType:        msgType,
		height:         height,
		round:          round,
		value:          value,
		lastValidRound: validRound,
		voteLevel:      voteLevel,
	}
}

func NewProposalMessage(height, round uint64, value *consensus.Proposable, validRound int64) *Message {
	return newMessage(MSG_PROPOSAL, height, round, value, validRound, VOTE_LEVEL_EMPTY)
}

func NewPreVoteMessage(height, round uint64, value *consensus.Proposable) *Message {
	return newMessage(MSG_PREVOTE, height, round, value, ROUND_EMPTY, VOTE_LEVEL_EMPTY)
}

func NewPreCommitMessage(height, round uint64, value *consensus.Proposable) *Message {
	return newMessage(MSG_PRECOMMIT, height, round, value, ROUND_EMPTY, VOTE_LEVEL_EMPTY)
}

func NewEmptyMessage() *Message {
	return newMessage(MSG_EMPTY, 0, 0, nil, ROUND_EMPTY, VOTE_LEVEL_EMPTY)
}
