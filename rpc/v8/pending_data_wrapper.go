package rpcv8

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/sync/pendingdata"
)

func (h *Handler) PendingData() (core.PendingData, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil && !errors.Is(err, core.ErrPendingDataNotFound) {
		return nil, err
	}
	// If pending network is polling pending block and running on < 0.14.0
	if err == nil && pending.Variant() == core.PendingBlockVariant {
		return pending, nil
	}

	// If pending data variant is not `Pending` or err is `core.ErrPendingDataNotFound`
	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, err
	}

	emptyPending, err := pendingdata.MakeEmptyPendingForParent(h.bcReader, latestHeader)
	if err != nil {
		return nil, err
	}
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
	pendingData, err := h.syncReader.PendingData()
	if err != nil {
		if errors.Is(err, core.ErrPendingDataNotFound) {
			return h.bcReader.HeadState()
		}
		return nil, nil, err
	}

	if pendingData.Variant() != core.PendingBlockVariant {
		return h.bcReader.HeadState()
	}

	return pendingdata.PendingState(pendingData, h.bcReader)
}
