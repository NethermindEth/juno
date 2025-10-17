package state

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

const (
	contractEncSize          = 3*felt.Bytes + 8 // nonce + class hash + storage root + deploy height
	contractEmptyRootEncSize = 2*felt.Bytes + 8 // nonce + class hash + deploy height
)

type stateContract struct {
	Nonce          felt.Felt `cbor:"1,keyasint,omitempty"` // Contract's nonce
	ClassHash      felt.Felt `cbor:"2,keyasint,omitempty"` // Hash of the contract's class
	DeployedHeight uint64    `cbor:"3,keyasint,omitempty"` // Block height at which the contract is deployed
	StorageRoot    felt.Felt `cbor:"4,keyasint,omitempty"` // Root hash of the contract's storage
}

func newContractDeployed(classHash felt.Felt, deployHeight uint64) stateContract {
	return stateContract{
		Nonce:          felt.Zero,
		ClassHash:      classHash,
		StorageRoot:    felt.Zero,
		DeployedHeight: deployHeight,
	}
}

// Marshals the contract into a byte slice.
// If the storage root is zero, it will not be included in the marshalled data.
func (s *stateContract) MarshalBinary() ([]byte, error) {
	if s.StorageRoot.IsZero() {
		return s.marshalEmptyRoot()
	}

	return s.marshalFull()
}

func (s *stateContract) marshalFull() ([]byte, error) {
	buf := make([]byte, contractEncSize)

	copy(buf[0:felt.Bytes], s.Nonce.Marshal())
	copy(buf[felt.Bytes:2*felt.Bytes], s.ClassHash.Marshal())
	copy(buf[2*felt.Bytes:3*felt.Bytes], s.StorageRoot.Marshal())
	binary.BigEndian.PutUint64(buf[3*felt.Bytes:], s.DeployedHeight)

	return buf, nil
}

func (s *stateContract) marshalEmptyRoot() ([]byte, error) {
	buf := make([]byte, contractEmptyRootEncSize)

	copy(buf[0:felt.Bytes], s.Nonce.Marshal())
	copy(buf[felt.Bytes:2*felt.Bytes], s.ClassHash.Marshal())
	binary.BigEndian.PutUint64(buf[2*felt.Bytes:], s.DeployedHeight)

	return buf, nil
}

// Unmarshals the contract from a byte slice
func (s *stateContract) UnmarshalBinary(data []byte) error {
	switch len(data) {
	case contractEncSize:
		return s.unmarshalFull(data)
	case contractEmptyRootEncSize:
		return s.unmarshalEmptyRoot(data)
	default:
		return fmt.Errorf("invalid length for state contract: got %d, want %d or %d", len(data), contractEncSize, contractEmptyRootEncSize)
	}
}

func (s *stateContract) unmarshalFull(data []byte) error {
	s.Nonce.SetBytes(data[:felt.Bytes])
	s.ClassHash.SetBytes(data[felt.Bytes : 2*felt.Bytes])
	s.StorageRoot.SetBytes(data[2*felt.Bytes : 3*felt.Bytes])
	s.DeployedHeight = binary.BigEndian.Uint64(data[3*felt.Bytes:])

	return nil
}

func (s *stateContract) unmarshalEmptyRoot(data []byte) error {
	s.Nonce.SetBytes(data[:felt.Bytes])
	s.ClassHash.SetBytes(data[felt.Bytes : 2*felt.Bytes])
	s.DeployedHeight = binary.BigEndian.Uint64(data[2*felt.Bytes:])

	return nil
}

// Calculates and returns the commitment of the contract
func (s *stateContract) commitment() felt.Felt {
	h1 := crypto.Pedersen(&s.ClassHash, &s.StorageRoot)
	h2 := crypto.Pedersen(&h1, &s.Nonce)
	return crypto.Pedersen(&h2, &felt.Zero)
}
