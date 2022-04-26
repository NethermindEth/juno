package abi

import (
	"bytes"
	"encoding/json"
	"github.com/NethermindEth/juno/pkg/db"
	"testing"
)

func loadABIsFromFiles() ([]Abi, error) {
	var result []Abi
	paths, err := loadABIPaths()
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		rawData, err := testAssets.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var abi Abi
		if err := json.Unmarshal(rawData, &abi); err != nil {
			return nil, err
		}
		result = append(result, abi)
	}
	return result, nil
}

type testManagerPutABI struct {
	Abi             Abi
	ContractAddress string
	BlockNumber     uint64
	Error           bool
	Panic           bool
}

func TestManager(t *testing.T) {
	// Init the ABI manager
	database := db.New(t.TempDir(), 0)
	manager := NewABIManager(database)
	// Load ABI test files
	abis, err := loadABIsFromFiles()
	if err != nil {
		t.Error(err)
	}
	// Build the tests
	var tests []testManagerPutABI
	for i, abi := range abis {
		tests = append(tests, testManagerPutABI{
			Abi:             abi,
			ContractAddress: "05b3796b27c2f4bac1e824f6bee8eb13017a2c4a30ab70ec9a2bd35a63dd0619",
			BlockNumber:     uint64(i),
		})
	}
	// Run all tests
	for _, test := range tests {
		err := manager.PutABI(test.ContractAddress, test.BlockNumber, &test.Abi)
		if err != nil {
			t.Error(err)
		}
		abi, err := manager.GetABI(test.ContractAddress, test.BlockNumber)
		if err != nil {
			t.Error(err)
		}
		raw1, err := json.Marshal(&test.Abi)
		if err != nil {
			t.Error(err)
		}
		raw2, err := json.Marshal(abi)
		if err != nil {
			t.Error(err)
		}
		if bytes.Compare(raw1, raw2) != 0 {
			t.Error("ABI must be equal before and after put/get operations")
		}
	}
}
