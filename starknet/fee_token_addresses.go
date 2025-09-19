package starknet

import "github.com/NethermindEth/juno/core/felt"

// FeeTokenAddresses holds the L2 token contract addresses for STRK and ETH as returned by the
// feeder gateway's "get_contract_addresses" endpoint. These addresses are used to
// identify the respective fee tokens on the network.
type FeeTokenAddresses struct {
	StrkL2TokenAddress felt.Felt `json:"strk_l2_token_address"`
	EthL2TokenAddress  felt.Felt `json:"eth_l2_token_address"`
}
