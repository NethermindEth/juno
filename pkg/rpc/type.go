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
