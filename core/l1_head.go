package core

import "github.com/NethermindEth/juno/core/felt"

type L1Head struct {
	BlockNumber uint64     `cbor:"1,keyasint"`
	BlockHash   *felt.Felt `cbor:"2,keyasint"`
	StateRoot   *felt.Felt `cbor:"3,keyasint"`
}
