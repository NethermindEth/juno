package pendingdata

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

const BlockHashLag uint64 = 10

var (
	BlockHashStorageContract         = &felt.One
	ErrUnsupportedPendingDataVariant = errors.New("unsupported pending data variant")
)

// needsPreConfirmed reports whether the given protocol version string
// indicates blocks >= v0.14.0 (i.e., pre_confirmed path is required).
func NeedsPreConfirmed(protocolVersion string) (bool, error) {
	ver, err := core.ParseBlockVersion(protocolVersion)
	if err != nil {
		return false, err
	}
	return ver.GreaterThanEqual(core.Ver0_14_0), nil
}

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

func MakeEmptyPendingForParent(
	bcReader blockchain.Reader,
	latestHeader *core.Header,
) (core.Pending, error) {
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
		return core.Pending{}, err
	}

	pending := core.Pending{
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
) (core.PreConfirmed, error) {
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
		return core.PreConfirmed{}, err
	}

	preConfirmed := core.PreConfirmed{
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

func MakeEmptyPendingDataForParent(
	bcReader blockchain.Reader,
	latestHeader *core.Header,
) (core.PendingData, error) {
	needPreConfirmed, err := NeedsPreConfirmed(latestHeader.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	if needPreConfirmed {
		preConfirmed, err := MakeEmptyPreConfirmedForParent(bcReader, latestHeader)
		if err != nil {
			return nil, err
		}
		return &preConfirmed, nil
	}

	pending, err := MakeEmptyPendingForParent(bcReader, latestHeader)
	if err != nil {
		return nil, err
	}
	return &pending, nil
}

func ResolvePendingBaseState(
	pending *core.Pending,
	stateReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	return stateReader.StateAtBlockHash(pending.Block.ParentHash)
}

// ResolvePreConfirmedBaseState resolves the base state for pre-confirmed blocks
func ResolvePreConfirmedBaseState(
	preConfirmed *core.PreConfirmed,
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

// ResolvePendingDataBaseState determines the appropriate base state for pending data
// and returns the state reader along with its closer function.
func ResolvePendingDataBaseState(
	pending core.PendingData,
	stateReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	switch p := pending.(type) {
	case *core.PreConfirmed:
		return ResolvePreConfirmedBaseState(p, stateReader)
	case *core.Pending:
		return ResolvePendingBaseState(p, stateReader)
	default:
		return nil, nil, ErrUnsupportedPendingDataVariant
	}
}

// PendingState is a convenience function that combines
// base state resolution with pending state creation
func PendingState(
	pending core.PendingData,
	stateReader blockchain.Reader,
) (core.StateReader, blockchain.StateCloser, error) {
	baseState, baseStateCloser, err := ResolvePendingDataBaseState(pending, stateReader)
	if err != nil {
		return nil, nil, err
	}

	return pending.PendingState(baseState), baseStateCloser, nil
}

// PendingStateBeforeIndex is a convenience function that combines
// base state resolution with pending state before index creation
func PendingStateBeforeIndex(
	pending core.PendingData,
	stateReader blockchain.Reader,
	index uint,
) (core.StateReader, blockchain.StateCloser, error) {
	baseState, baseStateCloser, err := ResolvePendingDataBaseState(pending, stateReader)
	if err != nil {
		return nil, nil, err
	}

	pendingStateReader, err := pending.PendingStateBeforeIndex(baseState, index)
	if err != nil {
		// Clean up base state if pending state creation fails
		_ = baseStateCloser()
		return nil, nil, err
	}

	return pendingStateReader, baseStateCloser, nil
}
