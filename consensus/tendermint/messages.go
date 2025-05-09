package tendermint

// Todo: Signature over the messages needs to be handled somewhere. There are 2 options:
//	1. Add the signature to each message and extend the Validator Set interface to include VerifyMessageSignature
//	method.
//	2. The P2P layer signs the message before gossiping to other validators and verifies the signature before passing
//	the message to the consensus engine.
//  The benefit of P2P layer handling the verification of the signature is the that the consensus layer can assume
//  the message is from a validator in the validator set. However, this means that the P2P layer would need to be aware
//  of the validator set and would need access to the blockchain which may not be a good idea.

type Message[V Hashable[H], H Hash, A Addr] interface {
	Proposal[V, H, A] | Prevote[H, A] | Precommit[H, A]
}

type Proposal[V Hashable[H], H Hash, A Addr] struct {
	MessageHeader[A]
	ValidRound Round `cbor:"valid_round"`
	Value      *V    `cbor:"value"`
}

func (p Proposal[V, H, A]) msgType() MessageType {
	return MessageTypeProposal
}

func (p Proposal[V, H, A]) height() Height {
	return p.Height
}

type (
	Prevote[H Hash, A Addr]   Vote[H, A]
	Precommit[H Hash, A Addr] Vote[H, A]
)

func (p Prevote[H, A]) msgType() MessageType {
	return MessageTypePrevote
}

func (p Prevote[H, A]) height() Height {
	return p.Height
}

func (p Precommit[H, A]) msgType() MessageType {
	return MessageTypePrecommit
}

func (p Precommit[H, A]) height() Height {
	return p.Height
}

type Vote[H Hash, A Addr] struct {
	MessageHeader[A]
	ID *H `cbor:"id"`
}

type MessageHeader[A Addr] struct {
	Height Height `cbor:"height"`
	Round  Round  `cbor:"round"`
	Sender A      `cbor:"sender"`
}

// messages keep tracks of all the proposals, prevotes, precommits by creating a map structure as follows:
// height->round->address->[]Message

// Todo: would the following representation of message be better:
//
//	  height -> round -> address -> ID -> Message
//	How would we keep track of nil votes? In golan map key cannot be nil.
//	It is not easy to calculate a zero value when dealing with generics.
type messages[V Hashable[H], H Hash, A Addr] struct {
	proposals  map[Height]map[Round]map[A]Proposal[V, H, A]
	prevotes   map[Height]map[Round]map[A]Prevote[H, A]
	precommits map[Height]map[Round]map[A]Precommit[H, A]
}

func newMessages[V Hashable[H], H Hash, A Addr]() messages[V, H, A] {
	return messages[V, H, A]{
		proposals:  make(map[Height]map[Round]map[A]Proposal[V, H, A]),
		prevotes:   make(map[Height]map[Round]map[A]Prevote[H, A]),
		precommits: make(map[Height]map[Round]map[A]Precommit[H, A]),
	}
}

func addMessages[T any, A Addr](storage map[Height]map[Round]map[A]T, msg T, a A, h Height, r Round) {
	if _, ok := storage[h]; !ok {
		storage[h] = make(map[Round]map[A]T)
	}

	if _, ok := storage[h][r]; !ok {
		storage[h][r] = make(map[A]T)
	}

	if _, ok := storage[h][r][a]; !ok {
		storage[h][r][a] = msg
	}
}

// Todo: ensure duplicated messages are ignored.
func (m *messages[V, H, A]) addProposal(p Proposal[V, H, A]) {
	addMessages(m.proposals, p, p.Sender, p.Height, p.Round)
}

func (m *messages[V, H, A]) addPrevote(p Prevote[H, A]) {
	addMessages(m.prevotes, p, p.Sender, p.Height, p.Round)
}

func (m *messages[V, H, A]) addPrecommit(p Precommit[H, A]) {
	addMessages(m.precommits, p, p.Sender, p.Height, p.Round)
}

func (m *messages[V, H, A]) allMessages(h Height, r Round) (map[A]Proposal[V, H, A], map[A]Prevote[H, A],
	map[A]Precommit[H, A],
) {
	// Todo: Should they be copied?
	return m.proposals[h][r], m.prevotes[h][r], m.precommits[h][r]
}

func (m *messages[V, H, A]) deleteHeightMessages(h Height) {
	delete(m.proposals, h)
	delete(m.prevotes, h)
	delete(m.precommits, h)
}
