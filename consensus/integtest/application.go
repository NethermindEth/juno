package integtest

import (
	"math/rand"

	"github.com/NethermindEth/juno/core/felt"
)

type value int64

func (v value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(v))
}

type application struct{}

func (a application) Value() value {
	return value(rand.Int()) //nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
}

func (a application) Valid(v value) bool {
	return v >= 0
}
