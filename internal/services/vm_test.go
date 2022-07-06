package services

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
)

func setupDatabase(path string) {
	err := db.InitializeMDBXEnv(path, 1, 0)
	if err != nil {
		return
	}
}

func TestVMCall(t *testing.T) {
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fail()
	}
	codeDatabase, err := db.NewMDBXDatabase(env, "CODE")
	if err != nil {
		t.Fail()
	}
	binaryDatabase, err := db.NewMDBXDatabase(env, "BINARY_DATABASE")
	if err != nil {
		t.Fail()
	}
	stateDatabase, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fail()
	}
	VMService.Setup(stateDatabase, binaryDatabase, codeDatabase)

	if err := VMService.Run(); err != nil {
		t.Errorf("unexpected error starting the service: %s", err)
	}
	defer VMService.Close(context.Background())

	// TODO: Store some code to call.

	// XXX: Wait some time for the gRPC server to start. Note that this
	// might not be enough time in some cases so this test might have to
	// be restarted.
	time.Sleep(time.Second * 3)

	ret, err := VMService.Call(
		context.Background(),
		// Calldata.
		[]types.Felt{types.HexToFelt("0x84")},
		// Caller's address.
		types.HexToFelt("0x0"),
		// Contract's address.
		types.HexToFelt("0x57dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374"),
		// Root.
		types.HexToFelt("0x704dfcbc470377c68e6f5ffb83970ebd0d7c48d5b8d2f4ed61a24e795e034bd"),
		// Selector (StarkNet Keccak hash of the ASCII encoded string
		// "get_value").
		types.HexToFelt("0x26813d396fdb198e9ead934e4f7a592a8b88a059e45ab0eb6ee53494e8d45b0"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if ret[0] != "0x3" {
		t.Errorf("got %s, want 0x3 from executing cairo-lang call", ret[0])
	}
}
