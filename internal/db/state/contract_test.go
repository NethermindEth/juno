package state

import (
	"bytes"
	"encoding/hex"
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
	env, err := db.NewMDBXEnv(t.TempDir(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fatal(err)
	}
	binaryCodeDb, err := db.NewMDBXDatabase(env, "BINARY_CODE")
	if err != nil {
		t.Fatal(err)
	}
	codeDefinitionDb, err := db.NewMDBXDatabase(env, "CODE_DEFINITION")
	if err != nil {
		t.Fatal(err)
	}
	manager := NewStateManager(stateDb, binaryCodeDb, codeDefinitionDb)
	for _, code := range codes {
		manager.PutBinaryCode(code.Address, code.Code)
		obtainedCode := manager.GetBinaryCode(code.Address)
		if !equalCodes(t, code.Code, obtainedCode) {
			t.Errorf("Code are different afte Put-Get operation")
		}
	}
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
