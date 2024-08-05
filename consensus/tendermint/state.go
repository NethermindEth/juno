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

type Addr interface {
	// Ethereum Addresses are 20 bytes
	~[20]byte | felt.Felt
}

type Hash interface {
	~[32]byte | felt.Felt
}

type Hashable[H Hash] interface {
	Hash() H
}

type Application[V Hashable[H], H Hash] interface {
	// Value() returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() V

	// Valid() returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Blockchain[V Hashable[H], H Hash] interface {
	// Height() return the current blockchain height
	Height() uint

	// Commit() is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(V) error
}

// Todo: decide how to represent Addresses
type Validators[A Addr] interface {
	// TotolVotingPower() represents N which is required to calculate the thresholds.
	TotalVotingPower(height uint) uint

	// ValidatorVotingPower() returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(validatorAddr A) uint

	// Proposer() returns the proposer of the current round and height.
	Proposer(height, round uint) A
}

type Slasher[M Message[V, H], V Hashable[H], H Hash] interface {
	// Equivocation() informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

type Listener[M Message[V, H], V Hashable[H], H Hash] interface {
	// Listen would return consensus messages to Tendermint which are set // by the validator set.
	Listen() <-chan M
}

type Broadcaster[M Message[V, H], V Hashable[H], H Hash, A Addr] interface {
	// Broadcast() will broadcast the message to the whole validator set
	Broadcast(msg any)

	// SendMsg() would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(validatorAddr, msg any)
}

type timeoutFn func(round uint) time.Duration

type state[V Hashable[H], H Hash] struct {
	height uint
	round  uint
	step   step

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

func (s *state[V, H]) OnTimeoutPropose() {
	// To be executed after proposeTimeout expiry
}

func (s *state[V, H]) OnTimeoutPrevote() {
	// To be executed after prevoteTimeout expiry
}

func (s *state[V, H]) OnTimeoutPrecommit() {
	// To be executed after precommitTimeout expiry
}
