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
	H          height
	R          round
	ValidRound int
	Value      *V

	Sender A
}

type (
	Prevote[H Hash, A Addr]   Vote[H, A]
	Precommit[H Hash, A Addr] Vote[H, A]
)

type Vote[H Hash, A Addr] struct {
	H  height
	R  round
	ID *H

	Sender A
}

// messages keep tracks of all the proposals, prevotes, precommits by creating a map structure as follows:
// height->round->address->[]Message

// Todo: would the following representation of message be better:
//
//	  height -> round -> address -> ID -> Message
//	How would we keep track of nil votes? In golan map key cannot be nil.
//	It is not easy to calculate a zero value when dealing with generics.
type messages[V Hashable[H], H Hash, A Addr] struct {
	proposals  map[height]map[round]map[A][]Proposal[V, H, A]
	prevotes   map[height]map[round]map[A][]Prevote[H, A]
	precommits map[height]map[round]map[A][]Precommit[H, A]
}

func newMessages[V Hashable[H], H Hash, A Addr]() messages[V, H, A] {
	return messages[V, H, A]{
		proposals:  make(map[height]map[round]map[A][]Proposal[V, H, A]),
		prevotes:   make(map[height]map[round]map[A][]Prevote[H, A]),
		precommits: make(map[height]map[round]map[A][]Precommit[H, A]),
	}
}

func addMessages[T any, A Addr](storage map[height]map[round]map[A][]T, msg T, a A, h height, r round) {
	if _, ok := storage[h]; !ok {
		storage[h] = make(map[round]map[A][]T)
	}

	if _, ok := storage[h][r]; !ok {
		storage[h][r] = make(map[A][]T)
	}

	sendersMessages, ok := storage[h][r][a]
	if !ok {
		sendersMessages = []T{}
	}

	storage[h][r][a] = append(sendersMessages, msg)
}

// Todo: ensure duplicated messages are ignored.
func (m *messages[V, H, A]) addProposal(p Proposal[V, H, A]) {
	addMessages(m.proposals, p, p.Sender, p.H, p.R)
}

func (m *messages[V, H, A]) addPrevote(p Prevote[H, A]) {
	addMessages(m.prevotes, p, p.Sender, p.H, p.R)
}

func (m *messages[V, H, A]) addPrecommit(p Precommit[H, A]) {
	addMessages(m.precommits, p, p.Sender, p.H, p.R)
}

func (m *messages[V, H, A]) allMessages(h height, r round) (map[A][]Proposal[V, H, A], map[A][]Prevote[H, A],
	map[A][]Precommit[H, A],
) {
	// Todo: Should they be copied?
	return m.proposals[h][r], m.prevotes[h][r], m.precommits[h][r]
}

func (m *messages[V, H, A]) deleteHeightMessages(h height) {
	delete(m.proposals, h)
	delete(m.prevotes, h)
	delete(m.precommits, h)
}

func (m *messages[V, H, A]) deleteRoundMessages(h height, r round) {
	delete(m.proposals[h], r)
	delete(m.prevotes[h], r)
	delete(m.precommits[h], r)
}
