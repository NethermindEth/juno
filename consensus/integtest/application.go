package integtest

import (
	"math/rand"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core/felt"
)

type application struct{}

func (a application) Value() starknet.Value {
	// The whole package is for testing purpose only, so it's safe to use weak random.
	return starknet.Value(*new(felt.Felt).SetUint64(rand.Uint64())) //nolint:gosec
}

func (a application) Valid(v starknet.Value) bool {
	return true // TODO: We needs to introduce a constraint on the value.
}
