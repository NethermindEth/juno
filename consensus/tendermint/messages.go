package tendermint

type Message[V Hashable[H], H Hash] interface {
	Proposal[V, H] | Prevote[H] | Precommit[H]
}

type Proposal[V Hashable[H], H Hash] struct {
	height     uint
	round      uint
	validRound *uint
	value      V
}

type Prevote[H Hash] struct {
	vote[H]
}

type Precommit[H Hash] struct {
	vote[H]
}

type vote[H Hash] struct {
	height uint
	round  uint
	id     H
}

// messages keep tracks of all the proposals, prevotes, precommits by creating a map structure as follows:
// height->round->address->[]Message
type messages[V Hashable[H], H Hash, A Addr] struct {
	proposals  map[uint]map[uint]map[A][]Proposal[V, H]
	prevotes   map[uint]map[uint]map[A][]Prevote[H]
	precommits map[uint]map[uint]map[A][]Precommit[H]
}

func newMessages[V Hashable[H], H Hash, A Addr]() messages[V, H, A] {
	return messages[V, H, A]{
		proposals:  make(map[uint]map[uint]map[A][]Proposal[V, H]),
		prevotes:   make(map[uint]map[uint]map[A][]Prevote[H]),
		precommits: make(map[uint]map[uint]map[A][]Precommit[H]),
	}
}
