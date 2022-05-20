package abi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/internal/db"
	"math/big"
	"testing"
)

func loadABIsFromFiles() (result []struct {
	Abi      Abi
	Contract string
}, err error) {
	paths, err := loadABIPaths()
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		rawData, err := testAssets.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var x struct {
			Abi      Abi
			Contract string
		}
		if err := json.Unmarshal(rawData, &x); err != nil {
			return nil, err
		}
		result = append(result, x)
	}
	return result, nil
}

type testManagerPutABI struct {
	Abi             Abi
	ContractAddress big.Int
	Error           bool
	Panic           bool
}

func TestManager(t *testing.T) {
	// Init the ABI manager
	database := db.NewKeyValueDb(t.TempDir(), 0)
	manager := NewABIManager(database)
	// Load ABI test files
	abis, err := loadABIsFromFiles()
	if err != nil {
		t.Error(err)
	}
	// Build the tests
	var tests []testManagerPutABI
	for _, abi := range abis {
		tests = append(tests, testManagerPutABI{
			Abi:             abi.Abi,
			ContractAddress: fromHexString(abi.Contract),
		})
	}
	// Run all tests
	for _, test := range tests {
		manager.PutABI(test.ContractAddress, &test.Abi)
		abi := manager.GetABI(test.ContractAddress)
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

func TestManager_GetABI_NotFound(t *testing.T) {
	// Init the ABI manager
	database := db.NewKeyValueDb(t.TempDir(), 0)
	manager := NewABIManager(database)
	abi := manager.GetABI(fromHexString("1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac"))
	if abi != nil {
		t.Errorf("abi must be nil")
	}
}

func fromHexString(s string) big.Int {
	x, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic(any(fmt.Sprintf("invalid hex string: %s", s)))
	}
	return *x
}
