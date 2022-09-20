package state

import (
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"google.golang.org/protobuf/proto"
)

type Program struct {
	EncodedProgram interface{} `json:"program"`
}

func (m *Manager) GetContractState(hash *felt.Felt) (*state.ContractState, error) {
	raw, err := m.stateDatabase.Get(hash.ByteSlice())
	if err != nil {
		// Database error
		return nil, err
	}
	// Unmarshal to protobuf struct
	contractStatePB := &ContractState{}
	err = proto.Unmarshal(raw, contractStatePB)
	if err != nil {
		// Protobuf error
		return nil, err
	}
	// Build output struct
	contractState := state.ContractState{
		ContractHash: new(felt.Felt).SetBytes(contractStatePB.GetContractHash()),
		StorageRoot:  new(felt.Felt).SetBytes(contractStatePB.GetStorageRoot()),
	}
	return &contractState, nil
}

func (m *Manager) PutContractState(cs *state.ContractState) error {
	// Build protobuf struct
	contractStatePB := &ContractState{
		ContractHash: cs.ContractHash.ByteSlice(),
		StorageRoot:  cs.StorageRoot.ByteSlice(),
	}
	// Marshal to protobuf bytes
	raw, err := proto.Marshal(contractStatePB)
	if err != nil {
		// Protobuf error
		return err
	}
	// Put in database
	return m.stateDatabase.Put(cs.Hash().ByteSlice(), raw)
}
