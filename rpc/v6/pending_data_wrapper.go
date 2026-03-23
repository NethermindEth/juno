package rpcv6

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/sync/pendingdata"
)

func (h *Handler) PendingData() (core.PendingData, error) {
	_, err := h.syncReader.PendingData()
	if err != nil && !errors.Is(err, core.ErrPendingDataNotFound) {
		return nil, err
	}

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
	_, err := h.syncReader.PendingData()
	if err != nil {
		if errors.Is(err, core.ErrPendingDataNotFound) {
			return h.bcReader.HeadState()
		}
		return nil, nil, err
	}

	return h.bcReader.HeadState()
}
