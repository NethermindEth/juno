package rpcv6

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/sync"
)

func (h *Handler) Pending() (*core.Pending, error) {
	latestHeader, err := h.bcReader.HeadsHeader()
	if err != nil {
		return nil, err
	}

	emptyPending, err := sync.MakeEmptyPendingForParent(h.bcReader, latestHeader)
	if err != nil {
		return nil, err
	}
	return &emptyPending, nil
}

func (h *Handler) PendingBlock() *core.Block {
	pending, err := h.Pending()
	if err != nil {
		return nil
	}
	return pending.GetBlock()
}
