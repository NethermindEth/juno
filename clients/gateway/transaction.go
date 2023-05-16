package gateway

import (
	"github.com/NethermindEth/juno/core/felt"
)

type NumAsHex string

// BroadcastedTxn contains common properties for different types of broadcasted transactions
type BroadcastedTxn struct {
	MaxFee *felt.Felt `json:"max_fee" validate:"required"`
	// even though spec says there is version 0
	// in practice even pathfinder doesn't support it
	Version   NumAsHex     `json:"version" validate:"required"`
	Signature []*felt.Felt `json:"signature" validate:"required"`
	Nonce     *felt.Felt   `json:"nonce" validate:"required"`
}

type InvokeTxnV1 struct {
	SenderAddress *felt.Felt   `json:"sender_address" validate:"required"`
	Calldata      []*felt.Felt `json:"calldata" validate:"required"`
}

type BroadcastedInvokeTxn struct {
	BroadcastedTxn
	Type string `json:"type" validate:"required"`
	InvokeTxnV1
}
