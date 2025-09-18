package crypto

import "github.com/NethermindEth/juno/core/types/felt"

type HashFn func(*felt.Felt, *felt.Felt) *felt.Felt
