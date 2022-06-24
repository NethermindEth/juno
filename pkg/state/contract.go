package state

import (
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

type ContractState struct {
	ContractHash *types.Felt
	StorageRoot  *types.Felt
}

func (c *ContractState) Hash() *types.Felt {
	hashBig := pedersen.Digest(
		pedersen.Digest(
			pedersen.Digest(c.ContractHash.Big(), c.StorageRoot.Big()),
			big.NewInt(0),
		),
		big.NewInt(0),
	)
	hashFelt := types.BigToFelt(hashBig)
	return &hashFelt
}

func (c *ContractState) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (c *ContractState) UnmarshalBinary([]byte) error {
	return nil
}
