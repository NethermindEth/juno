package core

import "github.com/NethermindEth/juno/core/felt"

type L1Head struct {
	BlockNumber uint64
	BlockHash   *felt.Felt
	StateRoot   *felt.Felt
}
