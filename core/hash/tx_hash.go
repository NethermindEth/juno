package hash

import "github.com/NethermindEth/juno/core/felt"

type TransactionHash Hash

func (h *TransactionHash) AsFelt() *felt.Felt {
	return (*felt.Felt)(h)
}

func (h *TransactionHash) String() string {
	return (*felt.Felt)(h).String()
}
