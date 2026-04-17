package rpcv8

import (
	"github.com/NethermindEth/juno/core"
	pendingpkg "github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/sync"
)

//nolint:staticcheck // Pending is supported by RPCv8
func (h *Handler) Pending() (*pendingpkg.Pending, error) {
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
