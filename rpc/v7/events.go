package rpcv7

import (
	"github.com/NethermindEth/juno/core/felt"
)

type Event struct {
	From *felt.Felt   `json:"from_address,omitempty"`
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}
