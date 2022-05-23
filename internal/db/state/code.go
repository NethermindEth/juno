package state

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

func (x *Code) Equal(y *Code) bool {
	if x == y {
		return true
	}
	if len(x.Code) != len(y.Code) {
		return false
	}
	for i, inst := range y.Code {
		if inst != y.Code[i] {
			return false
		}
	}
	return true
}

// GetCode returns the ContractCode associated with the given contract address.
// If the contract code is not found, then nil is returned.
func (x *Manager) GetCode(contractAddress string) *Code {
	rawData, err := x.codeDatabase.Get([]byte(contractAddress))
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
func (x *Manager) PutCode(contractAddress string, code *Code) {
	rawData, err := proto.Marshal(code)
	if err != nil {
		panic(any(fmt.Errorf("marshal error: %s", err)))
	}
	if err := x.codeDatabase.Put([]byte(contractAddress), rawData); err != nil {
		panic(any(fmt.Errorf("database error: %s", err)))
	}
}
