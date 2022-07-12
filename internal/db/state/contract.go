package state

import (
	"encoding/json"

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

func (x *Manager) GetContract(contractHash *types.Felt) (*types.Contract, error) {
	rawData, err := x.contractDef.Get(contractHash.Bytes())
	if err != nil {
		return nil, err
	}
	if rawData == nil {
		return nil, nil
	}
	var codeDefinition CodeDefinition
	if err := proto.Unmarshal(rawData, &codeDefinition); err != nil {
		return nil, err
	}
	var contract types.Contract
	return &contract, json.Unmarshal([]byte(codeDefinition.GetDefinition()), &contract)
}

func (x *Manager) PutContract(contractHash *types.Felt, contract *types.Contract) error {
	codeDefinition := CodeDefinition{
		Definition: string(contract.FullDef),
	}
	rawData, err := proto.Marshal(&codeDefinition)
	if err != nil {
		return err
	}
	return x.contractDef.Put(contractHash.Bytes(), rawData)
}
