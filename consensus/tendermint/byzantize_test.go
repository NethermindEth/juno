package tendermint

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

// Todo: remove these comments

// Test cases covered:
// 1. proposer casts two conflicting proposals.
// 2. proposer proposes invalid value.

// Test cases not covered:
// 1. pre-votes for two different valid values
// 2. pre-votes for an invalid value
// ... many more ...

func TestByzantineProposer(t *testing.T) {
	myNode := new(felt.Felt).SetBytes([]byte("my node address"))
	node2, node3, node4 := new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(4)
	tm := func(r uint) time.Duration { return time.Second }

	t.Run("Two valid proposals for same height and round", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*node2)
		vals.addValidator(*node3)
		vals.addValidator(*node4)
		vals.addValidator(*myNode)

		algo := New[value, felt.Felt, felt.Felt](*myNode, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		height, round := uint(0), uint(0)
		validValue1, validValue2 := value(10), value(11)

		firstProposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     height,
			Round:      round,
			ValidRound: nil,
			Value:      &validValue1,
			Sender:     *node2,
		}
		secondProposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     height,
			Round:      round,
			ValidRound: nil,
			Value:      &validValue2,
			Sender:     *node2,
		}

		expectedPrevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue1.Hash()),
				Sender: *myNode,
			},
		}

		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(firstProposal)
		proposalListener.send(secondProposal)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 2, len(algo.messages.proposals[height][round][*node2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*myNode]))
		assert.Equal(t, firstProposal, algo.messages.proposals[height][round][*node2][0])
		assert.Equal(t, secondProposal, algo.messages.proposals[height][round][*node2][1])
		assert.Equal(t, expectedPrevote, algo.messages.prevotes[height][round][*myNode][0])

		assert.Equal(t, prevote, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})

	// Todo [rian]: I would have expected that a nil proposal is stored in the messages
	t.Run("Proposer proposes an invalid value", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*node2)
		vals.addValidator(*node3)
		vals.addValidator(*node4)
		vals.addValidator(*myNode)

		algo := New[value, felt.Felt, felt.Felt](*myNode, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		height, round, inValidValue := uint(0), uint(0), value(123)

		firstProposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     height,
			Round:      round,
			ValidRound: nil,
			Value:      &inValidValue,
			Sender:     *node2,
		}
		expectedPrevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     nil,
				Sender: *myNode,
			},
		}
		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(firstProposal)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 0, len(algo.messages.proposals[height][round][*node2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*myNode]))
		assert.Equal(t, expectedPrevote, algo.messages.prevotes[height][round][*myNode][0])

		assert.Equal(t, prevote, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})
}

func TestByzantinePrevoter(t *testing.T) {
	myNode := new(felt.Felt).SetBytes([]byte("my node address"))
	node2, node3, node4 := new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(4)
	tm := func(r uint) time.Duration { return time.Second }

	t.Run("Multiple equivocation prevotes for same height and round but different values", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*node2)
		vals.addValidator(*node3)
		vals.addValidator(*node4)
		vals.addValidator(*myNode)

		algo := New[value, felt.Felt, felt.Felt](*myNode, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		height, round := uint(0), uint(0)
		validValue1, validValue2, validValue3 := value(10), value(11), value(12)

		proposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     height,
			Round:      round,
			ValidRound: nil,
			Value:      &validValue1,
			Sender:     *node2,
		}

		validPrevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue1.Hash()),
				Sender: *node3,
			},
		}

		equivocationPrevote1 := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue2.Hash()),
				Sender: *node3,
			},
		}
		equivocationPrevote2 := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue2.Hash()),
				Sender: *node3,
			},
		}
		equivocationPrevote3 := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue3.Hash()),
				Sender: *node3,
			},
		}

		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(proposal)

		prevoteListener := listeners.PrevoteListener.(*senderAndReceiver[Prevote[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		prevoteListener.send(validPrevote)
		prevoteListener.send(equivocationPrevote1)
		prevoteListener.send(equivocationPrevote2)
		prevoteListener.send(equivocationPrevote3)

		algo.Start()
		time.Sleep(1 * time.Millisecond)
		algo.Stop()

		assert.Equal(t, 1, len(algo.messages.proposals[height][round][*node2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*myNode]))
		assert.Equal(t, 4, len(algo.messages.prevotes[height][round][*node3]))

		assert.Equal(t, prevote, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})

	// Todo: update if we decide to store invalid proposals
	// Todo: we store the bad prevote, but we don't slash them.
	// Todo [rian]: if *v.ID == *p.ID doesn't work for nils..
	t.Run("Prevotes on an invalid proposal", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*node2)
		vals.addValidator(*node3)
		vals.addValidator(*node4)
		vals.addValidator(*myNode)

		algo := New[value, felt.Felt, felt.Felt](*myNode, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		height, round, inValidValue1 := uint(0), uint(0), value(101)

		invalidProposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     height,
			Round:      round,
			ValidRound: nil,
			Value:      &inValidValue1,
			Sender:     *node2,
		}

		myNilPrevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     nil,
				Sender: *myNode,
			},
		}

		invalidPrevote := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(inValidValue1.Hash()),
				Sender: *node3,
			},
		}
		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		proposalListener.send(invalidProposal)

		algo.Start()
		time.Sleep(10 * time.Millisecond)
		prevoteListener := listeners.PrevoteListener.(*senderAndReceiver[Prevote[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		prevoteListener.send(invalidPrevote)

		algo.Stop()

		assert.Equal(t, 0, len(algo.messages.proposals[height][round][*node2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*myNode]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*node3]))
		assert.Equal(t, invalidPrevote, algo.messages.prevotes[height][round][*node3][0])
		assert.Equal(t, myNilPrevote, algo.messages.prevotes[height][round][*myNode][0])

		assert.Equal(t, prevote, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})

}

func TestByzantinePreCommitter(t *testing.T) {
	myNode := new(felt.Felt).SetBytes([]byte("my node address"))
	node2, node3, node4 := new(felt.Felt).SetUint64(2), new(felt.Felt).SetUint64(3), new(felt.Felt).SetUint64(4)
	tm := func(r uint) time.Duration { return time.Second }

	t.Run("Multiple equivocation precommits for same height and round but different values", func(t *testing.T) {
		listeners, broadcasters := testListenersAndBroadcasters()
		app, chain, vals := newApp(), newChain(), newVals()

		vals.addValidator(*node2)
		vals.addValidator(*node3)
		vals.addValidator(*node4)
		vals.addValidator(*myNode)

		algo := New[value, felt.Felt, felt.Felt](*myNode, app, chain, vals, listeners, broadcasters, tm, tm, tm)

		height, round := uint(0), uint(0)
		validValue1, validValue2, validValue3 := value(10), value(11), value(12)

		proposal := Proposal[value, felt.Felt, felt.Felt]{
			Height:     height,
			Round:      round,
			ValidRound: nil,
			Value:      &validValue1,
			Sender:     *node2,
		}
		validPrevoteNode3 := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue1.Hash()),
				Sender: *node3,
			},
		}
		validPrevoteNode4 := Prevote[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue1.Hash()),
				Sender: *node4,
			},
		}
		validPrecommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue1.Hash()),
				Sender: *node3,
			},
		}
		equivocationPrecommit := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue2.Hash()),
				Sender: *node3,
			},
		}
		equivocationPrecommit2 := Precommit[felt.Felt, felt.Felt]{
			Vote: Vote[felt.Felt, felt.Felt]{
				Height: height,
				Round:  round,
				ID:     utils.Ptr(validValue3.Hash()),
				Sender: *node3,
			},
		}

		proposalListener := listeners.ProposalListener.(*senderAndReceiver[Proposal[value, felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])
		prevoteListener := listeners.PrevoteListener.(*senderAndReceiver[Prevote[felt.Felt, felt.Felt], value,
			felt.Felt, felt.Felt])
		precommitListner := listeners.PrecommitListener.(*senderAndReceiver[Precommit[felt.Felt, felt.Felt],
			value, felt.Felt, felt.Felt])

		proposalListener.send(proposal)
		algo.Start()
		time.Sleep(50 * time.Millisecond)
		prevoteListener.send(validPrevoteNode3)
		prevoteListener.send(validPrevoteNode4)
		time.Sleep(50 * time.Millisecond)
		precommitListner.send(validPrecommit)
		precommitListner.send(equivocationPrecommit)
		precommitListner.send(equivocationPrecommit2)
		time.Sleep(50 * time.Millisecond)
		algo.Stop()
		assert.Equal(t, 1, len(algo.messages.proposals[height][round][*node2]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*myNode]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*node3]))
		assert.Equal(t, 1, len(algo.messages.prevotes[height][round][*node4]))
		assert.Equal(t, 3, len(algo.messages.precommits[height][round][*node3]))
		assert.Equal(t, validPrecommit, algo.messages.precommits[height][round][*node3][0])
		assert.Equal(t, equivocationPrecommit, algo.messages.precommits[height][round][*node3][1])
		assert.Equal(t, equivocationPrecommit2, algo.messages.precommits[height][round][*node3][2])

		assert.Equal(t, precommit, algo.state.step)
		assert.Equal(t, uint(0), algo.state.height)
		assert.Equal(t, uint(0), algo.state.round)
	})
}
