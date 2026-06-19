package node

import (
	"context"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/throttler"
)

var _ compiler.Compiler = (*ThrottledCompiler)(nil)

type ThrottledCompiler struct {
	*throttler.Throttler[compiler.Compiler]
}

func NewThrottledCompiler(
	res compiler.Compiler, concurrencyBudget uint, maxQueueLen uint64,
) *ThrottledCompiler {
	return &ThrottledCompiler{
		Throttler: throttler.NewThrottler(
			concurrencyBudget, &res, throttler.WithMaxQueueLen(maxQueueLen),
		),
	}
}

func (tc *ThrottledCompiler) Compile(
	ctx context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	var result *starknet.CasmClass
	err := tc.Do(func(c *compiler.Compiler) error {
		var cErr error
		result, cErr = (*c).Compile(ctx, sierra)
		return cErr
	})
	return result, err
}
