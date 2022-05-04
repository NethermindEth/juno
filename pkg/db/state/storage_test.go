package state

import (
	"bytes"
	"github.com/NethermindEth/juno/pkg/db"
	"math/big"
	"testing"
)

func TestContractAddress_Marshal(t *testing.T) {
	var tests = [...]struct {
		Address string
	}{
		{"1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac"},
		{"5f28c66afd8a6799ddbe1933bce2c144625031aafa881fa38fa830790eff204"},
		{"004ba0bca2ee5c2842a5529a1548f4c18fb6378deb54300fe38caeb88072b30d"},
		{"04a195621eab23e04dc7357e598eb06ea64830f2542d7bfa0b412a578134738f"},
	}
	for _, test := range tests {
		want, ok := new(big.Int).SetString(test.Address, 16)
		if !ok {
			t.Errorf("error parsing contract address %s", test.Address)
		}
		contractAddress := ContractAddress(test.Address)
		obtainedRaw, err := contractAddress.Marshal()
		if err != nil {
			t.Error(err)
		}
		if bytes.Compare(obtainedRaw, want.Bytes()) != 0 {
			t.Errorf("json.Marshal(%s) = %v, want: %v", contractAddress, obtainedRaw, want.Bytes())
		}
	}
}

func fromString(contractAddress string) big.Int {
	k, _ := new(big.Int).SetString(contractAddress, 16)
	return *k
}

func TestContractStorage_Marshal(t *testing.T) {
	var tests = [...]struct {
		ContractStorage ContractStorage
		Want            []byte
	}{
		{
			make([]contractStorageItem, 0),
			[]byte("{}"),
		},
		{
			[]contractStorageItem{
				{
					Key:   fromString("5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5"),
					Value: fromString("7e5"),
				},
			},
			[]byte("{\"5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5\":\"7e5\"}"),
		},
	}
	for i, test := range tests {
		result, err := test.ContractStorage.Marshal()
		if err != nil {
			t.Error(err)
		}
		if bytes.Compare(result, test.Want) != 0 {
			t.Errorf("marshal for contract storage at test index %d returns %s, want: %s", i, result, test.Want)
		}
	}
}

func TestContractStorage_Unmarshal(t *testing.T) {
	var tests = [...]struct {
		RawData []byte
		Want    ContractStorage
	}{
		{
			[]byte("{}"),
			make([]contractStorageItem, 0),
		},
		{
			[]byte("{\"5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5\":\"7e5\"}"),
			[]contractStorageItem{
				{
					Key:   fromString("5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5"),
					Value: fromString("7e5"),
				},
			},
		},
	}
	for _, test := range tests {
		contractStorage := new(ContractStorage)
		err := contractStorage.Unmarshal(test.RawData)
		if err != nil {
			t.Error(err)
		}
		for _, item := range test.Want {
			found := false
			for _, newItem := range *contractStorage {
				if newItem.Key.Cmp(&item.Key) == 0 {
					found = true
					if newItem.Value.Cmp(&item.Value) != 0 {
						t.Errorf("value for key %s is %s, want %s", item.Key.Text(16), newItem.Value.Text(16), item.Value.Text(16))
					}
				}
			}
			if !found {
				t.Errorf("key not found %s", item.Key.Text(16))
			}
		}
	}
}

func TestManager_Storage(t *testing.T) {
	var initialData = [...]struct {
		Contract    string
		Storage     ContractStorage
		BlockNumber uint64
	}{
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			[]contractStorageItem{
				{
					Key:   fromString("5"),
					Value: fromString("22b"),
				},
				{
					Key:   fromString("5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5"),
					Value: fromString("7e5"),
				},
			},
			3,
		},
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			[]contractStorageItem{
				{
					Key:   fromString("313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300"),
					Value: fromString("4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36"),
				},
				{
					Key:   fromString("313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620301"),
					Value: fromString("453ae0c9610197b18b13645c44d3d0a407083d96562e8752aab3fab616cecb0"),
				},
				{
					Key:   fromString("6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0"),
					Value: fromString("7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240"),
				},
			},
			5,
		},
	}
	codeDatabase := db.New(t.TempDir(), 0)
	storageDatabase := db.NewBlockSpecificDatabase(db.New(t.TempDir(), 0))
	manager := NewStateManager(codeDatabase, *storageDatabase)
	for _, data := range initialData {
		manager.PutStorage(data.Contract, data.BlockNumber, &data.Storage)
	}
	var tests = [...]struct {
		Contract    string
		BlockNumber uint64
		Ok          bool
		Checks      []struct {
			Key   string
			Value string
		}
	}{
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			0,
			false,
			nil,
		},
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			3,
			true,
			[]struct {
				Key   string
				Value string
			}{
				{
					"5",
					"22b",
				},
			},
		},
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			10000,
			true,
			[]struct {
				Key   string
				Value string
			}{
				{
					"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300",
					"4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
				},
			},
		},
	}
	for _, test := range tests {
		obtainedStorage, ok := manager.GetStorage(test.Contract, test.BlockNumber)
		if ok != test.Ok {
			t.Errorf("storage of contract %s must not found for bloc %d", test.Contract, test.BlockNumber)
		}
		if !ok && obtainedStorage != nil {
			t.Errorf("obtained storage must nil if ok is false, in contract %s", test.Contract)
		}
		if ok && obtainedStorage == nil {
			t.Errorf("obtained storage must not nil if ok is true, in contract %s", test.Contract)
		}
		if ok && test.Checks != nil {
			for _, check := range test.Checks {
				found := false
				for _, x := range *obtainedStorage {
					yKey := fromString(check.Key)
					yValue := fromString(check.Value)
					if x.Key.Cmp(&yKey) == 0 {
						found = true
						if x.Value.Cmp(&yValue) != 0 {
							t.Errorf("unexpected key value afeter Put-Get operations. Contract: %s, Key: %s, Obtained value: %s, Want: %s",
								test.Contract, x.Key.Text(16), x.Value.Text(16), yValue.Text(16))
						}
					}
				}
				if !found {
					t.Errorf("key not found: %s for contract: %s", check.Key, test.Contract)
				}
			}
		}
	}
}
