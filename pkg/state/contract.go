package state

import (
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

// ErrInvalidLength is returned when the length of the bytes retrieved from the db is not valid.
var ErrInvalidLength = errors.New("invalid length")

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
	data := make([]byte, types.FeltLength*2)
	copy(data[:types.FeltLength], c.ContractHash.Bytes())
	copy(data[types.FeltLength:], c.StorageRoot.Bytes())
	return data, nil
}

func (c *ContractState) UnmarshalBinary(data []byte) error {
	if len(data) != 2*types.FeltLength {
		return ErrInvalidLength
	}
	contractHash := types.BytesToFelt(data[:types.FeltLength])
	storageRoot := types.BytesToFelt(data[types.FeltLength:])
	c.ContractHash, c.StorageRoot = &contractHash, &storageRoot
	return nil
}
