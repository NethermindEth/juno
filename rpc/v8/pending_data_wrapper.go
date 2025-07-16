package rpcv8

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/sync"
)

func (h *Handler) PendingData() (core.PendingData, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil && !errors.Is(err, sync.ErrPendingBlockNotFound) {
		return nil, err
	}
	// If pending network is polling pending block and running on < 0.14.0
	if err == nil && pending.Variant() == core.PendingBlockVariant {
		return pending, nil
	}

	// If pending data variant is not `Pending` or err is `sync.ErrPendingBlockNotFound`
	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, err
	}
	emptyPending := emptyPendingForParent(latestHeader)
	return &emptyPending, nil
}

func (h *Handler) PendingBlock() *core.Block {
	pending, err := h.PendingData()
	if err != nil {
		return nil
	}
	return pending.GetBlock()
}

func (h *Handler) PendingState() (core.StateReader, func() error, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil {
		if errors.Is(err, sync.ErrPendingBlockNotFound) {
			return h.bcReader.HeadState()
		}
		return nil, nil, err
	}

	if pending.Variant() == core.PendingBlockVariant {
		state, closer, err := h.syncReader.PendingState()
		if err != nil {
			if !errors.Is(err, sync.ErrPendingBlockNotFound) {
				return h.bcReader.HeadState()
			}
			return nil, nil, err
		}

		return state, closer, nil
	}

	return h.bcReader.HeadState()
}

func emptyPendingForParent(parentHeader *core.Header) sync.Pending {
	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       parentHeader.Hash,
			Number:           parentHeader.Number + 1,
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

	return sync.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   parentHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}
}
