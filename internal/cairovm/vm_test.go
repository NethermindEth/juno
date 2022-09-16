package cairovm

import (
	"context"
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/felt"
	"gotest.tools/assert"

	"github.com/NethermindEth/juno/internal/db"
	statedb "github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
)

//go:embed test_contract.json
var testContract []byte

func TestVMCall(t *testing.T) {
	db.InitializeMDBXEnv(t.TempDir(), 5, 0)
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fatal("initialise database environment: " + err.Error())
	}

	contractDefDb, err := db.NewMDBXDatabase(env, "CODE")
	if err != nil {
		t.Fatal("new database: code: " + err.Error())
	}

	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fatal("new database: state: " + err.Error())
	}

	// TODO: Inject no-op logger.
	vm := New(statedb.NewManager(stateDb, contractDefDb), nil)

	if err := vm.Run(t.TempDir()); err != nil {
		t.Fatal("run virtual machine: " + err.Error())
	}
	defer vm.Close()

	// XXX: Wait some time for the gRPC server to start. Note that this
	// might not be enough time in some cases so this test might have to
	// be restarted.
	time.Sleep(time.Second * 3)

	testState := state.New(vm.manager, trie.EmptyNode.Hash())
	address := new(felt.Felt).SetHex("0x57dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374")
	hash := new(felt.Felt).SetHex("0x050b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b")

	var contract types.Contract
	if err := json.Unmarshal(testContract, &contract); err != nil {
		t.Fatal(err)
	}
	testState.SetContract(address, hash, &contract)

	slot := new(felt.Felt).SetHex("0x84")
	value := new(felt.Felt).SetHex("0x3")
	testState.SetSlots(address, []state.Slot{{
		Key:   slot,
		Value: value,
	}})

	// StarkNet Keccak hash of the ASCII encoded string "get_value".
	selector := new(felt.Felt).SetHex("0x26813d396fdb198e9ead934e4f7a592a8b88a059e45ab0eb6ee53494e8d45b0")

	returned, err := vm.Call(
		context.Background(),
		testState,                /* state */
		[]*felt.Felt{slot},       /* calldata */
		new(felt.Felt).SetZero(), /* caller address */
		address,                  /* contract address */
		selector,                 /* selector */
		new(felt.Felt).SetOne(),  /* sequencer address */
	)
	if err != nil {
		t.Fatalf("virtual machine call: " + err.Error())
	}

	want := new(felt.Felt).SetUint64(3)
	assert.Check(t, returned[0].Equal(want), "got = 0x%s, want 0x%s", returned[0].Hex(), want.Hex())
}
