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

type messageSet[M Message[V, H], V Hashable[H], H Hash, A Addr] map[uint]map[uint]map[A]M

type messages struct {
	// proposals  messageSet[]
	// prevotes   messageSet[]
	// precommits messageSet[]
}
