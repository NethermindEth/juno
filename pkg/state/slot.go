package state

import "github.com/NethermindEth/juno/pkg/felt"

type Slot struct {
	Key   *felt.Felt
	Value *felt.Felt
}
