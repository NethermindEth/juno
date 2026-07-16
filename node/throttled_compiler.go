package node

import (
	"context"
	"runtime"

	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/utils/throttler"
	"go.uber.org/zap"
)

var _ compiler.Compiler = (*ThrottledCompiler)(nil)

type ThrottledCompiler struct {
	*throttler.Throttler[compiler.Compiler]
}

func NewThrottledCompiler(
	comp compiler.Compiler, concurrencyBudget uint, maxQueueLen uint64,
) *ThrottledCompiler {
	return &ThrottledCompiler{
		Throttler: throttler.NewThrottler(
			concurrencyBudget, &comp, throttler.WithMaxQueueLen(maxQueueLen),
		),
	}
}

func (tc *ThrottledCompiler) Compile(
	ctx context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	var result *starknet.CasmClass
	err := tc.Do(ctx, func(c *compiler.Compiler) error {
		var cErr error
		result, cErr = (*c).Compile(ctx, sierra)
		return cErr
	})
	return result, err
}

// calculateCompilerConcurrencyBudget determines safe limits for concurrent compilations
// if this were not explicitly set
func calculateCompilerConcurrencyBudget(cfg *Config, logger log.StructuredLogger) (uint64, uint64) {
	maxConcurrency := cfg.MaxConcurrentCompilations
	availableMemoryMB := compiler.AvailableMemoryMB()
	if !cfg.MaxConcurrentCompilationsExplicit {
		maxConcurrency = compiler.ConcurrencyLimit(
			uint64(runtime.GOMAXPROCS(0)),
			availableMemoryMB,
			uint64(cfg.NodeMemoryReserve),
			uint64(cfg.MaxCompilationMemory),
		)
	}

	queueSize := cfg.MaxCompilationQueue
	if !cfg.MaxCompilationQueueExplicit {
		queueSize = 2 * maxConcurrency
	}

	logger.Info("Sierra compilation limits",
		zap.Uint64("maxConcurrency", maxConcurrency),
		zap.Uint64("queueSize", queueSize),
		zap.Uint64("availableMemoryMB", availableMemoryMB),
		zap.Uint("nodeMemoryReserveMB", cfg.NodeMemoryReserve),
		zap.Uint("maxCompilationMemoryMB", cfg.MaxCompilationMemory),
	)
	return maxConcurrency, queueSize
}
