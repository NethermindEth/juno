package state

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

const contractDataSize = 3*felt.Bytes + 8

type stateContract struct {
	Nonce        felt.Felt // Contract's nonce
	ClassHash    felt.Felt // Hash of the contract's class
	StorageRoot  felt.Felt // Root hash of the contract's storage
	DeployHeight uint64    // Block height at which the contract is deployed
}

func newStateContract(
	nonce felt.Felt,
	classHash felt.Felt,
	storageRoot felt.Felt,
	deployHeight uint64,
) *stateContract {
	return &stateContract{
		Nonce:        nonce,
		ClassHash:    classHash,
		StorageRoot:  storageRoot,
		DeployHeight: deployHeight,
	}
}

func newContractDeployed(classHash felt.Felt, deployHeight uint64) *stateContract {
	return &stateContract{
		Nonce:        felt.Zero,
		ClassHash:    classHash,
		StorageRoot:  felt.Zero,
		DeployHeight: deployHeight,
	}
}

// Marshals the contract into a byte slice
func (s *stateContract) MarshalBinary() ([]byte, error) {
	buf := make([]byte, contractDataSize)

	copy(buf[0:felt.Bytes], s.ClassHash.Marshal())
	copy(buf[felt.Bytes:2*felt.Bytes], s.Nonce.Marshal())
	copy(buf[2*felt.Bytes:3*felt.Bytes], s.StorageRoot.Marshal())
	binary.BigEndian.PutUint64(buf[3*felt.Bytes:], s.DeployHeight)

	return buf, nil
}

// Unmarshals the contract from a byte slice
func (s *stateContract) UnmarshalBinary(data []byte) error {
	if len(data) != contractDataSize {
		return fmt.Errorf("invalid length for state contract: got %d, want %d", len(data), contractDataSize)
	}

	s.ClassHash.SetBytes(data[:felt.Bytes])
	s.Nonce.SetBytes(data[felt.Bytes : 2*felt.Bytes])
	s.StorageRoot.SetBytes(data[2*felt.Bytes : 3*felt.Bytes])
	s.DeployHeight = binary.BigEndian.Uint64(data[3*felt.Bytes:])

	return nil
}

// Calculates and returns the commitment of the contract
func (s *stateContract) Commitment() *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(&s.ClassHash, &s.StorageRoot), &s.Nonce), &felt.Zero)
}
