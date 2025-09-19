package core

import "github.com/NethermindEth/juno/core/felt"

type ContractAddresses struct {
	CoreContractAddress *felt.Felt
	StrkL2TokenAddress  *felt.Felt
	EthL2TokenAddress   *felt.Felt
}
