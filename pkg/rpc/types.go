package rpc

import starknet_client "github.com/NethermindEth/juno/pkg/types"

type EventRequest struct {
	starknet_client.EventFilter
	starknet_client.ResultPageRequest
}

type EmittedEventArray []starknet_client.EmittedEvent

type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}
