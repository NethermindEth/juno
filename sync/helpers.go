package sync

import (
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
)

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

	if blockNumber < core.BlockHashLag {
		return stateDiff, nil
	}

	header, err := bc.BlockHeaderByNumber(blockNumber - core.BlockHashLag)
	if err != nil {
		return nil, err
	}

	stateDiff.StorageDiffs[*core.BlockHashStorageContract] = map[felt.Felt]*felt.Felt{
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
		BlockIdentifier:       feeder.PreConfirmedBlankIdentifier,
	}

	return preConfirmed, nil
}
