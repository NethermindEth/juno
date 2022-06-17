package services

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
)

func TestVMCall(t *testing.T) {
	codeDatabase := db.NewKeyValueDb(t.TempDir(), 0)
	storageDatabase := db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0))
	VMService.Setup(codeDatabase, storageDatabase)

	if err := VMService.Run(); err != nil {
		t.Errorf("unexpected error starting the service: %s", err)
	}
	defer VMService.Close(context.Background())

	// TODO: Store some code to call.

	// Wait some time for the grpc server to start.
	time.Sleep(time.Second * 2)

	ret, err := VMService.Call(
		context.Background(),
		types.HexToFelt("0x1"),
		types.HexToFelt("0x2"),
		types.HexToFelt("0x3"),
		types.HexToFelt("0x4"),
		types.HexToFelt("0x5"),
		types.HexToFelt("0x6"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(ret) != 2 || !bytes.Equal(ret[0], []byte("hello")) || !bytes.Equal(ret[1], []byte("world")) {
		t.Fatalf("unexpected return value: %s", ret)
	}
}
