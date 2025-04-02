package tendermint

import (
	"slices"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestPropose(t *testing.T) {
	nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
	val2, val3, val4 := new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(4)
	tm := func(r round) time.Duration { return time.Second }

	t.Run("Line 55 (Proposal): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight := height(0)
		rPrime, rPrimeVal := round(4), value(10)
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			H:          expectedHeight,
			R:          rPrime,
			ValidRound: -1,
			Value:      &rPrimeVal,
			Sender:     *val2,
		}

		val3Prevote := Prevote[felt.Felt, felt.Felt]{
			H:      expectedHeight,
			R:      rPrime,
			ID:     utils.HeapPtr(rPrimeVal.Hash()),
			Sender: *val3,
		}

		algo.futureMessages.addPrevote(val3Prevote)
		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(val2Proposal)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.proposals[expectedHeight][rPrime][*val2]))
		assert.Equal(t, val2Proposal, algo.messages.proposals[expectedHeight][rPrime][*val2][0])

		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val3]))
		assert.Equal(t, val3Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val3][0])

		// The step is not propose because the proposal which is received in round r' leads to consensus
		// engine broadcasting prevote to the proposal which changes the step from propose to prevote.
		assert.Equal(t, prevote, algo.state.s)
		assert.Equal(t, expectedHeight, algo.state.h)
		assert.Equal(t, rPrime, algo.state.r)
	})

	t.Run("Line 55 (Prevote): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		tm := func(r round) time.Duration { return 2 * time.Second }

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight := height(0)
		rPrime, rPrimeVal := round(4), value(10)
		val2Prevote := Prevote[felt.Felt, felt.Felt]{
			H:      expectedHeight,
			R:      rPrime,
			ID:     utils.HeapPtr(rPrimeVal.Hash()),
			Sender: *val2,
		}

		val3Prevote := Prevote[felt.Felt, felt.Felt]{
			H:      expectedHeight,
			R:      rPrime,
			ID:     utils.HeapPtr(rPrimeVal.Hash()),
			Sender: *val3,
		}

		algo.futureMessages.addPrevote(val2Prevote)
		prevoteListener := listeners.PrevoteListener.(*senderAndReceiver[Prevote[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		prevoteListener.send(val3Prevote)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val2]))
		assert.Equal(t, val2Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val2][0])

		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val3]))
		assert.Equal(t, val3Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val3][0])

		// The step here remains propose because a proposal is yet to be received to allow the node to send the
		// prevote for it.
		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, expectedHeight, algo.state.h)
		assert.Equal(t, rPrime, algo.state.r)
	})

	t.Run("Line 55 (Precommit): Start round r' when f+1 future round messages are received from round r'", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		expectedHeight := height(0)
		rPrime := round(4)
		round4Value := value(10)
		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      expectedHeight,
			R:      rPrime,
			ID:     utils.HeapPtr(round4Value.Hash()),
			Sender: *val2,
		}

		val3Prevote := Prevote[felt.Felt, felt.Felt]{
			H:      expectedHeight,
			R:      rPrime,
			ID:     utils.HeapPtr(round4Value.Hash()),
			Sender: *val3,
		}

		algo.futureMessages.addPrevote(val3Prevote)
		prevoteListener := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		prevoteListener.send(val2Precommit)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.precommits[expectedHeight][rPrime][*val2]))
		assert.Equal(t, val2Precommit, algo.messages.precommits[expectedHeight][rPrime][*val2][0])

		assert.Equal(t, 1, len(algo.messages.prevotes[expectedHeight][rPrime][*val3]))
		assert.Equal(t, val3Prevote, algo.messages.prevotes[expectedHeight][rPrime][*val3][0])

		// The step here remains propose because a proposal is yet to be received to allow the node to send the
		// prevote for it.
		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, expectedHeight, algo.state.h)
		assert.Equal(t, rPrime, algo.state.r)
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
			H:      0,
			R:      0,
			ID:     utils.HeapPtr(value(10).Hash()),
			Sender: *val2,
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *val3,
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *val4,
		}

		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		assert.Contains(t, algo.scheduledTms, timeout{s: precommit, h: 0, r: 0})

		assert.True(t, algo.state.timeoutPrecommitScheduled)
		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, height(0), algo.state.h)
		assert.Equal(t, round(0), algo.state.r)
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
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *nodeAddr,
		}
		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     utils.HeapPtr(value(10).Hash()),
			Sender: *val2,
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *val3,
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *val4,
		}

		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)
		precommitListner.send(nodePrecommit)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		// The reason there are 2 timeouts is because the first timeout is the proposeTimeout which is immediately
		// scheduled when nodes move to the next round, and it is not the proposer.
		// If the precommitTimeout was scheduled more than once, then the len of scheduledTms would be more than 2.
		assert.Equal(t, 2, len(algo.scheduledTms))
		assert.Contains(t, algo.scheduledTms, timeout{s: precommit, h: 0, r: 0})

		assert.True(t, algo.state.timeoutPrecommitScheduled)
		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, height(0), algo.state.h)
		assert.Equal(t, round(0), algo.state.r)
	})

	t.Run("OnTimeoutPrecommit: move to next round", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		nodeAddr := new(felt.Felt).SetBytes([]byte("my node address"))
		app, chain, vals := newApp(), newChain(), newVals()
		tmPrecommit := func(r round) time.Duration { return time.Nanosecond }

		vals.addValidator(*val2)
		vals.addValidator(*val3)
		vals.addValidator(*val4)
		vals.addValidator(*nodeAddr)

		algo := New[value, felt.Felt, felt.Felt](*nodeAddr, app, chain, vals, listeners, broadcasters, tm, tm, tmPrecommit)

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     utils.HeapPtr(value(10).Hash()),
			Sender: *val2,
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *val3,
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      0,
			R:      0,
			ID:     nil,
			Sender: *val4,
		}

		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		// The first timeout here is the nodes proposeTimeout from round 0, and since the precommit timout expired
		// before the proposeTimeout it is still in the slice. It will only be deleted after its expiry.
		// The second timeout here is the proposeTimeout for round 1, which is what we are interested in.
		assert.Equal(t, 2, len(algo.scheduledTms))
		assert.Contains(t, algo.scheduledTms, timeout{s: propose, h: 0, r: 1})

		assert.False(t, algo.state.timeoutPrecommitScheduled)
		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, height(0), algo.state.h)
		assert.Equal(t, round(1), algo.state.r)
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

		h, r := height(0), round(0)

		val := app.Value()
		vID := val.Hash()

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      h,
			R:      r,
			ID:     &vID,
			Sender: *val2,
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      h,
			R:      r,
			ID:     &vID,
			Sender: *val3,
		}
		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      h,
			R:      r,
			ID:     &vID,
			Sender: *val4,
		}

		// The node has received all the precommits but has received the corresponding proposal
		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addPrecommit(val3Precommit)
		algo.messages.addPrecommit(val4Precommit)

		// since val2 is the proposer of round 0, the proposal arrives after the precommits
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			H:          h,
			R:          r,
			ValidRound: -1,
			Value:      &val,
			Sender:     *val2,
		}

		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(val2Proposal)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		assert.Contains(t, algo.scheduledTms, timeout{s: propose, h: 1, r: 0})

		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, height(1), algo.state.h)
		assert.Equal(t, round(0), algo.state.r)

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

		h, r := height(0), round(0)

		val := app.Value()
		vID := val.Hash()

		val2Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      h,
			R:      r,
			ID:     &vID,
			Sender: *val2,
		}
		val2Proposal := Proposal[value, felt.Felt, felt.Felt]{
			H:          h,
			R:          r,
			ValidRound: -1,
			Value:      &val,
			Sender:     *val2,
		}
		val3Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      h,
			R:      r,
			ID:     &vID,
			Sender: *val3,
		}

		// The node has received all the precommits but has received the corresponding proposal
		algo.messages.addPrecommit(val2Precommit)
		algo.messages.addProposal(val2Proposal)
		algo.messages.addPrecommit(val3Precommit)

		val4Precommit := Precommit[felt.Felt, felt.Felt]{
			H:      h,
			R:      r,
			ID:     &vID,
			Sender: *val4,
		}

		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		precommitListner.send(val4Precommit)

		algo.Start()
		time.Sleep(5 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.scheduledTms))
		assert.Contains(t, algo.scheduledTms, timeout{s: propose, h: 1, r: 0})

		assert.Equal(t, propose, algo.state.s)
		assert.Equal(t, height(1), algo.state.h)
		assert.Equal(t, round(0), algo.state.r)

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

	t.Run("Line 28: receive an old proposal before timeout expiry", func(t *testing.T) {
	})
}
