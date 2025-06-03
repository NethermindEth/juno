package types

// Todo: Signature over the messages needs to be handled somewhere. There are 2 options:
//	1. Add the signature to each message and extend the Validator Set interface to include VerifyMessageSignature
//	method.
//	2. The P2P layer signs the message before gossiping to other validators and verifies the signature before passing
//	the message to the consensus engine.
//  The benefit of P2P layer handling the verification of the signature is the that the consensus layer can assume
//  the message is from a validator in the validator set. However, this means that the P2P layer would need to be aware
//  of the validator set and would need access to the blockchain which may not be a good idea.

type Message[V Hashable] interface {
	MsgType() MessageType
	GetHeight() Height
}

type Proposal[V Hashable] struct {
	MessageHeader
	ValidRound Round `cbor:"valid_round"`
	Value      *V    `cbor:"value"`
}

func (p Proposal[V]) MsgType() MessageType {
	return MessageTypeProposal
}

func (p Proposal[V]) GetHeight() Height {
	return p.Height
}

type (
	Prevote   Vote
	Precommit Vote
)

func (p Prevote) MsgType() MessageType {
	return MessageTypePrevote
}

func (p Prevote) GetHeight() Height {
	return p.Height
}

func (p Precommit) MsgType() MessageType {
	return MessageTypePrecommit
}

func (p Precommit) GetHeight() Height {
	return p.Height
}

type Vote struct {
	MessageHeader
	ID *Hash `cbor:"id"`
}

type MessageHeader struct {
	Height Height `cbor:"height"`
	Round  Round  `cbor:"round"`
	Sender Addr   `cbor:"sender"`
}

// messages keep tracks of all the proposals, prevotes, precommits by creating a map structure as follows:
// height->round->address->[]Message

// Todo: would the following representation of message be better:
//
//	  height -> round -> address -> ID -> Message
//	How would we keep track of nil votes? In golan map key cannot be nil.
//	It is not easy to calculate a zero value when dealing with generics.
type Messages[V Hashable] struct {
	Proposals  map[Height]map[Round]map[Addr]Proposal[V]
	Prevotes   map[Height]map[Round]map[Addr]Prevote
	Precommits map[Height]map[Round]map[Addr]Precommit
}

func NewMessages[V Hashable]() Messages[V] {
	return Messages[V]{
		Proposals:  make(map[Height]map[Round]map[Addr]Proposal[V]),
		Prevotes:   make(map[Height]map[Round]map[Addr]Prevote),
		Precommits: make(map[Height]map[Round]map[Addr]Precommit),
	}
}

// addMessages adds the message to the message set if it doesn't already exist. Return if the message was added.
func addMessages[T any](storage map[Height]map[Round]map[Addr]T, msg T, a Addr, h Height, r Round) bool {
	if _, ok := storage[h]; !ok {
		storage[h] = make(map[Round]map[Addr]T)
	}

	if _, ok := storage[h][r]; !ok {
		storage[h][r] = make(map[Addr]T)
	}

	if _, ok := storage[h][r][a]; !ok {
		storage[h][r][a] = msg
		return true
	}
	return false
}

// Todo: ensure duplicated messages are ignored.
func (m *Messages[V]) AddProposal(p Proposal[V]) bool {
	return addMessages(m.Proposals, p, p.Sender, p.Height, p.Round)
}

func (m *Messages[V]) AddPrevote(p Prevote) bool {
	return addMessages(m.Prevotes, p, p.Sender, p.Height, p.Round)
}

func (m *Messages[V]) AddPrecommit(p Precommit) bool {
	return addMessages(m.Precommits, p, p.Sender, p.Height, p.Round)
}

func (m *Messages[V]) AllMessages(h Height, r Round) (map[Addr]Proposal[V], map[Addr]Prevote,
	map[Addr]Precommit,
) {
	// Todo: Should they be copied?
	return m.Proposals[h][r], m.Prevotes[h][r], m.Precommits[h][r]
}

func (m *Messages[V]) DeleteHeightMessages(h Height) {
	delete(m.Proposals, h)
	delete(m.Prevotes, h)
	delete(m.Precommits, h)
}

// MessageType represents the type of message stored in the WAL.
type MessageType uint8

const (
	MessageTypeProposal MessageType = iota
	MessageTypePrevote
	MessageTypePrecommit
	MessageTypeTimeout
	MessageTypeUnknown
)

// String returns the string representation of the MessageType.
func (m MessageType) String() string {
	switch m {
	case MessageTypeProposal:
		return "proposal"
	case MessageTypePrevote:
		return "prevote"
	case MessageTypePrecommit:
		return "precommit"
	case MessageTypeTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}
