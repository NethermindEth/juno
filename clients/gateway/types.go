package gateway

import "github.com/NethermindEth/juno/core/felt"

type InvokeResponse struct {
	Code            string     `json:"code"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}
