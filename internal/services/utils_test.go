package services

import (
	"crypto/rand"
	"encoding/hex"
	"io/ioutil"
	"testing"

	"github.com/NethermindEth/juno/pkg/feeder"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
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

	value := stateUpdateResponseToStateDiff(feederVal, 0)

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
	abiPath := "./starknet/test_abi.json"

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

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
