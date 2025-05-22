package integtest

import (
	"math/rand"

	"github.com/NethermindEth/juno/consensus/starknet"
)

type application struct{}

func (a application) Value() starknet.Value {
	return starknet.Value(rand.Int()) //nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
}

func (a application) Valid(v starknet.Value) bool {
	return true // TODO: We needs to introduce a constraint on the value.
}
