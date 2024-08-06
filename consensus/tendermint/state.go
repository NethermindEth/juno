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

// Hashable's Hash() is used as ID()
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

// Todo: Constraining the Listern interface to Message mean that at instantiation type the specific message (Proposal,
// Prevote or Precommit) would need to be provided. This mean that 3 separete listener would need to be instantiated
// each with it own message type. The advantage of this is there is type safety and no type swtiching would need to
// done. The other option is to have Listener and Broadcaster  operate on any and loose all type saftey but it will
// mean there is need for only 1 Listener and Broadcaster. This also simplifyies message handling logic since there will
// only be one channel to read from instead of 3 channels
type Listener[M Message[V, H], V Hashable[H], H Hash] interface {
	// Listen would return consensus messages to Tendermint which are set // by the validator set.
	Listen() <-chan M
}

type Broadcaster[M Message[V, H], V Hashable[H], H Hash, A Addr] interface {
	// Broadcast() will broadcast the message to the whole validator set
	Broadcast(msg M)

	// SendMsg() would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(validatorAddr A, msg M)
}

type timeoutFn func(round uint) time.Duration

// Todo: Remove M as a constraint from state since it is too strict
type state[M Message[V, H], V Hashable[H], H Hash, A Addr] struct {
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

	messages messages[V, H, A]

	listener Listener[M, V, H]
}

func (s *state[M, V, H, A]) Start() {
	go s.startRound(0)

	for m := range s.listener.Listen() {
		switch any(m).(type) {
		case Proposal[V, H]:
		case Precommit[H]:
		case Prevote[H]:
		}
	}
}

func (s *state[M, V, H, A]) startRound(r uint) {
}

func (s *state[M, V, H, A]) OnTimeoutPropose() {
	// To be executed after proposeTimeout expiry
}

func (s *state[M, V, H, A]) OnTimeoutPrevote() {
	// To be executed after prevoteTimeout expiry
}

func (s *state[M, V, H, A]) OnTimeoutPrecommit() {
	// To be executed after precommitTimeout expiry
}
