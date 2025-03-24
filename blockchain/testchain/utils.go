package testchain

import (
	"fmt"
	"path"
)

func coreContractGeneralPath() string {
	return path.Join("starknet_contracts", "core")
}

func GetCoreSierraContractPath(contractName string) string {
	return path.Join(
		coreContractGeneralPath(),
		"target",
		"dev",
		fmt.Sprintf("starknet_core_%s.contract_class.json", contractName),
	)
}

func contractGeneralPath() string {
	return path.Join("starknet_contracts", "general")
}

func GetSierraContractPath(contractName string) string {
	return path.Join(
		contractGeneralPath(),
		contractName,
		fmt.Sprintf("%s.sierra.json", contractName),
	)
}
