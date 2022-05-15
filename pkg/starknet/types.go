package starknet

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// KV represents a key-value pair.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// DeployedContract represent the contracts that have been deployed in this block
// and the information stored on-chain
type DeployedContract struct {
	Address             string     `json:"address"`
	ContractHash        string     `json:"contract_hash"`
	ConstructorCallData []*big.Int `json:"constructor_call_data"`
}

// StateDiff Represent the deployed contracts and the storage diffs for those and
// for the one's already deployed
type StateDiff struct {
	DeployedContracts []DeployedContract `json:"deployed_contracts"`
	StorageDiffs      map[string][]KV    `json:"storage_diffs"`
}

// ContractInfo represent the info associated to one contract
type ContractInfo struct {
	contract  abi.ABI
	eventName string
	address   common.Address
}

// eventInfo represent the information retrieved from events that comes from L1
type eventInfo struct {
	block           uint64
	address         common.Address
	event           map[string]interface{}
	transactionHash common.Hash
}
