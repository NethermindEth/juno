package starknet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/pkg/types"

	"github.com/bxcodec/faker"

	"github.com/NethermindEth/juno/internal/db"
	dbAbi "github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/feeder"
	feederAbi "github.com/NethermindEth/juno/pkg/feeder/abi"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/ethereum/go-ethereum/common"
)

func TestRemove0x(t *testing.T) {
	tests := [...]struct {
		entry    string
		expected string
	}{
		{
			"0x001111",
			"1111",
		},
		{
			"04a1111",
			"4a1111",
		},
		{
			"000000001",
			"1",
		},
		{
			"000ssldkfmsd1111",
			"ssldkfmsd1111",
		},
		{
			"000111sp",
			"111sp",
		},
		{
			"0",
			"0",
		},
	}

	for _, test := range tests {
		answer := remove0x(test.entry)
		if answer != test.expected {
			t.Fail()
		}
	}
}

func TestStateUpdateResponseToStateDiff(t *testing.T) {
	kvs := []feeder.KV{
		{
			Key:   "Key1",
			Value: "Value1",
		},
		{
			Key:   "Key2",
			Value: "Value2",
		},
	}
	diff := feeder.StateDiff{
		DeployedContracts: []feeder.DeployedContract{
			{
				Address:      "address1",
				ContractHash: "contract_hash1",
			},
			{
				Address:      "address2",
				ContractHash: "contract_hash2",
			},
		},
		StorageDiffs: map[string][]feeder.KV{
			"key_address": kvs,
		},
	}
	feederVal := feeder.StateUpdateResponse{
		BlockHash: "BlockHash",
		NewRoot:   "NewRoot",
		OldRoot:   "OldRoot",
		StateDiff: diff,
	}

	value := stateUpdateResponseToStateDiff(feederVal)

	if len(value.DeployedContracts) != len(feederVal.StateDiff.DeployedContracts) {
		t.Fail()
	}
	if value.DeployedContracts[0].ContractHash != feederVal.StateDiff.DeployedContracts[0].ContractHash ||
		value.DeployedContracts[1].ContractHash != feederVal.StateDiff.DeployedContracts[1].ContractHash ||
		value.DeployedContracts[0].Address != feederVal.StateDiff.DeployedContracts[0].Address ||
		value.DeployedContracts[1].Address != feederVal.StateDiff.DeployedContracts[1].Address {
		t.Fail()
	}

	val, ok := diff.StorageDiffs["key_address"]
	if !ok {
		t.Fail()
	}
	val2, ok := feederVal.StateDiff.StorageDiffs["key_address"]
	if !ok {
		t.Fail()
	}

	if len(val) != len(val2) {
		for k, v := range val {
			if v.Key != val2[k].Key || v.Value != val2[k].Value {
				t.Fail()
			}
		}
	}
}

func TestGetAndUpdateValueOnDB(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)

	key := "key"
	value := 0

	err := updateNumericValueFromDB(database, key, uint64(value))
	if err != nil {
		t.Fail()
		return
	}
	fromDB, err := getNumericValueFromDB(database, key)
	if err != nil {
		t.Fail()
		return
	}

	if uint64(value+1) != fromDB {
		t.Fail()
	}

	zero, err := getNumericValueFromDB(database, "empty")
	if err != nil {
		t.Fail()
	}
	if zero != 0 {
		t.Fail()
	}
}

func TestFixedValues(t *testing.T) {
	// Test Mainnet address for Memory Pages Contract
	memoryAddressContract := getMemoryPagesContractAddress(1)
	if memoryAddressContract != starknetTypes.MemoryPagesContractAddressMainnet {
		t.Fail()
		return
	}

	// Test Goerli address for Memory Pages Contract
	memoryAddressContract = getMemoryPagesContractAddress(0)
	if memoryAddressContract != starknetTypes.MemoryPagesContractAddressGoerli {
		t.Fail()
		return
	}

	// Test Mainnet address for Gps Verifier Contract
	gpsVerifierContract := getGpsVerifierContractAddress(1)
	if gpsVerifierContract != starknetTypes.GpsVerifierContractAddressMainnet {
		t.Fail()
		return
	}

	// Test Goerli address for Gps Verifier Contract
	gpsVerifierContract = getGpsVerifierContractAddress(0)
	if gpsVerifierContract != starknetTypes.GpsVerifierContractAddressGoerli {
		t.Fail()
		return
	}

	// Test Initial Block for Starknet Contract in Mainnet
	initialBlock := initialBlockForStarknetContract(1)
	if initialBlock != starknetTypes.BlockOfStarknetDeploymentContractMainnet {
		t.Fail()
		return
	}

	// Test Initial Block For Starknet Contract in Goerli
	initialBlock = initialBlockForStarknetContract(0)
	if initialBlock != starknetTypes.BlockOfStarknetDeploymentContractGoerli {
		t.Fail()
		return
	}
}

func TestLoadContractInfo(t *testing.T) {
	contractAddress := "0x0"
	abiPath := "./abi/test_abi.json"

	abiAsBytes, err := ioutil.ReadFile(abiPath)
	if err != nil {
		t.Fail()
		return
	}

	contracts := make(map[common.Address]starknetTypes.ContractInfo)

	err = loadContractInfo(contractAddress, string(abiAsBytes), "logName", contracts)
	if err != nil {
		t.Fail()
		return
	}
	info, ok := contracts[common.HexToAddress(contractAddress)]
	if !ok {
		t.Fail()
	}
	if remove0x(info.Address.Hex()) != remove0x(contractAddress) {
		t.Fail()
	}
	if len(contracts) != 1 {
		t.Fail()
	}
	method, ok := contracts[common.HexToAddress(contractAddress)].Contract.Methods["f"]
	if !ok {
		t.Fail()
	}
	if method.Sig != "f((uint256,uint256[],(uint256,uint256)[]),(uint256,uint256),uint256)" {
		t.Fail()
	}
}

func TestUpdateState(t *testing.T) {
	// Note: `contract` in `DeployedContracts` and `StorageDiffs`.
	// This will never happen in practice, but we do that here so we can test the DeployedContract
	// and StorageDiff code paths in `updateState` easily.
	contract := starknetTypes.DeployedContract{
		Address:             "1",
		ContractHash:        "1",
		ConstructorCallData: nil,
	}
	storageDiff := starknetTypes.KV{Key: "a", Value: "b"}
	stateDiff := starknetTypes.StateDiff{
		DeployedContracts: []starknetTypes.DeployedContract{contract},
		StorageDiffs: map[string][]starknetTypes.KV{
			contract.Address: {storageDiff},
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
	database := db.Databaser(db.NewKeyValueDb(filepath.Join(t.TempDir(), "contractHash"), 0))
	hashService := services.NewContractHashService(database)
	go hashService.Run()
	txnDb := db.NewTransactionDb(db.NewKeyValueDb(t.TempDir(), 0).GetEnv())
	txn := txnDb.Begin()
	stateCommitment, err := updateState(txn, hashService, &stateDiff, "", 0)
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

func TestToDbAbi(t *testing.T) {
	inputAbi := feederAbi.Abi{
		Functions: []feederAbi.Function{
			{
				Name:    "a",
				Inputs:  []feederAbi.Variable{{Type: "a", Name: "a"}},
				Outputs: []feederAbi.Variable{{Type: "a", Name: "a"}},
			},
		},
		Events: []feederAbi.Event{
			{
				Name: "a",
				Data: []feederAbi.Variable{{Type: "a", Name: "a"}},
				Keys: []string{"a"},
			},
		},
		Structs: []feederAbi.Struct{
			{
				Members: []feederAbi.StructMember{
					{Variable: feederAbi.Variable{Name: "a", Type: "a"}, Offset: 0},
				},
				FieldCommon: feederAbi.FieldCommon{Type: "a"},
				Name:        "a",
				Size:        1,
			},
		},
		L1Handlers: []feederAbi.L1Handler{
			{
				Function: feederAbi.Function{
					Name:    "a",
					Inputs:  []feederAbi.Variable{{Type: "a", Name: "a"}},
					Outputs: []feederAbi.Variable{{Type: "a", Name: "a"}},
				},
			},
		},
		Constructor: &feederAbi.Constructor{
			Function: feederAbi.Function{
				Name:    "a",
				Inputs:  []feederAbi.Variable{{Type: "a", Name: "a"}},
				Outputs: []feederAbi.Variable{{Type: "a", Name: "a"}},
			},
		},
	}
	want := &dbAbi.Abi{
		Functions: []*dbAbi.Function{
			{
				Name:    "a",
				Inputs:  []*dbAbi.Function_Input{{Name: "a", Type: "a"}},
				Outputs: []*dbAbi.Function_Output{{Name: "a", Type: "a"}},
			},
		},
		Events: []*dbAbi.AbiEvent{
			{
				Name: "a",
				Data: []*dbAbi.AbiEvent_Data{{Name: "a", Type: "a"}},
				Keys: []string{"a"},
			},
		},
		Structs: []*dbAbi.Struct{
			{
				Fields: []*dbAbi.Struct_Field{{Name: "a", Type: "a", Offset: uint32(0)}},
				Name:   "a",
				Size:   uint64(1),
			},
		},
		L1Handlers: []*dbAbi.Function{
			{
				Name:    "a",
				Inputs:  []*dbAbi.Function_Input{{Name: "a", Type: "a"}},
				Outputs: []*dbAbi.Function_Output{{Name: "a", Type: "a"}},
			},
		},
		Constructor: &dbAbi.Function{
			Name:    "a",
			Inputs:  []*dbAbi.Function_Input{{Name: "a", Type: "a"}},
			Outputs: []*dbAbi.Function_Output{{Name: "a", Type: "a"}},
		},
	}

	got := toDbAbi(inputAbi)

	if !reflect.DeepEqual(want, got) {
		wantPretty, err1 := json.MarshalIndent(want, "", "    ")
		gotPretty, err2 := json.MarshalIndent(got, "", "    ")
		if err1 != nil || err2 != nil {
			t.Errorf("incorrect abi: want:\n%v,\n\n got:\n%v", want, got)
		} else {
			t.Errorf("incorrect abi: want:\n%s,\n\n got:\n%s", string(wantPretty), string(gotPretty))
		}
	}
}

func TestByteCodeToStateCode(t *testing.T) {
	sample := []string{"0x1", "0x123"}

	stateCode := byteCodeToStateCode(sample)

	if stateCode != nil && stateCode.Code != nil && len(stateCode.Code) != len(sample) {
		t.Fail()
	}
	for i, s := range sample {
		if remove0x(types.BytesToFelt(stateCode.Code[i]).Hex()) != remove0x(types.HexToFelt(s).Hex()) {
			t.Error("state code didn't match", s)
			t.Fail()
		}
	}
}

func TestTransactionToDBTransactionInvoke(t *testing.T) {
	TxnTest(t, "INVOKE")
}

func TestTransactionToDBTransactionDeploy(t *testing.T) {
	TxnTest(t, "DEPLOY")
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func TxnTest(t *testing.T, txnType string) {
	var txnSample feeder.TransactionInfo
	err := faker.FakeData(&txnSample)
	if err != nil {
		t.Fail()
		return
	}
	val, _ := randomHex(20)
	txnSample.Transaction.TransactionHash = val
	txnSample.Transaction.Type = txnType
	txn := feederTransactionToDBTransaction(&txnSample)

	if remove0x(txn.GetHash().Felt().Hex()) != remove0x(txnSample.Transaction.TransactionHash) {
		t.Fail()
	}
}

func TestFeederBlockToBlock(t *testing.T) {
	var b feeder.StarknetBlock

	err := faker.FakeData(&b)
	if err != nil {
		t.Fail()
		return
	}

	blockHash, _ := randomHex(20)
	b.BlockHash = blockHash
	newRoot, _ := randomHex(20)
	b.StateRoot = newRoot
	block := feederBlockToDBBlock(&b)

	if remove0x(block.BlockHash.Hex()) != remove0x(b.BlockHash) {
		t.Fail()
	}
	if remove0x(block.NewRoot.Hex()) != remove0x(b.StateRoot) {
		t.Fail()
	}
}
