package adapters

import (
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type VoteAdapter[H types.Hash, A types.Addr] interface {
	ToVote(*consensus.Vote) (*types.Vote[H, A], error)
	FromVote(*types.Vote[H, A], consensus.Vote_VoteType) (*consensus.Vote, error)
	FromPrevote(*types.Prevote[H, A]) (*consensus.Vote, error)
	FromPrecommit(*types.Precommit[H, A]) (*consensus.Vote, error)
}

type starknetVoteAdapter struct{}

var StarknetVoteAdapter VoteAdapter[hash.Hash, address.Address] = starknetVoteAdapter{}

func (a starknetVoteAdapter) ToVote(vote *consensus.Vote) (*starknet.Vote, error) {
	proposalCommitment := vote.GetProposalCommitment().GetElements()
	if proposalCommitment == nil {
		return nil, errors.New("proposal commitment is nil")
	}

	voter := vote.GetVoter().GetElements()
	if voter == nil {
		return nil, errors.New("voter is nil")
	}

	return &starknet.Vote{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(vote.GetBlockNumber()),
			Round:  types.Round(vote.GetRound()),
			Sender: address.Address(felt.FromBytes(voter)),
		},
		ID: utils.HeapPtr(hash.Hash(felt.FromBytes(proposalCommitment))),
	}, nil
}

func (a starknetVoteAdapter) FromVote(vote *starknet.Vote, voteType consensus.Vote_VoteType) (*consensus.Vote, error) {
	sender := vote.Sender.AsFelt().Bytes()
	id := vote.ID.AsFelt().Bytes()
	return &consensus.Vote{
		VoteType:           voteType,
		BlockNumber:        uint64(vote.MessageHeader.Height),
		Round:              uint32(vote.MessageHeader.Round),
		Voter:              &common.Address{Elements: sender[:]},
		ProposalCommitment: &common.Hash{Elements: id[:]},
	}, nil
}

func (a starknetVoteAdapter) FromPrevote(prevote *starknet.Prevote) (*consensus.Vote, error) {
	return a.FromVote((*starknet.Vote)(prevote), consensus.Vote_Prevote)
}

func (a starknetVoteAdapter) FromPrecommit(precommit *starknet.Precommit) (*consensus.Vote, error) {
	return a.FromVote((*starknet.Vote)(precommit), consensus.Vote_Precommit)
}
