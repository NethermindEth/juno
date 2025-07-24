package votecounter

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/stretchr/testify/assert"
)

func (v *VoteCounter[V, H, A]) AssertVote(t *testing.T, vote *types.Vote[H, A], voteType VoteType) {
	t.Helper()

	assert.Contains(t, v.roundData, vote.Round)
	assert.Contains(t, v.roundData[vote.Round].allVotes.ballots, vote.Sender)
	assert.True(t, v.roundData[vote.Round].allVotes.ballots[vote.Sender][voteType])

	if vote.ID != nil {
		assert.Contains(t, v.roundData[vote.Round].perIDVotes, *vote.ID)
		assert.Contains(t, v.roundData[vote.Round].perIDVotes[*vote.ID].ballots, vote.Sender)
		assert.True(t, v.roundData[vote.Round].perIDVotes[*vote.ID].ballots[vote.Sender][voteType])
	} else {
		assert.Contains(t, v.roundData[vote.Round].nilVotes.ballots, vote.Sender)
		assert.True(t, v.roundData[vote.Round].nilVotes.ballots[vote.Sender][voteType])
	}
}

// assertProposal asserts that the proposal message is in the state machine, except when the state machine advanced to the next height.
func (v *VoteCounter[V, H, A]) AssertProposal(t *testing.T, expectedMsg starknet.Proposal) {
	t.Helper()
	// New height will discard the previous height messages.
	if v.currentHeight > expectedMsg.Height {
		return
	}
	assert.Contains(t, v.roundData, expectedMsg.Round)
	assert.Equal(t, v.roundData[expectedMsg.Round].proposal, &expectedMsg)
}

func (v *VoteCounter[V, H, A]) AssertEmpty(t *testing.T) {
	t.Helper()
	assert.Empty(t, v.roundData)
	assert.NotContains(t, v.futureMessages, v.currentHeight)
}
