package abi

import (
	"testing"

	"github.com/NethermindEth/juno/internal/db"
)

func TestManager(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	database, err := db.NewMDBXDatabase(env, "ABI")
	if err != nil {
		t.Error(err)
	}
	manager := NewABIManager(database)

	for address, abi := range abis {
		manager.PutABI(address, abi)
		abi2 := manager.GetABI(address)
		if abi2 == nil {
			t.Errorf("ABI with key %s not found after insertion with the same key", address)
		}
		if !abi.Equal(abi2) {
			t.Errorf("ABI are not equal after Put-Get operations, address: %s", address)
		}
	}
	manager.Close()
}
