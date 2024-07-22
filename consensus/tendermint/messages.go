package tendermint

type Message interface {
	Proposal[Hashable] | Prevote | Precommit
}

type Proposal[V Hashable] struct {
	height     uint
	round      uint
	validRound *uint
	value      V
}

type Prevote struct {
	vote
}

type Precommit struct {
	vote
}

type vote struct {
	height uint
	round  uint
	id     Hash
}
