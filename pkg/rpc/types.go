package rpc

import (
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	pkg "github.com/NethermindEth/juno/pkg"
)

type Echo struct {
	Message string `json:"message"`
}

type BlockHashParams struct {
	RequestedScope pkg.RequestedScope `json:"requestedScope"`
	BlockHash      cmd.BlockHash      `json:"block_hash"`
}

type BlockNumberParams struct {
	BlockNumber    int                `json:"block_number"`
	RequestedScope pkg.RequestedScope `json:"requestedScope"`
}

type BlockHashResult pkg.BlockResponse

type BlockNumberResult pkg.BlockResponse

type BlockTransactionCountParams pkg.BlockHash
type BlockTransactionCountResult cmd.BlockTransactionCount

type StorageAt struct {
	// The address of the contract to read from
	ContractAddress cmd.Address `json:"contract_address"`
	// The key to the storage value for the given contract
	Key cmd.Felt `json:"key"`
	// The hash (id) of the requested block or a tag referencing the necessary block
	BlockHash cmd.BlockHashOrTag `json:"block_hash"`
}
