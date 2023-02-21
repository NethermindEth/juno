package rpc

import "github.com/NethermindEth/juno/core/felt"

type BlockNumberAndHash struct {
	Number uint64     `json:"block_number"`
	Hash   *felt.Felt `json:"block_hash"`
}
