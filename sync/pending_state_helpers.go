package sync

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// ResolvePendingDataBaseState determines the appropriate base state for pending data
// and returns the state reader along with its closer function.
func ResolvePendingDataBaseState(
	pending core.PendingData,
	stateReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	switch pending.Variant() {
	case core.PreConfirmedBlockVariant:
		return resolvePreConfirmedBaseState(pending, stateReader)
	case core.PendingBlockVariant:
		// For pending blocks, use the parent hash directly
		return stateReader.StateAtBlockHash(pending.GetBlock().ParentHash)
	default:
		return nil, nil, errors.New("unsupported pending data variant")
	}
}

// resolvePreConfirmedBaseState resolves the base state for pre-confirmed blocks
func resolvePreConfirmedBaseState(
	preConfirmed core.PendingData,
	stateResolver blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	preLatest := preConfirmed.GetPreLatest()

	// If pre-latest exists, use its parent hash as the base state
	if preLatest != nil {
		return stateResolver.StateAtBlockHash(preLatest.Block.ParentHash)
	}

	// Otherwise, use the parent of the pre-confirmed block
	blockNumber := preConfirmed.GetBlock().Number
	if blockNumber > 0 {
		return stateResolver.StateAtBlockNumber(blockNumber - 1)
	}

	// For genesis block (number 0), use zero hash to get empty state
	return stateResolver.StateAtBlockHash(&felt.Zero)
}

// GetPendingStateWithBaseState is a convenience function that combines
// base state resolution with pending state creation
func PendingState(
	pending core.PendingData,
	stateResolver blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	baseState, baseStateCloser, err := ResolvePendingDataBaseState(pending, stateResolver)
	if err != nil {
		return nil, nil, err
	}

	stateReader, err := pending.PendingState(baseState)
	if err != nil {
		// Clean up base state if pending state creation fails
		if closeErr := baseStateCloser(); closeErr != nil {
			return nil, nil, closeErr
		}
		return nil, nil, err
	}

	return stateReader, baseStateCloser, nil
}

// GetPendingStateBeforeIndexWithBaseState is a convenience function that combines
// base state resolution with pending state before index creation
func GetPendingStateBeforeIndexWithBaseState(
	pending core.PendingData,
	index uint,
	stateResolver blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	baseState, baseStateCloser, err := ResolvePendingDataBaseState(pending, stateResolver)
	if err != nil {
		return nil, nil, err
	}

	stateReader, err := pending.PendingStateBeforeIndex(index, baseState)
	if err != nil {
		// Clean up base state if pending state creation fails
		if closeErr := baseStateCloser(); closeErr != nil {
			return nil, nil, closeErr
		}
		return nil, nil, err
	}

	return stateReader, baseStateCloser, nil
}
