package rpc

import (
	"net/http"
)

const (
	BlockRange = 6
)

func (h *Handler) HandleReadyRequest(w http.ResponseWriter, r *http.Request) {
	if h.ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func (h *Handler) HandleReadySyncRequest(w http.ResponseWriter, r *http.Request) {
	if !h.ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	head, err := h.bcReader.HeadsHeader()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	highestBlockHeader := h.syncReader.HighestBlockHeader()
	if highestBlockHeader == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if head.Number > highestBlockHeader.Number {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if head.Number+BlockRange >= highestBlockHeader.Number {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}
