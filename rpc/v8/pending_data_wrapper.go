package rpcv8

import (
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/sync"
)

func (h *Handler) PendingData() (*core.PendingData, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil {
		return nil, err
	}
	// If IsPending == true, network is still polling pending block and running on < 0.14.0
	if pending.Variant() == core.PendingBlockVariant {
		return pending, nil
	} else {
		// If IsPending == false, network is still polling pre_confirmed block and running on >= 0.14.0
		// pendingID == latest
		latestHeader, err := h.bcReader.HeadsHeader()
		if err != nil {
			return nil, err
		}

		return emptyPendingForParent(latestHeader).AsPendingData(), nil
	}
}

func (h *Handler) PendingBlock() *core.Block {
	pending, err := h.PendingData()
	if err != nil {
		return nil
	}
	return pending.GetBlock()
}

func emptyPendingForParent(parentHeader *core.Header) *sync.Pending {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       parentHeader.Hash,
			SequencerAddress: parentHeader.SequencerAddress,
			Timestamp:        uint64(time.Now().Unix()),
			ProtocolVersion:  parentHeader.ProtocolVersion,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    parentHeader.L1GasPriceETH,
			L1GasPriceSTRK:   parentHeader.L1GasPriceSTRK,
			L2GasPrice:       parentHeader.L2GasPrice,
			L1DataGasPrice:   parentHeader.L1DataGasPrice,
			L1DAMode:         parentHeader.L1DAMode,
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff := &core.StateDiff{
		StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
		Nonces:            make(map[felt.Felt]*felt.Felt),
		DeployedContracts: make(map[felt.Felt]*felt.Felt),
		DeclaredV0Classes: make([]*felt.Felt, 0),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt),
		ReplacedClasses:   make(map[felt.Felt]*felt.Felt),
	}

	return &sync.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   parentHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}
}
