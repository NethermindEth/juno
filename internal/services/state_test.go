package services

import (
	"bytes"
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
	if err := db.InitializeMDBXEnv(t.TempDir(), 3, 0); err != nil {
		t.Error(err)
	}
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fail()
	}
	codeDatabase, err := db.NewMDBXDatabase(env, "CODE")
	if err != nil {
		t.Fail()
	}
	stateDatabase, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fail()
	}
	StateService.Setup(stateDatabase, codeDatabase)
	err = StateService.Run()
	if err != nil {
		t.Error(err)
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
