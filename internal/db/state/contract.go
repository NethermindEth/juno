package state

import (
	"bytes"
	"compress/gzip"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

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

func (m *Manager) GetContract(contractHash *felt.Felt) (*types.Contract, error) {
	rawData, err := m.contractDef.Get(contractHash.ByteSlice())
	if err != nil {
		return nil, err
	}
	var codeDefinition CodeDefinition
	if err := proto.Unmarshal(rawData, &codeDefinition); err != nil {
		return nil, err
	}
	var contract types.Contract
	return &contract, json.Unmarshal([]byte(codeDefinition.GetDefinition()), &contract)
}

func (x *Manager) PutContract(contractHash *felt.Felt, contract *types.Contract) error {
	fullDef := contract.FullDef
	var fullDefMap map[string]interface{}
	if len(fullDef) > 0 {
		if err := json.Unmarshal([]byte(fullDef), &fullDefMap); err != nil {
			return err
		}

		program := fullDefMap["program"]
		var c bytes.Buffer
		gz := gzip.NewWriter(&c)
		if _, err := gz.Write([]byte(fmt.Sprintf("%v", program))); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}

		encodedProgram := b64.StdEncoding.EncodeToString(c.Bytes())
		fullDefMap["program"] = encodedProgram

		compressedFullDef, err := json.Marshal(fullDefMap)
		if err != nil {
			return err
		}

		fullDef = compressedFullDef
	}

	codeDefinition := CodeDefinition{
		Definition: string(fullDef),
	}
	rawData, err := proto.Marshal(&codeDefinition)
	if err != nil {
		return err
	}
	return x.contractDef.Put(contractHash.ByteSlice(), rawData)
}
