package rpcv10

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

	if err == nil {
		return pending, nil
	}

	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, err
	}

	return pendingdata.MakeEmptyPendingDataForParent(h.bcReader, latestHeader)
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

	return pendingdata.PendingState(pendingData, h.bcReader)
}
