package rpcv9

import (
	"github.com/NethermindEth/juno/core"
)

func (h *Handler) PendingData() (*core.PendingData, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil {
		return nil, err
	}
	return pending, nil
}

func (h *Handler) PendingBlock() *core.Block {
	pending, err := h.PendingData()
	if err != nil {
		return nil
	}
	return pending.GetBlock()
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
