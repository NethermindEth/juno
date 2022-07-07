package state

import (
	"fmt"

	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

func (m *Manager) GetContractState(hash *types.Felt) (*state.ContractState, error) {
	raw, err := m.stateDatabase.Get(hash.Bytes())
	if err != nil {
		// Database error
		return nil, err
	}
	if raw == nil {
		// Not found
		return nil, nil
	}
	// Unmarshal to protobuf struct
	contractStatePB := &ContractState{}
	err = proto.Unmarshal(raw, contractStatePB)
	if err != nil {
		// Protobuf error
		return nil, err
	}
	// Build output struct
	contractHash := types.BytesToFelt(contractStatePB.GetContractHash())
	storageRoot := types.BytesToFelt(contractStatePB.GetStorageRoot())
	contractState := state.ContractState{
		ContractHash: &contractHash,
		StorageRoot:  &storageRoot,
	}
	return &contractState, nil
}

func (m *Manager) PutContractState(cs *state.ContractState) error {
	// Build protobuf struct
	contractStatePB := &ContractState{
		ContractHash: cs.ContractHash.Bytes(),
		StorageRoot:  cs.StorageRoot.Bytes(),
	}
	// Marshal to protobuf bytes
	raw, err := proto.Marshal(contractStatePB)
	if err != nil {
		// Protobuf error
		return err
	}
	// Put in database
	return m.stateDatabase.Put(cs.Hash().Bytes(), raw)
}

// GetBinaryCode returns the binary code associated with the given contract address.
// If the contract code is not found, then nil is returned.
func (m *Manager) GetBinaryCode(contractAddress []byte) *Code {
	rawData, err := m.binaryCodeDatabase.Get(contractAddress)
	if err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
	if rawData == nil {
		// notest
		return nil
	}
	code := new(Code)
	if err := proto.Unmarshal(rawData, code); err != nil {
		panic(any(fmt.Errorf("unmarshal error: %s", err)))
	}
	return code
}

// PutBinaryCode stores a new contract binary code into the database, associated with the
// given contract address. If the contract address already have a contract code
// in the database, then the value is updated.
func (m *Manager) PutBinaryCode(contractAddress []byte, code *Code) {
	rawData, err := proto.Marshal(code)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	if err := m.binaryCodeDatabase.Put(contractAddress, rawData); err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}

func (m *Manager) GetCodeDefinition(contractHash []byte) *CodeDefinition {
	rawData, err := m.codeDefinitionDatabase.Get(contractHash)
	if err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
	if rawData == nil {
		return nil
	}
	codeDefinition := new(CodeDefinition)
	if err := proto.Unmarshal(rawData, codeDefinition); err != nil {
		panic(any(fmt.Errorf("unmarshal error: %s", err)))
	}
	return codeDefinition
}

func (m *Manager) PutCodeDefinition(contractHash []byte, codeDefinition *CodeDefinition) {
	rawData, err := proto.Marshal(codeDefinition)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	if err := m.codeDefinitionDatabase.Put(contractHash, rawData); err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}
