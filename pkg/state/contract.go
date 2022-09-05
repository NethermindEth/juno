package state

import (
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/felt"
)

type ContractState struct {
	ContractHash *felt.Felt
	StorageRoot  *felt.Felt
	Nonce        *felt.Felt
}

func (c *ContractState) Hash() *felt.Felt {
	return pedersen.Digest(
		pedersen.Digest(
			pedersen.Digest(c.ContractHash, c.StorageRoot),
			new(felt.Felt),
		),
		new(felt.Felt),
	)
}
