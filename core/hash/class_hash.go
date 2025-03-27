package hash

import "github.com/NethermindEth/juno/core/felt"

type ClassHash Hash

func (h *ClassHash) AsFelt() *felt.Felt {
	return (*felt.Felt)(h)
}

func (h *ClassHash) String() string {
	return (*felt.Felt)(h).String()
}
