package types

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const (
	LatestBlockSynced                        = "latestBlockSynced"
	BlockOfStarknetDeploymentContractMainnet = 13617000
	BlockOfStarknetDeploymentContractGoerli  = 6725000
	MaxChunk                                 = 10000

	MemoryPagesContractAddressMainnet = "0x96375087b2f6efc59e5e0dd5111b4d090ebfdd8b"
	MemoryPagesContractAddressGoerli  = "0x743789ff2ff82bfb907009c9911a7da636d34fa7"
	GpsVerifierContractAddressMainnet = "0xa739b175325cca7b71fcb51c3032935ef7ac338f"
	GpsVerifierContractAddressGoerli  = "0x5ef3c980bf970fce5bbc217835743ea9f0388f4f"
)

// MemoryCell represents a memory cell in Cairo
type MemoryCell struct {
	Address *felt.Felt `json:"key"`
	Value   *felt.Felt `json:"Value"`
}

// DeployedContract represent the contracts that have been deployed in this Block
// and the information stored on-chain
type DeployedContract struct {
	Address             *felt.Felt   `json:"address"`
	Hash                *felt.Felt   `json:"contract_hash"`
	ConstructorCallData []*felt.Felt `json:"constructor_call_data"`
}

type StorageDiff map[*felt.Felt][]MemoryCell

// StateDiff Represent the deployed contracts and the storage diffs for those and
// for the one's already deployed
type StateDiff struct {
	StorageDiff       `json:"storage_diffs"`
	BlockNumber       int64              `json:"block_number"`
	NewRoot           *felt.Felt         `json:"new_root"`
	OldRoot           *felt.Felt         `json:"old_root"`
	DeployedContracts []DeployedContract `json:"deployed_contracts"`
}

// ContractInfo represent the info associated to one contract
type ContractInfo struct {
	Contract  abi.ABI
	EventName string
	Address   common.Address
}

// EventInfo represent the information retrieved from events that comes from L1
type EventInfo struct {
	Block              uint64
	Address            common.Address
	Event              map[string]interface{}
	TxnHash            common.Hash
	InitialBlockLogged int64
}

type Fact struct {
	StateRoot          *felt.Felt  `json:"state_root"`
	SequenceNumber     uint64      `json:"block_number"`
	Value              common.Hash `json:"value"`
	InitialBlockLogged int64       `json:"initial_block_logged"`
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

type TxnHash struct {
	Hash common.Hash
}

func (t TxnHash) Marshal() ([]byte, error) {
	return t.Hash.Bytes(), nil
}

func (t TxnHash) UnMarshal(bytes []byte) (IValue, error) {
	return TxnHash{
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
