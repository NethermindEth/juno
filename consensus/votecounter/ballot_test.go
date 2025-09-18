package votecounter

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/stretchr/testify/assert"
)

func TestVoteTypeConstants(t *testing.T) {
	assert.Equal(t, VoteType(0), Prevote)
	assert.Equal(t, VoteType(1), Precommit)
}

func TestBallotType(t *testing.T) {
	// Test ballot initialization
	b := ballot{false, false}
	assert.Len(t, b, 2)
	assert.False(t, b[Prevote])
	assert.False(t, b[Precommit])

	// Test ballot modification
	b[Prevote] = true
	assert.True(t, b[Prevote])
	assert.False(t, b[Precommit])

	b[Precommit] = true
	assert.True(t, b[Prevote])
	assert.True(t, b[Precommit])
}

func TestNewBallotSet(t *testing.T) {
	bs := newBallotSet[starknet.Address]()

	assert.NotNil(t, bs.ballots)
	assert.Len(t, bs.ballots, 0)
	assert.Len(t, bs.perVoteType, 2)
	assert.Equal(t, types.VotingPower(0), bs.perVoteType[Prevote])
	assert.Equal(t, types.VotingPower(0), bs.perVoteType[Precommit])
	assert.Equal(t, types.VotingPower(0), bs.total)
}

func TestBallotSetAdd(t *testing.T) {
	tests := []struct {
		addrIndex      uint64
		addrPower      types.VotingPower
		voteType       VoteType
		expected       bool
		newPerVoteType types.VotingPower
		newTotal       types.VotingPower
	}{
		{
			addrIndex:      1,
			addrPower:      10,
			voteType:       Prevote,
			expected:       true,
			newPerVoteType: 10,
			newTotal:       10,
		},
		{
			addrIndex:      2,
			addrPower:      5,
			voteType:       Precommit,
			expected:       true,
			newPerVoteType: 5,
			newTotal:       15,
		},
		{
			addrIndex:      1,
			addrPower:      10,
			voteType:       Prevote,
			expected:       false,
			newPerVoteType: 10,
			newTotal:       15,
		},
		{
			addrIndex:      1,
			addrPower:      10,
			voteType:       Precommit,
			expected:       true,
			newPerVoteType: 15,
			newTotal:       15,
		},
		{
			addrIndex:      1,
			addrPower:      10,
			voteType:       Precommit,
			expected:       false,
			newPerVoteType: 15,
			newTotal:       15,
		},
	}

	bs := newBallotSet[starknet.Address]()

	for _, tt := range tests {
		var isAble, voteType string

		if tt.expected {
			isAble = "able"
		} else {
			isAble = "not able"
		}

		switch tt.voteType {
		case Prevote:
			voteType = "prevote"
		case Precommit:
			voteType = "precommit"
		}

		testName := fmt.Sprintf("%s to add %s with power %d for address %d", isAble, voteType, tt.addrPower, tt.addrIndex)
		addr := starknet.Address(felt.FromUint64(tt.addrIndex))
		t.Run(testName, func(t *testing.T) {
			assert.Len(t, bs.perVoteType, 2)

			result := bs.add(&addr, tt.addrPower, tt.voteType)
			assert.Equal(t, tt.expected, result)

			assert.Contains(t, bs.ballots, addr)
			assert.True(t, bs.ballots[addr][tt.voteType])

			assert.Equal(t, tt.newPerVoteType, bs.perVoteType[tt.voteType])
			assert.Equal(t, tt.newTotal, bs.total)
		})
	}
}
