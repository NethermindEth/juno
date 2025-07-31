package builder

import (
	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/sync"
)

type BuildResult struct {
	Pending        *sync.Pending
	SimulateResult *blockchain.SimulateResult
	L2GasConsumed  uint64
}

func (b *BuildResult) ProposalCommitment() (types.ProposalCommitment, error) {
	version, err := semver.NewVersion(b.Pending.Block.ProtocolVersion)
	if err != nil {
		return types.ProposalCommitment{}, err
	}

	return types.ProposalCommitment{
		BlockNumber:           b.Pending.Block.Number,
		Builder:               *b.Pending.Block.SequencerAddress,
		ParentCommitment:      *b.Pending.Block.ParentHash,
		Timestamp:             b.Pending.Block.Timestamp,
		ProtocolVersion:       *version,
		OldStateRoot:          *b.Pending.StateUpdate.OldRoot,
		StateDiffCommitment:   *b.SimulateResult.BlockCommitments.StateDiffCommitment,
		TransactionCommitment: *b.SimulateResult.BlockCommitments.TransactionCommitment,
		EventCommitment:       *b.SimulateResult.BlockCommitments.EventCommitment,
		ReceiptCommitment:     *b.SimulateResult.BlockCommitments.ReceiptCommitment,
		ConcatenatedCounts:    b.SimulateResult.ConcatCount,
		L1GasPriceFRI:         *b.Pending.Block.L1GasPriceSTRK,
		L1DataGasPriceFRI:     *b.Pending.Block.L1DataGasPrice.PriceInFri,
		L2GasPriceFRI:         *b.Pending.Block.L2GasPrice.PriceInFri,
		L2GasUsed:             *new(felt.Felt).SetUint64(b.L2GasConsumed),
		L1DAMode:              b.Pending.Block.L1DAMode,
	}, nil
}
