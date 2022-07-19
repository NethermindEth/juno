package services

import (
	"io/ioutil"
	"testing"

	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/common"
)

func TestFixedValues(t *testing.T) {
	// Test Mainnet address for Memory Pages Contract
	memoryAddressContract := getMemoryPagesContractAddress(1)
	if memoryAddressContract != types.MemoryPagesContractAddressMainnet {
		t.Fail()
		return
	}

	// Test Goerli address for Memory Pages Contract
	memoryAddressContract = getMemoryPagesContractAddress(0)
	if memoryAddressContract != types.MemoryPagesContractAddressGoerli {
		t.Fail()
		return
	}

	// Test Mainnet address for Gps Verifier Contract
	gpsVerifierContract := getGpsVerifierContractAddress(1)
	if gpsVerifierContract != types.GpsVerifierContractAddressMainnet {
		t.Fail()
		return
	}

	// Test Goerli address for Gps Verifier Contract
	gpsVerifierContract = getGpsVerifierContractAddress(0)
	if gpsVerifierContract != types.GpsVerifierContractAddressGoerli {
		t.Fail()
		return
	}

	// Test Initial Block for Starknet Contract in Mainnet
	initialBlock := initialBlockForStarknetContract(1)
	if initialBlock != types.BlockOfStarknetDeploymentContractMainnet {
		t.Fail()
		return
	}

	// Test Initial Block For Starknet Contract in Goerli
	initialBlock = initialBlockForStarknetContract(0)
	if initialBlock != types.BlockOfStarknetDeploymentContractGoerli {
		t.Fail()
		return
	}
}

func TestLoadContractInfo(t *testing.T) {
	contractAddress := common.HexToAddress("0x0")
	abiPath := "./abi/test_abi.json"

	abiAsBytes, err := ioutil.ReadFile(abiPath)
	if err != nil {
		t.Errorf("reading abi from %s: %s", abiPath, err)
	}

	contracts := make(map[common.Address]types.ContractInfo)

	err = loadContractInfo(contractAddress.Hex(), string(abiAsBytes), "logName", contracts)
	if err != nil {
		t.Errorf("loading contract %s: %s", contractAddress, err)
	}
	info, ok := contracts[contractAddress]
	if !ok {
		t.Errorf("contract does not exist")
	}

	if info.Address != contractAddress {
		t.Errorf("addresses do not match: got %s, want %s", info.Address.Hex(), contractAddress)
	}

	if len(contracts) != 1 {
		t.Errorf("incorrect number of contracts: got %d, want 1", len(contracts))
	}

	method, ok := contracts[contractAddress].Contract.Methods["f"]
	if !ok {
		t.Errorf("method f does not exist on contract")
	}

	wantSig := "f((uint256,uint256[],(uint256,uint256)[]),(uint256,uint256),uint256)"
	if method.Sig != wantSig {
		t.Errorf("method f has incorrect signature: got %s, want %s", method.Sig, wantSig)
	}
}
