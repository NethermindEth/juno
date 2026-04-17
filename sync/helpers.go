package sync

import (
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
)

const BlockHashLag uint64 = 10

var BlockHashStorageContract = &felt.One

// makeStateDiffForEmptyBlock constructs a minimal state diff for an empty block.
// It optionally writes a historical block hash mapping when blockNumber >= blockHashLag.
func makeStateDiffForEmptyBlock(bc blockchain.Reader, blockNumber uint64) (*core.StateDiff, error) {
	stateDiff := &core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt, 1),
		Nonces:            make(map[felt.Felt]*felt.Felt, 0),
		DeployedContracts: make(map[felt.Felt]*felt.Felt, 0),
		DeclaredV0Classes: make([]*felt.Felt, 0),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt, 0),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt, 0),
		MigratedClasses:   make(map[felt.SierraClassHash]felt.CasmClassHash, 0),
	}

	if blockNumber < BlockHashLag {
		return stateDiff, nil
	}

	header, err := bc.BlockHeaderByNumber(blockNumber - BlockHashLag)
	if err != nil {
		return nil, err
	}

	stateDiff.StorageDiffs[*BlockHashStorageContract] = map[felt.Felt]*felt.Felt{
		*new(felt.Felt).SetUint64(header.Number): header.Hash,
	}
	return stateDiff, nil
}

// Deprecated: MakeEmptyPendingForParent constructs an empty pending.Pending placeholder used by
// rpc/v8 to serve "pending" block ID requests. Remove when rpc/v8 is deprecated.
func MakeEmptyPendingForParent(
	bcReader blockchain.Reader,
	latestHeader *core.Header,
) (pending.Pending, error) {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       latestHeader.Hash,
			SequencerAddress: latestHeader.SequencerAddress,
			Number:           latestHeader.Number + 1,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  latestHeader.ProtocolVersion,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    latestHeader.L1GasPriceETH,
			L1GasPriceSTRK:   latestHeader.L1GasPriceSTRK,
			L2GasPrice:       latestHeader.L2GasPrice,
			L1DataGasPrice:   latestHeader.L1DataGasPrice,
			L1DAMode:         latestHeader.L1DAMode,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff, err := makeStateDiffForEmptyBlock(bcReader, latestHeader.Number+1)
	if err != nil {
		return pending.Pending{}, err
	}

	pending := pending.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   latestHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.ClassDefinition, 0),
	}
	return pending, nil
}

func MakeEmptyPreConfirmedForParent(
	bcReader blockchain.Reader,
	latestHeader *core.Header,
) (pending.PreConfirmed, error) {
	receipts := make([]*core.TransactionReceipt, 0)
	preConfirmedBlock := &core.Block{
		// pre_confirmed block does not have parent hash
		Header: &core.Header{
			SequencerAddress: latestHeader.SequencerAddress,
			Number:           latestHeader.Number + 1,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  latestHeader.ProtocolVersion,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    latestHeader.L1GasPriceETH,
			L1GasPriceSTRK:   latestHeader.L1GasPriceSTRK,
			L2GasPrice:       latestHeader.L2GasPrice,
			L1DataGasPrice:   latestHeader.L1DataGasPrice,
			L1DAMode:         latestHeader.L1DAMode,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff, err := makeStateDiffForEmptyBlock(bcReader, latestHeader.Number+1)
	if err != nil {
		return pending.PreConfirmed{}, err
	}

	preConfirmed := pending.PreConfirmed{
		Block: preConfirmedBlock,
		StateUpdate: &core.StateUpdate{
			StateDiff: stateDiff,
		},
		NewClasses:            make(map[felt.Felt]core.ClassDefinition, 0),
		TransactionStateDiffs: make([]*core.StateDiff, 0),
		CandidateTxs:          make([]core.Transaction, 0),
	}

	return preConfirmed, nil
}

// ResolvePreConfirmedBaseState resolves the base state for pre-confirmed blocks
func ResolvePreConfirmedBaseState(
	preConfirmed *pending.PreConfirmed,
	stateReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	preLatest := preConfirmed.PreLatest
	// If pre-latest exists, use its parent hash as the base state
	if preLatest != nil {
		return stateReader.StateAtBlockHash(preLatest.Block.ParentHash)
	}

	// Otherwise, use the parent of the pre-confirmed block
	blockNumber := preConfirmed.Block.Number
	if blockNumber > 0 {
		return stateReader.StateAtBlockNumber(blockNumber - 1)
	}

	// For genesis block (number 0), use zero hash to get empty state
	return stateReader.StateAtBlockHash(&felt.Zero)
}

// PendingState is a convenience function that combines
// base state resolution with pending state creation
func PendingState(
	preConfirmed *pending.PreConfirmed,
	stateReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	baseState, baseStateCloser, err := ResolvePreConfirmedBaseState(preConfirmed, stateReader)
	if err != nil {
		return nil, nil, err
	}

	return preConfirmed.PendingState(baseState), baseStateCloser, nil
}

// PendingStateBeforeIndex is a convenience function that combines
// base state resolution with pending state before index creation
func PendingStateBeforeIndex(
	preConfirmed *pending.PreConfirmed,
	stateReader blockchain.Reader,
	index uint,
) (core.StateReader, blockchain.StateCloser, error) {
	baseState, baseStateCloser, err := ResolvePreConfirmedBaseState(preConfirmed, stateReader)
	if err != nil {
		return nil, nil, err
	}

	pendingStateReader, err := preConfirmed.PendingStateBeforeIndex(baseState, index)
	if err != nil {
		// Clean up base state if pending state creation fails
		_ = baseStateCloser()
		return nil, nil, err
	}

	return pendingStateReader, baseStateCloser, nil
}
