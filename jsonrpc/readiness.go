package jsonrpc

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/sync"
	"net/http"
)

const SyncBlockRange = 6

type ReadinessHandlers struct {
	bcReader   blockchain.Reader
	syncReader sync.Reader
}

func NewReadinessHandlers(bcReader blockchain.Reader, syncReader sync.Reader) *ReadinessHandlers {
	return &ReadinessHandlers{
		bcReader:   bcReader,
		syncReader: syncReader,
	}
}

func (h *ReadinessHandlers) HandleReady(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *ReadinessHandlers) HandleReadySync(w http.ResponseWriter, r *http.Request) {
	if !h.isSynced() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ReadinessHandlers) isSynced() bool {
	head, err := h.bcReader.HeadsHeader()
	if err != nil {
		return false
	}
	highestBlockHeader := h.syncReader.HighestBlockHeader()
	if highestBlockHeader == nil {
		return false
	}

	if head.Number > highestBlockHeader.Number {
		return false
	}

	if head.Number+SyncBlockRange >= highestBlockHeader.Number {
		return true
	} else {
		return false
	}
}
