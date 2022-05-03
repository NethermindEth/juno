package state

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// ContractCode is the database representation of the StarkNet contract code.
// The contract code is stored as a byte[], which is the result of the JSON encoding.
type ContractCode []big.Int

func (c *ContractCode) UnmarshalJSON(data []byte) error {
	var (
		code     []big.Int
		bytecode []string
	)
	if err := json.Unmarshal(data, &bytecode); err != nil {
		return err
	}
	for _, item := range bytecode {
		value, ok := new(big.Int).SetString(item[2:], 16)
		if !ok {
			return fmt.Errorf("error parsing %s[2:] into an big.Int of base 16", item)
		}
		code = append(code, *value)
	}
	*c = code
	return nil
}

func (c *ContractCode) MarshalJSON() ([]byte, error) {
	bytecode := make([]string, 0, len(*c))
	for _, item := range *c {
		value := "0x" + item.Text(16)
		bytecode = append(bytecode, value)
	}
	return json.Marshal(bytecode)
}

// GetCode returns the ContractCode associated with the given contract address.
// If the contract code is not found, then nil is returned. If any error happens the method panics.
func (x *Manager) GetCode(contractAddress string) *ContractCode {
	key, ok := new(big.Int).SetString(contractAddress, 16)
	if !ok {
		panic(any(InvalidContractAddress))
	}
	rawData, err := x.codeDatabase.Get(key.Bytes())
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	if rawData == nil {
		return nil
	}
	var data ContractCode
	if err := json.Unmarshal(rawData, &data); err != nil {
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err)))
	}
	return &data
}

// PutCode stores a new contract code into the database, associated with the given contract address.
// If the contract address already have a contract code in the database, then the value is updated.
func (x *Manager) PutCode(contractAddress string, code *ContractCode) {
	keyInt, ok := new(big.Int).SetString(contractAddress, 16)
	if !ok {
		panic(any(InvalidContractAddress))
	}
	key := keyInt.Bytes()
	rawData, err := json.Marshal(code)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}
	if err := x.codeDatabase.Put(key, rawData); err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
}
