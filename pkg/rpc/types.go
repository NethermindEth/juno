package rpc

import "github.com/NethermindEth/juno/pkg/types"

// TODO: Document.
type EventRequest struct {
	types.EventFilter
	types.ResultPageRequest
}

// TODO: Document.
type EmittedEventArray []types.EmittedEvent

// TODO: Document.
type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}
