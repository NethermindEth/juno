package tendermint

import (
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

type step uint

const (
	propose step = iota
	prevote
	precommit
)

// Todo: refactor to use more stricter constraints on the interfaces. For example, instead of Application having a Id(),
// the value of type T should be constraint to have ID() function.

type Hashable[T felt.Felt | [32]byte] interface {
	Id() T
}

type v struct{}

func (v *v) Id() felt.Felt {
	return felt.Zero
}

type Application[T any, K comparable] interface {
	// Value() returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() T

	// Valid() returns true if the provided value is valid according to the application context.
	Valid(T) bool

	// Id() returns the id of the value which is a unique identifier of the value being consider for the current
	// height. The votes include // the id of the value not the value itself
	Id(T) K
}

type Blockchain[T any] interface {
	// Height() return the current blockchain height
	Height() uint

	// Commit() is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(T) error
}

type Validators[T any] interface {
	// TotolVotingPower() represents N which is required to calculate the thresholds.
	TotalVotingPower(height uint) uint

	// ValidatorVotingPower() returns the voting power of the a single	  // validator. This is also required to implement various thresholds. // The assumption is that a single validator cannot have voting power // more than f.
	ValidatorVotingPower(validatorAddr T) uint

	// Proposer() returns the proposer of the current round and height.
	Proposer(height, round uint) T
}

type timeoutFn func(round uint) time.Duration

type state[T any] struct {
	height uint
	round  uint
	step   step

	lockedValue T
	validValue  T

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

func (s *state[T]) OnTimeoutPropose() {
	// To be executed after proposeTimeout expiry
}

func (s *state[T]) OnTimeoutPrevote() {
	// To be executed after prevoteTimeout expiry
}

func (s *state[T]) OnTimeoutPrecommit() {
	// To be executed after precommitTimeout expiry
}
