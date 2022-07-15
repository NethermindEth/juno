package services

import (
	starknetTypes "github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"strings"
)

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getGpsVerifierContractAddress(id int) string {
	if id == 1 {
		return starknetTypes.GpsVerifierContractAddressMainnet
	}
	return starknetTypes.GpsVerifierContractAddressGoerli
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getMemoryPagesContractAddress(id int) string {
	if id == 1 {
		return starknetTypes.MemoryPagesContractAddressMainnet
	}
	return starknetTypes.MemoryPagesContractAddressGoerli
}

// initialBlockForStarknetContract Returns the first block that we need to start to fetch the facts from l1
func initialBlockForStarknetContract(id int) int64 {
	if id == 1 {
		return starknetTypes.BlockOfStarknetDeploymentContractMainnet
	}
	return starknetTypes.BlockOfStarknetDeploymentContractGoerli
}

// loadContractInfo loads a contract ABI and set the events that later we are going to use
func loadContractInfo(contractAddress, abiValue, logName string, contracts map[common.Address]starknetTypes.ContractInfo) error {
	contractAddressHash := common.HexToAddress(contractAddress)
	contractFromAbi, err := loadAbiOfContract(abiValue)
	if err != nil {
		return err
	}
	contracts[contractAddressHash] = starknetTypes.ContractInfo{
		Contract:  contractFromAbi,
		EventName: logName,
	}
	return nil
}

// loadAbiOfContract loads the ABI of the contract from the
func loadAbiOfContract(abiVal string) (abi.ABI, error) {
	contractAbi, err := abi.JSON(strings.NewReader(abiVal))
	if err != nil {
		return abi.ABI{}, err
	}
	return contractAbi, nil
}

// removeOx remove the initial zeros and x at the beginning of the string
func remove0x(s string) string {
	answer := ""
	found := false
	for _, char := range s {
		found = found || (char != '0' && char != 'x')
		if found {
			answer = answer + string(char)
		}
	}
	if len(answer) == 0 {
		return "0"
	}
	return answer
}
