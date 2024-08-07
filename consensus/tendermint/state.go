package tendermint

import (
	"time"
)

type step uint

const (
	propose step = iota
	prevote
	precommit
)

type timeoutFn func(round uint) time.Duration

type state[V Hashable[H], H Hash] struct {
	round uint
	step  step

	lockedValue V
	validValue  V

	// The default value of lockedRound and validRound is -1. However, using int for one value is not good use of space,
	// therefore, uint is used and nil would represent -1.
	lockedRound *uint
	validRound  *uint

	// The following are round level variable therefore when a round changes they must be reset.
	line34Executed bool
	line36Executed bool
	line47Executed bool

	timeoutPropose   timeoutFn
	timeoutPrevote   timeoutFn
	timeoutPrecommit timeoutFn
}

func (s *state[V, H]) startRound(r uint) {
}

func (s *state[V, H]) OnTimeoutPropose() {
	// To be executed after proposeTimeout expiry
}

func (s *state[V, H]) OnTimeoutPrevote() {
	// To be executed after prevoteTimeout expiry
}

func (s *state[V, H]) OnTimeoutPrecommit() {
	// To be executed after precommitTimeout expiry
}
