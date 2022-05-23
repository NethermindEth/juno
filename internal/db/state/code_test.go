package state

import (
	db2 "github.com/NethermindEth/juno/internal/db"
	"testing"
)

var codes = []struct {
	Address string
	Code    *Code
}{
	{
		Address: "1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac",
		Code: &Code{Code: []string{
			"40780017fff7fff",
			"1",
			"208b7fff7fff7ffe",
			"400380007ffb7ffc",
			"400380017ffb7ffd",
			"800000000000010fffffffffffffffffffffffffffffffffffffffffffffffb",
			"107a2e2e5a8b6552e977246c45bfac446305174e86be2e5c74e8c0a20fd1de7",
		}},
	},
}

func TestManager_Code(t *testing.T) {
	codeDatabase := db2.NewKeyValueDb(t.TempDir(), 0)
	storageDatabase := db2.NewBlockSpecificDatabase(db2.NewKeyValueDb(t.TempDir(), 0))
	manager := NewStateManager(codeDatabase, *storageDatabase)
	for _, code := range codes {
		manager.PutCode(code.Address, code.Code)
		obtainedCode := manager.GetCode(code.Address)
		if !code.Code.Equal(obtainedCode) {
			t.Errorf("Code are different afte Put-Get operation")
		}
	}
}
