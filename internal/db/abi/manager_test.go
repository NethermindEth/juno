package abi

import (
	"github.com/NethermindEth/juno/internal/db"
	"testing"
)

func TestManager(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)
	manager := NewABIManager(database)

	for address, abi := range abis {
		manager.PutABI(address, &abi)
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
