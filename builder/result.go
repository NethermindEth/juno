package builder

import (
	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/types/felt"
)

type BuildResult struct {
	Preconfirmed   *core.PreConfirmed
	SimulateResult *blockchain.SimulateResult
	L2GasConsumed  uint64
}

func (b *BuildResult) ProposalCommitment() (types.ProposalCommitment, error) {
	version, err := semver.NewVersion(b.Preconfirmed.Block.ProtocolVersion)
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	return types.ProposalCommitment{
		BlockNumber:           b.Preconfirmed.Block.Number,
		Builder:               *b.Preconfirmed.Block.SequencerAddress,
		ParentCommitment:      *b.Preconfirmed.Block.ParentHash,
		Timestamp:             b.Preconfirmed.Block.Timestamp,
		ProtocolVersion:       *version,
		OldStateRoot:          *b.Preconfirmed.StateUpdate.OldRoot,
		StateDiffCommitment:   *b.SimulateResult.BlockCommitments.StateDiffCommitment,
		TransactionCommitment: *b.SimulateResult.BlockCommitments.TransactionCommitment,
		EventCommitment:       *b.SimulateResult.BlockCommitments.EventCommitment,
		ReceiptCommitment:     *b.SimulateResult.BlockCommitments.ReceiptCommitment,
		ConcatenatedCounts:    b.SimulateResult.ConcatCount,
		L1GasPriceFRI:         *b.Preconfirmed.Block.L1GasPriceSTRK,
		L1DataGasPriceFRI:     *b.Preconfirmed.Block.L1DataGasPrice.PriceInFri,
		L2GasPriceFRI:         *b.Preconfirmed.Block.L2GasPrice.PriceInFri,
		L2GasUsed:             *new(felt.Felt).SetUint64(b.L2GasConsumed),
		L1DAMode:              b.Preconfirmed.Block.L1DAMode,
	}, nil
}
