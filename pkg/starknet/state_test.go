package starknet

import (
	"github.com/ethereum/go-ethereum/common"
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/starknet/abi"
	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
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
	hashService.Close(context.Background())
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

func TestGetFactInfo(t *testing.T) {
	contractAbi, err := loadAbiOfContract(abi.StarknetAbi)
	if err != nil {
		t.Errorf("Could not finish test: failed to load contract ABI")
	}
	test := struct {
		logs []types.Log
		abi ethAbi.ABI
		fact string
		latestBlockSynced int64
	}{
		logs: []types.Log{
			{
				BlockNumber: 1,
				BlockHash: common.Hash{1},
				TxHash: common.Hash{2},
				Data: []byte("067aa4a01cc374131818ab8aaaed7b7609448c922fe8956d07a9420cc5bb0bf500000000000000000000000000000000000000000000000000000000000009a5"),
			},
		},
		abi: contractAbi,
		fact: "1",
		latestBlockSynced: 7148728157378602549,
	}
	want := &starknetTypes.Fact{
		StateRoot: "0x3036376161346130316363333734313331383138616238616161656437623736",
		BlockNumber: "7148728157378602549",
		Value: test.fact,
	}

	res, err := getFactInfo(test.logs, test.abi, test.fact, test.latestBlockSynced)
	if err != nil {
		t.Errorf("Error while searching for fact: %v", err)
	} else if res.Value != want.Value || res.BlockNumber != want.BlockNumber || res.StateRoot != want.StateRoot {
		t.Errorf("Incorrect fact:\n%+v\n, want\n%+v", res, want)
	}
}

func TestParsePages(t *testing.T) {
	pages := [][]int64{
		// First page: should be removed
		{
			0,
		},
		{
			// Deployed contracts
			4, // Number of memory cells with deployed contract info
			2, // Contract address
			3, // Contract hash
			1, // Number of constructor arguments
			2, // Constructor argument
			// State diffs
			1, // Number of diffs
			3, // Contract address
			1, // Number of updates
			3, // Key (Cairo memory address)
			4, // Value
		},
	}
	data := make([][]*big.Int, len(pages))
	for i, page := range pages {
		data[i] = make([]*big.Int, len(page))
		for j, x := range page {
			data[i][j] = big.NewInt(x)
		}
	}

	wantDiff := starknetTypes.StateDiff{
		DeployedContracts: []starknetTypes.DeployedContract{ 
			{ 
				Address: "02", // Contract address
				ContractHash: "03", // Contract hash
				ConstructorCallData: []*big.Int{ big.NewInt(2), }, // Constructor argument
			}, 
		},
		StorageDiffs: map[string][]starknetTypes.KV{
			"03": { // Contract address
				{
					Key: "03", // Cairo memory address
					Value: "04",
				},
			},
		},
	}

	stateDiff := parsePages(data)

	for i, contract := range wantDiff.DeployedContracts {
		testContract := stateDiff.DeployedContracts[i]
		if contract.Address != testContract.Address {
			t.Errorf("Incorrect contract address: %s, want %s", testContract.Address, contract.Address)
		}
		if contract.ContractHash != testContract.ContractHash {
			t.Errorf("Incorrect contract hash: %s, want %s", testContract.ContractHash, contract.ContractHash, )
		}
		for j, arg := range contract.ConstructorCallData {
			if arg.Cmp(testContract.ConstructorCallData[j]) != 0 {
				t.Errorf("Incorrect calldata: %d, want %d", testContract.ConstructorCallData[j], arg)
			}
		}
	}
	for address, diff := range wantDiff.StorageDiffs {
		testDiff, ok := stateDiff.StorageDiffs[address]
		if !ok {
			t.Errorf("Storage diff does not exist: want %s", address)
		}
		if diff[0].Key != testDiff[0].Key || diff[0].Value != testDiff[0].Value {
			t.Errorf("Incorrect storage diff: %+v, want %+v", testDiff[0], diff[0])
		}
		if len(testDiff) > 1 {
			t.Error("Too many storage diffs: expected one diff")
		}
	}
} 
