package tendermint

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

// Implements Hashable interface
type value uint64

func (t value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(t))
}

// Implements Application[value, felt.Felt] interface
type app struct {
	cur value
}

func newApp() *app { return &app{} }

func (a *app) Value() value {
	a.cur = (a.cur + 1) % 100
	return a.cur
}

func (a *app) Valid(v value) bool {
	return v < 100
}

// Implements Blockchain[value, felt.Felt] interface
type chain struct {
	curHeight            height
	decision             map[height]value
	decisionCertificates map[height][]Precommit[felt.Felt, felt.Felt]
}

func newChain() *chain {
	return &chain{
		decision:             make(map[height]value),
		decisionCertificates: make(map[height][]Precommit[felt.Felt, felt.Felt]),
	}
}

func (c *chain) Height() height {
	return c.curHeight
}

func (c *chain) Commit(h height, v value, precommits []Precommit[felt.Felt, felt.Felt]) {
	c.decision[c.curHeight] = v
	c.decisionCertificates[c.curHeight] = precommits
	c.curHeight++
}

// Implements Validators[felt.Felt] interface
type validators struct {
	totalVotingPower votingPower
	vals             []felt.Felt
}

func newVals() *validators { return &validators{} }

func (v *validators) TotalVotingPower(h height) votingPower {
	return v.totalVotingPower
}

func (v *validators) ValidatorVotingPower(validatorAddr felt.Felt) votingPower {
	return 1
}

// Proposer is implements round robin
func (v *validators) Proposer(h height, r round) felt.Felt {
	i := (uint(h) + uint(r)) % uint(v.totalVotingPower)
	return v.vals[i]
}

func (v *validators) addValidator(addr felt.Felt) {
	v.vals = append(v.vals, addr)
	v.totalVotingPower++
}

// Implements Listener[M Message[V, H], V Hashable[H], H Hash] and Broadcasters[V Hashable[H], H Hash, A Addr] interface
type senderAndReceiver[M Message[V, H, A], V Hashable[H], H Hash, A Addr] struct {
	mCh chan M
}

func (r *senderAndReceiver[M, _, _, _]) send(m M) {
	r.mCh <- m
}

func (r *senderAndReceiver[M, _, _, _]) Listen() <-chan M {
	return r.mCh
}

func (r *senderAndReceiver[M, _, _, _]) Broadcast(msg M) { r.mCh <- msg }

func (r *senderAndReceiver[M, _, _, A]) SendMsg(validatorAddr A, msg M) {}

func newSenderAndReceiver[M Message[V, H, A], V Hashable[H], H Hash, A Addr]() *senderAndReceiver[M, V, H, A] {
	return &senderAndReceiver[M, V, H, A]{mCh: make(chan M, 10)}
}

func testListenersAndBroadcasters() (Listeners[value, felt.Felt, felt.Felt], Broadcasters[value,
	felt.Felt, felt.Felt],
) {
	listeners := Listeners[value, felt.Felt, felt.Felt]{
		ProposalListener:  newSenderAndReceiver[Proposal[value, felt.Felt, felt.Felt], value, felt.Felt, felt.Felt](),
		PrevoteListener:   newSenderAndReceiver[Prevote[felt.Felt, felt.Felt], value, felt.Felt, felt.Felt](),
		PrecommitListener: newSenderAndReceiver[Precommit[felt.Felt, felt.Felt], value, felt.Felt, felt.Felt](),
	}

	broadcasters := Broadcasters[value, felt.Felt, felt.Felt]{
		ProposalBroadcaster:  newSenderAndReceiver[Proposal[value, felt.Felt, felt.Felt], value, felt.Felt, felt.Felt](),
		PrevoteBroadcaster:   newSenderAndReceiver[Prevote[felt.Felt, felt.Felt], value, felt.Felt, felt.Felt](),
		PrecommitBroadcaster: newSenderAndReceiver[Precommit[felt.Felt, felt.Felt], value, felt.Felt, felt.Felt](),
	}

	return listeners, broadcasters
}

func TestStartRound(t *testing.T) {
	nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
	val2, val3, val4 := new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(4)

	tm := func(r round) time.Duration { return time.Duration(r) * time.Nanosecond }

	t.Run("node is the proposer", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*nodeAddr)
		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight, expectedRound := height(0), round(0)
		expectedProposalMsg := Proposal[value, felt.Felt, felt.Felt]{
			H:          0,
			R:          0,
			ValidRound: -1,
			Value:      utils.HeapPtr(app.cur + 1),
			Sender:     *nodeAddr,
		}

		proposalBroadcaster := broadcasters.ProposalBroadcaster.(*senderAndReceiver[Proposal[value, felt.Felt,
			felt.Felt], value, felt.Felt, felt.Felt])

		algo.Start()
		algo.Stop()

		proposal := <-proposalBroadcaster.mCh

		assert.Equal(t, expectedProposalMsg, proposal)
		assert.Equal(t, 1, len(algo.messages.proposals[expectedHeight][expectedRound][*nodeAddr]))
		assert.Equal(t, expectedProposalMsg, algo.messages.proposals[expectedHeight][expectedRound][*nodeAddr][0])

		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, expectedHeight, algo.state.h)
		assert.Equal(t, expectedRound, algo.state.r)
	})

	t.Run("node is not the proposer: schedule timeoutPropose", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		algo.Start()
		algo.Stop()

		assert.Equal(t, 1, len(algo.scheduledTms))

		assert.Contains(t, algo.scheduledTms, timeout{s: propose, h: 0, r: 0})

		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, height(0), algo.state.h)
		assert.Equal(t, round(0), algo.state.r)
	})

	t.Run("OnTimeoutPropose: round zero the node is not the proposer thus send a prevote nil", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()
		// The algo needs to run for a minimum amount of time but it cannot be long enough the state to change
		// multiple times. This can happen if the timeouts are too small.
		tm := func(r round) time.Duration {
			if r == 0 {
				return time.Nanosecond
			}
			return time.Second
		}

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight, expectedRound := height(0), round(0)
		expectedPrevoteMsg := Prevote[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *nodeAddr,
		}

		prevoteBroadcaster := broadcasters.PrevoteBroadcaster.(*senderAndReceiver[Prevote[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])

		algo.Start()
		time.Sleep(time.Millisecond)
		algo.Stop()

		prevoteMsg := <-prevoteBroadcaster.mCh

		assert.Equal(t, expectedPrevoteMsg, prevoteMsg)
		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][expectedRound][*nodeAddr]))
		assert.Equal(t, expectedPrevoteMsg, algo.messages.prevotes[expectedHeight][expectedRound][*nodeAddr][0])

		assert.Equal(t, prevote, algo.state.s)
		assert.Equal(t, expectedHeight, algo.state.h)
		assert.Equal(t, expectedRound, algo.state.r)
	})
}

func TestThresholds(t *testing.T) {
	tests := []struct {
		n votingPower
		q votingPower
		f votingPower
	}{
		{1, 1, 0},
		{2, 2, 0},
		{3, 2, 0},
		{4, 3, 1},
		{5, 4, 1},
		{6, 4, 1},
		{7, 5, 2},
		{11, 8, 3},
		{15, 10, 4},
		{20, 14, 6},
		{100, 67, 33},
		{150, 100, 49},
		{2000, 1334, 666},
		{2509, 1673, 836},
		{3045, 2030, 1014},
		{7689, 5126, 2562},
		{10032, 6688, 3343},
		{12932, 8622, 4310},
		{15982, 10655, 5327},
		{301234, 200823, 100411},
		{301235, 200824, 100411},
		{301236, 200824, 100411},
	}

	for _, test := range tests {
		assert.Equal(t, test.q, q(test.n))
		assert.Equal(t, test.f, f(test.n))

		assert.True(t, 2*q(test.n) > test.n+f(test.n))
		assert.True(t, 2*(q(test.n)-1) <= test.n+f(test.n))
	}
}

// Todo: Add tests for round change where existing messages are processed
// Todo: Add malicious test
