package vote

import (
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/types/address"
	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/NethermindEth/juno/core/types/hash"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type VoteAdapter[H types.Hash, A types.Addr] interface {
	ToVote(*consensus.Vote) (types.Vote[H, A], error)
	FromVote(*types.Vote[H, A], consensus.Vote_VoteType) (consensus.Vote, error)
}

type starknetVoteAdapter struct{}

var StarknetVoteAdapter VoteAdapter[hash.Hash, address.Address] = starknetVoteAdapter{}

func (a starknetVoteAdapter) ToVote(vote *consensus.Vote) (starknet.Vote, error) {
	var id *starknet.Hash
	if proposalCommitment := vote.GetProposalCommitment().GetElements(); proposalCommitment != nil {
		id = felt.NewFromBytes[hash.Hash](proposalCommitment)
	}

	voter := vote.GetVoter().GetElements()
	if voter == nil {
		return starknet.Vote{}, errors.New("voter is nil")
	}

	return starknet.Vote{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(vote.GetBlockNumber()),
			Round:  types.Round(vote.GetRound()),
			Sender: felt.FromBytes[address.Address](voter),
		},
		ID: id,
	}, nil
}

func (a starknetVoteAdapter) FromVote(vote *starknet.Vote, voteType consensus.Vote_VoteType) (consensus.Vote, error) {
	sender := vote.Sender.AsFelt().Bytes()

	// This is optional since a vote can be NIL.
	var id *common.Hash
	if vote.ID != nil {
		bytes := vote.ID.AsFelt().Bytes()
		id = &common.Hash{Elements: bytes[:]}
	}

	return consensus.Vote{
		VoteType:           voteType,
		BlockNumber:        uint64(vote.MessageHeader.Height),
		Round:              uint32(vote.MessageHeader.Round),
		Voter:              &common.Address{Elements: sender[:]},
		ProposalCommitment: id,
	}, nil
}
