package votecounter

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type roundMap[V types.Hashable[H], H types.Hash, A types.Addr] = map[types.Round]*roundData[V, H, A]

type roundData[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	proposal               *types.Proposal[V, H, A]
	uncountedProposerPower types.VotingPower
	perIDVotes             map[H]*ballotSet[A]
	nilVotes               ballotSet[A]
	allVotes               ballotSet[A]
}

func newRoundData[V types.Hashable[H], H types.Hash, A types.Addr]() roundData[V, H, A] {
	return roundData[V, H, A]{
		proposal:               nil,
		uncountedProposerPower: 0,
		perIDVotes:             make(map[H]*ballotSet[A]),
		nilVotes:               newBallotSet[A](),
		allVotes:               newBallotSet[A](),
	}
}

func (r *roundData[V, H, A]) setProposal(
	proposal *types.Proposal[V, H, A],
	votingPower types.VotingPower,
) bool {
	if r.proposal != nil {
		return false
	}
	r.proposal = proposal

	proposerBallot, ok := r.allVotes.ballots[proposal.Sender]
	if proposerNotVoted := !ok || (!proposerBallot[Prevote] && !proposerBallot[Precommit]); proposerNotVoted {
		r.uncountedProposerPower = votingPower
	}
	return true
}

func (r *roundData[V, H, A]) addVote(
	vote *types.Vote[H, A],
	votingPower types.VotingPower,
	voteType VoteType,
) bool {
	var perVote *ballotSet[A]
	if vote.ID != nil {
		if perVote = r.perIDVotes[*vote.ID]; perVote == nil {
			perVote = utils.HeapPtr(newBallotSet[A]())
			r.perIDVotes[*vote.ID] = perVote
		}
	} else {
		perVote = &r.nilVotes
	}

	if r.uncountedProposerPower > 0 && r.proposal != nil && r.proposal.Sender == vote.Sender {
		r.uncountedProposerPower = 0
	}

	r.allVotes.add(&vote.Sender, votingPower, voteType)
	return perVote.add(&vote.Sender, votingPower, voteType)
}

func (r *roundData[V, H, A]) countVote(voteType VoteType, id *H) types.VotingPower {
	var perVote *ballotSet[A]
	if id != nil {
		if perVote = r.perIDVotes[*id]; perVote == nil {
			return 0
		}
	} else {
		perVote = &r.nilVotes
	}

	return perVote.perVoteType[voteType]
}

func (r *roundData[V, H, A]) countAny(voteType VoteType) types.VotingPower {
	return r.allVotes.perVoteType[voteType]
}

func (r *roundData[V, H, A]) countFutureMessageSenders() types.VotingPower {
	return r.allVotes.total + r.uncountedProposerPower
}
