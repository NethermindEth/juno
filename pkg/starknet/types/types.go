package types

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

const (
	LatestBlockSynced                        = "latestBlockSynced"
	LatestFactSaved                          = "latestFactSaved"
	LatestFactSynced                         = "latestFactSynced"
	BlockOfStarknetDeploymentContractMainnet = 13627000
	BlockOfStarknetDeploymentContractGoerli  = 5853000
	MaxChunk                                 = 10000
)

// KV represents a key-Value pair.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"Value"`
}

// DeployedContract represent the contracts that have been deployed in this Block
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
	Contract  abi.ABI
	EventName string
	Address   common.Address
}

// EventInfo represent the information retrieved from events that comes from L1
type EventInfo struct {
	Block           uint64
	Address         common.Address
	Event           map[string]interface{}
	TransactionHash common.Hash
}

type Fact struct {
	StateRoot   string `json:"state_root"`
	BlockNumber string `json:"block_number"`
	Value       string `json:"value"`
}

func (f Fact) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func (f Fact) UnMarshal(bytes []byte) (IValue, error) {
	var val Fact
	err := json.Unmarshal(bytes, &val)
	if err != nil {
		return nil, err
	}
	return val, nil
}

type TransactionHash struct {
	Hash common.Hash
}

func (t TransactionHash) Marshal() ([]byte, error) {
	return t.Hash.Bytes(), nil
}

func (t TransactionHash) UnMarshal(bytes []byte) (IValue, error) {
	return TransactionHash{
		Hash: common.BytesToHash(bytes),
	}, nil
}

type PagesHash struct {
	Bytes [][32]byte
}

func (p PagesHash) Marshal() ([]byte, error) {
	return json.Marshal(p.Bytes)
}

func (p PagesHash) UnMarshal(bytes []byte) (IValue, error) {
	var val PagesHash
	err := json.Unmarshal(bytes, &val.Bytes)
	if err != nil {
		return nil, err
	}
	return val, nil
}
