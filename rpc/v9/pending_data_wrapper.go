package rpcv9

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commonstate"
	"github.com/NethermindEth/juno/sync"
)

func (h *Handler) PendingData() (core.PendingData, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil && !errors.Is(err, sync.ErrPendingBlockNotFound) {
		return nil, err
	}

	if err == nil {
		return pending, nil
	}

	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, err
	}

	blockVer, err := core.ParseBlockVersion(latestHeader.ProtocolVersion)
	if err != nil {
		return nil, err
	}

	if blockVer.GreaterThanEqual(core.Ver0_14_0) {
		emptyPreConfirmed := emptyPreConfirmedForParent(latestHeader)
		return &emptyPreConfirmed, nil
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

func (h *Handler) PendingState() (commonstate.StateReader, func() error, error) {
	state, closer, err := h.syncReader.PendingState()
	if err != nil {
		if errors.Is(err, sync.ErrPendingBlockNotFound) {
			return h.bcReader.HeadState()
		}
		return nil, nil, err
	}

	return state, closer, nil
}

func (h *Handler) PendingBlockFinalityStatus() TxnFinalityStatus {
	pending, err := h.PendingData()
	if err != nil {
		return 0
	}

	switch pending.Variant() {
	case core.PreConfirmedBlockVariant:
		return TxnPreConfirmed
	case core.PendingBlockVariant:
		return TxnAcceptedOnL2
	}

	return 0
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

func emptyPreConfirmedForParent(parentHeader *core.Header) core.PreConfirmed {
	receipts := make([]*core.TransactionReceipt, 0)
	preConfirmedBlock := &core.Block{
		// pre_confirmed block does not have parent hash
		Header: &core.Header{
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

	preConfirmed := core.PreConfirmed{
		Block: preConfirmedBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   parentHeader.GlobalStateRoot,
			StateDiff: stateDiff,
		},
		NewClasses:            make(map[felt.Felt]core.Class, 0),
		TransactionStateDiffs: make([]*core.StateDiff, 0),
		CandidateTxs:          make([]core.Transaction, 0),
	}

	return preConfirmed
}
