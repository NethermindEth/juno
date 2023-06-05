package gateway

import (
	"github.com/NethermindEth/juno/core/felt"
)

type NumAsHex string

// BroadcastedTxn contains common properties for different types of broadcasted transactions
type BroadcastedTxnCmn struct {
	MaxFee *felt.Felt `json:"max_fee" validate:"required"`
	// even though spec says there is version 0
	// in practice even pathfinder doesn't support it
	Version   NumAsHex     `json:"version" validate:"required"`
	Signature []*felt.Felt `json:"signature" validate:"required"`
	Nonce     *felt.Felt   `json:"nonce" validate:"required"`
}

type BroadcastedInvokeTxn struct {
	BroadcastedTxnCmn
	Type          string       `json:"type" validate:"required"`
	SenderAddress *felt.Felt   `json:"sender_address" validate:"required"`
	Calldata      []*felt.Felt `json:"calldata" validate:"required"`
}

type AddInvokeTxResponse struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
}
