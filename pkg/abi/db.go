package abi

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/db"
)

var database db.Databaser

func GetABI(contractAddress string) (*Abi, error) {
	data, err := database.Get([]byte(contractAddress))
	if err != nil {
		return nil, err
	}
	abi := new(Abi)
	if err := json.Unmarshal(data, abi); err != nil {
		return nil, err
	}
	return abi, nil
}

func PutABI(contractAddress string, abi *Abi) error {
	data, err := json.Marshal(abi)
	if err != nil {
		return err
	}
	return database.Put([]byte(contractAddress), data)
}

func HasAbi(contractAddress string) (bool, error) {
	return database.Has([]byte(contractAddress))
}
