package state

import (
	"testing"

	"gotest.tools/assert"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
)

var contractStates = []*state.ContractState{
	{
		ContractHash: new(felt.Felt).SetBytes([]byte{1, 2, 3}),
		StorageRoot:  new(felt.Felt).SetBytes([]byte{4, 5, 6}),
		Nonce:        new(felt.Felt).SetUint64(1),
	},
}

func TestManagerPutGetContractState(t *testing.T) {
	manager := newTestManager(t)
	defer manager.Close()

	for _, contractState := range contractStates {
		if err := manager.PutContractState(contractState); err != nil {
			t.Fatal(err)
		}
	}

	for _, contractState := range contractStates {
		got, err := manager.GetContractState(contractState.Hash())
		if err != nil {
			t.Fatal(err)
		}
		assert.DeepEqual(t, contractState, got)
	}
}

func TestManagerContractStateNotFound(t *testing.T) {
	manager := newTestManager(t)
	defer manager.Close()

	_, err := manager.GetContractState(new(felt.Felt).SetBytes([]byte{1, 2, 3}))
	assert.ErrorType(t, err, db.ErrNotFound)
}

func newTestManager(t *testing.T) *Manager {
	if err := db.InitializeMDBXEnv(t.TempDir(), 2, 0); err != nil {
		t.Fatal(err)
	}
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fatal(err)
	}
	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fatal(err)
	}
	contractDefDb, err := db.NewMDBXDatabase(env, "CONTRACT_DEF")
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(stateDb, contractDefDb)
}
