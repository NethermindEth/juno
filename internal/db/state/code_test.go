package state

import (
	"bytes"
	"embed"
	"encoding/json"
	db2 "github.com/NethermindEth/juno/internal/db"
	"math/big"
	"testing"
)

//go:embed test_assets/contract-codes
var contractCodesAssets embed.FS

func loadContractCodePaths() ([]string, error) {
	items, err := contractCodesAssets.ReadDir("test_assets/contract-codes")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, item := range items {
		if !item.IsDir() {
			paths = append(paths, "test_assets/contract-codes/"+item.Name())
		}
	}
	return paths, nil
}

func TestContractCode_UnmarshalJSON(t *testing.T) {
	paths, err := loadContractCodePaths()
	if err != nil {
		t.Error(err)
	}
	var tests []struct {
		Contract string `json:"contract"`
		Asserts  []struct {
			Index int    `json:"index"`
			Value string `json:"value"`
		}
		Bytecode json.RawMessage `json:"bytecode"`
	}
	for _, path := range paths {
		rawData, err := contractCodesAssets.ReadFile(path)
		if err != nil {
			t.Error(err)
		}
		var test struct {
			Contract string `json:"contract"`
			Asserts  []struct {
				Index int    `json:"index"`
				Value string `json:"value"`
			}
			Bytecode json.RawMessage `json:"bytecode"`
		}
		if err := json.Unmarshal(rawData, &test); err != nil {
			t.Error(err)
		}
		tests = append(tests, test)
	}

	for _, test := range tests {
		var code ContractCode
		if err := json.Unmarshal(test.Bytecode, &code); err != nil {
			t.Error(err)
		}
		for _, x := range test.Asserts {
			want, ok := new(big.Int).SetString(x.Value[2:], 16)
			if !ok {
				t.Error("invalid test case")
			}
			if code[x.Index].Cmp(want) != 0 {
				t.Errorf("the contract code with contract address %s have at index %d of the bytecode the value %s after UnmarshalJSON, want: %s",
					test.Contract,
					x.Index,
					"0x"+code[x.Index].Text(16),
					x.Value,
				)
			}
		}
	}
}

func TestContractCode_MarshalJSON(t *testing.T) {
	paths, err := loadContractCodePaths()
	if err != nil {
		t.Error(err)
	}
	var tests []struct {
		Contract string `json:"contract"`
		Asserts  []struct {
			Index int    `json:"index"`
			Value string `json:"value"`
		}
		Bytecode json.RawMessage `json:"bytecode"`
	}
	for _, path := range paths {
		rawData, err := contractCodesAssets.ReadFile(path)
		if err != nil {
			t.Error(err)
		}
		var test struct {
			Contract string `json:"contract"`
			Asserts  []struct {
				Index int    `json:"index"`
				Value string `json:"value"`
			}
			Bytecode json.RawMessage `json:"bytecode"`
		}
		if err := json.Unmarshal(rawData, &test); err != nil {
			t.Error(err)
		}
		tests = append(tests, test)
	}

	for _, test := range tests {
		var code ContractCode
		if err := json.Unmarshal(test.Bytecode, &code); err != nil {
			t.Error(err)
		}
		rawCode, err := json.Marshal(&code)
		if err != nil {
			t.Error(err)
		}
		var codeMap []string
		if err := json.Unmarshal(rawCode, &codeMap); err != nil {
			t.Error(err)
		}
		for _, x := range test.Asserts {
			if codeMap[x.Index] != x.Value {
				t.Errorf("the contract code with contract address %s have at index %d of the bytecode the value %s after MarshalJSON, want: %s",
					test.Contract,
					x.Index,
					"0x"+code[x.Index].Text(16),
					x.Value,
				)
			}
		}
	}
}

func TestManager_Code(t *testing.T) {
	paths, err := loadContractCodePaths()
	if err != nil {
		t.Error(err)
	}
	var tests []struct {
		Contract string `json:"contract"`
		Asserts  []struct {
			Index int    `json:"index"`
			Value string `json:"value"`
		}
		Bytecode json.RawMessage `json:"bytecode"`
	}
	for _, path := range paths {
		rawData, err := contractCodesAssets.ReadFile(path)
		if err != nil {
			t.Error(err)
		}
		var test struct {
			Contract string `json:"contract"`
			Asserts  []struct {
				Index int    `json:"index"`
				Value string `json:"value"`
			}
			Bytecode json.RawMessage `json:"bytecode"`
		}
		if err := json.Unmarshal(rawData, &test); err != nil {
			t.Error(err)
		}
		tests = append(tests, test)
	}
	codeDatabase := db2.NewKeyValueDb(t.TempDir(), 0)
	storageDatabase := db2.NewBlockSpecificDatabase(db2.NewKeyValueDb(t.TempDir(), 0))
	manager := NewStateManager(codeDatabase, *storageDatabase)
	for _, test := range tests {
		var code ContractCode
		if err := json.Unmarshal(test.Bytecode, &code); err != nil {
			t.Error(err)
		}
		beforeRaw, err := json.Marshal(&code)
		if err != nil {
			t.Error(err)
		}
		manager.PutCode(test.Contract, &code)
		obtainedCode := manager.GetCode(test.Contract)
		afterRaw, err := json.Marshal(obtainedCode)
		if err != nil {
			t.Error(err)
		}
		if bytes.Compare(beforeRaw, afterRaw) != 0 {
			t.Errorf("contract code for contract %s mistmash after Put-Get operation with the manager", test.Contract)
		}
	}
}
