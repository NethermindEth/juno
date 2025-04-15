package testchain

import (
	"fmt"
	"path"
	"runtime"
)

func contractsFolder() string {
	// Only dumb way (that I know of) for reliably getting the root of a go project :(
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get runtime caller at 0")
	}
	root := path.Join(path.Dir(b), "..", "..")

	return path.Join(root, "starknet_contracts")
}

func coreContractGeneralPath() string {
	return path.Join(contractsFolder(), "core")
}

func GetCoreSierraContractPath(contractName string) string {
	return path.Join(
		coreContractGeneralPath(),
		"target",
		"release",
		fmt.Sprintf("starknet_core_%s.contract_class.json", contractName),
	)
}

func contractGeneralPath() string {
	return path.Join(contractsFolder(), "general")
}

func GetSierraContractPath(contractName string) string {
	return path.Join(
		contractGeneralPath(),
		contractName,
		fmt.Sprintf("%s.sierra.json", contractName),
	)
}
