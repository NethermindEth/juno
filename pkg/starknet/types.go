package starknet

import (
	"encoding/json"
	base "github.com/NethermindEth/juno/pkg/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

const (
	latestBlockSynced                        = "latestBlockSynced"
	latestFactSaved                          = "latestFactSaved"
	latestFactSynced                         = "latestFactSynced"
	blockOfStarknetDeploymentContractMainnet = 13627000
	blockOfStarknetDeploymentContractGoerli  = 5853000
	MaxChunk                                 = 10000
)

// KV represents a key-Value pair.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"Value"`
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

type Fact struct {
	StateRoot   string `json:"state_root"`
	BlockNumber string `json:"block_number"`
	Value       string `json:"value"`
}

func (f Fact) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func (f Fact) UnMarshal(bytes []byte) (base.IValue, error) {
	var val Fact
	err := json.Unmarshal(bytes, &val)
	if err != nil {
		return nil, err
	}
	return val, nil
}

type TransactionHash struct {
	hash common.Hash
}

func (t TransactionHash) Marshal() ([]byte, error) {
	return t.hash.Bytes(), nil
}

func (t TransactionHash) UnMarshal(bytes []byte) (base.IValue, error) {
	return TransactionHash{
		hash: common.BytesToHash(bytes),
	}, nil
}

type PagesHash struct {
	bytes [][32]byte
}

func (p PagesHash) Marshal() ([]byte, error) {
	return json.Marshal(p.bytes)
}

func (p PagesHash) UnMarshal(bytes []byte) (base.IValue, error) {
	var val PagesHash
	err := json.Unmarshal(bytes, &val.bytes)
	if err != nil {
		return nil, err
	}
	return val, nil
}
