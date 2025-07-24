package votecounter

import "github.com/NethermindEth/juno/consensus/types"

type VoteType uint8

const (
	Prevote VoteType = iota
	Precommit
)

type ballot []bool

type ballotSet[A types.Addr] struct {
	ballots     map[A]ballot
	perVoteType []types.VotingPower
	total       types.VotingPower
}

func newBallotSet[A types.Addr]() ballotSet[A] {
	return ballotSet[A]{
		ballots:     make(map[A]ballot),
		perVoteType: make([]types.VotingPower, 2),
		total:       0,
	}
}

func (b *ballotSet[A]) add(addr *A, addrPower types.VotingPower, voteType VoteType) bool {
	if _, ok := b.ballots[*addr]; !ok {
		b.ballots[*addr] = ballot{false, false}
		b.total += addrPower
	}

	if b.ballots[*addr][voteType] {
		return false
	}

	b.ballots[*addr][voteType] = true
	b.perVoteType[voteType] += addrPower
	return true
}
