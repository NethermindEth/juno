package types

import (
	"math/big"
)

type StateUpdate struct {
	StateDiff      *StateDiff `json:"state_diff"`
	NewRoot        string     `json:"new_root"`
	NewBlockNumber uint64     `json:"new_block_number"`
}

type MemoryCell struct {
	Address string
	Value   string
}

type StateDiff struct {
	DeployedContracts []DeployedContract      `json:"deployed_contracts"`
	StorageDiffs      map[string][]MemoryCell `json:"storage_diffs"` // contract addresses -> memory diffs
}

type DeployedContract struct {
	Address             string     `json:"address"`
	Hash                string     `json:"contract_hash"`
	ConstructorCallData []*big.Int `json:"constructor_call_data"`
}
