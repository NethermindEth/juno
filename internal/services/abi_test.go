package services

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"math/big"
	"testing"
)

//go:embed test_assets/abi/*
var testAssets embed.FS

func loadABIPaths() ([]string, error) {
	items, err := testAssets.ReadDir("test_assets/abi")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, item := range items {
		if !item.IsDir() {
			paths = append(paths, "test_assets/abi/"+item.Name())
		}
	}
	return paths, nil
}

func loadAbis() (map[string]abi.Abi, error) {
	paths, err := loadABIPaths()
	if err != nil {
		return nil, err
	}
	abis := make(map[string]abi.Abi)
	// Generate tests with ABI files
	for _, p := range paths {
		rawData, err := testAssets.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var item struct {
			Contract string
			Abi      json.RawMessage
		}
		if err := json.Unmarshal(rawData, &item); err != nil {
			return nil, err
		}
		var x abi.Abi
		if err := json.Unmarshal(item.Abi, &x); err != nil {
			return nil, err
		}
		abis[item.Contract] = x
	}
	return abis, nil
}

func TestAbiService_StoreGet(t *testing.T) {
	abis, err := loadAbis()
	if err != nil {
		t.Errorf("unexpected error loading the abis: %s", err)
	}
	database := db.NewKeyValueDb(t.TempDir(), 0)
	AbiService.Setup(database)
	if err := AbiService.Run(); err != nil {
		t.Errorf("unexpeted error in Run: %s", err)
	}
	defer AbiService.Close(context.Background())

	for address, value := range abis {
		key, _ := new(big.Int).SetString(address, 16)
		AbiService.StoreAbi(*key, &value)
	}

	for address, value := range abis {
		key, _ := new(big.Int).SetString(address, 16)
		result := AbiService.GetAbi(*key)
		if result == nil {
			t.Errorf("abi not foud for key: %s", address)
		}
		rawValue, err := value.MarshalJSON()
		if err != nil {
			t.Errorf("error marshaling test value")
		}
		rawResult, err := result.MarshalJSON()
		if err != nil {
			t.Errorf("error marshaling GetAbi result")
		}
		if bytes.Compare(rawValue, rawResult) != 0 {
			t.Errorf("abi is diferent after Store/Get operations")
		}
	}
}
