package tendermint

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestByzantine(t *testing.T) {
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
