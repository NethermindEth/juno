package address

import "github.com/NethermindEth/juno/core/felt"

type Address felt.Felt

func (a *Address) AsFelt() *felt.Felt {
	return (*felt.Felt)(a)
}

func (a *Address) String() string {
	return (*felt.Felt)(a).String()
}
