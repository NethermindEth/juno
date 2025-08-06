package votecounter

import (
	"iter"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type Validators[A types.Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(types.Height) types.VotingPower

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(types.Height, *A) types.VotingPower

	// Proposer returns the proposer of the current round and height.
	Proposer(types.Height, types.Round) A
}

type VoteCounter[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	validators        Validators[A]
	currentHeight     types.Height
	totalVotingPower  types.VotingPower
	faultyVotingPower types.VotingPower
	quorumVotingPower types.VotingPower
	roundData         roundMap[V, H, A]
	futureMessages    map[types.Height]roundMap[V, H, A]
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](validators Validators[A], height types.Height) VoteCounter[V, H, A] {
	totalVotingPower := validators.TotalVotingPower(height)
	return VoteCounter[V, H, A]{
		validators:        validators,
		currentHeight:     height,
		totalVotingPower:  totalVotingPower,
		faultyVotingPower: f(totalVotingPower),
		quorumVotingPower: q(totalVotingPower),
		roundData:         make(roundMap[V, H, A]),
		futureMessages:    make(map[types.Height]roundMap[V, H, A]),
	}
}

func (v *VoteCounter[V, H, A]) LoadFromMessages(messages iter.Seq2[types.Message[V, H, A], error]) error {
	for msg, err := range messages {
		if err != nil {
			return err
		}
		switch msg := msg.(type) {
		case *types.Proposal[V, H, A]:
			v.AddProposal(msg)
		case *types.Prevote[H, A]:
			v.AddPrevote(msg)
		case *types.Precommit[H, A]:
			v.AddPrecommit(msg)
		}
	}
	return nil
}

func (v *VoteCounter[V, H, A]) StartNewHeight() {
	v.currentHeight++
	v.totalVotingPower = v.validators.TotalVotingPower(v.currentHeight)
	v.faultyVotingPower = f(v.totalVotingPower)
	v.quorumVotingPower = q(v.totalVotingPower)

	clear(v.roundData)
	var ok bool
	if v.roundData, ok = v.futureMessages[v.currentHeight]; !ok {
		v.roundData = make(roundMap[V, H, A])
	} else {
		delete(v.futureMessages, v.currentHeight)
	}
}

func (v *VoteCounter[V, H, A]) getRoundDataAndVotingPower(header *types.MessageHeader[A]) (*roundData[V, H, A], types.VotingPower, bool) {
	if header.Height < v.currentHeight {
		return nil, 0, false
	}

	votingPower := v.validators.ValidatorVotingPower(header.Height, &header.Sender)

	if header.Height == v.currentHeight {
		return getOrCreateRoundData(v.roundData, header.Round), votingPower, true
	}

	futureRoundMap, ok := v.futureMessages[header.Height]
	if !ok {
		futureRoundMap = make(roundMap[V, H, A])
		v.futureMessages[header.Height] = futureRoundMap
	}
	return getOrCreateRoundData(futureRoundMap, header.Round), votingPower, true
}

func getOrCreateRoundData[V types.Hashable[H], H types.Hash, A types.Addr](
	roundMap map[types.Round]*roundData[V, H, A],
	round types.Round,
) *roundData[V, H, A] {
	entry, ok := roundMap[round]
	if !ok {
		entry = utils.HeapPtr(newRoundData[V, H, A]())
		roundMap[round] = entry
	}
	return entry
}

func (v *VoteCounter[V, H, A]) AddProposal(proposal *types.Proposal[V, H, A]) bool {
	roundData, votingPower, ok := v.getRoundDataAndVotingPower(&proposal.MessageHeader)
	if !ok {
		return false
	}

	if expectedProposer := v.validators.Proposer(proposal.Height, proposal.Round); types.AddrCmp(proposal.Sender, expectedProposer) {
		return false
	}
	return roundData.setProposal(proposal, votingPower)
}

func (v *VoteCounter[V, H, A]) AddPrevote(prevote *types.Prevote[H, A]) bool {
	roundData, votingPower, ok := v.getRoundDataAndVotingPower(&prevote.MessageHeader)
	if !ok {
		return false
	}

	return roundData.addVote((*types.Vote[H, A])(prevote), votingPower, Prevote)
}

func (v *VoteCounter[V, H, A]) AddPrecommit(precommit *types.Precommit[H, A]) bool {
	roundData, votingPower, ok := v.getRoundDataAndVotingPower(&precommit.MessageHeader)
	if !ok {
		return false
	}
	return roundData.addVote((*types.Vote[H, A])(precommit), votingPower, Precommit)
}

func (v *VoteCounter[V, H, A]) GetProposal(round types.Round) *types.Proposal[V, H, A] {
	roundData, ok := v.roundData[round]
	if !ok {
		return nil
	}

	return roundData.proposal
}

func (v *VoteCounter[V, H, A]) HasQuorumForVote(round types.Round, voteType VoteType, id *H) bool {
	roundData, ok := v.roundData[round]
	if !ok {
		return false
	}
	return roundData.countVote(voteType, id) >= v.quorumVotingPower
}

func (v *VoteCounter[V, H, A]) HasQuorumForAny(round types.Round, voteType VoteType) bool {
	roundData, ok := v.roundData[round]
	if !ok {
		return false
	}

	return roundData.countAny(voteType) >= v.quorumVotingPower
}

func (v *VoteCounter[V, H, A]) HasNonFaultyFutureMessage(round types.Round) bool {
	roundData, ok := v.roundData[round]
	if !ok {
		return false
	}

	return roundData.countFutureMessageSenders() > v.faultyVotingPower
}

func (v *VoteCounter[V, H, A]) Proposer(round types.Round) A {
	return v.validators.Proposer(v.currentHeight, round)
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower types.VotingPower) types.VotingPower {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower types.VotingPower) types.VotingPower {
	// Unfortunately there is no ceiling function for integers in go.
	d := totalVotingPower * 2
	q := d / 3
	r := d % 3
	if r > 0 {
		q++
	}
	return q
}
