package contracts

import (
	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

type StarknetContract struct {
	*Starknet
	DeploymentBlock uint64
}

func NewStarknetContract(address ethcommon.Address, deploymentBlock uint64, backend bind.ContractBackend) (*StarknetContract, error) {
	starknet, err := NewStarknet(address, backend)
	if err != nil {
		return nil, err
	}
	return &StarknetContract{
		Starknet:        starknet,
		DeploymentBlock: deploymentBlock,
	}, nil
}

type GpsStatementVerifierContract struct {
	*GpsStatementVerifier
	DeploymentBlock uint64
}

func NewGpsStatementVerifierContract(address ethcommon.Address, deploymentBlock uint64, backend bind.ContractBackend) (*GpsStatementVerifierContract, error) {
	verifier, err := NewGpsStatementVerifier(address, backend)
	if err != nil {
		return nil, err
	}
	return &GpsStatementVerifierContract{
		GpsStatementVerifier: verifier,
		DeploymentBlock:      deploymentBlock,
	}, nil
}

type MemoryPageFactRegistryContract struct {
	*MemoryPageFactRegistry
	DeploymentBlock uint64
	Abi             *ethabi.ABI
}

func NewMemoryPageFactRegistryContract(address ethcommon.Address, deploymentBlock uint64, backend bind.ContractBackend) (*MemoryPageFactRegistryContract, error) {
	registry, err := NewMemoryPageFactRegistry(address, backend)
	if err != nil {
		return nil, err
	}

	abi, err := MemoryPageFactRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	return &MemoryPageFactRegistryContract{
		MemoryPageFactRegistry: registry,
		DeploymentBlock:        deploymentBlock,
		Abi:                    abi,
	}, nil
}
