package sync

import (
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
)

func TestUpdateState(t *testing.T) {
	// Note: `contract` in `DeployedContracts` and `StorageDiffs`.
	// This will never happen in practice, but we do that here so we can test the DeployedContract
	// and StorageDiff code paths in `updateState` easily.
	contract := types.DeployedContract{
		Address:             "1",
		Hash:                "1",
		ConstructorCallData: nil,
	}
	storageDiff := types.MemoryCell{Address: "a", Value: "b"}
	stateDiff := types.StateUpdate{
		StateDiff: types.StateDiff{
			DeployedContracts: []types.DeployedContract{contract},
			StorageDiffs: map[string][]types.MemoryCell{
				contract.Address: {storageDiff},
			},
		},
		NewBlockNumber: 0,
	}
	// Want
	stateTrie := trie.New(store.New(), 251)
	storageTrie := trie.New(store.New(), 251)
	key, _ := new(big.Int).SetString(storageDiff.Address, 16)
	val, _ := new(big.Int).SetString(storageDiff.Value, 16)
	storageTrie.Put(key, val)
	hash, _ := new(big.Int).SetString(contract.Hash, 16)
	address, _ := new(big.Int).SetString(contract.Address, 16)
	stateTrie.Put(address, contractState(hash, storageTrie.Commitment()))

	// Actual
	contractHashMap := map[string]*big.Int{
		contract.Address: big.NewInt(1),
	}
	env, err := db.NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	database, err := db.NewMDBXDatabase(env, "TEST-DB")
	if err != nil {
		t.Error(err)
	}

	var stateCommitment *string
	err = database.RunTxn(func(txn db.DatabaseOperations) (err error) {
		stateCommitment, err = updateState(txn, stateDiff, contractHashMap)
		return err
	})
	if err != nil {
		t.Error("Error updating state")
	}

	want := stateTrie.Commitment()
	commitment, _ := new(big.Int).SetString(*stateCommitment, 16)
	if commitment.Cmp(want) != 0 {
		t.Error("State roots do not match")
	}
}
