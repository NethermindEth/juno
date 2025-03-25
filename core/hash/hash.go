package hash

import "github.com/NethermindEth/juno/core/felt"

type Hash felt.Felt

func (h *Hash) AsFelt() *felt.Felt {
	return (*felt.Felt)(h)
}

func (h *Hash) String() string {
	return (*felt.Felt)(h).String()
}
