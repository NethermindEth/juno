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

type timeoutFn func(round uint) time.Duration

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

type Listeners[V Hashable[H], H Hash] struct {
	ProposalListener  Listener[Proposal[V, H], V, H]
	PrevoteListener   Listener[Prevote[H], V, H]
	PrecommitListener Listener[Precommit[H], V, H]
}

type Broadcasters[V Hashable[H], H Hash, A Addr] struct {
	ProposalBroadcaster  Broadcaster[Proposal[V, H], V, H, A]
	PrevoteBroadcaster   Broadcaster[Prevote[H], V, H, A]
	PrecommitBroadcaster Broadcaster[Precommit[H], V, H, A]
}

type Tendermint[V Hashable[H], H Hash, A Addr] struct {
	nodeAddr A

	height   uint
	state    state[V, H] // Todo: Does state need to be protected?
	messages messages[V, H, A]

	timeoutPropose   timeoutFn
	timeoutPrevote   timeoutFn
	timeoutPrecommit timeoutFn

	application Application[V, H]
	blockchain  Blockchain[V, H]
	validators  Validators[A]

	listeners    Listeners[V, H]
	broadcasters Broadcasters[V, H, A]
}

type state[V Hashable[H], H Hash] struct {
	round uint
	step  step

	lockedValue *V
	validValue  *V

	// The default value of lockedRound and validRound is -1. However, using int for one value is not good use of space,
	// therefore, uint is used and nil would represent -1.
	lockedRound *uint
	validRound  *uint

	// The following are round level variable therefore when a round changes they must be reset.
	line34Executed bool
	line36Executed bool
	line47Executed bool
}

// Todo: Add Slashers later
func New[V Hashable[H], H Hash, A Addr](addr A, app Application[V, H], chain Blockchain[V, H], vals Validators[A],
	listeners Listeners[V, H], broadcasters Broadcasters[V, H, A], tmPropose, tmPrevote, tmPrecommit timeoutFn,
) *Tendermint[V, H, A] {
	return &Tendermint[V, H, A]{
		nodeAddr:         addr,
		height:           chain.Height(),
		state:            state[V, H]{},
		messages:         newMessages[V, H, A](),
		timeoutPropose:   tmPropose,
		timeoutPrevote:   tmPrevote,
		timeoutPrecommit: tmPrecommit,
		application:      app,
		blockchain:       chain,
		validators:       vals,
		listeners:        listeners,
		broadcasters:     broadcasters,
	}
}

func (t *Tendermint[V, H, A]) Start() {
	go t.startRound(0)

	for {
		select {
		// case proposal := <-t.listeners.ProposalListener.Listen():
		// case prevote := <-t.listeners.PrevoteListener.Listen():
		// case precommit := <-t.listeners.PrecommitListener.Listen():
		// todo: deal with timeouts here?
		}
	}
}

func (t *Tendermint[V, H, A]) startRound(r uint) {
	if r <= t.state.round {
		return
	}

	t.state.round = r
	t.state.step = propose

	t.state.line34Executed = false
	t.state.line36Executed = false
	t.state.line47Executed = false

	if t.validators.Proposer(t.height, r) == t.nodeAddr {
		var proposalValue *V
		if t.state.validValue != nil {
			proposalValue = t.state.validValue
		} else {
			v := t.application.Value()
			proposalValue = &v
		}

		proposalMessage := Proposal[V, H]{
			height:     t.height,
			round:      r,
			validRound: t.state.validRound,
			value:      proposalValue,
		}

		t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
	} else {
		// Todo: do timeout need to be cancelled?
		time.AfterFunc(t.timeoutPropose(r), func() { t.OnTimeoutPropose(t.height, r) })
	}
}

func (t *Tendermint[_, H, _]) OnTimeoutPropose(h, r uint) {
	if t.height == h && t.state.round == r && t.state.step == propose {
		t.broadcasters.PrevoteBroadcaster.Broadcast(Prevote[H]{
			vote: vote[H]{
				height: t.height,
				round:  t.state.round,
				id:     nil,
			},
		})
		t.state.step = prevote
	}
}

func (t *Tendermint[_, _, _]) OnTimeoutPrevote() {
	// To be executed after prevoteTimeout expiry
}

func (t *Tendermint[_, _, _]) OnTimeoutPrecommit() {}
