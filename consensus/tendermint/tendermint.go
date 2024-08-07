package tendermint

import "github.com/NethermindEth/juno/core/felt"

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
	proposalListener  Listener[Proposal[V, H], V, H]
	prevoteListener   Listener[Prevote[H], V, H]
	precommitListener Listener[Precommit[H], V, H]
}

type Broadcasters[V Hashable[H], H Hash, A Addr] struct {
	proposalBroadcaster  Broadcaster[Proposal[V, H], V, H, A]
	prevoteBroadcaster   Broadcaster[Prevote[H], V, H, A]
	precommitBroadcaster Broadcaster[Precommit[H], V, H, A]
}

type Tendermint[V Hashable[H], H Hash, A Addr] struct {
	// Todo: Does state need to be protected?
	height   uint
	state    state[V, H]
	messages messages[V, H, A]

	application Application[V, H]
	blockchain  Blockchain[V, H]
	validators  Validators[A]

	listeners    Listeners[V, H]
	broadcasters Broadcasters[V, H, A]
}

// Todo: Add Slashers later
func New[V Hashable[H], H Hash, A Addr](app Application[V, H], chain Blockchain[V, H], vals Validators[A],

// listeners Listeners[V, H], broadcasters Broadcasters[V, H, A],
) Tendermint[V, H, A] {
	return Tendermint[V, H, A]{
		height:      chain.Height(),
		state:       state[V, H]{},
		messages:    newMessages[V, H, A](),
		application: app,
		blockchain:  chain,
		validators:  vals,
		// listeners:    listeners,
		// broadcasters: broadcasters,
	}
}
