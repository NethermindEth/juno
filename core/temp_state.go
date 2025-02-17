package core

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/encoder"
)

const globalTrieHeight = 251

var (
	stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion  = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core StateHistoryReader
type StateHistoryReader interface {
	StateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (*felt.Felt, error)
	ContractNonce(addr *felt.Felt) (*felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (*felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClass, error)

	ClassTrie() (*trie.Trie, error)
	ContractTrie() (*trie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error)
}

type DeclaredClass struct {
	At    uint64 // block number at which the class was declared
	Class Class
}

func (d *DeclaredClass) MarshalBinary() ([]byte, error) {
	classEnc, err := encoder.Marshal(d.Class)
	if err != nil {
		return nil, err
	}

	size := 8 + len(classEnc)
	buf := make([]byte, size)
	binary.BigEndian.PutUint64(buf[:8], d.At)
	copy(buf[8:], classEnc)

	return buf, nil
}

func (d *DeclaredClass) UnmarshalBinary(data []byte) error {
	if len(data) < 8 { //nolint:mnd
		return errors.New("data too short to unmarshal DeclaredClass")
	}

	d.At = binary.BigEndian.Uint64(data[:8])
	return encoder.Unmarshal(data[8:], &d.Class)
}
