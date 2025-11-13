package crypto

import "github.com/NethermindEth/juno/core/felt"

type HashFn func(*felt.Felt, *felt.Felt) felt.Felt
