package tendermint

type Proposal[T any] struct {
	height     uint
	round      uint
	validRound *uint
	value      T
}

type Prevote[T comparable] struct {
	vote[T]
}

type Precommit[T comparable] struct {
	vote[T]
}

type vote[T comparable] struct {
	height uint
	round  uint
	id     T
}
