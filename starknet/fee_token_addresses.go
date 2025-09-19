package starknet

import "github.com/NethermindEth/juno/core/felt"

// ContractAddresses represents the partial response from the "get_contract_addresses" endpoint
type FeeTokenAddresses struct {
	StrkL2TokenAddress *felt.Felt `json:"strk_l2_token_address"`
	EthL2TokenAddress  *felt.Felt `json:"eth_l2_token_address"`
}
