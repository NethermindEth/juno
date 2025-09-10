package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
)

func (s *stateMachine[V, H, A]) sendProposal(value *V) actions.Action[V, H, A] {
	proposalMessage := types.Proposal[V, H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: s.state.height,
			Round:  s.state.round,
			Sender: s.nodeAddr,
		},
		ValidRound: s.state.validRound,
		Value:      value,
	}

	s.voteCounter.AddProposal(&proposalMessage)

	return (*actions.BroadcastProposal[V, H, A])(&proposalMessage)
}

func (s *stateMachine[V, H, A]) setStepAndSendPrevote(id *H) actions.Action[V, H, A] {
	vote := types.Prevote[H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: s.state.height,
			Round:  s.state.round,
			Sender: s.nodeAddr,
		},
		ID: id,
	}

	s.voteCounter.AddPrevote(&vote)
	s.state.step = types.StepPrevote

	return (*actions.BroadcastPrevote[H, A])(&vote)
}

func (s *stateMachine[V, H, A]) setStepAndSendPrecommit(id *H) actions.Action[V, H, A] {
	vote := types.Precommit[H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: s.state.height,
			Round:  s.state.round,
			Sender: s.nodeAddr,
		},
		ID: id,
	}

	s.voteCounter.AddPrecommit(&vote)
	s.state.step = types.StepPrecommit

	return (*actions.BroadcastPrecommit[H, A])(&vote)
}
