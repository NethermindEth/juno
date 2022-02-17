package rpc

import (
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	pkg "github.com/NethermindEth/juno/pkg"
)

type BlockHashParams struct {
	RequestedScope pkg.RequestedScope `json:"requestedScope"`
	BlockHash      cmd.BlockHash      `json:"block_hash"`
}
