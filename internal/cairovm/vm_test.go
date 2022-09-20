package cairovm

import (
	"context"
	_ "embed"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/NethermindEth/juno/internal/db/class"

	"github.com/NethermindEth/juno/pkg/felt"

	"github.com/NethermindEth/juno/internal/db"
	statedb "github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/trie"
)

//go:embed test.cairo.json
var testContract []byte

func TestVMCall(t *testing.T) {
	if err := db.InitializeMDBXEnv(t.TempDir(), 5, 0); err != nil {
		t.Fatal(err)
	}
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fail()
	}
	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fail()
	}
	classDb, err := db.NewMDBXDatabase(env, "CLASS")
	if err != nil {
		t.Fail()
	}
	classManager := class.NewClassManager(classDb)
	vm := New(statedb.NewManager(stateDb), classManager)

	if err := vm.Run(t.TempDir()); err != nil {
		t.Errorf("unexpected error starting the service: %s", err)
	}
	defer vm.Close()

	// XXX: Wait some time for the gRPC server to start. Note that this
	// might not be enough time in some cases so this test might have to
	// be restarted.
	time.Sleep(time.Second * 3)

	stateTest := state.New(vm.manager, trie.EmptyNode.Hash())
	b, _ := new(big.Int).SetString("2483955865838519930787573649413589905962103032695051953168137837593959392116", 10)
	address := new(felt.Felt).SetBigInt(b)
	hash := new(felt.Felt).SetHex("0x050b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b")
	var fullContract feeder.FullContract
	if err := json.Unmarshal(testContract, &fullContract); err != nil {
		t.Fatal(err)
	}
	if err := stateTest.InitNewContract(address, hash); err != nil {
		t.Fatal(err)
	}
	contractClass, err := types.NewContractClassFromFeeder(&fullContract)
	if err := classManager.PutClass(hash, contractClass); err != nil {
		t.Fatal(err)
	}
	slot := new(felt.Felt).SetHex("0x84")
	value := new(felt.Felt).SetHex("0x3")
	if err := stateTest.SetSlots(address, []state.Slot{{
		Key:   slot,
		Value: value,
	}}); err != nil {
		t.Fatal(err)
	}

	ret, err := vm.Call(
		context.Background(),
		// State
		stateTest,
		// Calldata.
		[]*felt.Felt{slot},
		// Caller's address.
		new(felt.Felt).SetHex("0x0"),
		// Contract's address.
		address,
		// Selector (StarkNet Keccak hash of the ASCII encoded string "get_value").
		new(felt.Felt).SetHex("0x26813d396fdb198e9ead934e4f7a592a8b88a059e45ab0eb6ee53494e8d45b0"),
		// Sequencer
		new(felt.Felt).SetHex("0x000000000000000000000000000000000000000000000000000000000000001"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !ret[0].Equal(new(felt.Felt).SetHex("0x3")) {
		t.Errorf("got %s, want 0x3 from executing cairo-lang call", ret[0])
	}
}
