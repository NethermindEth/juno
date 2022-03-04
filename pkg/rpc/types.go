package rpc

import cmd "github.com/NethermindEth/juno/cmd/starknet"

type EventRequest struct {
	cmd.EventFilter
	cmd.ResultPageRequest
}

type EmittedEventArray []cmd.EmittedEvent

type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}
