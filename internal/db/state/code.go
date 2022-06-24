package state

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// GetBinaryCode returns the binary code associated with the given contract address.
// If the contract code is not found, then nil is returned.
func (x *Manager) GetBinaryCode(contractAddress []byte) *Code {
	rawData, err := x.binaryCodeDatabase.Get(contractAddress)
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
func (x *Manager) PutBinaryCode(contractAddress []byte, code *Code) {
	rawData, err := proto.Marshal(code)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	if err := x.binaryCodeDatabase.Put(contractAddress, rawData); err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}

func (x *Manager) GetCodeDefinition(contractHash []byte) *CodeDefinition {
	rawData, err := x.codeDefinitionDatabase.Get(contractHash)
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

func (x *Manager) PutCodeDefinition(contractHash []byte, codeDefinition *CodeDefinition) {
	rawData, err := proto.Marshal(codeDefinition)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	if err := x.codeDefinitionDatabase.Put(contractHash, rawData); err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}
