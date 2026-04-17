package builder

import (
	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
)

type BuildResult struct {
	PreConfirmed   *pending.PreConfirmed
	SimulateResult *blockchain.SimulateResult
	L2GasConsumed  uint64
}

func (b *BuildResult) ProposalCommitment() (types.ProposalCommitment, error) {
	version, err := semver.NewVersion(b.PreConfirmed.Block.ProtocolVersion)
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	return types.ProposalCommitment{
		BlockNumber:           b.PreConfirmed.Block.Number,
		Builder:               *b.PreConfirmed.Block.SequencerAddress,
		ParentCommitment:      *b.PreConfirmed.Block.ParentHash,
		Timestamp:             b.PreConfirmed.Block.Timestamp,
		ProtocolVersion:       *version,
		OldStateRoot:          *b.PreConfirmed.StateUpdate.OldRoot,
		StateDiffCommitment:   *b.SimulateResult.BlockCommitments.StateDiffCommitment,
		TransactionCommitment: *b.SimulateResult.BlockCommitments.TransactionCommitment,
		EventCommitment:       *b.SimulateResult.BlockCommitments.EventCommitment,
		ReceiptCommitment:     *b.SimulateResult.BlockCommitments.ReceiptCommitment,
		ConcatenatedCounts:    b.SimulateResult.ConcatCount,
		L1GasPriceFRI:         *b.PreConfirmed.Block.L1GasPriceSTRK,
		L1DataGasPriceFRI:     *b.PreConfirmed.Block.L1DataGasPrice.PriceInFri,
		L2GasPriceFRI:         *b.PreConfirmed.Block.L2GasPrice.PriceInFri,
		L2GasUsed:             *new(felt.Felt).SetUint64(b.L2GasConsumed),
		L1DAMode:              b.PreConfirmed.Block.L1DAMode,
	}, nil
}
