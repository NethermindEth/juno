package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/state"
	"google.golang.org/protobuf/proto"
)

var codes = []struct {
	Address []byte
	Code    *state.Code
}{
	{
		Address: decodeString("1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac"),
		Code: &state.Code{Code: [][]byte{
			decodeString("40780017fff7fff"),
			decodeString("1"),
			decodeString("208b7fff7fff7ffe"),
			decodeString("400380007ffb7ffc"),
			decodeString("400380017ffb7ffd"),
			decodeString("800000000000010fffffffffffffffffffffffffffffffffffffffffffffffb"),
			decodeString("107a2e2e5a8b6552e977246c45bfac446305174e86be2e5c74e8c0a20fd1de7"),
		}},
	},
}

func stateServiceInitServices(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	codeDb, err := db.NewMDBXDatabase(env, "CODE")
	if err != nil {
		t.Error(err)
	}
	storageDb, err := db.NewMDBXDatabase(env, "STORAGE")
	if err != nil {
		t.Error(err)
	}
	storageDatabase := db.NewBlockSpecificDatabase(storageDb)
	StateService.Setup(codeDb, storageDatabase)
	err = StateService.Run()
	if err != nil {
		t.Error(err)
	}
}

func TestStateService_Code(t *testing.T) {
	stateServiceInitServices(t)
	defer StateService.Close(context.Background())

	for _, code := range codes {
		if err := StateService.StoreCode(code.Address, code.Code); err != nil {
			t.Error(err)
		}
		obtainedCode, err := StateService.GetCode(code.Address)
		if err != nil {
			if db.IsNotFound(err) {
				t.Errorf("code not found: %s", err)
			} else {
				t.Errorf("error: %s", err)
			}
		}
		if !equalCodes(t, code.Code, obtainedCode) {
			t.Errorf("Code are different afte Put-Get operation")
		}
	}
}

func TestService_Storage(t *testing.T) {
	initialData := [...]struct {
		Contract    string
		Storage     *state.Storage
		BlockNumber uint64
	}{
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			&state.Storage{Storage: map[string]string{
				"5": "22b",
				"5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5": "7e5",
			}},
			3,
		},
		{
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			&state.Storage{Storage: map[string]string{
				"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300": "4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
				"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620301": "453ae0c9610197b18b13645c44d3d0a407083d96562e8752aab3fab616cecb0",
				"6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0": "7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
			}},
			5,
		},
	}
	stateServiceInitServices(t)
	defer StateService.Close(context.Background())

	for _, data := range initialData {
		if err := StateService.UpdateStorage(data.Contract, data.BlockNumber, data.Storage); err != nil {
			t.Error(err)
		}
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
		obtainedStorage, err := StateService.GetStorage(test.Contract, test.BlockNumber)
		if err != nil && !db.IsNotFound(err) {
			t.Error(err)
		}
		if test.Ok && db.IsNotFound(err) {
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
}

func decodeString(s string) []byte {
	x, _ := hex.DecodeString(s)
	return x
}

func equalCodes(t *testing.T, a, b *state.Code) bool {
	aRaw, err := proto.Marshal(a)
	if err != nil {
		t.Errorf("marshal error: %s", err)
	}
	bRaw, err := proto.Marshal(b)
	if err != nil {
		t.Errorf("marshal error: %s", err)
	}
	return bytes.Compare(aRaw, bRaw) == 0
}
