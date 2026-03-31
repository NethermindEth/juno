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

	emptyPreConfirmed, err := pendingdata.MakeEmptyPreConfirmedForParent(h.bcReader, latestHeader)
	if err != nil {
		return nil, err
	}
	return &emptyPreConfirmed, nil
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
