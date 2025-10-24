package crypto

import "github.com/NethermindEth/juno/core/felt"

type Digest interface {
	Update(...*felt.Felt) Digest
	// todo(rdr): change Finish to return by value
	Finish() *felt.Felt
}
