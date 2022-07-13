package abi

import (
	"testing"

	"gotest.tools/assert"

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
		err := manager.PutABI(address, abi)
		if err != nil {
			t.Errorf("TestManager: call put abi failed: %s", err)
		}
		abi2, err := manager.GetABI(address)
		if err != nil {
			t.Errorf("TestManager: call get abi failed: %s", err)
		}
		if abi2 == nil {
			t.Errorf("ABI with key %s not found after insertion with the same key", address)
		}
		if !abi.Equal(abi2) {
			t.Errorf("ABI are not equal after Put-Get operations, address: %s", address)
		}
	}
	manager.Close()
}

func TestManager_withUnknownABI_shouldReturnError(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	database, err := db.NewMDBXDatabase(env, "ABI")
	if err != nil {
		t.Error(err)
	}
	manager := NewABIManager(database)

	for address := range abis {
		_, err := manager.GetABI(address)
		assert.ErrorContains(t, err, db.ErrNotFound.Error())
	}
	manager.Close()
}
