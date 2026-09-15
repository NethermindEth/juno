package tracecache

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/vm"
)

// FromVM retains Traces and GasConsumed from vm.ExecutionResults.
// Requested InitialReads must be non-nil, even for empty blocks.
//
// It assumes the supplied transactions cover the full block and sets BlockTrace.Complete to true.
// For range execution, use Combine on the [Range] before publication.
func FromVM(
	transactions []core.Transaction,
	result *vm.ExecutionResults,
	initialReads bool,
) (*BlockTrace, error) {
	if len(result.Traces) != len(transactions) {
		return nil, fmt.Errorf(
			"VM returned an unexpected number of transaction traces: expected %d, received %d",
			len(transactions),
			len(result.Traces),
		)
	}
	if len(result.GasConsumed) != len(result.Traces) {
		return nil, fmt.Errorf(
			"VM returned an unexpected number of gas results: expected %d, received %d",
			len(result.Traces),
			len(result.GasConsumed),
		)
	}
	if initialReads && result.InitialReads == nil {
		return nil, errors.New("VM omitted initial reads for block trace")
	}
	block := &BlockTrace{Complete: true, Traces: make([]TransactionTrace, len(transactions))}
	for i := range transactions {
		block.Traces[i] = TransactionTrace{
			Hash:    *transactions[i].Hash(),
			vmTrace: &result.Traces[i],
			Gas:     result.GasConsumed[i],
		}
	}
	if initialReads {
		block.InitialReads = result.InitialReads
	}
	return block, nil
}

// FromFeeder converts a starknet.BlockTrace using kinds and gas from receipts.
func FromFeeder(
	kinds []vm.TransactionType,
	receipts []*core.TransactionReceipt,
	source *starknet.BlockTrace,
) (*BlockTrace, error) {
	if len(kinds) != len(source.Traces) {
		return nil, fmt.Errorf(
			"feeder returned an unexpected number of transaction traces: expected %d, received %d",
			len(kinds),
			len(source.Traces),
		)
	}
	gas := make(map[felt.Felt]*core.GasConsumed, len(receipts))
	for _, receipt := range receipts {
		if receipt.ExecutionResources != nil && receipt.ExecutionResources.TotalGasConsumed != nil {
			gas[*receipt.TransactionHash] = receipt.ExecutionResources.TotalGasConsumed
		}
	}
	block := &BlockTrace{Complete: true, Source: Feeder, Traces: make([]TransactionTrace, len(kinds))}
	for i, kind := range kinds {
		sourceTrace := &source.Traces[i]
		block.Traces[i] = TransactionTrace{
			Hash:        sourceTrace.TransactionHash,
			Type:        kind,
			feederTrace: sourceTrace,
		}
		if receiptGas := gas[sourceTrace.TransactionHash]; receiptGas != nil {
			block.Traces[i].Gas = *receiptGas
		}
	}
	return block, nil
}
