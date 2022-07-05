package abi

import (
	"errors"
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
		if err := manager.PutABI(address, abi); err != nil {
			t.Error(err)
		}
		abi2, err := manager.GetABI(address)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				t.Errorf("ABI not found for address %s", address)
			}
			t.Error(err)
		}
		if !abi.Equal(abi2) {
			t.Errorf("ABI are not equal after Put-Get operations, address: %s", address)
		}
	}
	manager.Close()
}
