package votecounter

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

const (
	testHeight = types.Height(1337)
	testRound  = types.Round(1338)
)

type testCase interface {
	fmt.Stringer
	run(*testing.T, *roundData[starknet.Value, starknet.Hash, starknet.Address], map[starknet.Address]types.VotingPower)
	filter(filter ...FilterFn) bool
	getSender() starknet.Address
}

type proposalTestCase struct {
	addrIndex    uint64
	proposal     starknet.Proposal
	resultReturn bool
}

func (c *proposalTestCase) expectSkip() *proposalTestCase {
	c.resultReturn = false
	return c
}

func (c *proposalTestCase) String() string {
	value := felt.Felt(*c.proposal.Value)
	return fmt.Sprintf("validator %d propose %d", c.addrIndex, value.Uint64())
}

func (c *proposalTestCase) run(
	t *testing.T,
	roundData *roundData[starknet.Value, starknet.Hash, starknet.Address],
	votingPower map[starknet.Address]types.VotingPower,
) {
	assert.Equal(t, c.resultReturn, roundData.setProposal(&c.proposal, votingPower[c.proposal.Sender]))
}

func (c *proposalTestCase) filter(filter ...FilterFn) bool {
	return len(filter) == 0
}

func (c *proposalTestCase) getSender() starknet.Address {
	return c.proposal.Sender
}

type voteTestCase struct {
	addrIndex    uint64
	voteType     VoteType
	idIndex      *uint64
	vote         starknet.Vote
	resultReturn bool
}

func newVoteTestCase(addrIndex voter, voteType VoteType, idIndex *uint64) *voteTestCase {
	var id *starknet.Hash
	if idIndex != nil {
		value := starknet.Value(felt.FromUint64(*idIndex))
		id = utils.HeapPtr(value.Hash())
	}

	return &voteTestCase{
		addrIndex: uint64(addrIndex),
		voteType:  voteType,
		idIndex:   idIndex,
		vote: starknet.Vote{
			MessageHeader: starknet.MessageHeader{
				Height: testHeight,
				Round:  testRound,
				Sender: starknet.Address(felt.FromUint64(uint64(addrIndex))),
			},
			ID: id,
		},
		resultReturn: true,
	}
}

func (c *voteTestCase) expectSkip() *voteTestCase {
	c.resultReturn = false
	return c
}

func (c *voteTestCase) String() string {
	var voteType string
	switch c.voteType {
	case Prevote:
		voteType = "prevote"
	case Precommit:
		voteType = "precommit"
	}

	id := "nil"
	if c.idIndex != nil {
		id = fmt.Sprintf("%d", *c.idIndex)
	}

	return fmt.Sprintf("validator %d %s %s", c.addrIndex, voteType, id)
}

func (c *voteTestCase) run(
	t *testing.T,
	roundData *roundData[starknet.Value, starknet.Hash, starknet.Address],
	votingPower map[starknet.Address]types.VotingPower,
) {
	assert.Equal(t, c.resultReturn, roundData.addVote(&c.vote, votingPower[c.vote.Sender], c.voteType))
}

func (c *voteTestCase) filter(filter ...FilterFn) bool {
	for _, f := range filter {
		if !f(&c.vote, c.voteType) {
			return false
		}
	}

	return true
}

func (c *voteTestCase) getSender() starknet.Address {
	return c.vote.Sender
}

type voter uint64

func (v voter) propose(idIndex uint64) *proposalTestCase {
	return &proposalTestCase{
		addrIndex: uint64(v),
		proposal: types.Proposal[starknet.Value, starknet.Hash, starknet.Address]{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: testHeight,
				Round:  testRound,
				Sender: starknet.Address(felt.FromUint64(uint64(v))),
			},
			ValidRound: -1,
			Value:      utils.HeapPtr(starknet.Value(felt.FromUint64(idIndex))),
		},
		resultReturn: true,
	}
}

func (v voter) prevote(id uint64) *voteTestCase {
	return newVoteTestCase(v, Prevote, &id)
}

func (v voter) precommit(id uint64) *voteTestCase {
	return newVoteTestCase(v, Precommit, &id)
}

func (v voter) prevoteNil() *voteTestCase {
	return newVoteTestCase(v, Prevote, nil)
}

func (v voter) precommitNil() *voteTestCase {
	return newVoteTestCase(v, Precommit, nil)
}

type FilterFn func(vote *starknet.Vote, voteType VoteType) bool

func countVotes(
	testCases []testCase,
	votingPower map[starknet.Address]types.VotingPower,
) func(filter ...FilterFn) types.VotingPower {
	return func(filter ...FilterFn) types.VotingPower {
		voters := make(map[starknet.Address]struct{})
		total := types.VotingPower(0)
		for _, testCase := range testCases {
			sender := testCase.getSender()
			if _, ok := voters[sender]; ok {
				continue
			}

			if testCase.filter(filter...) {
				voters[sender] = struct{}{}
				total += votingPower[sender]
			}
		}

		return total
	}
}

func isPrevote(vote *starknet.Vote, voteType VoteType) bool {
	return voteType == Prevote
}

func isPrecommit(vote *starknet.Vote, voteType VoteType) bool {
	return voteType == Precommit
}

func isVotingID(id starknet.Hash) FilterFn {
	return func(vote *starknet.Vote, voteType VoteType) bool {
		return vote.ID != nil && *vote.ID == id
	}
}

func isVotingNil(vote *starknet.Vote, voteType VoteType) bool {
	return vote.ID == nil
}

func buildVotingPower(votingPower ...types.VotingPower) map[starknet.Address]types.VotingPower {
	votingPowerMap := make(map[starknet.Address]types.VotingPower)
	for i, power := range votingPower {
		votingPowerMap[starknet.Address(felt.FromUint64(uint64(i)))] = power
	}
	return votingPowerMap
}

func getAllIDs(testCases []testCase) []starknet.Hash {
	ids := make(map[starknet.Hash]struct{})
	for _, testCase := range testCases {
		switch testCase := testCase.(type) {
		case *voteTestCase:
			if testCase.vote.ID != nil {
				ids[*testCase.vote.ID] = struct{}{}
			}
		case *proposalTestCase:
			ids[testCase.proposal.Value.Hash()] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(ids))
}

func TestRoundData(t *testing.T) {
	votingPower := simpleVotingPower
	tests := simpleTestScenarios
	roundData := newRoundData[starknet.Value, starknet.Hash, starknet.Address]()
	allIDs := getAllIDs(tests)
	for i, testCase := range tests {
		t.Run(testCase.String(), func(t *testing.T) {
			testCase.run(t, &roundData, votingPower)

			countVotes := countVotes(tests[:i+1], votingPower)

			assert.Equal(t, countVotes(isVotingNil, isPrevote), roundData.countVote(Prevote, nil))
			assert.Equal(t, countVotes(isVotingNil, isPrecommit), roundData.countVote(Precommit, nil))

			for _, id := range allIDs {
				assert.Equal(t, countVotes(isVotingID(id), isPrevote), roundData.countVote(Prevote, &id))
				assert.Equal(t, countVotes(isVotingID(id), isPrecommit), roundData.countVote(Precommit, &id))
			}

			assert.Equal(t, countVotes(isPrevote), roundData.countAny(Prevote))
			assert.Equal(t, countVotes(isPrecommit), roundData.countAny(Precommit))

			assert.Equal(t, countVotes(), roundData.countFutureMessageSenders())
		})
	}
}
