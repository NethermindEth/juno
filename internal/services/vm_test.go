package services

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
)

/*
// testGetCompiledContract retrieves the compiled test contract from the
// current directory for use in the VMService.Call function.
func testGetCompiledContract(addr *big.Int) ([]byte, error) {
	return os.ReadFile("test_contract_def.json")
}
*/

func TestVMCall(t *testing.T) {
	codeDatabase := db.NewKeyValueDb(t.TempDir(), 0)
	storageDatabase := db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0))
	VMService.Setup(codeDatabase, storageDatabase)

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
		[]types.Felt{types.HexToFelt("0x132")},
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

	// DEBUG.
	if len(ret) != 2 ||
		!bytes.Equal(ret[0], []byte("patricia_node:0704dfcbc470377c68e6f5ffb83970ebd0d7c48d5b8d2f4ed61a24e795e034bd")) ||
		!bytes.Equal(ret[1], []byte("contract_state:002e9723e54711aec56e3fb6ad1bb8272f64ec92e0a43a20feed943b1d4f73c5")) {
		t.Fatalf("unexpected return value: %s", ret)
	}
}
