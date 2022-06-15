package state

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// GetCode returns the ContractCode associated with the given contract address.
// If the contract code is not found, then nil is returned.
func (x *Manager) GetCode(contractAddress []byte) *Code {
	rawData, err := x.codeDatabase.Get(contractAddress)
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

// PutCode stores a new contract code into the database, associated with the
// given contract address. If the contract address already have a contract code
// in the database, then the value is updated.
func (x *Manager) PutCode(contractAddress []byte, code *Code) {
	rawData, err := proto.Marshal(code)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	if err := x.codeDatabase.Put(contractAddress, rawData); err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}
