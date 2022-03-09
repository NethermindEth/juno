package rpc

import "github.com/NethermindEth/juno/pkg/types"

// EventRequest represent allOf types.EventFilter and types.ResultPageRequest
type EventRequest struct {
	types.EventFilter
	types.ResultPageRequest
}

// EmittedEventArray represent a set of emmited events
type EmittedEventArray []types.EmittedEvent

// EventResponse represent the struct of the response of events
type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}
