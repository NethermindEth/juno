package proposer

import (
	"errors"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type ProposerAdapter[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	ProposalInit(*types.Proposal[V, H, A]) (types.ProposalInit, error)
	ProposalBlockInfo(*builder.BuildResult) (types.BlockInfo, error)
	ProposalTransactions(*builder.BuildResult) ([]types.Transaction, error)
	ProposalCommitment(*builder.BuildResult) (types.ProposalCommitment, error)
	ProposalFin(*types.Proposal[V, H, A]) (types.ProposalFin, error)
}

type starknetProposerAdapter struct{}

func NewStarknetProposerAdapter() ProposerAdapter[starknet.Value, starknet.Hash, starknet.Address] {
	return &starknetProposerAdapter{}
}

func (a *starknetProposerAdapter) ProposalInit(proposal *starknet.Proposal) (types.ProposalInit, error) {
	return types.ProposalInit{
		BlockNum:   proposal.Height,
		Round:      proposal.Round,
		ValidRound: proposal.ValidRound,
		Proposer:   felt.Felt(proposal.Sender),
	}, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalBlockInfo(buildResult *builder.BuildResult) (types.BlockInfo, error) {
	return types.BlockInfo{
		BlockNumber:       buildResult.Preconfirmed.Block.Number,
		Builder:           *buildResult.Preconfirmed.Block.SequencerAddress,
		Timestamp:         buildResult.Preconfirmed.Block.Timestamp,
		L2GasPriceFRI:     *buildResult.Preconfirmed.Block.L2GasPrice.PriceInFri,
		L1GasPriceWEI:     *buildResult.Preconfirmed.Block.L1GasPriceETH,
		L1DataGasPriceWEI: *buildResult.Preconfirmed.Block.L1DataGasPrice.PriceInWei,
		EthToStrkRate:     felt.One, // TODO: Double check if this is used
		L1DAMode:          buildResult.Preconfirmed.Block.L1DAMode,
	}, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalTransactions(buildResult *builder.BuildResult) ([]types.Transaction, error) {
	transactions := make([]types.Transaction, len(buildResult.Preconfirmed.Block.Transactions))
	for i := range buildResult.Preconfirmed.Block.Transactions {
		var class core.ClassDefinition
		var paidFeeOnL1 *felt.Felt

		switch tx := buildResult.Preconfirmed.Block.Transactions[i].(type) {
		case *core.DeclareTransaction:
			var ok bool
			if class, ok = buildResult.Preconfirmed.NewClasses[*tx.ClassHash]; !ok {
				return nil, errors.New("class not found")
			}
		case *core.L1HandlerTransaction:
			paidFeeOnL1 = felt.One.Clone()
		}

		transactions[i] = types.Transaction{
			Transaction: buildResult.Preconfirmed.Block.Transactions[i],
			Class:       class,
			PaidFeeOnL1: paidFeeOnL1,
		}
	}

	return transactions, nil
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalCommitment(buildResult *builder.BuildResult) (types.ProposalCommitment, error) {
	return buildResult.ProposalCommitment()
}

// TODO: Implement this function properly
func (a *starknetProposerAdapter) ProposalFin(proposal *starknet.Proposal) (types.ProposalFin, error) {
	return types.ProposalFin(proposal.Value.Hash()), nil
}
