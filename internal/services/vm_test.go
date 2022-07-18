package services

import (
	"context"
	_ "embed"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/felt"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
)

func setupDatabase(path string) {
	err := db.InitializeMDBXEnv(path, 1, 0)
	if err != nil {
		return
	}
}

//go:embed test.cairo.json
var testContract []byte

func TestVMCall(t *testing.T) {
	db.InitializeMDBXEnv(t.TempDir(), 5, 0)
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fail()
	}
	contractDefDb, err := db.NewMDBXDatabase(env, "CODE")
	if err != nil {
		t.Fail()
	}
	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fail()
	}
	VMService.Setup(stateDb, contractDefDb)

	if err := VMService.Run(); err != nil {
		t.Errorf("unexpected error starting the service: %s", err)
	}
	defer VMService.Close(context.Background())

	// XXX: Wait some time for the gRPC server to start. Note that this
	// might not be enough time in some cases so this test might have to
	// be restarted.
	time.Sleep(time.Second * 3)

	state := state.New(VMService.manager, trie.EmptyNode.Hash())
	b, _ := new(big.Int).SetString("2483955865838519930787573649413589905962103032695051953168137837593959392116", 10)
	address := new(felt.Felt).SetBigInt(b)
	hash := new(felt.Felt).SetHex("0x050b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b")
	var contract types.Contract
	if err := json.Unmarshal(testContract, &contract); err != nil {
		t.Fatal(err)
	}
	state.SetCode(address, hash, &contract)
	slot := new(felt.Felt).SetHex("0x84")
	value := new(felt.Felt).SetHex("0x3")
	state.SetSlot(address, slot, value)

	feltp := func(f felt.Felt) felt.Felt {
		return f
	}

	ret, err := VMService.Call(
		context.Background(),
		// State
		state,
		// Calldata.
		[]felt.Felt{feltp(slot)},
		// Caller's address.
		feltp(types.HexToFelt("0x0")),
		// Contract's address.
		&address,
		// Selector (StarkNet Keccak hash of the ASCII encoded string "get_value").
		feltp(types.HexToFelt("0x26813d396fdb198e9ead934e4f7a592a8b88a059e45ab0eb6ee53494e8d45b0")),
		// Sequencer
		feltp(types.HexToFelt("0x000000000000000000000000000000000000000000000000000000000000001")),
	)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if ret[0].Cmp(feltp(types.HexToFelt("0x3"))) != 0 {
		t.Errorf("got %s, want 0x3 from executing cairo-lang call", ret[0])
	}
}
