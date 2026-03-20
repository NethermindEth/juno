package vote

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name          string
	starknetVote  starknet.Message
	consensusVote *consensus.Vote
}

// createTestHashBytes creates the byte representation of a hash
func createTestHashBytes(val uint64) []byte {
	f := felt.FromUint64[felt.Felt](val)
	bytes := f.Bytes()
	return bytes[:]
}

// feltToBytes converts a felt.Felt value to a byte slice
func feltToBytes(f felt.Felt) []byte {
	bytes := f.Bytes()
	return bytes[:]
}

func extractVote(t *testing.T, message starknet.Message) (*starknet.Vote, consensus.Vote_VoteType) {
	switch m := message.(type) {
	case *starknet.Prevote:
		return (*starknet.Vote)(m), consensus.Vote_Prevote
	case *starknet.Precommit:
		return (*starknet.Vote)(m), consensus.Vote_Precommit
	}
	require.Fail(t, "Invalid message type")
	return nil, 0
}

func assertStarknetVote(t *testing.T, expected, actual *starknet.Vote) {
	assert.Equal(t, expected.MessageHeader.Height, actual.MessageHeader.Height, "Height should match original")
	assert.Equal(t, expected.MessageHeader.Round, actual.MessageHeader.Round, "Round should match original")
	assert.Equal(t, expected.MessageHeader.Sender, actual.MessageHeader.Sender, "Sender should match original")

	if expected.ID == nil {
		assert.Nil(t, actual.ID, "ID should be nil")
	} else {
		require.NotNil(t, actual.ID, "ID should not be nil")
		assert.Equal(t, *expected.ID, *actual.ID, "ID should match original")
	}
}

func assertConsensusVote(t *testing.T, expected, actual *consensus.Vote) {
	// Verify the consensus vote matches expected
	assert.Equal(t, expected.VoteType, actual.VoteType, "VoteType should match")
	assert.Equal(t, expected.BlockNumber, actual.BlockNumber, "BlockNumber should match")
	assert.Equal(t, expected.Round, actual.Round, "Round should match")
	assert.Equal(t, expected.Voter.Elements, actual.Voter.Elements, "Voter should match")

	if expected.ProposalCommitment == nil {
		assert.Nil(t, actual.ProposalCommitment, "ProposalCommitment should be nil")
	} else {
		require.NotNil(t, actual.ProposalCommitment, "ProposalCommitment should not be nil")
		assert.Equal(t, expected.ProposalCommitment.Elements, actual.ProposalCommitment.Elements, "ProposalCommitment should match")
	}
}

func TestStarknetVoteAdapter_RoundTrip(t *testing.T) {
	tests := []testCase{
		{
			name: "valid vote with ID - prevote",
			starknetVote: &starknet.Prevote{
				MessageHeader: starknet.MessageHeader{
					Height: 100,
					Round:  5,
					Sender: starknet.Address(felt.Zero),
				},
				ID: felt.NewFromUint64[starknet.Hash](0x123456789abcdef),
			},
			consensusVote: &consensus.Vote{
				VoteType:    consensus.Vote_Prevote,
				BlockNumber: 100,
				Round:       5,
				Voter: &common.Address{
					Elements: feltToBytes(felt.Zero),
				},
				ProposalCommitment: &common.Hash{
					Elements: createTestHashBytes(0x123456789abcdef),
				},
			},
		},
		{
			name: "valid vote with ID - precommit",
			starknetVote: &starknet.Precommit{
				MessageHeader: starknet.MessageHeader{
					Height: types.Height(50),
					Round:  types.Round(3),
					Sender: felt.FromUint64[starknet.Address](42),
				},
				ID: felt.NewFromUint64[starknet.Hash](0xdeadbeefcafebabe),
			},
			consensusVote: &consensus.Vote{
				VoteType:    consensus.Vote_Precommit,
				BlockNumber: 50,
				Round:       3,
				Voter: &common.Address{
					Elements: feltToBytes(felt.FromUint64[felt.Felt](42)),
				},
				ProposalCommitment: &common.Hash{
					Elements: createTestHashBytes(0xdeadbeefcafebabe),
				},
			},
		},
		{
			name: "nil vote (no ID) - prevote",
			starknetVote: &starknet.Prevote{
				MessageHeader: starknet.MessageHeader{
					Height: types.Height(200),
					Round:  types.Round(10),
					Sender: felt.FromUint64[starknet.Address](999),
				},
				ID: nil,
			},
			consensusVote: &consensus.Vote{
				VoteType:           consensus.Vote_Prevote,
				BlockNumber:        200,
				Round:              10,
				Voter:              &common.Address{Elements: feltToBytes(felt.FromUint64[felt.Felt](999))},
				ProposalCommitment: nil,
			},
		},
		{
			name: "nil vote (no ID) - precommit",
			starknetVote: &starknet.Precommit{
				MessageHeader: starknet.MessageHeader{
					Height: types.Height(300),
					Round:  types.Round(15),
					Sender: felt.FromUint64[starknet.Address](1234),
				},
				ID: nil,
			},
			consensusVote: &consensus.Vote{
				VoteType:           consensus.Vote_Precommit,
				BlockNumber:        300,
				Round:              15,
				Voter:              &common.Address{Elements: feltToBytes(felt.FromUint64[felt.Felt](1234))},
				ProposalCommitment: nil,
			},
		},
		{
			name: "zero height and round",
			starknetVote: &starknet.Prevote{
				MessageHeader: starknet.MessageHeader{
					Height: types.Height(0),
					Round:  types.Round(0),
					Sender: felt.FromUint64[starknet.Address](100),
				},
				ID: nil,
			},
			consensusVote: &consensus.Vote{
				VoteType:           consensus.Vote_Prevote,
				BlockNumber:        0,
				Round:              0,
				Voter:              &common.Address{Elements: feltToBytes(felt.FromUint64[felt.Felt](100))},
				ProposalCommitment: nil,
			},
		},
		{
			name: "max values",
			starknetVote: &starknet.Prevote{
				MessageHeader: starknet.MessageHeader{
					Height: types.Height(^uint64(0)),
					Round:  types.Round(^uint32(0)),
					Sender: felt.FromUint64[starknet.Address](^uint64(0)),
				},
				ID: felt.NewFromUint64[starknet.Hash](^uint64(0)),
			},
			consensusVote: &consensus.Vote{
				VoteType:    consensus.Vote_Prevote,
				BlockNumber: ^uint64(0),
				Round:       ^uint32(0),
				Voter: &common.Address{
					Elements: feltToBytes(felt.FromUint64[felt.Felt](^uint64(0))),
				},
				ProposalCommitment: &common.Hash{
					Elements: createTestHashBytes(^uint64(0)),
				},
			},
		},
		{
			name: "empty hash",
			starknetVote: &starknet.Precommit{
				MessageHeader: starknet.MessageHeader{
					Height: types.Height(1),
					Round:  types.Round(1),
					Sender: starknet.Address(felt.Zero),
				},
				ID: felt.NewFromUint64[starknet.Hash](0),
			},
			consensusVote: &consensus.Vote{
				VoteType:    consensus.Vote_Precommit,
				BlockNumber: 1,
				Round:       1,
				Voter: &common.Address{
					Elements: feltToBytes(felt.Zero),
				},
				ProposalCommitment: &common.Hash{
					Elements: createTestHashBytes(0),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Case 1: starknet.Vote -> consensus.Vote -> starknet.Vote (round trip)
			t.Run("Starknet -> Consensus -> Starknet round trip", func(t *testing.T) {
				// Convert starknet.Vote to consensus.Vote
				starknetVote, voteType := extractVote(t, tt.starknetVote)
				consensusResult, err := StarknetVoteAdapter.FromVote(starknetVote, voteType)
				require.NoError(t, err, "FromVote should not return error")

				// Verify the consensus vote matches expected
				assertConsensusVote(t, tt.consensusVote, &consensusResult)

				// Convert back to starknet.Vote
				starknetResult, err := StarknetVoteAdapter.ToVote(&consensusResult)
				require.NoError(t, err, "ToVote should not return error")

				// Verify the original starknet vote is preserved
				assertStarknetVote(t, starknetVote, &starknetResult)
			})

			// Test Case 2: consensus.Vote -> starknet.Vote -> consensus.Vote (round trip)
			t.Run("Consensus -> Starknet -> Consensus round trip", func(t *testing.T) {
				// Convert consensus.Vote to starknet.Vote
				starknetVote, voteType := extractVote(t, tt.starknetVote)
				starknetResult, err := StarknetVoteAdapter.ToVote(tt.consensusVote)
				require.NoError(t, err, "ToVote should not return error")

				// Verify the starknet vote matches expected
				assertStarknetVote(t, starknetVote, &starknetResult)

				// Convert back to consensus.Vote
				consensusResult, err := StarknetVoteAdapter.FromVote(&starknetResult, voteType)
				require.NoError(t, err, "FromVote should not return error")

				// Verify the original consensus vote is preserved
				assertConsensusVote(t, tt.consensusVote, &consensusResult)
			})
		})
	}
}

type errorTestCase struct {
	name           string
	errorSubstring string
	consensusVote  *consensus.Vote
}

func TestStarknetVoteAdapter_ErrorCases(t *testing.T) {
	t.Run("ToVote error cases", func(t *testing.T) {
		tests := []errorTestCase{
			{
				name:           "nil voter",
				errorSubstring: "voter is nil",
				consensusVote: &consensus.Vote{
					VoteType:           consensus.Vote_Prevote,
					BlockNumber:        100,
					Round:              5,
					Voter:              nil,
					ProposalCommitment: nil,
				},
			},
			{
				name:           "voter with nil elements",
				errorSubstring: "voter is nil",
				consensusVote: &consensus.Vote{
					VoteType:    consensus.Vote_Prevote,
					BlockNumber: 100,
					Round:       5,
					Voter: &common.Address{
						Elements: nil,
					},
					ProposalCommitment: nil,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := StarknetVoteAdapter.ToVote(tt.consensusVote)

				assert.Error(t, err, "ToVote should return error")
				assert.Contains(t, err.Error(), tt.errorSubstring, "Error message should contain expected substring")
				assert.Equal(t, starknet.Vote{}, result, "Result should be zero when error occurs")
			})
		}
	})
}
