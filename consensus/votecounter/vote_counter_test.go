package votecounter

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func assertVoteCounter(
	t *testing.T,
	voteCounter *VoteCounter[starknet.Value, starknet.Hash, starknet.Address],
	tests []testCase,
	votingPower map[starknet.Address]types.VotingPower,
) {
	t.Helper()
	allIDs := getAllIDs(tests)
	countVotes := countVotes(tests, votingPower)
	f := f(voteCounter.validators.TotalVotingPower(testHeight))
	q := q(voteCounter.validators.TotalVotingPower(testHeight))

	assert.Equal(
		t,
		countVotes(isVotingNil, isPrevote) >= q,
		voteCounter.HasQuorumForVote(testRound, Prevote, nil),
	)
	assert.Equal(
		t,
		countVotes(isVotingNil, isPrecommit) >= q,
		voteCounter.HasQuorumForVote(testRound, Precommit, nil),
	)

	for _, id := range allIDs {
		assert.Equal(
			t,
			countVotes(isVotingID(id), isPrevote) >= q,
			voteCounter.HasQuorumForVote(testRound, Prevote, &id),
		)
		assert.Equal(
			t,
			countVotes(isVotingID(id), isPrecommit) >= q,
			voteCounter.HasQuorumForVote(testRound, Precommit, &id),
		)

		assert.Equal(
			t,
			countVotes(isVotingID(id), isPrecommit) >= q,
			voteCounter.HasFuturePrecommitQuorum(testHeight+1, testRound, &id),
		)
	}

	assert.Equal(t, countVotes(isPrevote) >= q, voteCounter.HasQuorumForAny(testRound, Prevote))
	assert.Equal(t, countVotes(isPrecommit) >= q, voteCounter.HasQuorumForAny(testRound, Precommit))

	assert.Equal(t, countVotes() > f, voteCounter.HasNonFaultyFutureMessage(testRound))
}

func testMultipleHeights[T any](
	t *testing.T,
	message *T,
	expectReturn bool,
	fn func(*T) bool,
	decreaseHeight func(*T),
	increaseHeight func(*T),
) {
	t.Run("Past height", func(t *testing.T) {
		msg := *message
		decreaseHeight(&msg)
		assert.Equal(t, false, fn(&msg))
	})

	t.Run("Current height", func(t *testing.T) {
		assert.Equal(t, expectReturn, fn(message))
	})

	t.Run("Future height", func(t *testing.T) {
		msg := *message
		increaseHeight(&msg)
		assert.Equal(t, expectReturn, fn(&msg))
	})
}

func TestVoteCounter(t *testing.T) {
	tests := simpleTestScenarios

	t.Run("Add", func(t *testing.T) {
		var firstProposal *starknet.Proposal

		voteCounter := New[starknet.Value](simpleMockValidator, testHeight)
		for i, testCase := range tests {
			t.Run(tests[i].String(), func(t *testing.T) {
				switch testCase := testCase.(type) {
				case *proposalTestCase:
					t.Run("Try with invalid proposer", func(t *testing.T) {
						proposal := testCase.proposal
						proposal.Sender = felt.FromUint64[starknet.Address](uint64(len(tests)))
						assert.False(t, voteCounter.AddProposal(&proposal))
					})

					testMultipleHeights(
						t,
						&testCase.proposal,
						testCase.resultReturn,
						voteCounter.AddProposal,
						func(proposal *starknet.Proposal) {
							proposal.Height--
						},
						func(proposal *starknet.Proposal) {
							proposal.Height++
						},
					)

					if testCase.resultReturn && firstProposal == nil {
						firstProposal = &testCase.proposal
					}
				case *voteTestCase:
					switch testCase.voteType {
					case Prevote:
						testMultipleHeights(
							t,
							(*starknet.Prevote)(&testCase.vote),
							testCase.resultReturn,
							voteCounter.AddPrevote,
							func(prevote *starknet.Prevote) {
								prevote.Height--
							},
							func(prevote *starknet.Prevote) {
								prevote.Height++
							},
						)
					case Precommit:
						testMultipleHeights(
							t,
							(*starknet.Precommit)(&testCase.vote),
							testCase.resultReturn,
							voteCounter.AddPrecommit,
							func(precommit *starknet.Precommit) {
								precommit.Height--
							},
							func(precommit *starknet.Precommit) {
								precommit.Height++
							},
						)

					default:
						t.Fatalf("Unknown vote type: %T", testCase.voteType)
					}
				default:
					t.Fatalf("Unknown test case: %T", testCase)
				}

				assertVoteCounter(t, &voteCounter, tests[:i+1], simpleVotingPower)
				assert.Equal(t, firstProposal, voteCounter.GetProposal(testRound))
			})
		}

		t.Run("StartNewHeight with future messages", func(t *testing.T) {
			voteCounter.StartNewHeight()
			assertVoteCounter(t, &voteCounter, tests, simpleVotingPower)
		})

		t.Run("StartNewHeight with no messages", func(t *testing.T) {
			voteCounter.StartNewHeight()
			assertVoteCounter(t, &voteCounter, nil, simpleVotingPower)
		})
	})

	t.Run("GetProposal with invalid round", func(t *testing.T) {
		voteCounter := New[starknet.Value](simpleMockValidator, testHeight)
		assert.Nil(t, voteCounter.GetProposal(testRound))
	})
}

func TestThresholds(t *testing.T) {
	tests := []struct {
		n types.VotingPower
		q types.VotingPower
		f types.VotingPower
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
