package rpc

import "github.com/NethermindEth/juno/pkg/types"

type EventRequest struct {
	types.EventFilter
	types.ResultPageRequest
}

type EmittedEventArray []types.EmittedEvent

type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}
