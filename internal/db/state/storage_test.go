package state

import (
	"testing"

	"github.com/NethermindEth/juno/internal/db"
)

func TestManager_Storage(t *testing.T) {
	initialData := [...]struct {
		Contract    string
		Storage     Storage
		BlockNumber uint64
	}{
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			Storage{Storage: map[string]string{
				"5": "22b",
				"5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5": "7e5",
			}},
			3,
		},
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			Storage{Storage: map[string]string{
				"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300": "4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
				"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620301": "453ae0c9610197b18b13645c44d3d0a407083d96562e8752aab3fab616cecb0",
				"6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0": "7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
			}},
			5,
		},
	}
	codeDatabase := db.NewKeyValueDb(t.TempDir(), 0)
	codeDefinitionDb := db.NewKeyValueDb(t.TempDir(), 0)
	storageDatabase := db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0))
	manager := NewStateManager(codeDatabase, codeDefinitionDb, storageDatabase)
	for _, data := range initialData {
		manager.PutStorage(data.Contract, data.BlockNumber, &data.Storage)
	}
	tests := [...]struct {
		Contract    string
		BlockNumber uint64
		Ok          bool
		Checks      map[string]string
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
			map[string]string{
				"5": "22b",
			},
		},
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			10000,
			true,
			map[string]string{
				"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300": "4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
			},
		},
	}
	for _, test := range tests {
		obtainedStorage := manager.GetStorage(test.Contract, test.BlockNumber)
		if test.Ok && obtainedStorage == nil {
			t.Errorf("storage of contract %s must not found for bloc %d", test.Contract, test.BlockNumber)
		}
		if obtainedStorage != nil && test.Checks != nil {
			for checkKey, checkValue := range test.Checks {
				resultValue, ok := obtainedStorage.Storage[checkKey]
				if !ok {
					t.Errorf("key %s not found", checkKey)
				}
				if resultValue != checkValue {
					t.Errorf("unexpected key value after Put-Get operations. Contract: %s, Key: %s, Obtained value: %s, Want: %s",
						test.Contract, checkKey, resultValue, resultValue)
				}
			}
		}
	}
	manager.Close()
}
