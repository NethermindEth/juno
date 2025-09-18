package address

import "github.com/NethermindEth/juno/core/types/felt"

type ContractAddress Address

func (a *ContractAddress) AsFelt() *felt.Felt {
	return (*felt.Felt)(a)
}

func (a *ContractAddress) String() string {
	return (*felt.Felt)(a).String()
}
