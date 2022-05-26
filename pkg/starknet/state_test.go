package starknet

import (
	"math/big"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/internal/services"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"testing"
)

func TestUpdateState(t *testing.T) {
	// Note: `contract` in `DeployedContracts` and `StorageDiffs`.
	// This will never happen in practice, but we do that here so we can test the DeployedContract
	// and StorageDiff code paths in `updateState` easily.
	contract := starknetTypes.DeployedContract{
		Address: "1",
		ContractHash: "1",
		ConstructorCallData: nil,
	}
	storageDiff := starknetTypes.KV{ Key: "a", Value: "b", }
	stateDiff := starknetTypes.StateDiff{
		DeployedContracts: []starknetTypes.DeployedContract{ contract, },
		StorageDiffs: map[string][]starknetTypes.KV{ 
			 contract.Address: { storageDiff, },
		},
	}

	// Want
	stateTrie := trie.New(store.New(), 251)
	storageTrie := trie.New(store.New(), 251)
	key, _ := new(big.Int).SetString(storageDiff.Key, 16)
	val, _ := new(big.Int).SetString(storageDiff.Value, 16)
	storageTrie.Put(key, val)
	hash, _ := new(big.Int).SetString(contract.ContractHash, 16)
	address, _ := new(big.Int).SetString(contract.Address, 16)
	stateTrie.Put(address, contractState(hash, storageTrie.Commitment()))

	// Actual
	database := db.Databaser(db.NewKeyValueDb(t.TempDir() + "/contractHash", 0))
	hashService := services.NewContractHashService(database)
	go hashService.Run()
	txnDb := db.NewTransactionDb(db.NewKeyValueDb(t.TempDir(), 0).GetEnv())
	txn := txnDb.Begin()
	stateCommitment, err := updateState(txn, hashService, stateDiff, "", "0")
	hashService.Close()
	if err != nil {
		t.Error("Error updating state")
	}
	txn.Commit()

	want := stateTrie.Commitment()
	commitment, _ := new(big.Int).SetString(stateCommitment, 16)
	if commitment.Cmp(want) != 0 {
		t.Error("State roots do not match")
	}
}
