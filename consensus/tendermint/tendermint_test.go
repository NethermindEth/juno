package tendermint

import (
	"fmt"
	"slices"
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
	curHeight            uint
	decision             map[uint]value
	decisionCertificates map[uint][]Precommit[felt.Felt, felt.Felt]
}

func newChain() *chain {
	return &chain{
		decision:             make(map[uint]value),
		decisionCertificates: make(map[uint][]Precommit[felt.Felt, felt.Felt]),
	}
}

func (c *chain) Height() uint {
	return c.curHeight
}

func (c *chain) Commit(h uint, v value, precommits []Precommit[felt.Felt, felt.Felt]) {
	c.decision[c.curHeight] = v
	c.decisionCertificates[c.curHeight] = precommits
	c.curHeight++
}

// Implements Validators[felt.Felt] interface
type validators struct {
	totalVotingPower uint
	vals             []felt.Felt
}

func newVals() *validators { return &validators{} }

func (v *validators) TotalVotingPower(h uint) uint {
	return v.totalVotingPower
}

func (v *validators) ValidatorVotingPower(validatorAddr felt.Felt) uint {
	return 1
}

// Proposer is implements round robin
func (v *validators) Proposer(h, r uint) felt.Felt {
	i := (h + r) % v.totalVotingPower
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

	tm := func(r uint) time.Duration { return time.Duration(r) * time.Nanosecond }

	t.Run("node is the proposer", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*nodeAddr)
		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight, expectedRound := uint(0), uint(0)
		expectedProposalMsg := Proposal[value, felt.Felt, felt.Felt]{
			Height:     0,
			Round:      0,
			ValidRound: nil,
			Value:      utils.Ptr(app.cur + 1),
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

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, expectedHeight, algo.state.height)
		assert.Equal(t, expectedRound, algo.state.round)
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
		scheduledTm := algo.scheduledTms[0]

		assert.Equal(t, propose, scheduledTm.s)
		assert.Equal(t, uint(0), scheduledTm.h)
		assert.Equal(t, uint(0), scheduledTm.r)

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})

	t.Run("OnTimeoutPropose: round zero the node is not the proposer thus send a prevote nil", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()
		// The algo needs to run for a minimum amount of time but it cannot be long enough the state to change
		// multiple times. This can happen if the timeouts are too small.
		tm := func(r uint) time.Duration {
			if r == 0 {
				fmt.Println("R is ", r)
				return time.Nanosecond
			}
			return time.Second
		}

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight, expectedRound := uint(0), uint(0)
		expectedPrevoteMsg := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *nodeAddr,
			},
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

		assert.Equal(t, prevote, algo.state.step)
		assert.Equal(t, expectedHeight, algo.state.height)
		assert.Equal(t, expectedRound, algo.state.round)
	})
}

func TestPropose(t *testing.T) {
	nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
	val2, val3, val4 := new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(4)
	tm := func(r uint) time.Duration { return time.Second }

	t.Run("Line 55 (Proposal): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight := uint(0)
		rPrime := uint(4)
		round4Value := value(10)
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     expectedHeight,
			Round:      rPrime,
			ValidRound: nil,
			Value:      &round4Value,
			Sender:     *val2,
		}

		val3Prevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: expectedHeight,
				Round:  rPrime,
				ID:     utils.Ptr(round4Value.Hash()),
				Sender: *val3,
			},
		}

		algo.messages.addPrevote(val3Prevote)
		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(val2Proposal)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.proposals[expectedHeight][rPrime][*val2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val3]))
		assert.Equal(t, val3Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val3][0])

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, expectedHeight, algo.state.height)
		assert.Equal(t, rPrime, algo.state.round)
	})

	t.Run("Line 55 (Prevote): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight := uint(0)
		rPrime := uint(4)
		round4Value := value(10)
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     expectedHeight,
			Round:      rPrime,
			ValidRound: nil,
			Value:      &round4Value,
			Sender:     *val2,
		}

		val3Prevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: expectedHeight,
				Round:  rPrime,
				ID:     utils.Ptr(round4Value.Hash()),
				Sender: *val3,
			},
		}

		algo.messages.addProposal(val2Proposal)
		prevoteListener := listeners.PrevoteListener.(*senderAndReceiver[Prevote[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		prevoteListener.send(val3Prevote)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.proposals[expectedHeight][rPrime][*val2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val3]))
		assert.Equal(t, val3Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val3][0])

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, expectedHeight, algo.state.height)
		assert.Equal(t, rPrime, algo.state.round)
	})

	t.Run("Line 55 (Precommit): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight := uint(0)
		rPrime := uint(4)
		round4Value := value(10)
		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: expectedHeight,
				Round:  rPrime,
				ID:     utils.Ptr(round4Value.Hash()),
				Sender: *val2,
			},
		}

		val3Prevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: expectedHeight,
				Round:  rPrime,
				ID:     utils.Ptr(round4Value.Hash()),
				Sender: *val3,
			},
		}

		algo.messages.addPrevote(val3Prevote)
		prevoteListener := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		prevoteListener.send(val2Precommit)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.precommits[expectedHeight][rPrime][*val2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val3]))
		assert.Equal(t, val3Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val3][0])

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, expectedHeight, algo.state.height)
		assert.Equal(t, rPrime, algo.state.round)
	})

	t.Run("Line 47: schedule timeout precommit", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     utils.Ptr(value(10).Hash()),
				Sender: *val2,
			},
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *val3,
			},
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *val4,
			},
		}

		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		scheduledTm := algo.scheduledTms[1]

		assert.Equal(t, precommit, scheduledTm.s)
		assert.Equal(t, uint(0), scheduledTm.h)
		assert.Equal(t, uint(0), scheduledTm.r)

		assert.True(t, algo.state.line47Executed)
		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})

	t.Run("Line 47: don't schedule timeout precommit multiple times", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		nodePrecommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *nodeAddr,
			},
		}
		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     utils.Ptr(value(10).Hash()),
				Sender: *val2,
			},
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *val3,
			},
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *val4,
			},
		}

		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)
		precommitListner.send(nodePrecommit)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		scheduledTm := algo.scheduledTms[1]

		assert.Equal(t, precommit, scheduledTm.s)
		assert.Equal(t, uint(0), scheduledTm.h)
		assert.Equal(t, uint(0), scheduledTm.r)

		assert.True(t, algo.state.line47Executed)
		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})

	t.Run("OnTimeoutPrecommit: move to next round", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
		app, chain, vals := newApp(), newChain(), newVals()
		tmPrecommit := func(r uint) time.Duration { return time.Nanosecond }

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tmPrecommit)

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     utils.Ptr(value(10).Hash()),
				Sender: *val2,
			},
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *val3,
			},
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: 0,
				Round:  0,
				ID:     nil,
				Sender: *val4,
			},
		}

		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		scheduledTm := algo.scheduledTms[1]

		assert.False(t, algo.state.line47Executed)
		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(1), algo.state.round)

		assert.Equal(t, propose, scheduledTm.s)
		assert.Equal(t, uint(0), scheduledTm.h)
		assert.Equal(t, uint(1), scheduledTm.r)
	})

	t.Run("Line 49 (Proposal): commit the value", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		h, r := uint(0), uint(0)

		val := app.Value()
		vID := val.Hash()

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: h,
				Round:  r,
				ID:     &vID,
				Sender: *val2,
			},
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: h,
				Round:  r,
				ID:     &vID,
				Sender: *val3,
			},
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: h,
				Round:  r,
				ID:     &vID,
				Sender: *val4,
			},
		}

		// The node has received all the precommits but has received the corresponding proposal
		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)
		algo.messages.addPrecommit(val4Precommit)

		// since val2 is the proposer of round 0, the proposal arrives after the precommits
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     h,
			Round:      r,
			ValidRound: nil,
			Value:      &val,
			Sender:     *val2,
		}

		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(val2Proposal)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		scheduledTm := algo.scheduledTms[1]

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, uint(1), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)

		assert.Equal(t, propose, scheduledTm.s)
		assert.Equal(t, uint(1), scheduledTm.h)
		assert.Equal(t, uint(0), scheduledTm.r)

		precommits := []Precommit[felt.Felt, felt.Felt]{val2Precommit, val3Precommit, val4Precommit}
		assert.Equal(t, chain.decision[0], val)
		for _, p := range chain.decisionCertificates[0] {
			assert.True(t, slices.Contains(precommits, p))
		}

		assert.Equal(t, 0, len(algo.messages.proposals))
		assert.Equal(t, 0, len(algo.messages.prevotes))
		assert.Equal(t, 0, len(algo.messages.precommits))
	})

	t.Run("Line 49 (Precommit): commit the value", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		h, r := uint(0), uint(0)

		val := app.Value()
		vID := val.Hash()

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: h,
				Round:  r,
				ID:     &vID,
				Sender: *val2,
			},
		}
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     h,
			Round:      r,
			ValidRound: nil,
			Value:      &val,
			Sender:     *val2,
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: h,
				Round:  r,
				ID:     &vID,
				Sender: *val3,
			},
		}

		// The node has received all the precommits but has received the corresponding proposal
		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addProposal(val2Proposal)
		algo.messages.addPrecommit(val3Precommit)

		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: h,
				Round:  r,
				ID:     &vID,
				Sender: *val4,
			},
		}

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		scheduledTm := algo.scheduledTms[1]

		assert.Equal(t, propose, algo.state.step)
		assert.Equal(t, uint(1), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)

		assert.Equal(t, propose, scheduledTm.s)
		assert.Equal(t, uint(1), scheduledTm.h)
		assert.Equal(t, uint(0), scheduledTm.r)

		precommits := []Precommit[felt.Felt, felt.Felt]{val2Precommit, val3Precommit, val4Precommit}
		assert.Equal(t, chain.decision[0], val)
		for _, p := range chain.decisionCertificates[0] {
			assert.True(t, slices.Contains(precommits, p))
		}

		assert.Equal(t, 0, len(algo.messages.proposals))
		assert.Equal(t, 0, len(algo.messages.prevotes))
		assert.Equal(t, 0, len(algo.messages.precommits))
	})

	t.Run("Line 22: receive a new proposal before timeout expiry", func(t *testing.T) {
	})
	//
	//t.Run("Line 28: receive an old proposal before timeout expiry", func(t *testing.T) {
	//
	//})
}
