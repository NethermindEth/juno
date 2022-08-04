package sync

import (
	"strings"

	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getGpsVerifierContractAddress(id int) string {
	if id == 1 {
		return types.GpsVerifierContractAddressMainnet
	}
	return types.GpsVerifierContractAddressGoerli
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getMemoryPagesContractAddress(id int) string {
	if id == 1 {
		return types.MemoryPagesContractAddressMainnet
	}
	return types.MemoryPagesContractAddressGoerli
}

// initialBlockForStarknetContract Returns the first block that we need to start to fetch the facts from l1
func initialBlockForStarknetContract(id int) int64 {
	if id == 1 {
		return types.BlockOfStarknetDeploymentContractMainnet
	}
	return types.BlockOfStarknetDeploymentContractGoerli
}

// loadContractInfo loads a contract ABI and set the events that later we are going to use
func loadContractInfo(contractAddress, abiValue, logName string, contracts map[common.Address]types.ContractInfo) error {
	contractAddressHash := common.HexToAddress(contractAddress)
	contractFromAbi, err := loadAbiOfContract(abiValue)
	if err != nil {
		return err
	}
	contracts[contractAddressHash] = types.ContractInfo{
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
