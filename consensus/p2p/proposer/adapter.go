package proposer

import (
	"iter"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type ProposerAdapter[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	ProposalInit(types.Proposal[V, H, A]) (*consensus.ProposalInit, error)
	ProposalBlockInfo(types.Proposal[V, H, A]) (*consensus.BlockInfo, error)
	ProposalTransactions(types.Proposal[V, H, A]) (iter.Seq[*consensus.ConsensusTransaction], error)
	ProposalCommitment(types.Proposal[V, H, A]) (*consensus.ProposalCommitment, error)
	ProposalFin(types.Proposal[V, H, A]) (*consensus.ProposalFin, error)
}

type starknetProposerAdapter struct{}

func NewStarknetProposerAdapter() ProposerAdapter[starknet.Value, starknet.Hash, starknet.Address] {
	return &starknetProposerAdapter{}
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalInit(proposal starknet.Proposal) (*consensus.ProposalInit, error) {
	var validRound *uint32
	if proposal.ValidRound != -1 {
		validRound = utils.HeapPtr(uint32(proposal.ValidRound))
	}

	senderBytes := proposal.Sender.AsFelt().Bytes()
	sender := &common.Address{
		Elements: senderBytes[:],
	}

	return &consensus.ProposalInit{
		BlockNumber: uint64(proposal.Height),
		Round:       uint32(proposal.Round),
		ValidRound:  validRound,
		Proposer:    sender,
	}, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalBlockInfo(proposal starknet.Proposal) (*consensus.BlockInfo, error) {
	return &consensus.BlockInfo{
		Timestamp: uint64(*proposal.Value),
	}, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalTransactions(proposal starknet.Proposal) (iter.Seq[*consensus.ConsensusTransaction], error) {
	return func(yield func(*consensus.ConsensusTransaction) bool) {}, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalCommitment(proposal starknet.Proposal) (*consensus.ProposalCommitment, error) {
	return &consensus.ProposalCommitment{}, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalFin(proposal starknet.Proposal) (*consensus.ProposalFin, error) {
	hash := proposal.Value.Hash()
	hashBytes := hash.AsFelt().Bytes()
	return &consensus.ProposalFin{
		ProposalCommitment: &common.Hash{
			Elements: hashBytes[:],
		},
	}, nil
}
