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
	Height     uint
	Round      uint
	ValidRound int
	Value      *V

	Sender A
}

type Prevote[H Hash, A Addr] struct {
	Vote[H, A]
}

type Precommit[H Hash, A Addr] struct {
	Vote[H, A]
}

type Vote[H Hash, A Addr] struct {
	Height uint
	Round  uint
	ID     *H

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
	proposals  map[uint]map[uint]map[A][]Proposal[V, H, A]
	prevotes   map[uint]map[uint]map[A][]Prevote[H, A]
	precommits map[uint]map[uint]map[A][]Precommit[H, A]
}

func newMessages[V Hashable[H], H Hash, A Addr]() messages[V, H, A] {
	return messages[V, H, A]{
		proposals:  make(map[uint]map[uint]map[A][]Proposal[V, H, A]),
		prevotes:   make(map[uint]map[uint]map[A][]Prevote[H, A]),
		precommits: make(map[uint]map[uint]map[A][]Precommit[H, A]),
	}
}

// Todo: ensure duplicated messages are ignored.
func (m *messages[V, H, A]) addProposal(p Proposal[V, H, A]) {
	if _, ok := m.proposals[p.Height]; !ok {
		m.proposals[p.Height] = make(map[uint]map[A][]Proposal[V, H, A])
	}

	if _, ok := m.proposals[p.Height][p.Round]; !ok {
		m.proposals[p.Height][p.Round] = make(map[A][]Proposal[V, H, A])
	}

	sendersProposals, ok := m.proposals[p.Height][p.Round][p.Sender]
	if !ok {
		sendersProposals = []Proposal[V, H, A]{}
	}

	m.proposals[p.Height][p.Round][p.Sender] = append(sendersProposals, p)
}

func (m *messages[V, H, A]) addPrevote(p Prevote[H, A]) {
	if _, ok := m.prevotes[p.Height]; !ok {
		m.prevotes[p.Height] = make(map[uint]map[A][]Prevote[H, A])
	}

	if _, ok := m.prevotes[p.Height][p.Round]; !ok {
		m.prevotes[p.Height][p.Round] = make(map[A][]Prevote[H, A])
	}

	sendersPrevotes, ok := m.prevotes[p.Height][p.Round][p.Sender]
	if !ok {
		sendersPrevotes = []Prevote[H, A]{}
	}

	m.prevotes[p.Height][p.Round][p.Sender] = append(sendersPrevotes, p)
}

func (m *messages[V, H, A]) addPrecommit(p Precommit[H, A]) {
	if _, ok := m.precommits[p.Height]; !ok {
		m.precommits[p.Height] = make(map[uint]map[A][]Precommit[H, A])
	}

	if _, ok := m.precommits[p.Height][p.Round]; !ok {
		m.precommits[p.Height][p.Round] = make(map[A][]Precommit[H, A])
	}

	sendersPrecommits, ok := m.precommits[p.Height][p.Round][p.Sender]
	if !ok {
		sendersPrecommits = []Precommit[H, A]{}
	}

	m.precommits[p.Height][p.Round][p.Sender] = append(sendersPrecommits, p)
}

func (m *messages[V, H, A]) allMessages(h, r uint) (map[A][]Proposal[V, H, A], map[A][]Prevote[H, A],
	map[A][]Precommit[H, A],
) {
	// Todo: Should they be copied?
	return m.proposals[h][r], m.prevotes[h][r], m.precommits[h][r]
}

func (m *messages[V, H, A]) deleteHeightMessages(h uint) {
	delete(m.proposals, h)
	delete(m.prevotes, h)
	delete(m.precommits, h)
}

func (m *messages[V, H, A]) deleteRoundMessages(h, r uint) {
	delete(m.proposals[h], r)
	delete(m.prevotes[h], r)
	delete(m.precommits[h], r)
}
