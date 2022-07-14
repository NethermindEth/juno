package state

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"google.golang.org/protobuf/proto"
)

var codes = []struct {
	Address []byte
	Code    *Code
}{
	{
		Address: decodeString("1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac"),
		Code: &Code{Code: [][]byte{
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

func TestManager_Code(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	codeDatabase, err := db.NewMDBXDatabase(env, "CODE")
	if err != nil {
		t.Error(err)
	}
	storageDb, err := db.NewMDBXDatabase(env, "STORAGE")
	if err != nil {
		t.Error(err)
	}
	storageDatabase := db.NewBlockSpecificDatabase(storageDb)
	manager := NewStateManager(codeDatabase, storageDatabase)
	for _, code := range codes {
		if err := manager.PutCode(code.Address, code.Code); err != nil {
			t.Error(err)
		}
		obtainedCode, err := manager.GetCode(code.Address)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				t.Errorf("Code not found for address %s", code.Address)
			}
			t.Error(err)
		}
		if !equalCodes(t, code.Code, obtainedCode) {
			t.Errorf("Code are different afte Put-Get operation")
		}
	}
	manager.Close()
}

func decodeString(s string) []byte {
	x, _ := hex.DecodeString(s)
	return x
}

func equalCodes(t *testing.T, a, b *Code) bool {
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
