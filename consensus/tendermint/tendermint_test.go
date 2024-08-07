package tendermint_test

import (
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
)

// Implements Hashable interface
type value uint64

func (t value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(t))
}

// Implements Application[value, felt.Felt] interface
type app struct{}

func newApp() *app { return &app{} }

func (a *app) Value() value {
	return value(rand.Intn(100))
}

func (a *app) Valid(v value) bool {
	return v < 100
}

// Implements Blockchain[value, felt.Felt] interface
type chain struct {
	curHeight uint
	decision  map[uint]value
}

func newChain() *chain {
	return &chain{decision: make(map[uint]value)}
}

func (c *chain) Height() uint {
	return c.curHeight
}

func (c *chain) Commit(v value) error {
	c.decision[c.curHeight] = v
	c.curHeight++
	return nil
}

// Implements Validators[felt.Felt] interface
type validators struct {
	totalVotingPower uint
	vals             []*validator
}

type validator struct {
	addr felt.Felt
	// power is effective weight
	power         uint
	currentWeight int
}

func newVals() *validators { return &validators{} }

func (v *validators) TotalVotingPower(height uint) uint {
	return v.totalVotingPower
}

func (v *validators) ValidatorVotingPower(validatorAddr felt.Felt) uint {
	// since a validator mush stake therefore the voting power will always be more than zero, thus,
	// if an address is provided which is not part of the validator set then 0 will be returned.
	for _, val := range v.vals {
		if val.addr == validatorAddr {
			return val.power
		}
	}
	return 0
}

// Proposer is implements weighted round robin regardless of height and round
func (v *validators) Proposer(_, _ uint) felt.Felt {
	// Add effective wight to current weight
	for _, val := range v.vals {
		val.currentWeight += int(val.power)
	}

	// Find validator with the highest current weight
	valWithHighestWeight := new(validator)
	// valWithHighestWeight := validator{}
	for _, val := range v.vals {
		if val.currentWeight > valWithHighestWeight.currentWeight {
			valWithHighestWeight = val
		}
	}

	// Decrease the chosen proposer's current weight by total voting power
	valWithHighestWeight.currentWeight -= int(v.totalVotingPower)
	return valWithHighestWeight.addr
}

func (v *validators) addValidator(addr felt.Felt, power uint) {
	if power < 1 {
		return
	}

	v.vals = append(v.vals, &validator{addr: addr, power: power})
	v.totalVotingPower += power
}

// Implements Listener[M Message[V, H], V Hashable[H], H Hash] and Broadcasters[V Hashable[H], H Hash, A Addr] interface
type senderAndReceiver[M tendermint.Message[V, H], V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr] struct {
	mCh chan M
}

func (r *senderAndReceiver[M, _, _, _]) send(m M) {
	r.mCh <- m
}

func (r *senderAndReceiver[M, _, _, _]) Listen() <-chan M {
	return r.mCh
}

func (r *senderAndReceiver[M, _, _, _]) Broadcast(msg M) {}

func (r *senderAndReceiver[M, _, _, A]) SendMsg(validatorAddr A, msg M) {}

func newReceiver[M tendermint.Message[V, H], V tendermint.Hashable[H], H tendermint.Hash,
	A tendermint.Addr]() *senderAndReceiver[M, V, H, A] {
	return &senderAndReceiver[M, V, H, A]{mCh: make(chan M, 0)}
}

func TestTendermint(t *testing.T) {
	proposalSenderAndReceiver := newReceiver[tendermint.Proposal[value, felt.Felt], value, felt.Felt, felt.Felt]()
	prevoteSenderAndReceiver := newReceiver[tendermint.Prevote[felt.Felt], value, felt.Felt, felt.Felt]()
	precommitSenderAndReceiver := newReceiver[tendermint.Precommit[felt.Felt], value, felt.Felt, felt.Felt]()

	listeners := tendermint.Listeners[value, felt.Felt]{
		ProposalListener:  proposalSenderAndReceiver,
		PrevoteListener:   prevoteSenderAndReceiver,
		PrecommitListener: precommitSenderAndReceiver,
	}

	broadcasters := tendermint.Broadcasters[value, felt.Felt, felt.Felt]{
		ProposalBroadcaster:  proposalSenderAndReceiver,
		PrevoteBroadcaster:   prevoteSenderAndReceiver,
		PrecommitBroadcaster: precommitSenderAndReceiver,
	}

	tendermint.New[value, felt.Felt, felt.Felt](newApp(), newChain(), newVals(), listeners, broadcasters)
}
