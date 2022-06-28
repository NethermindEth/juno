package state

import (
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

func (m *Manager) GetContractState(address []byte) (*state.ContractState, error) {
    raw, err := m.contractStateDatabase.Get(address)
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
        StorageRoot: &storageRoot,
    }
    return &contractState, nil
}

func (m *Manager) PutContractState(address []byte, cs *state.ContractState) error {
    // Build protobuf struct
    contractStatePB := &ContractState{
        ContractHash: cs.ContractHash.Bytes(),
        StorageRoot: cs.StorageRoot.Bytes(),
    }
    // Marshal to protobuf bytes
    raw, err := proto.Marshal(contractStatePB)
    if err != nil {
        // Protobuf error
        return err
    }
    // Put in database
    return m.contractStateDatabase.Put(address, raw)
} 

