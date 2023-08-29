package rpc

import "github.com/NethermindEth/juno/utils"

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1131
type BlockWithTxs struct {
	Status utils.BlockStatus `json:"status"`
	utils.BlockHeader
	Transactions []*Transaction `json:"transactions"`
}
