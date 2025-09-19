package crypto

import "github.com/NethermindEth/juno/core/types/felt"

type Digest interface {
	Update(...*felt.Felt) Digest
	Finish() *felt.Felt
}
